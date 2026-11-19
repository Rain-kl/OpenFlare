use crate::protocol::config::{CachePolicy, RouteCache};
use pingora::{
    cache::{
        cache_control::CacheControl, eviction::simple_lru::Manager, filters, key::HashBinary,
        lock::CacheLock, CacheKey, CacheMeta, CacheMetaDefaults, CacheOptionOverrides, CachePhase,
        MemCache, NoCacheReason, RespCacheable, VarianceBuilder,
    },
    proxy::Session,
};
use pingora_http::{RequestHeader, ResponseHeader};
use std::{sync::LazyLock, time::Duration};

const CACHE_CAPACITY: usize = 128 * 1024 * 1024;
const MAX_OBJECT_SIZE: usize = 16 * 1024 * 1024;
static STORAGE: LazyLock<MemCache> = LazyLock::new(MemCache::new);
static EVICTION: LazyLock<Manager> =
    LazyLock::new(|| Manager::new_with_watermark(CACHE_CAPACITY, Some(32_768)));
static CACHE_LOCK: LazyLock<CacheLock> = LazyLock::new(|| CacheLock::new(Duration::from_secs(60)));

const DEFAULT_KEY_TEMPLATE: &str = "$scheme$host$request_uri";

pub(super) fn enable(session: &mut Session, policy: &CachePolicy, route: &RouteCache) {
    let req = session.req_header();
    if req.method != http::Method::GET
        || req.headers.contains_key("authorization")
        || req.headers.contains_key("upgrade")
        || req.headers.contains_key("range")
        || !path_matches(route, &policy.static_extensions, req.uri.path())
    {
        return;
    }
    let mut timeouts = CacheOptionOverrides::default();
    timeouts.wait_timeout = Some(Duration::from_millis(policy.lock_timeout_ms));
    session.cache.enable(
        &*STORAGE,
        Some(&*EVICTION),
        None,
        policy.lock_enabled.then_some(&*CACHE_LOCK),
        Some(timeouts),
    );
    session.cache.set_max_file_size_bytes(MAX_OBJECT_SIZE);
}

fn path_matches(route: &RouteCache, static_extensions: &[String], path: &str) -> bool {
    let suffix = |rules: &[String]| {
        let extension = path.rsplit_once('.').map(|(_, extension)| extension);
        extension.is_some_and(|extension| {
            rules
                .iter()
                .any(|rule| extension.eq_ignore_ascii_case(rule.trim_start_matches('.')))
        })
    };
    match route.policy.as_str() {
        "all" | "url" => true,
        "path_exact" => route.rules.iter().any(|rule| path == rule),
        "path_prefix" => route.rules.iter().any(|rule| path.starts_with(rule)),
        "suffix" => suffix(&route.rules),
        _ => suffix(static_extensions),
    }
}

pub(super) fn key(
    request: &RequestHeader,
    policy: &CachePolicy,
    scheme: &str,
    host: &str,
    digest: &str,
) -> CacheKey {
    let default = policy.key_template.is_empty() || policy.key_template == DEFAULT_KEY_TEMPLATE;
    let path = request.uri.path();
    let args = request.uri.query().unwrap_or_default();
    let request_uri = request
        .uri
        .path_and_query()
        .map_or(path, |value| value.as_str());
    let key = if default {
        format!("{scheme}|{host}|{request_uri}")
    } else {
        let mut rendered = String::new();
        let mut parts = policy.key_template.split('$');
        rendered.push_str(parts.next().unwrap_or_default());
        for part in parts {
            let end = part
                .find(|c: char| !(c.is_ascii_alphanumeric() || c == '_'))
                .unwrap_or(part.len());
            let (variable, suffix) = part.split_at(end);
            rendered.push_str(match variable {
                "scheme" => scheme,
                "host" => host,
                "request_uri" => request_uri,
                "uri" => path,
                "args" | "query_string" => args,
                "is_args" if !args.is_empty() => "?",
                "request_method" => request.method.as_str(),
                _ => "",
            });
            rendered.push_str(suffix);
        }
        rendered
    };
    CacheKey::new(format!("{digest}|{scheme}|{host}|{key}"), "")
}

pub(crate) fn validate_policy(policy: &CachePolicy) -> anyhow::Result<()> {
    for fragment in policy.key_template.split('$').skip(1) {
        let variable = fragment
            .split(|c: char| !(c.is_ascii_alphanumeric() || c == '_'))
            .next()
            .unwrap_or_default();
        if !matches!(
            variable,
            "scheme"
                | "host"
                | "request_uri"
                | "uri"
                | "args"
                | "query_string"
                | "is_args"
                | "request_method"
        ) {
            anyhow::bail!("unsupported cache key variable {variable}");
        }
    }
    Ok(())
}

pub(super) fn response(
    session: &Session,
    response: &ResponseHeader,
    policy: &CachePolicy,
) -> RespCacheable {
    if response.headers.contains_key("set-cookie")
        || response.headers.get_all("vary").iter().any(|value| {
            value.to_str().map_or(true, |value| {
                value.split(',').any(|name| name.trim() == "*")
            })
        })
    {
        return RespCacheable::Uncacheable(NoCacheReason::OriginNotCache);
    }
    let controls = CacheControl::from_resp_headers(response);
    let defaults = CacheMetaDefaults::new(
        default_ttl,
        if policy.stale_updating { 60 } else { 0 },
        if policy.stale_on_error { 60 } else { 0 },
    );
    filters::resp_cacheable(
        controls.as_ref(),
        response.clone(),
        session.req_header().headers.contains_key("authorization"),
        &defaults,
    )
}

fn default_ttl(status: http::StatusCode) -> Option<Duration> {
    let seconds = match status.as_u16() {
        200 | 206 | 301 => 7200,
        302 | 303 => 1200,
        404 | 410 => 180,
        _ => return None,
    };
    Some(Duration::from_secs(seconds))
}

pub(super) fn variance(meta: &CacheMeta, request: &RequestHeader) -> Option<HashBinary> {
    let mut builder = VarianceBuilder::new();
    for value in meta
        .response_header()
        .headers
        .get_all("vary")
        .iter()
        .filter_map(|value| value.to_str().ok())
    {
        for name in value
            .split(',')
            .map(str::trim)
            .filter(|name| !name.is_empty())
        {
            let joined = request
                .headers
                .get_all(name)
                .iter()
                .flat_map(|value| value.as_bytes().iter().copied().chain([0]))
                .collect();
            builder.add_owned_name_value(name.to_ascii_lowercase(), joined);
        }
    }
    builder.finalize()
}

pub(super) fn status(session: &Session) -> &'static str {
    match session.cache.phase() {
        CachePhase::Hit => "HIT",
        CachePhase::Miss => "MISS",
        CachePhase::Stale => "STALE",
        CachePhase::StaleUpdating => "UPDATING",
        CachePhase::Expired => "EXPIRED",
        CachePhase::Revalidated => "REVALIDATED",
        CachePhase::Bypass => "BYPASS",
        _ => "-",
    }
}
