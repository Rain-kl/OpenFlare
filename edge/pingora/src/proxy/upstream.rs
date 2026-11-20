use super::{ProxyService, RequestLogContext};
use anyhow::Result as AnyhowResult;
use http::Uri;
use pingora::{proxy::Session, upstreams::peer::HttpPeer, Error, Result};
use pingora_error::ErrorType::{HTTPStatus, InternalError};
use std::net::IpAddr;
use url::Url;

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
                        let mut peer = HttpPeer::new(address, tls, sni.to_owned());
                        let performance = self.performance.current();
                        if let Some(settings) = &performance.policy.settings {
                            peer.group_key = performance.upstream_generation;
                            settings.upstream.configure(&mut peer);
                        } else {
                            peer.options.verify_cert = false;
                            peer.options.verify_hostname = false;
                        }
                        return Ok(Box::new(peer));
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
            .is_some_and(|route| !route.is_pages() && ctx.upstream_index < route.upstreams.len());
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
