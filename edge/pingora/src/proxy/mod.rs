pub(crate) mod cache;
mod listener;
mod request;
mod request_preview;
mod responses;
mod upstream;

pub use listener::run_server;

use crate::{
    config::{AppliedConfig, SnapshotStore},
    policy::{self, RateLimiter},
    runtime::performance::LivePolicy,
    telemetry::{AccessLogBuffer, NodeAccessLog},
};
use async_trait::async_trait;
use pingora::{
    proxy::{ProxyHttp, Session},
    upstreams::peer::HttpPeer,
    Error, Result,
};
use pingora_error::ErrorType::{HTTPStatus, InternalError};
use pingora_http::ResponseHeader;
use std::{
    path::PathBuf,
    sync::Arc,
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};

#[derive(Debug)]
pub struct RequestLogContext {
    started_at: Instant,
    host: String,
    path: String,
    remote_addr: String,
    user_agent: String,
    snapshot: Option<Arc<AppliedConfig>>,
    upstream_index: usize,
    upstream_start: usize,
    remote_ip: String,
    https: bool,
    request_header_bytes: usize,
    request_limit: u64,
    request_bytes: u64,
    response_rate: u64,
    next_response_write: Option<Instant>,
    origin_error_body: Option<bytes::Bytes>,
    origin_error_replaced: bool,
    permit: Option<policy::RequestPermit>,
    pow_cookie: Option<String>,
    waf: Option<policy::WafAudit>,
}

impl Default for RequestLogContext {
    fn default() -> Self {
        Self {
            started_at: Instant::now(),
            host: String::new(),
            path: String::new(),
            remote_addr: String::new(),
            user_agent: String::new(),
            snapshot: None,
            upstream_index: 0,
            upstream_start: 0,
            remote_ip: String::new(),
            https: false,
            request_header_bytes: 0,
            request_limit: 0,
            request_bytes: 0,
            response_rate: 0,
            next_response_write: None,
            origin_error_body: None,
            origin_error_replaced: false,
            permit: None,
            pow_cookie: None,
            waf: None,
        }
    }
}

impl RequestLogContext {
    fn new(session: &Session, host: &str) -> Self {
        let req = session.req_header();
        let path = req.uri.path_and_query().map_or("", |value| value.as_str());
        let header_bytes = req
            .headers
            .iter()
            .map(|(name, value)| name.as_str().len() + value.len() + 4)
            .sum::<usize>();
        Self {
            host: host.to_owned(),
            path: path.to_owned(),
            request_header_bytes: header_bytes + req.method.as_str().len() + path.len() + 14,
            https: session
                .as_downstream()
                .digest()
                .is_some_and(|digest| digest.ssl_digest.is_some()),
            remote_addr: session
                .as_downstream()
                .client_addr()
                .and_then(|addr| addr.as_inet())
                .map(|addr| addr.ip().to_string())
                .unwrap_or_default(),
            user_agent: req
                .headers
                .get("user-agent")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_owned(),
            ..Self::default()
        }
    }
}

pub struct DataPlaneState {
    pub shared: SnapshotStore,
    pub state_dir: PathBuf,
    pub listeners: crate::runtime::Listeners,
    pub access_logs: AccessLogBuffer,
}

pub struct ProxyService {
    cache: cache::EdgeCache,
    state: SnapshotStore,
    rate_limiter: RateLimiter,
    login_limiter: RateLimiter,
    connections: policy::ConcurrencyLimiter,
    state_dir: PathBuf,
    geo_ip: crate::geoip::GeoIpStore,
    pow: policy::PowState,
    access_logs: AccessLogBuffer,
    performance: LivePolicy,
}

#[async_trait]
impl ProxyHttp for ProxyService {
    type CTX = RequestLogContext;

    fn new_ctx(&self) -> Self::CTX {
        RequestLogContext::default()
    }

    async fn upstream_peer(
        &self,
        session: &mut Session,
        ctx: &mut Self::CTX,
    ) -> Result<Box<HttpPeer>> {
        self.select_upstream(session, ctx).await
    }

    fn fail_to_connect(
        &self,
        session: &mut Session,
        peer: &HttpPeer,
        ctx: &mut Self::CTX,
        error: Box<Error>,
    ) -> Box<Error> {
        self.retry_upstream(session, peer, ctx, error)
    }

    async fn request_filter(&self, session: &mut Session, ctx: &mut Self::CTX) -> Result<bool> {
        self.handle_request(session, ctx).await
    }

    async fn request_body_filter(
        &self,
        _: &mut Session,
        body: &mut Option<bytes::Bytes>,
        _: bool,
        ctx: &mut Self::CTX,
    ) -> Result<()> {
        ctx.request_bytes = ctx
            .request_bytes
            .saturating_add(body.as_ref().map_or(0, |chunk| chunk.len() as u64));
        if ctx.request_limit != 0 && ctx.request_bytes > ctx.request_limit {
            return Err(Error::explain(
                HTTPStatus(413),
                "request body exceeds the configured limit",
            ));
        }
        Ok(())
    }

    async fn response_filter(
        &self,
        session: &mut Session,
        response: &mut ResponseHeader,
        ctx: &mut Self::CTX,
    ) -> Result<()> {
        self.filter_response(session, response, ctx).await
    }

    fn response_body_filter(
        &self,
        session: &mut Session,
        body: &mut Option<bytes::Bytes>,
        end: bool,
        ctx: &mut Self::CTX,
    ) -> Result<Option<Duration>> {
        self.filter_response_body(session, body, end, ctx)
    }

    async fn fail_to_proxy(
        &self,
        session: &mut Session,
        error: &Error,
        ctx: &mut Self::CTX,
    ) -> pingora::proxy::FailToProxy {
        self.respond_to_failure(session, error, ctx).await
    }

    fn request_cache_filter(&self, session: &mut Session, ctx: &mut Self::CTX) -> Result<()> {
        if let Some(snapshot) = ctx.snapshot.as_ref() {
            if let Some(policy) = snapshot.config.cache.as_ref() {
                if let Some(route) = snapshot
                    .route_for_host(&ctx.host)
                    .filter(|route| !route.is_pages())
                {
                    if let Some(route_cache) = route.cache.as_ref() {
                        if route.site_auth_ttl_hours == 0 && !route.basic_auth_enabled {
                            let performance = self.performance.current();
                            self.cache.enable(
                                session,
                                policy,
                                route_cache,
                                performance.policy.settings.as_ref().map(|s| &s.cache),
                            );
                        }
                    }
                }
            }
        }
        Ok(())
    }

    fn cache_key_callback(
        &self,
        session: &Session,
        ctx: &mut Self::CTX,
    ) -> Result<pingora::cache::CacheKey> {
        let snapshot = ctx
            .snapshot
            .as_ref()
            .ok_or_else(|| Error::explain(InternalError, "cache snapshot disappeared"))?;
        Ok(cache::key(
            session.req_header(),
            if ctx.https { "https" } else { "http" },
            &ctx.host,
            &snapshot.payload_checksum,
            session.cache.max_file_size_bytes().unwrap_or_default(),
        ))
    }

    fn cache_vary_filter(
        &self,
        meta: &pingora::cache::CacheMeta,
        _: &mut Self::CTX,
        request: &pingora_http::RequestHeader,
    ) -> Option<pingora::cache::key::HashBinary> {
        cache::variance(meta, request)
    }

    fn response_cache_filter(
        &self,
        session: &Session,
        response: &ResponseHeader,
        ctx: &mut Self::CTX,
    ) -> Result<pingora::cache::RespCacheable> {
        let Some(policy) = ctx
            .snapshot
            .as_ref()
            .and_then(|snapshot| snapshot.config.cache.as_ref())
        else {
            return Err(Error::explain(InternalError, "cache policy disappeared"));
        };
        Ok(cache::response(session, response, policy))
    }

    async fn upstream_request_filter(
        &self,
        session: &mut Session,
        request: &mut pingora_http::RequestHeader,
        ctx: &mut Self::CTX,
    ) -> Result<()> {
        self.prepare_upstream_request(session, request, ctx).await
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
        let waf = ctx.waf.take();
        let path = if waf.is_some() {
            ctx.path
                .split_once('?')
                .map_or(ctx.path.as_str(), |(path, _)| path)
        } else {
            ctx.path.as_str()
        };
        let log = NodeAccessLog {
            logged_at_unix: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|value| value.as_secs() as i64)
                .unwrap_or_default(),
            remote_addr: ctx.remote_addr.clone(),
            host: ctx.host.clone(),
            path: path.to_owned(),
            user_agent: ctx.user_agent.clone(),
            cache_status: cache::status(session).to_owned(),
            status_code: i32::from(status),
            bytes_sent: downstream.body_bytes_sent() as i64,
            request_length: (downstream.body_bytes_read() + ctx.request_header_bytes) as i64,
            request_time_ms: ctx.started_at.elapsed().as_millis() as i64,
            waf,
        };
        let spool = self.access_logs.clone();
        match tokio::task::spawn_blocking(move || spool.push(log)).await {
            Ok(Ok(())) => {}
            Ok(Err(error)) => tracing::warn!(%error, "unable to queue access log"),
            Err(error) => tracing::warn!(%error, "access log spool task stopped"),
        }
    }
}
