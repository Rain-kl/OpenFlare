use anyhow::{bail, Context, Result};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::path::Path;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Config {
    pub routes: Vec<Route>,
    #[serde(default)]
    pub openresty_config: Value,
    #[serde(default)]
    pub waf: Value,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Route {
    #[serde(default)]
    pub id: u64,
    #[serde(default)]
    pub site_name: String,
    #[serde(default)]
    pub domains: Vec<String>,
    #[serde(default)]
    pub origin_url: String,
    #[serde(default)]
    pub upstreams: Vec<String>,
    #[serde(default = "default_true")]
    pub enabled: bool,
}

fn default_true() -> bool {
    true
}

#[derive(Debug, Clone)]
pub struct AppliedConfig {
    pub version: String,
    pub checksum: String,
    pub config: Config,
    pub payload_checksum: String,
}

impl AppliedConfig {
    pub fn routes(&self) -> impl Iterator<Item = &Route> {
        self.config.routes.iter().filter(|route| route.enabled)
    }
}

pub fn decode(version: &str, checksum: &str, source_config_json: &str) -> Result<AppliedConfig> {
    if version.trim().is_empty()
        || checksum.trim().is_empty()
        || source_config_json.trim().is_empty()
    {
        bail!("active configuration is incomplete");
    }
    let config: Config =
        serde_json::from_str(source_config_json).context("failed to decode source_config_json")?;
    validate(&config)?;
    let payload_checksum = format!("{:x}", Sha256::digest(source_config_json.as_bytes()));
    Ok(AppliedConfig {
        version: version.to_owned(),
        checksum: checksum.to_owned(),
        config,
        payload_checksum,
    })
}

fn validate(config: &Config) -> Result<()> {
    if config.routes.is_empty() {
        bail!("configuration has no routes");
    }
    for route in &config.routes {
        if !route.enabled {
            continue;
        }
        if route.domains.is_empty() {
            bail!("enabled route {} has no domains", route.display_name());
        }
        if route.upstreams.is_empty() && route.origin_url.is_empty() {
            bail!("enabled route {} has no upstream", route.display_name());
        }
        let upstream_count = route.upstreams.len();
        let expected = if upstream_count == 0 {
            1
        } else {
            upstream_count
        };
        let total = route
            .upstreams
            .iter()
            .chain(std::iter::once(&route.origin_url))
            .count();
        if total != expected {
            bail!(
                "enabled route {} has inconsistent upstream fields",
                route.display_name()
            );
        }
    }
    Ok(())
}

impl Route {
    pub fn display_name(&self) -> &str {
        if self.site_name.is_empty() {
            "unnamed"
        } else {
            &self.site_name
        }
    }

    pub fn upstreams(&self) -> Vec<String> {
        if self.upstreams.is_empty() {
            vec![self.origin_url.clone()]
        } else {
            self.upstreams.clone()
        }
    }
}

pub fn persist_snapshot(state_dir: &Path, config: &AppliedConfig) -> Result<()> {
    std::fs::create_dir_all(state_dir)?;
    let temporary = state_dir.join("snapshot.json.tmp");
    let final_path = state_dir.join("snapshot.json");
    let document = serde_json::json!({
        "version": config.version,
        "checksum": config.checksum,
        "payload_checksum": config.payload_checksum,
        "source_config_json": config.config,
    });
    std::fs::write(&temporary, serde_json::to_vec_pretty(&document)?)?;
    std::fs::rename(&temporary, &final_path)?;
    Ok(())
}

pub fn restore_snapshot(state_dir: &Path) -> Result<Option<AppliedConfig>> {
    let path = state_dir.join("snapshot.json");
    if !path.exists() {
        return Ok(None);
    }
    let document: Value = serde_json::from_slice(&std::fs::read(&path)?)?;
    let version = document
        .get("version")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let checksum = document
        .get("checksum")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let payload = document
        .get("source_config_json")
        .ok_or_else(|| anyhow::anyhow!("snapshot is missing source_config_json"))?;
    let encoded = serde_json::to_string(payload)?;
    Ok(Some(decode(version, checksum, &encoded)?))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn decodes_enabled_routes_and_ignores_openresty_tuning() {
        let source = r#"{
            "routes": [{"site_name":"example","domains":["example.com"],"origin_url":"http://127.0.0.1:9080","enabled":true}],
            "openresty_config": {"worker_processes":"auto"},
            "waf": {"rule_groups":[]}
        }"#;
        let config = decode("v1", "checksum", source).unwrap();
        assert_eq!(config.routes().count(), 1);
        assert_eq!(
            config.routes().next().unwrap().upstreams(),
            vec!["http://127.0.0.1:9080"]
        );
    }

    #[test]
    fn rejects_enabled_route_without_upstream() {
        let source =
            r#"{"routes":[{"site_name":"example","domains":["example.com"],"enabled":true}]}"#;
        assert!(decode("v1", "checksum", source).is_err());
    }
}
