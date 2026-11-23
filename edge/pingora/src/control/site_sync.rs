use super::{
    config_sync::{Manifest, Resource, SITE_KEY},
    websocket::Wake,
    Runtime,
};
use crate::config::{self, AppliedConfig};
use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub(super) struct ActiveConfigMeta {
    pub(super) version: String,
    pub(super) checksum: String,
    #[serde(default)]
    pub(super) engine: String,
    #[serde(default)]
    pub(super) release_sequence: i64,
    #[serde(default)]
    pub(super) payload_checksum: String,
}

pub(super) struct SiteSync {
    latest_path: PathBuf,
    latest: Option<ActiveConfigMeta>,
    // Each process retries the latest revision once even when it failed before startup.
    failed: Option<(i64, String)>,
}

impl SiteSync {
    pub(super) fn open(state_dir: &Path, current: Option<&AppliedConfig>) -> Self {
        let latest_path = state_dir.join("snapshot.latest.json");
        let latest = std::fs::File::open(&latest_path)
            .ok()
            .and_then(|file| serde_json::from_reader::<_, ActiveConfigMeta>(file).ok())
            .filter(|latest| {
                current.is_none_or(|current| latest.release_sequence > current.release_sequence)
            })
            .or_else(|| {
                current.map(|current| ActiveConfigMeta {
                    version: current.version.clone(),
                    checksum: current.checksum.clone(),
                    engine: "pingora".into(),
                    release_sequence: current.release_sequence,
                    payload_checksum: current.payload_checksum.clone(),
                })
            });
        Self {
            latest_path,
            latest,
            failed: None,
        }
    }

    pub(super) fn latest_version(&self) -> &str {
        self.latest
            .as_ref()
            .map_or("", |latest| latest.version.as_str())
    }

    pub(super) fn last_error(&self) -> &str {
        self.failed.as_ref().map_or("", |(_, error)| error.as_str())
    }

    fn observe(&mut self, resource: &Resource) -> Result<ActiveConfigMeta> {
        let target = match &resource.metadata {
            Some(metadata) => serde_json::from_value::<ActiveConfigMeta>(metadata.clone())?,
            None => {
                let cached = self.latest.as_ref().filter(|latest| {
                    latest.version == resource.revision
                        && latest.payload_checksum == resource.sha256
                });
                cached
                    .context("unknown site snapshot revision without metadata")?
                    .clone()
            }
        };
        if let Some(latest) = &self.latest {
            anyhow::ensure!(
                target.release_sequence > latest.release_sequence || target == *latest,
                "controller returned a stale or conflicting site snapshot revision"
            );
        }
        if self.latest.as_ref() != Some(&target) {
            if let Err(error) = config::persist_private_json(&self.latest_path, &target) {
                tracing::warn!(%error, "could not persist latest site snapshot revision");
            }
            self.latest = Some(target.clone());
        }
        Ok(target)
    }

    pub(super) async fn reconcile(
        &mut self,
        runtime: &Runtime,
        node_id: &str,
        manifest: &Manifest,
        wake: &Wake,
    ) {
        let force = wake.take_force();
        if force {
            self.failed = None;
        }
        if self
            .reconcile_resources(runtime, node_id, manifest, force)
            .await
            && force
        {
            wake.requeue_force();
        }
    }

    // Reports whether a forced attempt must be retried after a transfer failure.
    async fn reconcile_resources(
        &mut self,
        runtime: &Runtime,
        node_id: &str,
        manifest: &Manifest,
        force: bool,
    ) -> bool {
        let Some(resource) = manifest.resources.get(SITE_KEY) else {
            return true;
        };
        let target = match self.observe(resource) {
            Ok(target) => target,
            Err(error) => {
                tracing::warn!(%error, "site snapshot descriptor was rejected");
                return true;
            }
        };
        let current = runtime.state.current();
        let is_current = current
            .as_ref()
            .is_some_and(|current| current.release_sequence == target.release_sequence);
        let blocked = self
            .failed
            .as_ref()
            .is_some_and(|(sequence, _)| *sequence == target.release_sequence);
        let apply = !blocked && (force || !is_current);
        let groups = match manifest.ip_groups(runtime, current.as_deref(), apply).await {
            Ok(groups) => groups,
            Err(error) => {
                tracing::warn!(%error, "WAF IP group reconciliation failed");
                return true;
            }
        };
        if let Err(error) = runtime.state.apply_ip_groups(&runtime.state_dir, &groups) {
            tracing::warn!(%error, "WAF IP group update was not persisted");
        }
        if !apply {
            if is_current && !blocked {
                if let Err(error) = runtime.refresh_pages().await {
                    tracing::warn!(%error, "active Pages packages could not be refreshed");
                }
            }
            if let Some(active) = current.as_deref() {
                if let Err(error) = runtime.fetch_missing_keys(active).await {
                    tracing::warn!(%error, "active TLS private keys could not be hydrated");
                }
            }
            return false;
        }

        match runtime
            .apply_site_snapshot(&target, resource, &groups)
            .await
        {
            Ok(()) => {
                self.failed = None;
                runtime
                    .report_result(node_id, &target, "success", "configuration applied")
                    .await;
            }
            Err(error) if retryable_transfer(&error) => {
                tracing::warn!(version = %target.version, %error, "site snapshot transfer will retry");
                return true;
            }
            Err(error) => {
                let message = error.to_string();
                tracing::warn!(version = %target.version, %error, "site snapshot application failed");
                runtime
                    .report_result(node_id, &target, "failed", &message)
                    .await;
                self.failed = Some((target.release_sequence, message));
            }
        }
        false
    }
}

fn retryable_transfer(error: &anyhow::Error) -> bool {
    error.downcast_ref::<reqwest::Error>().is_some_and(|error| {
        error.status().is_none_or(|status| {
            status.is_server_error()
                || status == reqwest::StatusCode::CONFLICT
                || status == reqwest::StatusCode::TOO_MANY_REQUESTS
        })
    })
}

impl Runtime {
    pub(super) async fn apply_site_snapshot(
        &self,
        target: &ActiveConfigMeta,
        resource: &Resource,
        ip_groups: &[crate::protocol::config::WafIPGroup],
    ) -> Result<()> {
        anyhow::ensure!(
            target.engine == "pingora" && target.payload_checksum == resource.sha256,
            "invalid site snapshot metadata"
        );
        let body = resource.body(self, SITE_KEY).await?;
        let mut candidate = config::decode(
            &target.version,
            &target.checksum,
            target.release_sequence,
            &target.payload_checksum,
            &body,
        )?;
        candidate.merge_ip_groups(ip_groups);
        self.prepare_pages(&mut candidate).await?;
        self.fetch_missing_keys(&candidate).await?;
        self.state.commit(&self.state_dir, candidate)?;
        if let Some(active) = self.state.current() {
            self.keys.retain_for(&active);
        }
        Ok(())
    }
}
