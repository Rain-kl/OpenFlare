use anyhow::{bail, Context, Result};
use bytes::Bytes;
use http::{HeaderMap, HeaderName, HeaderValue, Method, Request, Response, StatusCode, Version};
use http_body_util::{combinators::BoxBody, BodyExt, Empty, Full};
use hyper::body::Incoming;
use hyper::service::service_fn;
use hyper_util::rt::{TokioExecutor, TokioIo};
use hyper_util::server::conn::auto::Builder as ServerBuilder;
use openflare_tunnel_common::cidr::{connect_validated_tcp, CidrPolicy, ConnectTargetError};
use openflare_tunnel_common::protocol::{
    HealthCheckResponse, TunnelHTTPRouteRule, HEADER_OPENFLARE_ROUTE_ID,
    HEADER_OPENFLARE_TUNNEL_ID, PATH_CONNECTOR_HEALTH,
};
use openflare_tunnel_common::TUNNEL_VERSION;
use rustls::client::danger::{HandshakeSignatureValid, ServerCertVerified, ServerCertVerifier};
use rustls::pki_types::{CertificateDer, ServerName, UnixTime};
use rustls::{ClientConfig, DigitallySignedStruct, RootCertStore, SignatureScheme};
use serde::Deserialize;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;
use tokio::net::TcpListener;
use tokio::sync::{broadcast, RwLock};
use tokio_rustls::TlsConnector;

pub type RespBody = BoxBody<Bytes, hyper::Error>;

#[derive(Debug, Default, Deserialize)]
struct RouteOptions {
    #[serde(default)]
    insecure_skip_verify: bool,
    #[serde(default)]
    connect_timeout_ms: u64,
    #[serde(default)]
    request_timeout_ms: u64,
    #[serde(default)]
    path_prefix: String,
}

#[derive(Debug, Clone)]
pub struct CompiledRoute {
    pub rule: TunnelHTTPRouteRule,
    pub path_prefix: String,
    pub insecure_skip_verify: bool,
    pub connect_timeout: Duration,
    pub request_timeout: Duration,
    pub target_tls: bool,
    pub target_host: String,
    pub target_port: u16,
}

pub struct IngressState {
    routes: RwLock<Vec<CompiledRoute>>,
    policy: RwLock<Arc<CidrPolicy>>,
    tls_verified: TlsConnector,
    tls_insecure: TlsConnector,
}

impl IngressState {
    pub fn new() -> Result<Self> {
        let mut roots = RootCertStore::empty();
        roots.extend(webpki_roots::TLS_SERVER_ROOTS.iter().cloned());
        let verified_cfg = ClientConfig::builder_with_provider(Arc::new(
            rustls::crypto::ring::default_provider(),
        ))
        .with_safe_default_protocol_versions()?
        .with_root_certificates(roots)
        .with_no_client_auth();

        let insecure_cfg = ClientConfig::builder_with_provider(Arc::new(
            rustls::crypto::ring::default_provider(),
        ))
        .with_safe_default_protocol_versions()?
        .dangerous()
        .with_custom_certificate_verifier(Arc::new(NoCertificateVerification))
        .with_no_client_auth();

        Ok(Self {
            routes: RwLock::new(Vec::new()),
            policy: RwLock::new(Arc::new(CidrPolicy::default())),
            tls_verified: TlsConnector::from(Arc::new(verified_cfg)),
            tls_insecure: TlsConnector::from(Arc::new(insecure_cfg)),
        })
    }

    pub async fn update_config(
        &self,
        rules: &[TunnelHTTPRouteRule],
        allowed_cidrs: &[String],
        blocked_cidrs: &[String],
    ) -> Result<()> {
        let policy = Arc::new(CidrPolicy::parse(allowed_cidrs, blocked_cidrs)?);
        let mut compiled = Vec::with_capacity(rules.len());
        for rule in rules {
            match compile_route(rule) {
                Ok(c) => compiled.push(c),
                Err(err) => {
                    tracing::warn!(route_id = rule.route_id, target = %rule.target_addr, %err, "skipping invalid tunnel route target");
                }
            }
        }
        *self.policy.write().await = policy;
        *self.routes.write().await = compiled;
        Ok(())
    }

    async fn match_route(&self, headers: &HeaderMap, path: &str) -> Option<CompiledRoute> {
        let guard = self.routes.read().await;
        if let Some(route_id) = headers
            .get(HEADER_OPENFLARE_ROUTE_ID)
            .and_then(|v| v.to_str().ok())
            .and_then(|s| s.parse::<u64>().ok())
        {
            if let Some(found) = guard.iter().find(|r| r.rule.route_id == route_id) {
                return Some(found.clone());
            }
        }

        let host = headers
            .get(http::header::HOST)
            .and_then(|v| v.to_str().ok())
            .map(strip_port)
            .unwrap_or_default();

        guard
            .iter()
            .filter(|r| {
                let host_ok = r.rule.domains.is_empty()
                    || (!host.is_empty()
                        && r.rule.domains.iter().any(|d| domain_matches(d, &host)));
                let path_ok = r.path_prefix.is_empty() || path.starts_with(&r.path_prefix);
                host_ok && path_ok
            })
            .max_by_key(|r| r.path_prefix.len())
            .cloned()
    }
}

pub async fn spawn_ingress_server(
    mut shutdown: broadcast::Receiver<bool>,
) -> Result<(Arc<IngressState>, SocketAddr)> {
    let state = Arc::new(IngressState::new()?);
    let listener = TcpListener::bind("127.0.0.1:0").await?;
    let addr = listener.local_addr()?;
    let state_clone = state.clone();

    tokio::spawn(async move {
        loop {
            tokio::select! {
                _ = shutdown.recv() => return,
                accepted = listener.accept() => {
                    match accepted {
                        Ok((stream, _)) => {
                            let _ = stream.set_nodelay(true);
                            let st = state_clone.clone();
                            tokio::spawn(async move {
                                let io = TokioIo::new(stream);
                                let svc = service_fn(move |req| {
                                    let st = st.clone();
                                    async move { Ok::<_, hyper::Error>(handle_request(st, req).await) }
                                });
                                if let Err(err) = ServerBuilder::new(TokioExecutor::new())
                                    .serve_connection_with_upgrades(io, svc)
                                    .await
                                {
                                    tracing::debug!(%err, "ingress connection closed");
                                }
                            });
                        }
                        Err(err) => {
                            tracing::warn!(%err, "ingress accept error");
                        }
                    }
                }
            }
        }
    });

    Ok((state, addr))
}

pub async fn handle_request(
    state: Arc<IngressState>,
    mut req: Request<Incoming>,
) -> Response<RespBody> {
    if req.uri().path() == PATH_CONNECTOR_HEALTH && req.method() == Method::GET {
        let payload = HealthCheckResponse {
            status: "ok".to_owned(),
            version: TUNNEL_VERSION.to_owned(),
            role: "connector".to_owned(),
            relay_id: None,
            draining: None,
        };
        let bytes = serde_json::to_vec(&payload).unwrap_or_else(|_| b"{}".to_vec());
        return Response::builder()
            .status(StatusCode::OK)
            .header(http::header::CONTENT_TYPE, "application/json")
            .body(full_body(bytes))
            .unwrap();
    }

    let path = req.uri().path().to_owned();
    let Some(route) = state.match_route(req.headers(), &path).await else {
        return text_response(StatusCode::NOT_FOUND, "tunnel route not found");
    };

    let policy = state.policy.read().await.clone();
    let (stream, _) = match connect_validated_tcp(
        &route.target_host,
        route.target_port,
        &policy,
        route.connect_timeout,
    )
    .await
    {
        Ok(conn) => conn,
        Err(ConnectTargetError::Blocked(msg)) => {
            tracing::warn!(route_id = route.rule.route_id, target = %route.rule.target_addr, reason = %msg, "blocked tunnel target by CIDR policy");
            return text_response(StatusCode::FORBIDDEN, &msg);
        }
        Err(ConnectTargetError::Io(err)) => {
            tracing::debug!(route_id = route.rule.route_id, target = %route.rule.target_addr, %err, "failed connecting to private origin");
            return text_response(StatusCode::BAD_GATEWAY, "origin unreachable");
        }
    };

    // Strip internal routing metadata before reaching the user's private service.
    req.headers_mut().remove(HEADER_OPENFLARE_TUNNEL_ID);
    req.headers_mut().remove(HEADER_OPENFLARE_ROUTE_ID);

    if !route.rule.origin_host.trim().is_empty() {
        if let Ok(val) = HeaderValue::from_str(route.rule.origin_host.trim()) {
            req.headers_mut().insert(http::header::HOST, val);
        }
    }

    if route.target_tls {
        let connector = if route.insecure_skip_verify {
            &state.tls_insecure
        } else {
            &state.tls_verified
        };
        let sni_host = if !route.rule.origin_host.trim().is_empty() {
            strip_port(route.rule.origin_host.trim())
        } else {
            route.target_host.clone()
        };
        let Ok(server_name) = ServerName::try_from(sni_host) else {
            return text_response(StatusCode::BAD_GATEWAY, "invalid origin SNI");
        };
        let tls_stream = match tokio::time::timeout(
            route.connect_timeout,
            connector.connect(server_name, stream),
        )
        .await
        {
            Ok(Ok(s)) => s,
            Ok(Err(err)) => {
                tracing::debug!(%err, "TLS handshake to private origin failed");
                return text_response(StatusCode::BAD_GATEWAY, "origin TLS handshake failed");
            }
            Err(_) => {
                return text_response(StatusCode::GATEWAY_TIMEOUT, "origin TLS handshake timed out");
            }
        };
        forward_http_stream(TokioIo::new(tls_stream), req, route.request_timeout).await
    } else {
        forward_http_stream(TokioIo::new(stream), req, route.request_timeout).await
    }
}

async fn forward_http_stream<S>(
    io: S,
    mut req: Request<Incoming>,
    request_timeout: Duration,
) -> Response<RespBody>
where
    S: hyper::rt::Read + hyper::rt::Write + Unpin + Send + 'static,
{
    let is_upgrade = req
        .headers()
        .get(http::header::CONNECTION)
        .and_then(|v| v.to_str().ok())
        .is_some_and(|v| v.to_ascii_lowercase().contains("upgrade"))
        && req.headers().contains_key(http::header::UPGRADE);

    let (mut sender, conn) = match hyper::client::conn::http1::handshake(io).await {
        Ok(pair) => pair,
        Err(err) => {
            tracing::debug!(%err, "origin HTTP handshake failed");
            return text_response(StatusCode::BAD_GATEWAY, "origin handshake failed");
        }
    };

    if is_upgrade {
        tokio::spawn(async move {
            let _ = conn.with_upgrades().await;
        });
        let client_upgrade = hyper::upgrade::on(&mut req);
        let mut upstream_req = Request::builder()
            .method(req.method().clone())
            .uri(req.uri().clone())
            .version(Version::HTTP_11);
        if let Some(h) = upstream_req.headers_mut() {
            *h = req.headers().clone();
        }
        let Ok(upstream_req) = upstream_req.body(empty_body()) else {
            return text_response(StatusCode::INTERNAL_SERVER_ERROR, "build request failed");
        };
        let mut upstream_resp = match sender.send_request(upstream_req).await {
            Ok(r) => r,
            Err(err) => {
                tracing::debug!(%err, "origin upgrade request failed");
                return text_response(StatusCode::BAD_GATEWAY, "origin upgrade failed");
            }
        };
        if upstream_resp.status() == StatusCode::SWITCHING_PROTOCOLS {
            let status = upstream_resp.status();
            let headers = upstream_resp.headers().clone();
            let server_upgrade = hyper::upgrade::on(&mut upstream_resp);
            tokio::spawn(async move {
                if let (Ok(c), Ok(s)) = (client_upgrade.await, server_upgrade.await) {
                    let mut c_io = TokioIo::new(c);
                    let mut s_io = TokioIo::new(s);
                    let _ = tokio::io::copy_bidirectional(&mut c_io, &mut s_io).await;
                }
            });
            let mut resp = Response::builder().status(status).body(empty_body()).unwrap();
            *resp.headers_mut() = headers;
            return resp;
        }
        let (parts, body) = upstream_resp.into_parts();
        return Response::from_parts(parts, body.boxed());
    }

    tokio::spawn(async move {
        let _ = conn.await;
    });

    let (parts, body) = req.into_parts();
    let body_bytes = match body.collect().await {
        Ok(b) => b.to_bytes(),
        Err(_) => return text_response(StatusCode::BAD_REQUEST, "invalid request body"),
    };
    let mut upstream_req = Request::builder()
        .method(parts.method)
        .uri(parts.uri)
        .version(Version::HTTP_11);
    if let Some(headers) = upstream_req.headers_mut() {
        for (k, v) in &parts.headers {
            if !is_hop_by_hop(k) {
                if let (Ok(kn), Ok(vn)) = (
                    HeaderName::from_bytes(k.as_str().as_bytes()),
                    HeaderValue::from_bytes(v.as_bytes()),
                ) {
                    headers.append(kn, vn);
                }
            }
        }
    }
    let Ok(upstream_req) = upstream_req.body(full_body(body_bytes.to_vec())) else {
        return text_response(StatusCode::INTERNAL_SERVER_ERROR, "build origin request failed");
    };

    match tokio::time::timeout(request_timeout, sender.send_request(upstream_req)).await {
        Ok(Ok(resp)) => {
            let (resp_parts, resp_body) = resp.into_parts();
            Response::from_parts(resp_parts, resp_body.boxed())
        }
        Ok(Err(err)) => {
            tracing::debug!(%err, "origin request failed");
            text_response(StatusCode::BAD_GATEWAY, "origin request failed")
        }
        Err(_) => text_response(StatusCode::GATEWAY_TIMEOUT, "origin request timed out"),
    }
}

fn compile_route(rule: &TunnelHTTPRouteRule) -> Result<CompiledRoute> {
    let opts: RouteOptions = if rule.options_json.trim().is_empty() {
        RouteOptions::default()
    } else {
        serde_json::from_str(&rule.options_json).unwrap_or_default()
    };

    let raw = rule.target_addr.trim();
    if raw.is_empty() {
        bail!("empty target_addr");
    }
    let proto_lower = rule.target_protocol.trim().to_ascii_lowercase();
    let explicit_url = raw.starts_with("http://") || raw.starts_with("https://");
    let target_tls = if explicit_url {
        raw.starts_with("https://")
    } else {
        proto_lower == "https"
    };

    let (target_host, target_port) = if explicit_url {
        let parsed = url::Url::parse(raw).context("parse URL target_addr")?;
        let host = parsed
            .host_str()
            .context("target_addr URL missing host")?
            .to_owned();
        let port = parsed
            .port_or_known_default()
            .unwrap_or(if target_tls { 443 } else { 80 });
        (host, port)
    } else if let Ok(sock) = raw.parse::<SocketAddr>() {
        (sock.ip().to_string(), sock.port())
    } else if let Some((h, p)) = raw.rsplit_once(':') {
        let trimmed_host = h.trim_start_matches('[').trim_end_matches(']').to_owned();
        let port = p.parse::<u16>().context("invalid port in target_addr")?;
        (trimmed_host, port)
    } else {
        (
            raw.trim_start_matches('[')
                .trim_end_matches(']')
                .to_owned(),
            if target_tls { 443 } else { 80 },
        )
    };

    let connect_ms = if opts.connect_timeout_ms > 0 {
        opts.connect_timeout_ms
    } else {
        5000
    };
    let request_ms = if opts.request_timeout_ms > 0 {
        opts.request_timeout_ms
    } else {
        30000
    };

    Ok(CompiledRoute {
        rule: rule.clone(),
        path_prefix: opts.path_prefix.trim().to_owned(),
        insecure_skip_verify: opts.insecure_skip_verify,
        connect_timeout: Duration::from_millis(connect_ms),
        request_timeout: Duration::from_millis(request_ms),
        target_tls,
        target_host,
        target_port,
    })
}

fn strip_port(host: &str) -> String {
    if let Some(rest) = host.strip_prefix('[') {
        if let Some((inner, _)) = rest.split_once(']') {
            return inner.to_ascii_lowercase();
        }
    }
    match host.rsplit_once(':') {
        Some((h, p)) if p.chars().all(|c| c.is_ascii_digit()) => h.to_ascii_lowercase(),
        _ => host.to_ascii_lowercase(),
    }
}

fn domain_matches(pattern: &str, host: &str) -> bool {
    let p = pattern.trim().to_ascii_lowercase();
    if p == host {
        return true;
    }
    if let Some(suffix) = p.strip_prefix("*.") {
        if let Some(prefix) = host.strip_suffix(suffix) {
            return prefix.ends_with('.') && !prefix[..prefix.len() - 1].contains('.');
        }
    }
    false
}

fn is_hop_by_hop(name: &HeaderName) -> bool {
    matches!(
        name.as_str(),
        "connection"
            | "keep-alive"
            | "proxy-authenticate"
            | "proxy-authorization"
            | "te"
            | "trailer"
            | "transfer-encoding"
            | "upgrade"
    )
}

fn full_body(bytes: Vec<u8>) -> RespBody {
    Full::new(Bytes::from(bytes))
        .map_err(|never| match never {})
        .boxed()
}

fn empty_body() -> RespBody {
    Empty::<Bytes>::new()
        .map_err(|never| match never {})
        .boxed()
}

fn text_response(status: StatusCode, msg: &str) -> Response<RespBody> {
    Response::builder()
        .status(status)
        .header(http::header::CONTENT_TYPE, "text/plain; charset=utf-8")
        .body(full_body(msg.as_bytes().to_vec()))
        .unwrap()
}

#[derive(Debug)]
struct NoCertificateVerification;

impl ServerCertVerifier for NoCertificateVerification {
    fn verify_server_cert(
        &self,
        _end_entity: &CertificateDer<'_>,
        _intermediates: &[CertificateDer<'_>],
        _server_name: &ServerName<'_>,
        _ocsp_response: &[u8],
        _now: UnixTime,
    ) -> Result<ServerCertVerified, rustls::Error> {
        Ok(ServerCertVerified::assertion())
    }

    fn verify_tls12_signature(
        &self,
        _message: &[u8],
        _cert: &CertificateDer<'_>,
        _dss: &DigitallySignedStruct,
    ) -> Result<HandshakeSignatureValid, rustls::Error> {
        Ok(HandshakeSignatureValid::assertion())
    }

    fn verify_tls13_signature(
        &self,
        _message: &[u8],
        _cert: &CertificateDer<'_>,
        _dss: &DigitallySignedStruct,
    ) -> Result<HandshakeSignatureValid, rustls::Error> {
        Ok(HandshakeSignatureValid::assertion())
    }

    fn supported_verify_schemes(&self) -> Vec<SignatureScheme> {
        rustls::crypto::ring::default_provider()
            .signature_verification_algorithms
            .supported_schemes()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn blocks_forbidden_cidr_targets_with_403() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let target_addr = listener.local_addr().unwrap();

        let state = Arc::new(IngressState::new().unwrap());
        let rules = vec![TunnelHTTPRouteRule {
            route_id: 9,
            site_name: "app".into(),
            domains: vec!["app.example.com".into()],
            target_addr: target_addr.to_string(),
            target_protocol: "http".into(),
            origin_host: String::new(),
            options_json: "{}".into(),
        }];
        state
            .update_config(&rules, &[], &["127.0.0.0/8".into()])
            .await
            .unwrap();

        let gw_listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let gw_addr = gw_listener.local_addr().unwrap();
        let st = state.clone();
        tokio::spawn(async move {
            loop {
                if let Ok((stream, _)) = gw_listener.accept().await {
                    let st = st.clone();
                    tokio::spawn(async move {
                        let io = TokioIo::new(stream);
                        let _ = ServerBuilder::new(TokioExecutor::new())
                            .serve_connection(
                                io,
                                service_fn(move |req| {
                                    let st = st.clone();
                                    async move { Ok::<_, hyper::Error>(handle_request(st, req).await) }
                                }),
                            )
                            .await;
                    });
                }
            }
        });

        let resp = reqwest::Client::new()
            .get(format!("http://{gw_addr}/hello"))
            .header(HEADER_OPENFLARE_ROUTE_ID, "9")
            .send()
            .await
            .unwrap();
        assert_eq!(resp.status(), StatusCode::FORBIDDEN);
    }
}
