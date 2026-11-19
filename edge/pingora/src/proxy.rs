use crate::control::SharedState;
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
use pingora::Result;
use pingora_error::{Error, ErrorType::*};
use std::sync::Arc;
use url::Url;

pub struct ProxyService {
    state: SharedState,
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
        if config.route_for_host(&host).is_none() {
            let _ = session.respond_error(404).await;
            return Ok(true);
        }
        Ok(false)
    }
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
