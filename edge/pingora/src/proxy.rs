use crate::control::SharedState;
use crate::policy::{self, PolicyDecision, RateLimiter, RequestContext, SITE_AUTH_PATH};
use anyhow::Result as AnyhowResult;
use async_trait::async_trait;
use pingora::listeners::tls::TlsSettings;
use pingora::listeners::TlsAccept;
use pingora::proxy::http_proxy_service;
use pingora::proxy::{ProxyHttp, Session};
use pingora::server::configuration::Opt;
use pingora::server::Server;
use pingora::tls::ext;
use pingora::tls::ssl::{NameType, SslRef};
use pingora::upstreams::peer::HttpPeer;
use pingora::{Error, Result};
use pingora_error::ErrorType::*;
use pingora_http::ResponseHeader;
use std::path::PathBuf;
use std::sync::{Arc, OnceLock};
use std::time::Instant;
use url::Url;

pub struct ProxyService {
    state: SharedState,
    rate_limiter: &'static RateLimiter,
    state_dir: &'static str,
}

#[async_trait]
impl ProxyHttp for ProxyService {
    type CTX = ();

    fn new_ctx(&self) -> Self::CTX {}

    async fn upstream_peer(
        &self,
        session: &mut Session,
        _ctx: &mut Self::CTX,
    ) -> Result<Box<HttpPeer>> {
        let config = match self.state.current() {
            Some(config) => config,
            None => {
                return Err(Error::explain(
                    HTTPStatus(503),
                    "configuration is not ready",
                ))
            }
        };
        let host = match request_host(session) {
            Some(host) => host,
            None => return Err(Error::explain(HTTPStatus(400), "Host header is invalid")),
        };
        let route = match config.route_for_host(&host) {
            Some(route) => route,
            None => return Err(Error::explain(HTTPStatus(404), "no route matches host")),
        };
        let upstream = match route.upstreams().first().cloned() {
            Some(upstream) => upstream,
            None => return Err(Error::explain(HTTPStatus(503), "route has no upstream")),
        };
        let upstream = normalize_upstream(&upstream)
            .map_err(|error| Error::because(InternalError, "invalid upstream", error))?;
        if upstream.scheme == "https" {
            Ok(Box::new(HttpPeer::new(upstream.addr, true, upstream.host)))
        } else {
            Ok(Box::new(HttpPeer::new(upstream.addr, false, upstream.host)))
        }
    }

    async fn request_filter(&self, session: &mut Session, _ctx: &mut Self::CTX) -> Result<bool>
    where
        Self::CTX: Send + Sync,
    {
        let Some(config) = self.state.current() else {
            let _ = session.respond_error(503).await;
            return Ok(true);
        };
        let Some(host) = request_host(session) else {
            let _ = session.respond_error(400).await;
            return Ok(true);
        };
        let Some(route) = config.route_for_host(&host) else {
            let _ = session.respond_error(404).await;
            return Ok(true);
        };
        let Some(context) = build_request_context(session, &host) else {
            let _ = session.respond_error(400).await;
            return Ok(true);
        };
        if route.site_auth_ttl_hours != 0 && context.path == SITE_AUTH_PATH {
            let decision = policy::site_auth_login(route, &context, policy::unix_now());
            return respond_policy_decision(session, decision).await;
        }
        if route.is_pages() {
            if serve_pages(session, route, self.state_dir).await {
                return Ok(true);
            }
            let _ = session.respond_error(404).await;
            return Ok(true);
        }
        for decision in [
            policy::basic_auth(route, &context),
            policy::site_auth(route, &context, true, policy::unix_now()),
        ] {
            if !matches!(decision, PolicyDecision::Continue) {
                return respond_policy_decision(session, decision).await;
            }
        }
        if policy::pow_required(route, &context, false) {
            let _ = session
                .respond_error_with_body(503, "Proof of work is required".into())
                .await;
            return Ok(true);
        }
        if !route.limit.req_per_ip.is_empty()
            && !policy::request_rate_limit(
                self.rate_limiter,
                &route.site_name,
                context.remote_addr,
                &route.limit.req_per_ip,
                Instant::now(),
            )
        {
            let _ = session.respond_error(429).await;
            return Ok(true);
        }
        if route.site_auth_ttl_hours != 0 {
            let filtered_cookie = policy::strip_site_auth_cookie(context.cookie);
            let req = session.req_header_mut();
            req.remove_header("Cookie");
            req.insert_header("Cookie", filtered_cookie)
                .map_err(|error| {
                    Error::because(InternalError, "failed to rewrite cookie", error)
                })?;
        }
        Ok(false)
    }
}

static RATE_LIMITER: OnceLock<RateLimiter> = OnceLock::new();
static STATE_DIR: OnceLock<String> = OnceLock::new();

fn global_rate_limiter() -> &'static RateLimiter {
    RATE_LIMITER.get_or_init(RateLimiter::default)
}

fn global_state_dir() -> &'static str {
    STATE_DIR.get_or_init(|| "/var/lib/openflare-edge".to_owned())
}

fn build_request_context<'a>(session: &'a Session, host: &'a str) -> Option<RequestContext<'a>> {
    let path_and_query = session.req_header().uri.path_and_query()?;
    let uri = path_and_query.as_str();
    let (path, query) = uri.split_once('?').unwrap_or((uri, ""));
    let method = session.req_header().method.as_str();
    let header = |name: &str| session.req_header().headers.get(name)?.to_str().ok();
    let origin_header = session
        .req_header()
        .headers
        .get("Origin")
        .and_then(|v| v.to_str().ok());
    Some(RequestContext {
        method,
        host,
        path,
        query,
        remote_addr: Box::leak(
            session
                .as_downstream()
                .client_addr()
                .map(|addr| addr.to_string())
                .unwrap_or_default()
                .into_boxed_str(),
        ),
        user_agent: header("User-Agent").unwrap_or(""),
        cookie: header("Cookie").unwrap_or(""),
        authorization: header("Authorization").unwrap_or(""),
        content_type: header("Content-Type").unwrap_or(""),
        body: b"",
        origin_header,
    })
}

async fn respond_policy_decision(session: &mut Session, decision: PolicyDecision) -> Result<bool> {
    if let PolicyDecision::Respond {
        status,
        headers,
        body,
    } = decision
    {
        let mut response = ResponseHeader::build(status, Some(4))
            .map_err(|error| Error::because(InternalError, "failed to build response", error))?;
        for (name, value) in headers {
            response
                .insert_header(name, value)
                .map_err(|error| Error::because(InternalError, "failed to set header", error))?;
        }
        response
            .insert_header("Content-Length", body.len())
            .map_err(|error| Error::because(InternalError, "failed to set length", error))?;
        session
            .write_response_header(Box::new(response), true)
            .await?;
        if !body.is_empty() {
            session
                .write_response_body(Some(body.as_bytes().into()), true)
                .await?;
        }
        return Ok(true);
    }
    Ok(false)
}

async fn serve_pages(session: &mut Session, route: &crate::config::Route, state_dir: &str) -> bool {
    let Some(deployment) = route.pages.as_ref() else {
        return false;
    };
    let root = PathBuf::from(state_dir)
        .join("pages")
        .join("projects")
        .join(deployment.project_id.to_string())
        .join("current");
    let uri = session.req_header().uri.to_string();
    let path = uri.split('?').next().unwrap_or("");
    let Some(file) = policy::pages_resolve(deployment, &root, path) else {
        return false;
    };
    let Ok(contents) = tokio::fs::read(file).await else {
        return false;
    };
    let Ok(mut response) = ResponseHeader::build(200, Some(4)) else {
        return false;
    };
    let _ = response.insert_header("Content-Type", "application/octet-stream");
    let _ = response.insert_header("Content-Length", contents.len());
    if session
        .write_response_header(Box::new(response), true)
        .await
        .is_err()
    {
        return false;
    }
    session
        .write_response_body(Some(contents.into()), true)
        .await
        .is_ok()
}

#[derive(Debug)]
struct DynamicCertificateCallback {
    state: SharedState,
}

#[async_trait]
impl TlsAccept for DynamicCertificateCallback {
    async fn certificate_callback(&self, ssl: &mut SslRef) {
        let sni = ssl
            .servername(NameType::HOST_NAME)
            .map(str::to_owned)
            .unwrap_or_default();
        if sni.is_empty() {
            tracing::warn!("TLS connection has no SNI; rejecting certificate selection");
            return;
        };
        let Some(config) = self.state.current() else {
            tracing::warn!(sni, "TLS handshake arrived before configuration was ready");
            return;
        };
        let Some(certificate) = config.certificate_for_host(&sni) else {
            tracing::warn!(sni, "no TLS certificate matches SNI");
            return;
        };
        if let Err(error) = ext::ssl_use_certificate(ssl, &certificate.cert) {
            tracing::error!(sni, error = %error, "failed to install TLS certificate");
        }
        if let Err(error) = ext::ssl_use_private_key(ssl, &certificate.key) {
            tracing::error!(sni, error = %error, "failed to install TLS private key");
        }
    }
}

fn request_host(session: &Session) -> Option<String> {
    let value = session.req_header().headers.get("Host")?;
    let host = value.to_str().ok()?.trim().trim_end_matches('.');
    if host.is_empty() {
        return None;
    }
    let host = host
        .rsplit_once(':')
        .map_or(host, |(host, _)| host)
        .to_lowercase();
    Some(host)
}

struct NormalizedUpstream {
    scheme: String,
    host: String,
    addr: String,
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
        .to_owned();
    let port = parsed
        .port_or_known_default()
        .ok_or_else(|| anyhow::anyhow!("upstream has no port"))?;
    Ok(NormalizedUpstream {
        scheme,
        addr: format!("{host}:{port}"),
        host,
    })
}

pub fn spawn(
    state: SharedState,
    http_listen: &str,
    tls_listen: Option<&str>,
) -> anyhow::Result<()> {
    std::net::TcpListener::bind(http_listen)?;
    if let Some(listen) = tls_listen {
        std::net::TcpListener::bind(listen)?;
    }
    let mut server = Server::new(Some(Opt::default()))?;
    server.bootstrap();
    let mut service = http_proxy_service(
        &Arc::clone(&server.configuration),
        ProxyService {
            state: state.clone(),
            rate_limiter: global_rate_limiter(),
            state_dir: global_state_dir(),
        },
    );
    service.add_tcp(http_listen);
    if let Some(listen) = tls_listen {
        let mut settings = TlsSettings::with_callbacks(Box::new(DynamicCertificateCallback {
            state: state.clone(),
        }))?;
        settings.enable_h2();
        service.add_tls_with_settings(listen, None, settings);
    }
    server.add_service(service);
    std::thread::spawn(move || {
        tracing::info!("proxy server stopped");
        server.run_forever();
    });
    Ok(())
}

#[cfg(test)]
mod tests {
    #[test]
    fn normalizes_request_host_without_port() {
        assert_eq!(
            request_host_from_value("Example.COM:8443"),
            Some("example.com".to_owned())
        );
    }

    fn request_host_from_value(value: &str) -> Option<String> {
        let host = value.trim().trim_end_matches('.');
        if host.is_empty() {
            return None;
        }
        Some(
            host.rsplit_once(':')
                .map_or(host, |(host, _)| host)
                .to_lowercase()
                .to_owned(),
        )
    }
}
