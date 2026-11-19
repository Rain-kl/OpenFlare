use crate::control::SharedState;
use crate::policy::{
    self, AccessLogBuffer, NodeAccessLog, PolicyDecision, RateLimiter, RequestContext, POW_COOKIE,
    SITE_AUTH_PATH,
};
use anyhow::Result as AnyhowResult;
use async_trait::async_trait;
use http::Uri;
use pingora::listeners::tls::TlsSettings;
use pingora::listeners::TlsAccept;
use pingora::proxy::http_proxy_service;
use pingora::proxy::{ProxyHttp, Session};
use pingora::server::configuration::Opt;
use pingora::server::configuration::ServerConf;
use pingora::server::Server;
use pingora::tls::ext;
use pingora::tls::ssl::{NameType, SslRef};
use pingora::upstreams::peer::HttpPeer;
use pingora::{Error, Result};
use pingora_error::ErrorType::*;
use pingora_http::ResponseHeader;
use std::path::{Path, PathBuf};
use std::sync::{Arc, OnceLock};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use url::Url;

const MAX_INSPECTED_BODY_BYTES: usize = 64 * 1024;

#[derive(Debug)]
pub struct RequestLogContext {
    started_at: Instant,
    host: String,
    path: String,
    remote_addr: String,
    user_agent: String,
}

impl RequestLogContext {
    fn new(session: &Session, host: &str) -> Self {
        let uri = session
            .req_header()
            .uri
            .path_and_query()
            .map(|value| value.as_str())
            .unwrap_or_default();
        Self {
            started_at: Instant::now(),
            host: host.to_owned(),
            path: uri.to_owned(),
            remote_addr: session
                .as_downstream()
                .client_addr()
                .map(|addr| addr.to_string())
                .unwrap_or_default(),
            user_agent: session
                .req_header()
                .headers
                .get("User-Agent")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_owned(),
        }
    }
}

pub struct DataPlaneState {
    pub shared: SharedState,
    pub state_dir: PathBuf,
    pub http_listen: String,
    pub tls_listen: Option<String>,
    pub access_logs: AccessLogBuffer,
}

pub struct ProxyService {
    state: SharedState,
    rate_limiter: &'static RateLimiter,
    state_dir: &'static str,
    pow: &'static policy::PowState,
    access_logs: AccessLogBuffer,
}

#[async_trait]
impl ProxyHttp for ProxyService {
    type CTX = RequestLogContext;

    fn new_ctx(&self) -> Self::CTX {
        RequestLogContext {
            started_at: Instant::now(),
            host: String::new(),
            path: String::new(),
            remote_addr: String::new(),
            user_agent: String::new(),
        }
    }

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
        let pages_target = route.pages.as_ref().and_then(|deployment| {
            let path = session.req_header().uri.path();
            policy::pages_api_target(deployment, path)
        });
        let upstream = match pages_target.or_else(|| route.upstreams().first().cloned()) {
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

    async fn request_filter(&self, session: &mut Session, ctx: &mut Self::CTX) -> Result<bool>
    where
        Self::CTX: Send + Sync,
    {
        if let Some(host) = request_host(session) {
            *ctx = RequestLogContext::new(session, &host);
        }
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
        let page = config.pow_page();
        let context = match build_request_context(session, &host).await {
            Ok(context) => context,
            Err(error) => return Err(error),
        };
        if route.site_auth_ttl_hours != 0 && context.path == SITE_AUTH_PATH {
            let decision = policy::site_auth_login(route, &context, policy::unix_now());
            return respond_policy_decision(session, decision).await;
        }
        if context.path.starts_with(policy::POW_API_PREFIX) {
            return self.serve_pow_api(session, route, page, &context).await;
        }
        if context.path.starts_with(policy::POW_STATIC_PREFIX) {
            return serve_pow_static(session, &context.path).await;
        }
        let pow_session =
            self.pow
                .valid_session(&context.host, &context.cookie, policy::unix_now());
        if let Some(token) = pow_session.as_ref() {
            if let Some(config) = route.pow.as_ref() {
                self.pow.refresh_session(
                    &context.host,
                    token,
                    config.session_ttl,
                    policy::unix_now(),
                );
            }
        }
        match policy::execute_waf(&config.config.waf, route, &context, pow_session.is_some()) {
            Ok(policy::WafDecision::Allow) => {}
            Ok(policy::WafDecision::Block { status, body }) => {
                let _ = session
                    .respond_error_with_body(status, body.into_bytes().into())
                    .await;
                return Ok(true);
            }
            Ok(policy::WafDecision::Takeover) => {
                return self.issue_pow(session, route, page, &context).await
            }
            Err(error) => {
                tracing::error!(site = %route.site_name, error, "WAF graph execution failed");
                let _ = session.respond_error(500).await;
                return Ok(true);
            }
        }
        if route.is_pages() {
            if let Some(deployment) = route.pages.as_ref() {
                if policy::pages_api_target(deployment, &context.path).is_some() {
                    return Ok(false);
                }
            }
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
        if policy::pow_required(route, &context, pow_session.is_some()) {
            return self.issue_pow(session, route, page, &context).await;
        }
        if !route.limit.req_per_ip.is_empty()
            && !policy::request_rate_limit(
                self.rate_limiter,
                &route.site_name,
                &context.remote_addr,
                &route.limit.req_per_ip,
                Instant::now(),
            )
        {
            let _ = session.respond_error(429).await;
            return Ok(true);
        }
        if route.site_auth_ttl_hours != 0 {
            let filtered_cookie = policy::strip_site_auth_cookie(&context.cookie);
            let req = session.req_header_mut();
            req.remove_header("Cookie");
            req.insert_header("Cookie", filtered_cookie)
                .map_err(|error| {
                    Error::because(InternalError, "failed to rewrite cookie", error)
                })?;
        }
        Ok(false)
    }

    async fn upstream_request_filter(
        &self,
        session: &mut Session,
        upstream_request: &mut pingora_http::RequestHeader,
        _ctx: &mut Self::CTX,
    ) -> Result<()> {
        let Some(host) = request_host(session) else {
            return Ok(());
        };
        let Some(config) = self.state.current() else {
            return Ok(());
        };
        let Some(route) = config.route_for_host(&host) else {
            return Ok(());
        };
        let Some(deployment) = route.pages.as_ref() else {
            return Ok(());
        };
        let path = session.req_header().uri.path();
        let Some(target) = policy::pages_api_target(deployment, path) else {
            return Ok(());
        };
        let Ok(url) = Url::parse(&target) else {
            return Err(Error::explain(HTTPStatus(502), "invalid Pages API target"));
        };
        let path_and_query = match url.query() {
            Some(query) => format!("{}?{query}", url.path()),
            None => url.path().to_owned(),
        };
        match Uri::builder().path_and_query(path_and_query).build() {
            Ok(uri) => upstream_request.set_uri(uri),
            Err(error) => {
                return Err(Error::because(
                    InternalError,
                    "rewrite Pages API path",
                    anyhow::anyhow!(error),
                ))
            }
        }
        Ok(())
    }

    async fn logging(&self, session: &mut Session, _error: Option<&Error>, ctx: &mut Self::CTX)
    where
        Self::CTX: Send + Sync,
    {
        let status = session
            .as_downstream()
            .response_written()
            .map(|response| response.status.as_u16())
            .unwrap_or_default();
        let downstream = session.as_downstream();
        self.access_logs.push(NodeAccessLog {
            logged_at_unix: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|value| value.as_secs() as i64)
                .unwrap_or_default(),
            remote_addr: ctx.remote_addr.clone(),
            host: ctx.host.clone(),
            path: ctx.path.clone(),
            user_agent: ctx.user_agent.clone(),
            cache_status: "-".to_owned(),
            status_code: i32::from(status),
            bytes_sent: downstream.body_bytes_sent() as i64,
            request_length: downstream.body_bytes_read() as i64,
            request_time_ms: ctx.started_at.elapsed().as_millis() as i64,
        });
    }
}

static RATE_LIMITER: OnceLock<RateLimiter> = OnceLock::new();
static STATE_DIR: OnceLock<String> = OnceLock::new();
static GEO_IP: OnceLock<Option<policy::GeoIp>> = OnceLock::new();
static POW_STATE: OnceLock<policy::PowState> = OnceLock::new();

fn global_rate_limiter() -> &'static RateLimiter {
    RATE_LIMITER.get_or_init(RateLimiter::default)
}

fn global_state_dir() -> &'static str {
    STATE_DIR.get_or_init(|| "/var/lib/openflare-edge".to_owned())
}

fn global_pow_state() -> &'static policy::PowState {
    POW_STATE.get_or_init(policy::PowState::default)
}

fn reads_request_body(session: &Session) -> bool {
    let method = session.req_header().method.as_str();
    let path = session.req_header().uri.path();
    if path == policy::SITE_AUTH_PATH && matches!(method, "POST" | "PUT" | "PATCH") {
        return true;
    }
    path.starts_with(policy::POW_API_PREFIX)
        && path.ends_with("/pass-challenge")
        && matches!(method, "POST" | "PUT" | "PATCH")
}

fn global_geo_ip() -> Option<&'static policy::GeoIp> {
    GEO_IP
        .get_or_init(|| {
            let state_dir = Path::new(global_state_dir());
            policy::GeoIp::open(
                Some(&state_dir.join("GeoLite2-Country.mmdb")),
                Some(&state_dir.join("GeoLite2-City.mmdb")),
            )
        })
        .as_ref()
}

async fn build_request_context(session: &mut Session, host: &str) -> Result<RequestContext> {
    let path_and_query = session
        .req_header()
        .uri
        .path_and_query()
        .ok_or_else(|| Error::explain(HTTPStatus(400), "request URI has no path"))?
        .as_str()
        .to_owned();
    let (path, query) = path_and_query
        .split_once('?')
        .map(|(path, query)| (path.to_owned(), query.to_owned()))
        .unwrap_or((path_and_query, String::new()));
    let user_agent = session
        .req_header()
        .headers
        .get("User-Agent")
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned);
    let cookie = session
        .req_header()
        .headers
        .get("Cookie")
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned);
    let authorization = session
        .req_header()
        .headers
        .get("Authorization")
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned);
    let content_type = session
        .req_header()
        .headers
        .get("Content-Type")
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned);
    let origin_header = session
        .req_header()
        .headers
        .get("Origin")
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned);
    let accept_language = session
        .req_header()
        .headers
        .get("Accept-Language")
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned)
        .unwrap_or_default();
    let remote_addr = session
        .as_downstream()
        .client_addr()
        .map(|addr| addr.to_string())
        .unwrap_or_default();
    let geo = global_geo_ip().and_then(|geo| geo.lookup(&remote_addr));
    let mut body = Vec::new();
    if reads_request_body(session) {
        while let Some(chunk) = session.as_downstream_mut().read_request_body().await? {
            if body.len().saturating_add(chunk.len()) > MAX_INSPECTED_BODY_BYTES {
                let _ = session.as_downstream_mut().drain_request_body().await;
                let _ = session.respond_error(413).await;
                return Err(Error::explain(HTTPStatus(413), "request body is too large"));
            }
            body.extend_from_slice(&chunk);
        }
    }
    Ok(RequestContext {
        method: session.req_header().method.as_str().to_owned(),
        host: host.to_owned(),
        path: path.to_owned(),
        query: query.to_owned(),
        remote_addr,
        user_agent: user_agent.unwrap_or_default(),
        cookie: cookie.unwrap_or_default(),
        authorization: authorization.unwrap_or_default(),
        content_type: content_type.unwrap_or_default(),
        body,
        origin_header,
        accept_language,
        geo,
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

async fn serve_pow_static(session: &mut Session, path: &str) -> Result<bool> {
    let relative = path
        .strip_prefix(policy::POW_STATIC_PREFIX)
        .unwrap_or_default()
        .trim_start_matches('/');
    let Some(relative) = sanitize_static_path(relative) else {
        let _ = session.respond_error(404).await;
        return Ok(true);
    };
    let Some((contents, content_type)) = policy::pow_static(relative) else {
        let _ = session.respond_error(404).await;
        return Ok(true);
    };
    let mut response = ResponseHeader::build(200, Some(2)).map_err(|error| {
        Error::because(InternalError, "failed to build PoW static response", error)
    })?;
    let _ = response.insert_header("Content-Type", content_type);
    let _ = response.insert_header("Content-Length", contents.len());
    session
        .write_response_header(Box::new(response), true)
        .await?;
    session
        .write_response_body(Some(contents.to_vec().into()), true)
        .await?;
    Ok(true)
}

fn sanitize_static_path(value: &str) -> Option<&str> {
    (!value.is_empty()
        && !value.starts_with('/')
        && !value.contains('\\')
        && !value.contains('\0')
        && value
            .split('/')
            .all(|segment| segment != ".." && !segment.is_empty()))
    .then_some(value)
}

impl ProxyService {
    async fn issue_pow(
        &self,
        session: &mut Session,
        route: &crate::config::Route,
        page: &crate::policy::PoWPageConfig,
        context: &RequestContext,
    ) -> Result<bool> {
        let Some(config) = route.pow.as_ref() else {
            let _ = session.respond_error(503).await;
            return Ok(true);
        };
        let Some((id, challenge)) = self.pow.issue(config, route, context, policy::unix_now())
        else {
            let _ = session.respond_error(503).await;
            return Ok(true);
        };
        let body = policy::render_pow_challenge(config, route, page, context, &id, &challenge);
        let mut response = ResponseHeader::build(200, Some(4))
            .map_err(|error| Error::because(InternalError, "failed to build challenge", error))?;
        let _ = response.insert_header("Content-Type", "text/html; charset=utf-8");
        let _ = response.insert_header("Cache-Control", "no-store");
        let _ = response.insert_header("Content-Length", body.len());
        session
            .write_response_header(Box::new(response), true)
            .await?;
        session
            .write_response_body(Some(body.into_bytes().into()), true)
            .await?;
        Ok(true)
    }

    async fn serve_pow_api(
        &self,
        session: &mut Session,
        route: &crate::config::Route,
        page: &crate::policy::PoWPageConfig,
        context: &RequestContext,
    ) -> Result<bool> {
        if context.path.ends_with("/make-challenge") {
            return self.issue_pow(session, route, page, context).await;
        }
        if !context.path.ends_with("/pass-challenge") {
            let _ = session.respond_error(404).await;
            return Ok(true);
        }
        let mut form = policy::parse_form(context.query.as_bytes());
        form.extend(policy::parse_form(&context.body));
        let id = form.get("id").map(String::as_str).unwrap_or_default();
        let response = form.get("response").map(String::as_str).unwrap_or_default();
        let nonce = form.get("nonce").map(String::as_str).unwrap_or_default();
        match self
            .pow
            .verify(route, context, id, response, nonce, policy::unix_now())
        {
            Ok((token, redirect)) => {
                let config = route.pow.as_ref();
                let ttl = config.map(|config| config.session_ttl).unwrap_or(600);
                let cookie =
                    format!("{POW_COOKIE}={token}; Path=/; HttpOnly; SameSite=Lax; Max-Age={ttl}");
                let mut response = ResponseHeader::build(303, Some(4)).map_err(|error| {
                    Error::because(InternalError, "failed to build PoW response", error)
                })?;
                let _ = response.insert_header("Location", redirect);
                let _ = response.insert_header("Set-Cookie", cookie);
                let _ = response.insert_header("Content-Length", 0);
                session
                    .write_response_header(Box::new(response), true)
                    .await?;
                Ok(true)
            }
            Err(status) => {
                let _ = session.respond_error(status).await;
                Ok(true)
            }
        }
    }
}

fn content_type(file_name: Option<&std::ffi::OsStr>) -> &'static str {
    let name = file_name
        .and_then(|value| value.to_str())
        .unwrap_or_default();
    match name
        .rsplit_once('.')
        .map(|(_, extension)| extension.to_ascii_lowercase())
    {
        Some(extension) => match extension.as_str() {
            "html" | "htm" => "text/html; charset=utf-8",
            "css" => "text/css; charset=utf-8",
            "js" | "mjs" => "text/javascript; charset=utf-8",
            "json" => "application/json; charset=utf-8",
            "svg" => "image/svg+xml",
            "png" => "image/png",
            "jpg" | "jpeg" => "image/jpeg",
            "webp" => "image/webp",
            "avif" => "image/avif",
            "ico" => "image/x-icon",
            "txt" => "text/plain; charset=utf-8",
            "xml" => "application/xml; charset=utf-8",
            "wasm" => "application/wasm",
            "woff" => "font/woff",
            "woff2" => "font/woff2",
            _ => "application/octet-stream",
        },
        None => "application/octet-stream",
    }
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
    let fallback = policy::pages_fallback(deployment);
    let file = policy::pages_resolve(deployment, &root, path).or_else(|| {
        let fallback = root.join(&fallback);
        fallback.is_file().then_some(fallback)
    });
    let Some(file) = file else {
        return false;
    };
    let Ok(contents) = tokio::fs::read(&file).await else {
        return false;
    };
    let Ok(mut response) = ResponseHeader::build(200, Some(4)) else {
        return false;
    };
    let _ = response.insert_header("Content-Type", content_type(file.file_name()));
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

pub fn run_server(state: DataPlaneState) -> anyhow::Result<()> {
    let DataPlaneState {
        shared,
        state_dir,
        http_listen,
        tls_listen,
        access_logs,
    } = state;
    STATE_DIR
        .set(state_dir.to_string_lossy().into_owned())
        .map_err(|_| anyhow::anyhow!("proxy state directory was already initialized"))?;
    std::fs::create_dir_all(&state_dir)?;
    let snapshot_path = state_dir.join("snapshot.json");
    let pid_file = state_dir.join("openflare-edge.pid");
    std::fs::write(&pid_file, format!("{}\n", std::process::id()))?;
    let conf = ServerConf {
        daemon: false,
        working_directory: Some(state_dir.clone()),
        error_log: Some(
            state_dir
                .join("openflare-edge.log")
                .to_string_lossy()
                .into_owned(),
        ),
        pid_file: pid_file.to_string_lossy().into_owned(),
        upgrade_sock: state_dir
            .join("openflare-edge-upgrade.sock")
            .to_string_lossy()
            .into_owned(),
        ..ServerConf::default()
    };
    let mut server = Server::new_with_opt_and_conf(Some(Opt::default()), conf);
    let bootstrap = server.bootstrap_as_a_service();
    let mut service = http_proxy_service(
        &Arc::clone(&server.configuration),
        ProxyService {
            state: shared.clone(),
            rate_limiter: global_rate_limiter(),
            state_dir: global_state_dir(),
            pow: global_pow_state(),
            access_logs,
        },
    );
    service.add_tcp(&http_listen);
    if let Some(listen) = tls_listen.as_deref() {
        let mut settings = TlsSettings::with_callbacks(Box::new(DynamicCertificateCallback {
            state: shared.clone(),
        }))?;
        settings.enable_h2();
        service.add_tls_with_settings(listen, None, settings);
    }
    let proxy = server.add_service(service);
    proxy.add_dependency(&bootstrap);
    std::thread::spawn(move || watch_snapshot(&snapshot_path, &shared));
    server.run_forever()
}

fn watch_snapshot(snapshot_path: &Path, shared: &SharedState) {
    let mut last_seen = None;
    loop {
        let marker = std::fs::metadata(snapshot_path)
            .map(|metadata| {
                (
                    metadata.len(),
                    metadata
                        .modified()
                        .map_err(|error| {
                            tracing::error!(error = %error, "failed to read snapshot mtime");
                            error
                        })
                        .ok(),
                )
            })
            .ok();
        if marker != last_seen {
            if let Some((size, _)) = marker {
                match std::fs::read_to_string(snapshot_path) {
                    Ok(document) => match crate::config::restore_snapshot_from_str(&document) {
                        Ok(Some(applied)) => {
                            if let Err(error) = shared.set_config(applied) {
                                tracing::error!(error = %error, "failed to adopt snapshot");
                            }
                        }
                        Ok(None) => {}
                        Err(error) => {
                            tracing::error!(size, error = %error, "invalid snapshot; keeping current configuration");
                        }
                    },
                    Err(error) => {
                        tracing::error!(path = %snapshot_path.display(), error = %error, "failed to read snapshot");
                    }
                }
            }
            last_seen = marker;
        }
        std::thread::sleep(Duration::from_millis(500));
    }
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
