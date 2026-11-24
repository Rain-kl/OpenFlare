use super::{ProxyService, RequestLogContext};
use crate::policy::{self, PolicyDecision, RequestContext};
use pingora::{proxy::Session, Error, Result};
use pingora_error::ErrorType::{
    ConnectionClosed, HTTPStatus, InternalError, ReadError, WriteError,
};
use pingora_http::ResponseHeader;
use std::time::{Duration, Instant};

impl ProxyService {
    pub(super) async fn filter_response(
        &self,
        session: &mut Session,
        response: &mut ResponseHeader,
        ctx: &mut RequestLogContext,
    ) -> Result<()> {
        let Some(snapshot) = ctx.snapshot.as_ref() else {
            return Ok(());
        };
        let Some(route) = snapshot.route_for_host(&ctx.host) else {
            return Ok(());
        };
        if let Some(cookie) = &ctx.pow_cookie {
            response.append_header("set-cookie", cookie.as_str())?;
        }
        if response.status == http::StatusCode::SWITCHING_PROTOCOLS {
            session.as_downstream_mut().set_read_timeout(None);
            return Ok(());
        }
        if route.site_auth_ttl_hours != 0 {
            response.insert_header("cache-control", "private, no-store")?;
        }
        if !route.is_pages() {
            if let Some(page) = snapshot.config.origin_error_page.as_ref() {
                if let Some(body) = origin_error_body(
                    page,
                    session.req_header().method.as_str(),
                    &ctx.host,
                    response.status.as_u16(),
                ) {
                    response.insert_header("content-type", "text/html; charset=utf-8")?;
                    response.insert_header("content-length", body.len())?;
                    response.remove_header("content-encoding");
                    response.remove_header("transfer-encoding");
                    response.remove_header("etag");
                    if session.req_header().method != http::Method::HEAD {
                        ctx.origin_error_body = Some(body.into_bytes().into());
                        ctx.origin_error_replaced = true;
                    }
                }
            }
        }
        if snapshot.config.gzip && should_compress(response, snapshot.config.gzip_min_length) {
            if let Some(compression) = session
                .downstream_modules_ctx
                .get_mut::<pingora::modules::http::compression::ResponseCompression>(
            ) {
                compression.adjust_algorithm_level(
                    pingora::protocols::http::compression::Algorithm::Gzip,
                    5,
                );
            }
        }
        Ok(())
    }

    pub(super) fn filter_response_body(
        &self,
        _: &mut Session,
        body: &mut Option<bytes::Bytes>,
        _: bool,
        ctx: &mut RequestLogContext,
    ) -> Result<Option<Duration>> {
        if ctx.origin_error_replaced {
            *body = ctx.origin_error_body.take();
        }
        let Some(chunk) = body
            .as_ref()
            .filter(|chunk| ctx.response_rate != 0 && !chunk.is_empty())
        else {
            return Ok(None);
        };
        let now = Instant::now();
        let ready = ctx.next_response_write.unwrap_or(now).max(now);
        ctx.next_response_write = ready.checked_add(Duration::from_secs_f64(
            chunk.len() as f64 / ctx.response_rate as f64,
        ));
        Ok(Some(ready.saturating_duration_since(now)))
    }

    pub(super) async fn respond_to_failure(
        &self,
        session: &mut Session,
        error: &Error,
        ctx: &mut RequestLogContext,
    ) -> pingora::proxy::FailToProxy {
        let status = match error.etype() {
            HTTPStatus(status) => *status,
            _ => match error.esource() {
                pingora::ErrorSource::Upstream => 502,
                pingora::ErrorSource::Downstream => match error.etype() {
                    WriteError | ReadError | ConnectionClosed => 0,
                    _ => 400,
                },
                _ if error.etype() == &ConnectionClosed => 0,
                _ => 500,
            },
        };
        if status > 0 && session.as_downstream().response_written().is_none() {
            let page = ctx
                .snapshot
                .as_ref()
                .filter(|snapshot| {
                    snapshot
                        .route_for_host(&ctx.host)
                        .is_some_and(|route| !route.is_pages())
                })
                .and_then(|snapshot| snapshot.config.origin_error_page.as_ref())
                .filter(|page| !page.get_only);
            let result = if let Some(body) = page.and_then(|page| {
                origin_error_body(
                    page,
                    session.req_header().method.as_str(),
                    &ctx.host,
                    status,
                )
            }) {
                respond_policy_decision(
                    session,
                    PolicyDecision::Respond {
                        status,
                        headers: vec![("Content-Type".into(), "text/html; charset=utf-8".into())],
                        body,
                    },
                )
                .await
                .map(|_| ())
            } else {
                session.respond_error(status).await
            };
            if let Err(error) = result {
                tracing::debug!(%error, status, "could not write proxy failure response");
            }
        }
        pingora::proxy::FailToProxy {
            error_code: status,
            can_reuse_downstream: false,
        }
    }
}

fn origin_error_body(
    page: &crate::protocol::config::OriginErrorPage,
    method: &str,
    host: &str,
    status: u16,
) -> Option<String> {
    if (page.get_only && method != "GET") || !page.status_codes.contains(&status) {
        return None;
    }
    Some(
        page.html
            .replace("{{status}}", &status.to_string())
            .replace("{{host}}", host),
    )
}

fn should_compress(response: &ResponseHeader, minimum: u64) -> bool {
    let headers = &response.headers;
    let size = headers
        .get("content-length")
        .and_then(|value| value.to_str().ok())
        .and_then(|value| value.parse::<u64>().ok());
    let content = headers
        .get("content-type")
        .and_then(|value| value.to_str().ok())
        .unwrap_or_default();
    size.is_none_or(|size| size >= minimum)
        && (content.starts_with("text/")
            || content.starts_with("application/json")
            || content.starts_with("application/javascript")
            || content.starts_with("application/xml")
            || content.starts_with("image/svg+xml"))
}

pub(super) async fn respond_policy_decision(
    session: &mut Session,
    decision: PolicyDecision,
) -> Result<bool> {
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
            .write_response_header(
                Box::new(response),
                body.is_empty() || session.req_header().method == http::Method::HEAD,
            )
            .await?;
        if !body.is_empty() && session.req_header().method != http::Method::HEAD {
            session
                .write_response_body(Some(body.into_bytes().into()), true)
                .await?;
        }
        return Ok(true);
    }
    Ok(false)
}

pub(super) async fn serve_pow_static(session: &mut Session, path: &str) -> Result<bool> {
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
    pub(super) async fn issue_pow(
        &self,
        session: &mut Session,
        route: &crate::protocol::config::Route,
        config: &crate::protocol::config::PoWConfig,
        page: &crate::protocol::config::PoWPageConfig,
        context: &RequestContext,
    ) -> Result<bool> {
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

    pub(super) async fn serve_pow_api(
        &self,
        session: &mut Session,
        route: &crate::protocol::config::Route,
        context: &RequestContext,
        https: bool,
    ) -> Result<bool> {
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
            Ok((token, redirect, ttl)) => {
                let cookie = policy::session_cookie(&token, ttl, https);
                let mut headers = vec![
                    ("Set-Cookie".into(), cookie),
                    ("Cache-Control".into(), "no-store".into()),
                ];
                if form.get("redir").is_some_and(|redir| !redir.is_empty()) {
                    headers.push(("Location".into(), redirect));
                    respond_policy_decision(
                        session,
                        PolicyDecision::Respond {
                            status: 302,
                            headers,
                            body: String::new(),
                        },
                    )
                    .await
                } else {
                    headers.push(("Content-Type".into(), "application/json".into()));
                    respond_policy_decision(
                        session,
                        PolicyDecision::Respond {
                            status: 200,
                            headers,
                            body: r#"{"ok":true}"#.to_owned(),
                        },
                    )
                    .await
                }
            }
            Err(status) => {
                let _ = session.respond_error(status).await;
                Ok(true)
            }
        }
    }
}
