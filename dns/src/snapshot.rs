use crate::engine::CompiledPolicy;
use crate::protocol::{ApiEnvelope, SnapshotEnvelope};
use crate::quality::SnapshotQualityOracle;
use anyhow::{Context, Result};
use arc_swap::ArcSwap;
use reqwest::Client;
use reqwest::header::{ETAG, IF_NONE_MATCH};
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use tracing::{info, warn};

const SNAPSHOT_FILE_NAME: &str = "last-snapshot.bin";

pub struct CompiledSnapshot {
    pub raw: SnapshotEnvelope,
    pub policies: Vec<CompiledPolicy>,
    pub oracle: SnapshotQualityOracle,
}

impl CompiledSnapshot {
    pub fn from_envelope(raw: SnapshotEnvelope) -> Self {
        let policies = raw
            .policies
            .iter()
            .cloned()
            .map(CompiledPolicy::compile)
            .collect();
        let oracle = SnapshotQualityOracle::from_snapshot(raw.quality.as_ref());
        Self {
            raw,
            policies,
            oracle,
        }
    }

    pub fn empty() -> Self {
        Self::from_envelope(SnapshotEnvelope {
            version: 0,
            etag: String::new(),
            generated_at: String::new(),
            delegation_domain: String::new(),
            ns_hosts: Vec::new(),
            default_ttl: 60,
            nodes: Vec::new(),
            policies: Vec::new(),
            domain_routes: Vec::new(),
            quality: None,
        })
    }
}

pub struct SnapshotStore {
    current: ArcSwap<CompiledSnapshot>,
    cache_path: Option<PathBuf>,
}

impl SnapshotStore {
    pub fn new(cache_dir: Option<&str>) -> Self {
        let cache_path = cache_dir
            .filter(|s| !s.trim().is_empty())
            .map(|dir| Path::new(dir).join(SNAPSHOT_FILE_NAME));
        let initial = cache_path
            .as_deref()
            .and_then(load_disk_snapshot)
            .unwrap_or_else(CompiledSnapshot::empty);
        if initial.raw.version > 0 {
            info!(
                "loaded offline SmartDNS snapshot v{} from disk cache",
                initial.raw.version
            );
        }
        Self {
            current: ArcSwap::from_pointee(initial),
            cache_path,
        }
    }

    pub fn with_snapshot(snap: CompiledSnapshot) -> Self {
        Self {
            current: ArcSwap::from_pointee(snap),
            cache_path: None,
        }
    }

    #[allow(dead_code)]
    pub fn load(&self) -> Arc<CompiledSnapshot> {
        self.current.load_full()
    }

    pub fn guard(&self) -> arc_swap::Guard<Arc<CompiledSnapshot>> {
        self.current.load()
    }

    pub fn update_from_envelope(&self, env: SnapshotEnvelope) {
        if let Some(ref path) = self.cache_path {
            if let Err(err) = save_disk_snapshot(path, &env) {
                warn!("persist snapshot cache {:?} failed: {}", path, err);
            }
        }
        let compiled = CompiledSnapshot::from_envelope(env);
        self.current.store(Arc::new(compiled));
    }

    pub fn current_version_and_etag(&self) -> (u64, String) {
        let snap = self.current.load();
        (snap.raw.version, snap.raw.etag.clone())
    }
}

pub fn load_disk_snapshot(path: &Path) -> Option<CompiledSnapshot> {
    let bytes = fs::read(path).ok()?;
    let env: SnapshotEnvelope = serde_json::from_slice(&bytes).ok()?;
    Some(CompiledSnapshot::from_envelope(env))
}

pub fn save_disk_snapshot(path: &Path, env: &SnapshotEnvelope) -> Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let tmp = path.with_extension("tmp");
    let data = serde_json::to_vec(env)?;
    fs::write(&tmp, &data)?;
    fs::rename(&tmp, path)?;
    Ok(())
}

/// Fetches a new compiled snapshot from controller when ETag differs; returns true if updated.
pub async fn pull_snapshot(
    client: &Client,
    controller_url: &str,
    token: &str,
    store: &SnapshotStore,
) -> Result<bool> {
    let base = controller_url.trim_end_matches('/');
    let url = format!("{base}/api/v1/dns-agent/snapshot");
    let (_, etag) = store.current_version_and_etag();

    let mut req = client.get(&url).header("X-DNS-Token", token);
    if !etag.is_empty() {
        req = req.header(IF_NONE_MATCH, etag.as_str());
    }

    let resp = req.send().await.context("send snapshot request")?;
    if resp.status() == reqwest::StatusCode::NOT_MODIFIED {
        return Ok(false);
    }
    let status = resp.status();
    if !status.is_success() {
        anyhow::bail!("snapshot HTTP {}", status);
    }

    let header_etag = resp
        .headers()
        .get(ETAG)
        .and_then(|v| v.to_str().ok())
        .map(str::to_string);

    let env_body: ApiEnvelope<SnapshotEnvelope> = resp.json().await.context("decode snapshot json")?;
    let mut env = env_body
        .data
        .context("controller returned empty snapshot data")?;
    if env.etag.is_empty() {
        if let Some(h) = header_etag {
            env.etag = h;
        }
    }
    info!(
        "applied SmartDNS snapshot v{} (nodes={}, policies={}, routes={})",
        env.version,
        env.nodes.len(),
        env.policies.len(),
        env.domain_routes.len()
    );
    store.update_from_envelope(env);
    Ok(true)
}
