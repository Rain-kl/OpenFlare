use crate::protocol::config::{CachePolicy, RouteCache};
use crate::runtime::performance::{node_memory_limit, Cache, DiskCacheSettings, Startup};
use anyhow::{Context, Result};
use async_trait::async_trait;
use pingap_cache::{new_cache_backend, update_available_memory, HttpCache, PAGE_SIZE};
use pingora::{
    cache::{
        cache_control::CacheControl,
        filters,
        key::{hash_key, HashBinary},
        lock::CacheLock,
        CacheKey, CacheMeta, CacheMetaDefaults, CacheOptionOverrides, CachePhase, NoCacheReason,
        RespCacheable, VarianceBuilder,
    },
    proxy::Session,
    server::ShutdownWatch,
    services::background::BackgroundService,
};
use pingora_http::{RequestHeader, ResponseHeader};
use std::{
    fs::DirBuilder,
    os::unix::fs::DirBuilderExt,
    path::Path,
    sync::LazyLock,
    time::{Duration, SystemTime},
};
use tokio::time::{interval, MissedTickBehavior};

const MIB: usize = 1024 * 1024;
const DEFAULT_CAPACITY_MIB: usize = 128;
const DEFAULT_MAX_OBJECT_MIB: usize = 16;
const DISK_CACHE_DIR: &str = "http-cache";
static CACHE_LOCK: LazyLock<CacheLock> = LazyLock::new(|| CacheLock::new(Duration::from_secs(60)));

pub(super) struct EdgeCache {
    storage: &'static HttpCache,
    max_object_bytes: u64,
}

impl EdgeCache {
    pub(super) fn open(state_dir: &Path, startup: Option<&Startup>) -> Result<Self> {
        let capacity = startup.map_or(DEFAULT_CAPACITY_MIB, |settings| settings.cache_capacity_mib);
        let memory_limit = node_memory_limit();
        update_available_memory(memory_limit);
        if let Some(disk) = startup
            .map(|settings| &settings.disk_cache)
            .filter(|disk| disk.cache_disk_capacity_mib > 0)
        {
            match disk_storage(state_dir, disk, capacity) {
                Ok(storage) => {
                    // Pingap buffers disk objects in memory for both fills and hits.
                    let disk_bytes = disk.cache_disk_capacity_mib * MIB as u64;
                    return Ok(Self {
                        storage,
                        max_object_bytes: disk_bytes.min((memory_limit / 4).max(1)),
                    });
                }
                Err(error) => {
                    tracing::warn!(%error, "disk cache unavailable; using memory cache");
                }
            }
        }
        let storage = new_cache_backend(&format!(
            "memory://openflare?max_size={capacity}MiB&mode=compact"
        ))
        .context("initialize edge memory cache")?;
        Ok(Self {
            storage,
            max_object_bytes: storage.max_size(),
        })
    }

    pub(super) fn disk_cleanup(&self) -> Option<DiskCleanup> {
        self.storage.cache.inactive().map(|inactive| DiskCleanup {
            storage: self.storage,
            inactive,
        })
    }

    pub(super) fn enable(
        &self,
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
        session
            .cache
            .enable(self.storage, None, None, Some(&*CACHE_LOCK), Some(timeouts));
        let max_object =
            tuning.map_or(DEFAULT_MAX_OBJECT_MIB, |tuning| tuning.cache_max_object_mib);
        session
            .cache
            .set_max_file_size_bytes((max_object * MIB).min(self.max_object_bytes as usize));
    }
}

fn disk_storage(
    state_dir: &Path,
    settings: &DiskCacheSettings,
    memory_mib: usize,
) -> Result<&'static HttpCache> {
    let path = state_dir.join(DISK_CACHE_DIR);
    let directory = path
        .to_str()
        .filter(|path| !path.contains('?'))
        .context("invalid edge disk cache directory")?;
    DirBuilder::new()
        .recursive(true)
        .mode(0o700)
        .create(&path)
        .context("create edge disk cache directory")?;
    anyhow::ensure!(
        path.symlink_metadata()?.is_dir(),
        "edge disk cache path is not a directory"
    );
    tempfile::NamedTempFile::new_in(&path).context("check that edge disk cache is writable")?;

    let hot_pages = memory_mib * MIB / PAGE_SIZE;
    new_cache_backend(&format!(
        "{directory}?max_size={}MiB&inactive={}s&reading_max={}&writing_max={}&cache_max={hot_pages}&levels=2:2",
        settings.cache_disk_capacity_mib,
        settings.cache_disk_inactive_seconds,
        settings.cache_disk_reading_max,
        settings.cache_disk_writing_max,
    )).context("initialize edge disk cache")
}

pub(super) struct DiskCleanup {
    storage: &'static HttpCache,
    inactive: Duration,
}

#[async_trait]
impl BackgroundService for DiskCleanup {
    async fn start(&self, mut shutdown: ShutdownWatch) {
        let mut ticker = interval(self.inactive.min(Duration::from_secs(3600)));
        ticker.set_missed_tick_behavior(MissedTickBehavior::Skip);
        loop {
            tokio::select! {
                biased;
                _ = shutdown.changed() => return,
                _ = ticker.tick() => {
                    let Some(cutoff) = SystemTime::now().checked_sub(self.inactive) else {
                        continue;
                    };
                    if let Err(error) = self.storage.cache.clear(cutoff).await {
                        tracing::warn!(%error, "could not sweep inactive edge cache files");
                    }
                }
            }
        }
    }
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
    scheme: &str,
    host: &str,
    digest: &str,
    max_object_bytes: usize,
) -> CacheKey {
    let request_uri = request
        .uri
        .path_and_query()
        .map_or(request.uri.path(), |value| value.as_str());
    // Pingap logs the primary key on writes. Hash before handing over the URL,
    // including its query, so storage logs and filenames carry only opaque IDs.
    // Limiting the size also selects a namespace that can only contain admitted sizes.
    CacheKey::new(
        hash_key(format!(
            "{digest}|{max_object_bytes}|{scheme}|{host}|{request_uri}"
        )),
        "",
    )
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::performance::Settings;
    use pingora::cache::key::CacheHashKey;

    #[test]
    fn cache_keys_isolate_host_query_snapshot_and_size_limit() {
        let request = RequestHeader::build("GET", b"/route?ticket=secret", None).unwrap();
        let other = RequestHeader::build("GET", b"/route?ticket=other", None).unwrap();
        let base = key(&request, "https", "one.example", "snapshot-a", 4096);
        for changed in [
            key(&request, "http", "one.example", "snapshot-a", 4096),
            key(&request, "https", "two.example", "snapshot-a", 4096),
            key(&other, "https", "one.example", "snapshot-a", 4096),
            key(&request, "https", "one.example", "snapshot-b", 4096),
            key(&request, "https", "one.example", "snapshot-a", 8192),
        ] {
            assert_ne!(base.combined(), changed.combined());
        }
        assert_eq!(base.primary_key().len(), 16);
    }

    #[test]
    fn unavailable_disk_falls_back_to_memory() {
        let state = tempfile::tempdir().unwrap();
        let path = state.path().join(DISK_CACHE_DIR);
        std::fs::write(&path, b"not a cache directory").unwrap();
        let raw = include_str!("../../tests/performance_policy.json");
        let mut settings: Settings = serde_json::from_str(raw).unwrap();
        assert_eq!(settings.startup.disk_cache.cache_disk_capacity_mib, 0);
        assert_eq!(settings.startup.disk_cache.cache_disk_reading_max, 32);
        settings.startup.disk_cache.cache_disk_capacity_mib = 64;
        let cache = EdgeCache::open(state.path(), Some(&settings.startup)).unwrap();
        assert!(cache.disk_cleanup().is_none());
        assert_eq!(std::fs::read(path).unwrap(), b"not a cache directory");
    }
}
