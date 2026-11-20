use crate::protocol::config::{CachePolicy, RouteCache};
use crate::runtime::performance::Cache;
use pingora::{
    cache::{
        cache_control::CacheControl, eviction::simple_lru::Manager, filters, key::HashBinary,
        lock::CacheLock, CacheKey, CacheMeta, CacheMetaDefaults, CacheOptionOverrides, CachePhase,
        MemCache, NoCacheReason, RespCacheable, VarianceBuilder,
    },
    proxy::Session,
};
use pingora_http::{RequestHeader, ResponseHeader};
use std::{
    sync::{LazyLock, OnceLock},
    time::Duration,
};

const MIB: usize = 1024 * 1024;
const DEFAULT_CAPACITY_MIB: usize = 128;
const DEFAULT_MAX_OBJECT_MIB: usize = 16;
static STORAGE: LazyLock<MemCache> = LazyLock::new(MemCache::new);
static EVICTION: OnceLock<Manager> = OnceLock::new();
static CAPACITY: OnceLock<usize> = OnceLock::new();

pub(super) fn configure_capacity(capacity_mib: Option<usize>) {
    CAPACITY.get_or_init(|| capacity_mib.unwrap_or(DEFAULT_CAPACITY_MIB) * MIB);
}
static CACHE_LOCK: LazyLock<CacheLock> = LazyLock::new(|| CacheLock::new(Duration::from_secs(60)));

pub(super) fn enable(
    session: &mut Session,
    policy: &CachePolicy,
    route: &RouteCache,
    tuning: Option<&Cache>,
) {
    let req = session.req_header();
    if tuning.is_some_and(|tuning| !tuning.cache_enabled)
        || req.method != http::Method::GET
        || req.headers.contains_key("authorization")
        || req.headers.contains_key("upgrade")
        || req.headers.contains_key("range")
        || !path_matches(route, &policy.static_extensions, req.uri.path())
    {
        return;
    }
    let mut timeouts = CacheOptionOverrides::default();
    timeouts.wait_timeout = Some(tuning.map_or_else(
        || Duration::from_secs(5),
        |tuning| Duration::from_secs(tuning.cache_lock_wait_timeout),
    ));
    timeouts.max_lock_retries = tuning.map(|tuning| tuning.cache_max_lock_retries);
    let eviction = EVICTION.get_or_init(|| {
        Manager::new_with_watermark(
            *CAPACITY.get().unwrap_or(&(DEFAULT_CAPACITY_MIB * MIB)),
            Some(32_768),
        )
    });
    session.cache.enable(
        &*STORAGE,
        Some(eviction),
        None,
        Some(&*CACHE_LOCK),
        Some(timeouts),
    );
    let max_object = tuning.map_or(DEFAULT_MAX_OBJECT_MIB, |tuning| tuning.cache_max_object_mib);
    session.cache.set_max_file_size_bytes(max_object * MIB);
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

pub(super) fn key(request: &RequestHeader, scheme: &str, host: &str, digest: &str) -> CacheKey {
    let request_uri = request
        .uri
        .path_and_query()
        .map_or(request.uri.path(), |value| value.as_str());
    CacheKey::new(format!("{digest}|{scheme}|{host}|{request_uri}"), "")
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
