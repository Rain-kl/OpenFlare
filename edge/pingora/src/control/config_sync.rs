use super::Runtime;
use crate::{config::AppliedConfig, protocol::config::WafIPGroup};
use anyhow::{anyhow, ensure, Context, Result};
use serde::{de::DeserializeOwned, Deserialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

pub(crate) const VERSION: u8 = 1;
pub(super) const SITE_KEY: &str = "sites.v1";
pub(super) const PERFORMANCE_KEY: &str = "performance.v1";
const IP_GROUP_PREFIX: &str = "waf.ip-group.v1/";

#[derive(Debug, Deserialize)]
pub(super) struct Manifest {
    pub version: u8,
    pub resources: BTreeMap<String, Resource>,
}

#[derive(Debug, Deserialize)]
pub(super) struct Resource {
    pub sha256: String,
    pub metadata: Option<serde_json::Value>,
    #[serde(default)]
    pub inline_json: String,
    #[serde(default)]
    pub revision: String,
}

impl Resource {
    pub async fn body(&self, runtime: &Runtime, key: &str) -> Result<String> {
        let contents = if self.inline_json.is_empty() {
            runtime
                .client
                .get(format!("{}/api/v1/agent/config/pull", runtime.base_url))
                .header("X-Agent-Token", &runtime.token)
                .query(&[("key", key), ("sha256", self.sha256.as_str())])
                .send()
                .await?
                .error_for_status()?
                .text()
                .await?
        } else {
            self.inline_json.clone()
        };
        let received = format!("{:x}", Sha256::digest(contents.as_bytes()));
        ensure!(
            received == self.sha256,
            "configuration resource checksum mismatch for {key}"
        );
        Ok(contents)
    }

    pub async fn read<T: DeserializeOwned>(&self, runtime: &Runtime, key: &str) -> Result<T> {
        let contents = self.body(runtime, key).await?;
        serde_json::from_str(&contents)
            .with_context(|| format!("decode configuration resource {key}"))
    }
}

impl Manifest {
    pub async fn ip_groups(
        &self,
        runtime: &Runtime,
        current: Option<&AppliedConfig>,
        include_retained: bool,
    ) -> Result<Vec<WafIPGroup>> {
        let mut groups = Vec::new();
        for (key, resource) in &self.resources {
            let Some(id) = key.strip_prefix(IP_GROUP_PREFIX) else {
                continue;
            };
            let id = id
                .parse::<u64>()
                .with_context(|| format!("invalid IP group key {key}"))?;
            let retained = current.and_then(|snapshot| {
                snapshot
                    .config
                    .waf
                    .ip_groups
                    .iter()
                    .find(|group| group.id == id && group.checksum == resource.revision)
            });
            if let Some(group) = retained {
                if include_retained {
                    groups.push(group.clone());
                }
                continue;
            }
            let group = resource.read::<WafIPGroup>(runtime, key).await?;
            if group.id != id || group.checksum != resource.revision {
                return Err(anyhow!("invalid IP group resource {key}"));
            }
            groups.push(group);
        }
        Ok(groups)
    }
}
