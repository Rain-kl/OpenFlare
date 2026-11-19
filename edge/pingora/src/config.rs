use crate::policy::PoWPageConfig;
use anyhow::{bail, Context, Result};
use pingora::tls::pkey::{PKey, Private};
use pingora::tls::x509::X509;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::path::Path;

const CONTRACT_VERSION: i64 = 2;
const BASE_CAPABILITY: &str = "tls-sni-h2-v1";
const POLICY_CAPABILITY: &str = "policy-runtime-v1";

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Config {
    pub schema_version: i64,
    pub contract_version: i64,
    pub required_capabilities: Vec<String>,
    pub routes: Vec<Route>,
    #[serde(default)]
    pub waf: WafDocument,
    pub assets: Assets,
    #[serde(default)]
    pub pow_page: PoWPageConfig,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct WafDocument {
    #[serde(default)]
    pub rule_groups: Vec<WafRuleGroup>,
    #[serde(default)]
    pub ip_groups: Vec<WafIPGroup>,
    #[serde(default)]
    pub bindings: Vec<WafBinding>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct WafRuleGroup {
    pub id: u64,
    #[serde(default)]
    pub name: String,
    pub enabled: bool,
    pub is_global: bool,
    pub graph: WafGraph,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct WafGraph {
    #[serde(default)]
    pub entry: String,
    #[serde(default)]
    pub nodes: HashMap<String, WafNode>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct WafNode {
    #[serde(rename = "type", default)]
    pub node_type: String,
    #[serde(default)]
    pub config: Value,
    #[serde(default)]
    pub next: HashMap<String, String>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct WafIPGroup {
    pub id: u64,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub ip_list: Vec<String>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct WafBinding {
    #[serde(default)]
    pub route_id: u64,
    #[serde(default)]
    pub site_name: String,
    #[serde(default)]
    pub rule_group_ids: Vec<u64>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct Route {
    pub id: u64,
    pub site_name: String,
    pub domains: Vec<String>,
    pub domain_cert_ids: Vec<u64>,
    pub origin_url: String,
    #[serde(default)]
    pub origin_host: String,
    pub upstreams: Vec<String>,
    pub enabled: bool,
    pub enable_https: bool,
    pub enable_http2: bool,
    pub redirect_http: bool,
    #[serde(default)]
    pub limit: LimitConfig,
    #[serde(default)]
    pub pow_enabled: bool,
    #[serde(default)]
    pub pow: Option<PoWConfig>,
    #[serde(default)]
    pub basic_auth_enabled: bool,
    #[serde(default)]
    pub basic_auth_username: String,
    #[serde(default)]
    pub basic_auth_password: String,
    #[serde(default)]
    pub site_auth_ttl_hours: i64,
    #[serde(default)]
    pub site_auth_key: String,
    #[serde(default)]
    pub upstream_type: String,
    #[serde(default)]
    pub pages: Option<PagesDeployment>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct LimitConfig {
    #[serde(default)]
    pub conn_per_server: i64,
    #[serde(default)]
    pub conn_per_ip: i64,
    #[serde(default)]
    pub rate: String,
    #[serde(default)]
    pub req_per_ip: String,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct PoWConfig {
    #[serde(default = "default_pow_difficulty")]
    pub difficulty: u8,
    #[serde(default = "default_pow_algorithm")]
    pub algorithm: String,
    #[serde(default = "default_pow_session_ttl")]
    pub session_ttl: u64,
    #[serde(default = "default_pow_challenge_ttl")]
    pub challenge_ttl: u64,
    #[serde(default)]
    pub whitelist: PoWListConfig,
    #[serde(default)]
    pub blacklist: PoWListConfig,
    #[serde(default)]
    pub path_rules: Option<PoWPathRules>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct PoWListConfig {
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub ips: Vec<String>,
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub ip_cidrs: Vec<String>,
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub paths: Vec<String>,
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub path_regexes: Vec<String>,
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub user_agents: Vec<String>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct PoWPathRules {
    #[serde(default)]
    pub mode: String,
    #[serde(default)]
    pub whitelist: Vec<PoWPathMatch>,
    #[serde(default)]
    pub blacklist: Vec<PoWPathMatch>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct PoWPathMatch {
    #[serde(default)]
    pub match_type: String,
    #[serde(rename = "type", default)]
    pub match_type_alias: String,
    #[serde(default)]
    pub value: String,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct PagesDeployment {
    #[serde(default)]
    pub project_id: u64,
    #[serde(default)]
    pub project_slug: String,
    #[serde(default)]
    pub deployment_id: u64,
    #[serde(default)]
    pub deployment_number: i64,
    #[serde(default)]
    pub checksum: String,
    #[serde(default)]
    pub entry_file: String,
    #[serde(default)]
    pub spa_fallback_enabled: bool,
    #[serde(default)]
    pub spa_fallback_path: String,
    #[serde(default)]
    pub api_proxy_enabled: bool,
    #[serde(default)]
    pub api_proxy_path: String,
    #[serde(default)]
    pub api_proxy_pass: String,
    #[serde(default)]
    pub api_proxy_rewrite: String,
    #[serde(default)]
    pub local_root: String,
}

fn default_pow_difficulty() -> u8 {
    4
}
fn default_pow_algorithm() -> String {
    "fast".to_owned()
}
fn default_pow_session_ttl() -> u64 {
    600
}
fn default_pow_challenge_ttl() -> u64 {
    300
}

fn deserialize_null_default_vec<'de, D, T>(deserializer: D) -> Result<Vec<T>, D::Error>
where
    D: serde::Deserializer<'de>,
    T: serde::Deserialize<'de>,
{
    let value = Option::<Option<Vec<T>>>::deserialize(deserializer)?;
    Ok(value.flatten().unwrap_or_default())
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct Assets {
    #[serde(default)]
    pub certificates: HashMap<String, CertificateAsset>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct CertificateAsset {
    pub cert_pem: String,
    pub key_pem: String,
}

#[derive(Debug, Clone)]
pub struct LoadedCertificate {
    pub cert: X509,
    pub key: PKey<Private>,
}

#[derive(Debug, Clone)]
pub struct AppliedConfig {
    pub version: String,
    pub checksum: String,
    pub release_sequence: i64,
    pub payload_checksum: String,
    pub payload_json: String,
    pub config: Config,
    pub certificates: HashMap<u64, LoadedCertificate>,
}

impl AppliedConfig {
    pub fn routes(&self) -> impl Iterator<Item = &Route> {
        self.config.routes.iter().filter(|route| route.enabled)
    }

    pub fn pow_page(&self) -> &crate::policy::PoWPageConfig {
        &self.config.pow_page
    }

    pub fn certificate_for_host(&self, host: &str) -> Option<&LoadedCertificate> {
        let route = self.route_for_host(host)?;
        let cert_id = route.certificate_id_for_host(host)?;
        self.certificates.get(&cert_id)
    }

    pub fn route_for_host(&self, host: &str) -> Option<&Route> {
        find_route(self.routes(), host)
    }
}

pub fn decode(
    version: &str,
    checksum: &str,
    release_sequence: i64,
    payload_checksum: &str,
    payload_json: &str,
) -> Result<AppliedConfig> {
    if version.trim().is_empty()
        || checksum.trim().is_empty()
        || payload_checksum.trim().is_empty()
        || payload_json.trim().is_empty()
    {
        bail!("active configuration is incomplete");
    }
    if release_sequence <= 0 {
        bail!("active configuration has an invalid release sequence");
    }
    let digest = format!("{:x}", Sha256::digest(payload_json.as_bytes()));
    if !constant_time_equal(digest.as_bytes(), payload_checksum.trim().as_bytes()) {
        bail!("active configuration payload checksum mismatch");
    }
    let config: Config = serde_json::from_str(payload_json).context(format!(
        "failed to decode Pingora payload ({} bytes)",
        payload_json.len()
    ))?;
    validate(&config)?;
    let certificates = load_certificates(&config)?;
    Ok(AppliedConfig {
        version: version.to_owned(),
        checksum: checksum.to_owned(),
        release_sequence,
        payload_checksum: digest,
        payload_json: payload_json.to_owned(),
        config,
        certificates,
    })
}

fn validate(config: &Config) -> Result<()> {
    if config.schema_version != 1 || config.contract_version != CONTRACT_VERSION {
        bail!(
            "unsupported Pingora contract: schema {} / contract {}",
            config.schema_version,
            config.contract_version
        );
    }
    if !config
        .required_capabilities
        .iter()
        .any(|capability| capability == BASE_CAPABILITY)
    {
        bail!("configuration does not declare required capability {BASE_CAPABILITY}");
    }
    if !config
        .required_capabilities
        .iter()
        .any(|capability| capability == POLICY_CAPABILITY)
    {
        bail!("configuration does not declare required capability {POLICY_CAPABILITY}");
    }
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
        if route.domain_cert_ids.len() != route.domains.len() {
            bail!(
                "enabled route {} has inconsistent domain certificate references",
                route.display_name()
            );
        }
        for cert_id in route.domain_cert_ids.iter().filter(|id| **id != 0) {
            if !config
                .assets
                .certificates
                .contains_key(&cert_id.to_string())
            {
                bail!(
                    "certificate {cert_id} required by route {} is missing",
                    route.display_name()
                );
            }
        }
    }
    Ok(())
}

fn load_certificates(config: &Config) -> Result<HashMap<u64, LoadedCertificate>> {
    let mut certificates = HashMap::with_capacity(config.assets.certificates.len());
    for (id, asset) in &config.assets.certificates {
        let parsed_id = id
            .parse::<u64>()
            .with_context(|| format!("certificate asset has invalid identifier {id}"))?;
        let cert = X509::from_pem(asset.cert_pem.as_bytes())
            .with_context(|| format!("failed to decode certificate {id}"))?;
        let key = PKey::private_key_from_pem(asset.key_pem.as_bytes())
            .with_context(|| format!("failed to decode private key {id}"))?;
        certificates.insert(parsed_id, LoadedCertificate { cert, key });
    }
    Ok(certificates)
}

pub fn find_route<'a>(
    mut routes: impl Iterator<Item = &'a Route>,
    host: &str,
) -> Option<&'a Route> {
    let normalized_host = normalize_host(host);
    routes.find(|route| {
        route
            .domains
            .iter()
            .any(|domain| domain_matches(domain, &normalized_host))
    })
}

fn domain_matches(domain: &str, host: &str) -> bool {
    let domain = normalize_host(domain);
    if domain == host {
        return true;
    }
    let Some(prefix) = domain.strip_prefix("*.") else {
        return false;
    };
    let Some((first, rest)) = host.split_once('.') else {
        return false;
    };
    !first.is_empty() && rest == prefix
}

fn normalize_host(value: &str) -> String {
    value.trim().trim_end_matches('.').to_ascii_lowercase()
}

fn constant_time_equal(left: &[u8], right: &[u8]) -> bool {
    if left.len() != right.len() {
        return false;
    }
    left.iter()
        .zip(right)
        .fold(0u8, |difference, (left, right)| difference | (left ^ right))
        == 0
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

    pub fn certificate_id_for_host(&self, host: &str) -> Option<u64> {
        let normalized_host = normalize_host(host);
        self.domains
            .iter()
            .position(|domain| domain_matches(domain, &normalized_host))
            .and_then(|index| self.domain_cert_ids.get(index).copied())
            .filter(|id| *id != 0)
    }

    pub fn is_pages(&self) -> bool {
        self.upstream_type == "pages" && self.pages.is_some()
    }
}

pub fn persist_snapshot(state_dir: &Path, config: &AppliedConfig) -> Result<()> {
    std::fs::create_dir_all(state_dir)?;
    let temporary = state_dir.join("snapshot.json.tmp");
    let final_path = state_dir.join("snapshot.json");
    let document = serde_json::json!({
        "version": config.version,
        "checksum": config.checksum,
        "release_sequence": config.release_sequence,
        "payload_checksum": config.payload_checksum,
        "payload_json": config.payload_json,
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
    let document = std::fs::read_to_string(&path)?;
    restore_snapshot_from_str(&document)
}

pub fn restore_snapshot_from_str(document: &str) -> Result<Option<AppliedConfig>> {
    let document: Value = serde_json::from_str(document)?;
    let version = document
        .get("version")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let checksum = document
        .get("checksum")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let release_sequence = document
        .get("release_sequence")
        .and_then(Value::as_i64)
        .unwrap_or_default();
    let payload_checksum = document
        .get("payload_checksum")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let payload_json = document
        .get("payload_json")
        .and_then(Value::as_str)
        .ok_or_else(|| anyhow::anyhow!("snapshot is missing payload_json"))?;
    Ok(Some(decode(
        version,
        checksum,
        release_sequence,
        payload_checksum,
        payload_json,
    )?))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn certificate_pem() -> (&'static str, &'static str) {
        (
            "-----BEGIN CERTIFICATE-----\ninvalid\n-----END CERTIFICATE-----\n",
            "invalid",
        )
    }

    #[test]
    fn checks_payload_digest_before_decoding() {
        let payload = r#"{
            "schema_version":1,
            "contract_version":2,
            "required_capabilities":["tls-sni-h2-v1","policy-runtime-v1"],
            "routes":[{"id":1,"site_name":"example","domains":["example.com"],"domain_cert_ids":[0],"origin_url":"http://127.0.0.1:9080","upstreams":[],"enabled":true,"enable_https":true,"enable_http2":true,"redirect_http":true}],
            "assets":{"certificates":{}}
        }"#;
        let digest = format!("{:x}", Sha256::digest(payload.as_bytes()));
        assert!(decode("v1", "checksum", 1, &digest, payload).is_ok());
        assert!(decode("v1", "checksum", 1, "bad", payload).is_err());
    }

    #[test]
    fn snapshot_round_trips_original_payload_bytes() {
        let payload = r#"{"schema_version":1,"contract_version":2,"required_capabilities":["tls-sni-h2-v1","policy-runtime-v1"],"routes":[{"id":1,"site_name":"example","domains":["example.com"],"domain_cert_ids":[0],"origin_url":"http://127.0.0.1:9080","upstreams":[],"enabled":true,"enable_https":true,"enable_http2":true,"redirect_http":true}],"assets":{"certificates":{}}}"#;
        let digest = format!("{:x}", Sha256::digest(payload.as_bytes()));
        let applied = decode("v1", "checksum", 7, &digest, payload).unwrap();
        let state_dir = tempfile::tempdir().unwrap();
        persist_snapshot(state_dir.path(), &applied).unwrap();
        let restored = restore_snapshot(state_dir.path()).unwrap().unwrap();

        assert_eq!(restored.version, applied.version);
        assert_eq!(restored.checksum, applied.checksum);
        assert_eq!(restored.release_sequence, applied.release_sequence);
        assert_eq!(restored.payload_checksum, applied.payload_checksum);
        assert_eq!(restored.payload_json, payload);
    }

    #[test]
    fn matches_exact_and_single_level_wildcard_domains() {
        let routes = [
            Route {
                id: 1,
                site_name: "wildcard".into(),
                domains: vec!["*.example.com".into()],
                domain_cert_ids: vec![10],
                origin_url: "http://127.0.0.1:9080".into(),
                origin_host: String::new(),
                upstreams: vec![],
                enabled: true,
                enable_https: true,
                enable_http2: true,
                redirect_http: true,
                ..Route::default()
            },
            Route {
                id: 2,
                site_name: "exact".into(),
                domains: vec!["api.example.com".into()],
                domain_cert_ids: vec![20],
                origin_url: "http://127.0.0.1:9081".into(),
                origin_host: String::new(),
                upstreams: vec![],
                enabled: true,
                enable_https: true,
                enable_http2: true,
                redirect_http: true,
                ..Route::default()
            },
        ];
        let exact_first = [routes[1].clone(), routes[0].clone()];
        let wildcard_first = [routes[0].clone(), routes[1].clone()];
        assert_eq!(
            find_route(exact_first.iter(), "api.example.com").map(|r| r.id),
            Some(2)
        );
        assert_eq!(
            find_route(wildcard_first.iter(), "a.example.com").map(|r| r.id),
            Some(1)
        );
        assert!(find_route(wildcard_first.iter(), "a.b.example.com").is_none());
        assert_eq!(routes[0].certificate_id_for_host("a.example.com"), Some(10));
        assert_eq!(
            routes[1].certificate_id_for_host("api.example.com"),
            Some(20)
        );
    }

    #[test]
    fn rejects_missing_certificate_asset() {
        let (cert, key) = certificate_pem();
        let payload = format!(
            r#"{{"schema_version":1,"contract_version":1,"required_capabilities":["tls-sni-h2-v1"],"routes":[{{"id":1,"site_name":"example","domains":["example.com"],"domain_cert_ids":[10],"origin_url":"http://127.0.0.1:9080","upstreams":[],"enabled":true,"enable_https":true,"enable_http2":true,"redirect_http":true}}],"assets":{{"certificates":{{"10":{{"cert_pem":{:?},"key_pem":{:?}}}}}}}}}"#,
            cert, key
        );
        let digest = format!("{:x}", Sha256::digest(payload.as_bytes()));
        assert!(decode("v1", "checksum", 1, &digest, &payload).is_err());
    }
}
