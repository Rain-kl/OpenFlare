use super::responses::{respond_policy_decision, serve_pow_static};
use super::{ProxyService, RequestLogContext};
use crate::{
    config::AppliedConfig,
    policy::{self, PolicyDecision, RequestContext, SITE_AUTH_PATH},
};
use pingora::{proxy::Session, Error, Result};
use pingora_error::ErrorType::{ConnectionClosed, HTTPStatus, InternalError};
use std::{sync::Arc, time::Instant};

const MAX_INSPECTED_BODY_BYTES: usize = 64 * 1024;

impl ProxyService {
    pub(super) async fn handle_request(
        &self,
        session: &mut Session,
        ctx: &mut RequestLogContext,
    ) -> Result<bool> {
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
            let status = config.config.default_server_return_status;
            if status == 444 {
                return Err(Error::new(ConnectionClosed));
            }
            session.respond_error(status).await?;
            return Ok(true);
        };
        ctx.snapshot = Some(Arc::clone(&config));
        ctx.upstream_start = config.upstream_start(route.id);
        let has_tls = route.enable_https && route.certificate_id_for_host(&host).is_some();
        if !ctx.https && has_tls && (route.redirect_http || route.site_auth_ttl_hours > 0) {
            let location = format!(
                "https://{}{}",
                host,
                session
                    .req_header()
                    .uri
                    .path_and_query()
                    .map_or("/", |path| path.as_str())
            );
            return respond_policy_decision(
                session,
                PolicyDecision::Respond {
                    status: 301,
                    headers: vec![("Location".into(), location)],
                    body: String::new(),
                },
            )
            .await;
        }
        if ctx.https && !has_tls {
            session.respond_error(404).await?;
            return Ok(true);
        }
        let page = config.pow_page();
        let content_length = request_content_length(session);
        let login =
            route.site_auth_ttl_hours != 0 && session.req_header().uri.path() == SITE_AUTH_PATH;
        let max_body = if login {
            8192
        } else {
            config.config.max_request_body
        };
        ctx.request_limit = max_body;
        if max_body != 0 && content_length.is_some_and(|length| length > max_body) {
            session.respond_error(413).await?;
            return Ok(true);
        }
        let context = build_request_context(
            session,
            &host,
            &ctx.remote_addr,
            &config,
            route,
            &self.geo_ip,
        )
        .await?;
        ctx.remote_addr = context.remote_addr.clone();
        ctx.remote_ip = context.remote_addr.clone();
        let site_auth = policy::site_auth(route, &context, ctx.https, policy::unix_now());
        if !matches!(site_auth, PolicyDecision::Continue) {
            return respond_policy_decision(session, site_auth).await;
        }
        let pow_routed = route.pow_enabled || policy::has_waf_pow(&config.config.waf, route);
        if pow_routed && context.path.starts_with(policy::POW_API_PREFIX) {
            return self
                .serve_pow_api(session, route, &context, ctx.https)
                .await;
        }
        if pow_routed && context.path.starts_with(policy::POW_STATIC_PREFIX) {
            return serve_pow_static(session, &context.path).await;
        }
        let pow_session =
            self.pow
                .valid_session(&context.host, &context.cookie, policy::unix_now());
        if let Some(token) = pow_session.as_ref().filter(|_| pow_routed) {
            let ttl = route
                .pow
                .as_ref()
                .filter(|_| route.pow_enabled)
                .map(|config| config.session_ttl);
            if let Some(ttl) =
                self.pow
                    .refresh_session(&context.host, token, ttl, policy::unix_now())
            {
                ctx.pow_cookie = Some(policy::session_cookie(token, ttl, ctx.https));
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
            Ok(policy::WafDecision::Takeover(pow)) => {
                return self.issue_pow(session, route, &pow, page, &context).await
            }
            Err(error) => {
                tracing::error!(site = %route.site_name, error, "WAF graph execution failed");
                let _ = session.respond_error(500).await;
                return Ok(true);
            }
        }
        if login {
            if !policy::request_rate_limit_with_burst(
                &self.login_limiter,
                &route.site_name,
                &context.remote_addr,
                "10r/m",
                Some(11),
                Instant::now(),
            ) {
                session.respond_error(429).await?;
                return Ok(true);
            }
            let decision = policy::site_auth_login(
                route,
                &context,
                &config.config.site_auth_html,
                policy::unix_now(),
            );
            return respond_policy_decision(session, decision).await;
        }
        let pages_api = route.pages.as_ref().is_some_and(|deployment| {
            crate::pages::pages_api_target(deployment, &context.path).is_some()
        });
        if policy::pow_required(route, &context, pow_session.is_some()) {
            if let Some(pow) = route.pow.as_ref() {
                return self.issue_pow(session, route, pow, page, &context).await;
            }
        }
        if let Some(decision) = policy::service_worker(
            config.config.service_worker.as_ref(),
            route,
            &context,
            ctx.https,
        ) {
            return respond_policy_decision(session, decision).await;
        }
        if !config.config.websocket
            && !pages_api
            && session.req_header().headers.contains_key("upgrade")
        {
            session.req_header_mut().remove_header("upgrade");
            session.req_header_mut().remove_header("connection");
        }
        if !pages_api {
            let Some(permit) = self.connections.enter(route, &context.remote_addr) else {
                session.respond_error(503).await?;
                return Ok(true);
            };
            ctx.permit = Some(permit);
            ctx.response_rate =
                crate::protocol::snapshot::byte_size(&route.limit.rate).unwrap_or(0);
            if !route.limit.req_per_ip.is_empty()
                && !policy::request_rate_limit(
                    &self.rate_limiter,
                    &route.site_name,
                    &context.remote_addr,
                    &route.limit.req_per_ip,
                    Instant::now(),
                )
            {
                session.respond_error(429).await?;
                return Ok(true);
            }
            let decision = policy::basic_auth(route, &context);
            if !matches!(decision, PolicyDecision::Continue) {
                return respond_policy_decision(session, decision).await;
            }
        }
        if route.is_pages() && !pages_api {
            if crate::pages::serve_pages(
                session,
                route,
                &config,
                &self.state_dir,
                ctx.pow_cookie.as_deref(),
            )
            .await?
            {
                return Ok(true);
            }
            session.respond_error(404).await?;
            return Ok(true);
        }
        if route.site_auth_ttl_hours != 0
            && !route
                .custom_headers
                .iter()
                .any(|header| header.key.eq_ignore_ascii_case("cookie"))
        {
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
}

fn request_content_length(session: &Session) -> Option<u64> {
    session
        .req_header()
        .headers
        .get("content-length")?
        .to_str()
        .ok()?
        .parse()
        .ok()
}

async fn build_request_context(
    session: &mut Session,
    host: &str,
    peer_ip: &str,
    config: &AppliedConfig,
    route: &crate::protocol::config::Route,
    geo_ip: &crate::geoip::GeoIpStore,
) -> Result<RequestContext> {
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
    let headers = &session.req_header().headers;
    let is_trusted = config
        .config
        .trusted_proxy_cidrs
        .iter()
        .any(|cidr| policy::ip_in_cidr(peer_ip, cidr));
    let remote_addr = if is_trusted {
        headers
            .get("cf-connecting-ip")
            .and_then(|value| value.to_str().ok())
            .and_then(|value| value.trim().parse::<std::net::IpAddr>().ok())
            .map_or_else(|| peer_ip.to_owned(), |ip| ip.to_string())
    } else {
        peer_ip.to_owned()
    };
    let geo = geo_ip.lookup(&remote_addr);
    let authority = session
        .req_header()
        .uri
        .authority()
        .map(|value| value.as_str())
        .or_else(|| headers.get("host").and_then(|value| value.to_str().ok()))
        .unwrap_or(host)
        .to_owned();
    let sensitive_headers = policy::SECURITY_HEADERS
        .iter()
        .filter_map(|name| headers.get(*name).and_then(|value| value.to_str().ok()))
        .map(str::to_owned)
        .collect();
    let login = route.site_auth_ttl_hours != 0 && path == SITE_AUTH_PATH;
    let inspect = policy::waf_inspects_body(&config.config.waf, route)
        && request_content_length(session)
            .is_some_and(|size| size > 0 && size <= MAX_INSPECTED_BODY_BYTES as u64);
    let mut body = Vec::new();
    if login || inspect {
        if !login {
            session.as_downstream_mut().enable_retry_buffering();
        }
        let max = if login {
            8192
        } else {
            MAX_INSPECTED_BODY_BYTES
        };
        while let Some(chunk) = session.as_downstream_mut().read_request_body().await? {
            if body.len().saturating_add(chunk.len()) > max {
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
        sensitive_headers,
        authority,
        accept_language,
        geo,
    })
}

fn request_host(session: &Session) -> Option<String> {
    let req = session.req_header();
    if let Some(authority) = req.uri.authority() {
        return Some(authority.host().trim_end_matches('.').to_ascii_lowercase());
    }
    let authority: http::uri::Authority = req
        .headers
        .get("Host")?
        .to_str()
        .ok()?
        .trim()
        .parse()
        .ok()?;
    Some(authority.host().trim_end_matches('.').to_ascii_lowercase())
}
