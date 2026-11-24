use super::{ProxyService, RequestLogContext};
use crate::runtime::performance::AppliedPolicy;
use anyhow::Result as AnyhowResult;
use http::Uri;
use pingora::{protocols::tls::ALPN, proxy::Session, upstreams::peer::HttpPeer, Error, Result};
use pingora_error::ErrorType::{HTTPStatus, InternalError};
use std::net::{IpAddr, SocketAddr};
use url::Url;

const WEBSOCKET_PEER_GROUP_BIT: u64 = 1 << 63;

fn is_websocket_upgrade_header(req: &pingora_http::RequestHeader) -> bool {
    req.version == http::Version::HTTP_11
        && req
            .headers
            .get(http::header::UPGRADE)
            .is_some_and(|value| value.as_bytes().eq_ignore_ascii_case(b"websocket"))
}

fn apply_peer_policy(peer: &mut HttpPeer, performance: &AppliedPolicy, websocket_upgrade: bool) {
    if let Some(settings) = &performance.policy.settings {
        peer.group_key = performance.upstream_generation;
        settings.upstream.configure(peer);
    } else {
        peer.options.verify_cert = false;
        peer.options.verify_hostname = false;
    }
    if websocket_upgrade {
        peer.group_key |= WEBSOCKET_PEER_GROUP_BIT;
        peer.options.alpn = ALPN::H1;
        peer.options.read_timeout = None;
    }
}

impl ProxyService {
    pub(super) async fn select_upstream(
        &self,
        session: &mut Session,
        ctx: &mut RequestLogContext,
    ) -> Result<Box<HttpPeer>> {
        let route = ctx
            .snapshot
            .as_ref()
            .and_then(|config| config.route_for_host(&ctx.host))
            .ok_or_else(|| Error::explain(HTTPStatus(503), "configuration is not ready"))?;
        let websocket_upgrade = is_websocket_upgrade_header(session.req_header());
        if let Some(tunnel_id) = route.tunnel_id.filter(|_| route.is_tunnel()) {
            return self
                .select_tunnel_upstream(tunnel_id, websocket_upgrade, ctx)
                .await;
        }
        let pages_target = route.pages.as_ref().and_then(|deployment| {
            crate::pages::pages_api_target(deployment, session.req_header().uri.path())
        });
        let upstreams = pages_target.map_or_else(|| route.upstreams(), |target| vec![target]);
        let mut last_error = None;
        while ctx.upstream_index < upstreams.len() {
            let upstream =
                &upstreams[ctx.upstream_start.wrapping_add(ctx.upstream_index) % upstreams.len()];
            ctx.upstream_index += 1;
            let upstream = normalize_upstream(upstream)
                .map_err(|error| Error::because(InternalError, "invalid upstream", error))?;
            match tokio::net::lookup_host((upstream.host.as_str(), upstream.port)).await {
                Ok(mut addresses) => {
                    if let Some(address) = addresses.next() {
                        let tls = upstream.scheme == "https";
                        let sni = if route.is_pages() {
                            upstream.host.as_str()
                        } else if !route.origin_host.is_empty() {
                            route
                                .origin_host
                                .split(':')
                                .next()
                                .unwrap_or(&route.origin_host)
                        } else if tls && upstream.host.parse::<IpAddr>().is_ok() {
                            ctx.host.as_str()
                        } else {
                            upstream.host.as_str()
                        };
                        return Ok(self.configure_peer(
                            address,
                            tls,
                            sni.to_owned(),
                            websocket_upgrade,
                        ));
                    }
                }
                Err(error) => last_error = Some(error),
            };
        }
        if let Some(error) = last_error {
            return Err(Error::because(
                HTTPStatus(502),
                "failed to resolve upstream",
                error,
            ));
        }
        Err(Error::explain(
            HTTPStatus(502),
            "route has no available upstream",
        ))
    }

    async fn select_tunnel_upstream(
        &self,
        tunnel_id: u64,
        websocket_upgrade: bool,
        ctx: &mut RequestLogContext,
    ) -> Result<Box<HttpPeer>> {
        if ctx.upstream_index == 0 {
            ctx.tunnel_relays = self.tunnel_routing.endpoints_for_tunnel(tunnel_id);
        }
        if ctx.tunnel_relays.is_empty() {
            return Err(Error::explain(
                HTTPStatus(503),
                "tunnel has no healthy relay endpoint",
            ));
        }
        let mut last_error = None;
        while ctx.upstream_index < ctx.tunnel_relays.len() {
            let relay = ctx.tunnel_relays[ctx.upstream_index].clone();
            ctx.upstream_index += 1;
            let upstream = normalize_upstream(&relay.gateway_url).map_err(|error| {
                Error::because(InternalError, "invalid tunnel relay gateway URL", error)
            })?;
            let resolved = tokio::net::lookup_host((upstream.host.as_str(), upstream.port))
                .await
                .map(|mut addresses| addresses.next());
            match resolved {
                Ok(Some(address)) => {
                    let tls = upstream.scheme == "https";
                    ctx.active_tunnel_relay = Some(relay);
                    return Ok(self.configure_peer(address, tls, upstream.host, websocket_upgrade));
                }
                Ok(None) => {}
                Err(error) => last_error = Some(error),
            }
        }
        if let Some(error) = last_error {
            return Err(Error::because(
                HTTPStatus(502),
                "failed to resolve tunnel relay",
                error,
            ));
        }
        Err(Error::explain(
            HTTPStatus(503),
            "all tunnel relay gateways failed",
        ))
    }

    fn configure_peer(
        &self,
        address: SocketAddr,
        tls: bool,
        sni: String,
        websocket_upgrade: bool,
    ) -> Box<HttpPeer> {
        let mut peer = HttpPeer::new(address, tls, sni);
        let performance = self.performance.current();
        apply_peer_policy(&mut peer, &performance, websocket_upgrade);
        Box::new(peer)
    }

    pub(super) fn retry_upstream(
        &self,
        _session: &mut Session,
        _peer: &HttpPeer,
        ctx: &mut RequestLogContext,
        mut error: Box<Error>,
    ) -> Box<Error> {
        let has_fallback = ctx
            .snapshot
            .as_ref()
            .and_then(|snapshot| snapshot.route_for_host(&ctx.host))
            .is_some_and(|route| {
                if route.is_tunnel() {
                    ctx.upstream_index < ctx.tunnel_relays.len()
                } else {
                    !route.is_pages() && ctx.upstream_index < route.upstreams.len()
                }
            });
        error.set_retry(has_fallback);
        error
    }

    pub(super) async fn prepare_upstream_request(
        &self,
        session: &mut Session,
        upstream_request: &mut pingora_http::RequestHeader,
        ctx: &mut RequestLogContext,
    ) -> Result<()> {
        let Some(route) = ctx
            .snapshot
            .as_ref()
            .and_then(|config| config.route_for_host(&ctx.host))
        else {
            return Ok(());
        };
        let mut set_header = |name: &'static str, value: &str| {
            upstream_request
                .insert_header(name, value)
                .map_err(|error| {
                    Error::because(InternalError, "set upstream request header", error)
                })
        };
        let host = if route.is_pages() {
            session
                .req_header()
                .uri
                .authority()
                .map(|value| value.as_str())
                .or_else(|| {
                    session
                        .req_header()
                        .headers
                        .get("host")
                        .and_then(|value| value.to_str().ok())
                })
                .unwrap_or(&ctx.host)
        } else if route.origin_host.is_empty() {
            &ctx.host
        } else {
            &route.origin_host
        };
        set_header("host", host)?;
        set_header("x-real-ip", &ctx.remote_ip)?;
        let forwarded = session
            .req_header()
            .headers
            .get("x-forwarded-for")
            .and_then(|value| value.to_str().ok())
            .map_or_else(
                || ctx.remote_ip.clone(),
                |value| format!("{value}, {}", ctx.remote_ip),
            );
        set_header("x-forwarded-for", &forwarded)?;
        set_header(
            "x-forwarded-proto",
            if ctx.https { "https" } else { "http" },
        )?;
        if let Some(tunnel_id) = route.tunnel_id.filter(|_| route.is_tunnel()) {
            let tunnel_id_str = tunnel_id.to_string();
            let route_id_str = route.id.to_string();
            set_header("x-openflare-tunnel-id", &tunnel_id_str)?;
            set_header("x-openflare-route-id", &route_id_str)?;
            if let Some(relay) = ctx.active_tunnel_relay.as_ref() {
                set_header("x-openflare-relay-auth", &relay.gateway_secret)?;
            }
        }
        for header in &route.custom_headers {
            let name =
                http::header::HeaderName::from_bytes(header.key.as_bytes()).map_err(|error| {
                    Error::because(InternalError, "invalid configured header name", error)
                })?;
            upstream_request.insert_header(name, header.value.as_str())?;
        }
        let request_uri = &session.req_header().uri;
        let Some(path_and_query) = rewrite_upstream_uri(route, request_uri)? else {
            return Ok(());
        };
        let uri = Uri::builder()
            .path_and_query(path_and_query)
            .build()
            .map_err(|error| Error::because(InternalError, "rewrite Pages API path", error))?;
        upstream_request.set_uri(uri);
        Ok(())
    }
}

fn rewrite_upstream_uri(
    route: &crate::protocol::config::Route,
    request: &Uri,
) -> Result<Option<String>> {
    if route.is_tunnel() {
        return Ok(None);
    }
    if let Some(deployment) = route.pages.as_ref() {
        let Some(target) = crate::pages::pages_api_target(
            deployment,
            request
                .path_and_query()
                .map_or(request.path(), |value| value.as_str()),
        ) else {
            return Ok(None);
        };
        let url = Url::parse(&target)
            .map_err(|error| Error::because(HTTPStatus(502), "invalid Pages API target", error))?;
        return Ok(Some(url.query().map_or_else(
            || url.path().to_owned(),
            |query| format!("{}?{query}", url.path()),
        )));
    }
    let origin = Url::parse(&route.origin_url)
        .map_err(|error| Error::because(HTTPStatus(502), "invalid origin URI", error))?;
    if origin.path() == "/" && origin.query().is_none() {
        return Ok(None);
    }
    let path = format!(
        "{}{}",
        origin.path(),
        request.path().trim_start_matches('/')
    );
    let query = origin.query().or_else(|| request.query());
    Ok(Some(query.map_or_else(
        || path.clone(),
        |query| format!("{path}?{query}"),
    )))
}

struct NormalizedUpstream {
    scheme: String,
    host: String,
    port: u16,
}

fn normalize_upstream(value: &str) -> AnyhowResult<NormalizedUpstream> {
    let parsed = Url::parse(value)?;
    let scheme = parsed.scheme().to_lowercase();
    if scheme != "http" && scheme != "https" {
        anyhow::bail!("unsupported upstream scheme: {}", scheme);
    }
    let host = parsed
        .host_str()
        .ok_or_else(|| anyhow::anyhow!("upstream has no host"))?
        .trim_start_matches('[')
        .trim_end_matches(']')
        .to_owned();
    let port = parsed
        .port_or_known_default()
        .ok_or_else(|| anyhow::anyhow!("upstream has no port"))?;
    Ok(NormalizedUpstream { scheme, port, host })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::performance::{NodePolicy, Settings};
    use pingora::upstreams::peer::Peer;
    use pingora_http::RequestHeader;
    use std::time::Duration;

    fn test_addr() -> SocketAddr {
        "127.0.0.1:443".parse().unwrap()
    }

    fn policy_with(upstream_alpn: &str) -> AppliedPolicy {
        let mut settings: Settings =
            serde_json::from_str(include_str!("../../tests/performance_policy.json")).unwrap();
        let mut value = serde_json::to_value(&settings).unwrap();
        value["verify_cert"] = serde_json::json!(false);
        value["verify_hostname"] = serde_json::json!(false);
        value["read_timeout"] = serde_json::json!(30);
        value["write_timeout"] = serde_json::json!(30);
        value["upstream_alpn"] = serde_json::json!(upstream_alpn);
        settings = serde_json::from_value(value).unwrap();
        AppliedPolicy {
            policy: NodePolicy {
                settings: Some(settings),
            },
            upstream_generation: 7,
        }
    }

    #[test]
    fn detects_http11_websocket_upgrade_headers() {
        let mut ws = RequestHeader::build("GET", b"/ws", None).unwrap();
        ws.insert_header("Upgrade", "websocket").unwrap();
        assert!(is_websocket_upgrade_header(&ws));

        let mut ws_case = RequestHeader::build("GET", b"/ws", None).unwrap();
        ws_case.insert_header("Upgrade", "WebSocket").unwrap();
        assert!(is_websocket_upgrade_header(&ws_case));

        let mut h2_ws = RequestHeader::build("GET", b"/ws", None).unwrap();
        h2_ws.set_version(http::Version::HTTP_2);
        h2_ws.insert_header("Upgrade", "websocket").unwrap();
        assert!(!is_websocket_upgrade_header(&h2_ws));

        let mut h2c = RequestHeader::build("GET", b"/ws", None).unwrap();
        h2c.insert_header("Upgrade", "h2c").unwrap();
        assert!(!is_websocket_upgrade_header(&h2c));

        let plain = RequestHeader::build("GET", b"/", None).unwrap();
        assert!(!is_websocket_upgrade_header(&plain));
    }

    #[test]
    fn forces_h1_clears_read_timeout_and_isolates_pool_for_websocket_upgrades() {
        for (alpn_name, expected_alpn) in [("h2h1", ALPN::H2H1), ("h2", ALPN::H2)] {
            let applied = policy_with(alpn_name);

            let mut normal = HttpPeer::new(test_addr(), true, "origin.test".into());
            apply_peer_policy(&mut normal, &applied, false);
            assert_eq!(normal.options.alpn.to_string(), expected_alpn.to_string());
            assert_eq!(normal.options.read_timeout, Some(Duration::from_secs(30)));
            assert!(!normal.options.verify_cert);
            assert!(!normal.options.verify_hostname);
            assert_eq!(normal.group_key, 7);

            let mut ws = HttpPeer::new(test_addr(), true, "origin.test".into());
            apply_peer_policy(&mut ws, &applied, true);
            assert_eq!(ws.options.alpn.to_string(), ALPN::H1.to_string());
            assert_eq!(ws.options.read_timeout, None);
            assert_eq!(ws.options.write_timeout, Some(Duration::from_secs(30)));
            assert!(!ws.options.verify_cert);
            assert!(!ws.options.verify_hostname);
            assert_eq!(ws.group_key, 7 | WEBSOCKET_PEER_GROUP_BIT);
            assert_ne!(normal.reuse_hash(), ws.reuse_hash());
        }
    }

    #[test]
    fn websocket_peer_without_policy_keeps_disabled_tls_verify_and_isolates_pool() {
        let applied = AppliedPolicy {
            policy: NodePolicy::default(),
            upstream_generation: 0,
        };
        let mut normal = HttpPeer::new(test_addr(), true, "origin.test".into());
        apply_peer_policy(&mut normal, &applied, false);
        let mut ws = HttpPeer::new(test_addr(), true, "origin.test".into());
        apply_peer_policy(&mut ws, &applied, true);

        assert_eq!(ws.options.alpn.to_string(), ALPN::H1.to_string());
        assert_eq!(ws.options.read_timeout, None);
        assert!(!ws.options.verify_cert);
        assert!(!ws.options.verify_hostname);
        assert_eq!(ws.group_key, WEBSOCKET_PEER_GROUP_BIT);
        assert_ne!(normal.reuse_hash(), ws.reuse_hash());
    }
}
