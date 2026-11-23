use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PoWPageConfig {
    #[serde(default)]
    pub template: String,
    #[serde(default)]
    pub widget: String,
    #[serde(default)]
    pub html: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct CustomHeader {
    pub key: String,
    pub value: String,
}

#[derive(Debug, Clone)]
pub struct Config {
    pub schema_version: i64,
    pub contract_version: i64,
    pub required_capabilities: Vec<String>,
    pub routes: Vec<Route>,
    pub waf: WafDocument,
    pub assets: Assets,
    pub pow_page: PoWPageConfig,
    pub site_auth_html: String,
    pub trusted_proxy_cidrs: Vec<String>,
    pub max_request_body: u64,
    pub default_server_return_status: u16,
    pub origin_error_page: Option<OriginErrorPage>,
    pub service_worker: Option<ServiceWorker>,
    pub gzip: bool,
    pub gzip_min_length: u64,
    pub websocket: bool,
    pub cache: Option<CachePolicy>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct WafDocument {
    #[serde(default)]
    pub rule_groups: Vec<WafRuleGroup>,
    #[serde(default)]
    pub ip_groups: Vec<WafIPGroup>,
    #[serde(default)]
    pub bindings: Vec<WafBinding>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub request_whitelist: Vec<WafWhitelistEntry>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct WafWhitelistEntry {
    pub domain: String,
    pub path_match: WafPathMatch,
    pub path: String,
}

#[derive(Debug, Clone, Copy, Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum WafPathMatch {
    Prefix,
    Suffix,
    Regex,
    Exact,
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
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub ip_list: Vec<String>,
    #[serde(default)]
    pub checksum: String,
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
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub domain_cert_ids: Vec<u64>,
    pub origin_url: String,
    #[serde(default)]
    pub origin_host: String,
    #[serde(default)]
    pub custom_headers: Vec<CustomHeader>,
    #[serde(skip)]
    pub cache: Option<RouteCache>,
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub upstreams: Vec<String>,
    pub enabled: bool,
    pub enable_https: bool,
    pub enable_http2: bool,
    pub redirect_http: bool,
    #[serde(skip)]
    pub limit: LimitConfig,
    #[serde(default)]
    pub pow_enabled: bool,
    #[serde(default, rename = "pow_config")]
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
    pub tunnel_id: Option<u64>,
    #[serde(default, rename = "pages_deployment")]
    pub pages: Option<PagesDeployment>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TunnelEdgeRelayEndpoint {
    #[serde(default)]
    pub relay_id: u64,
    #[serde(default)]
    pub relay_public_id: String,
    #[serde(default)]
    pub gateway_url: String,
    #[serde(default)]
    pub gateway_secret: String,
    #[serde(default)]
    pub zone_tag: String,
    #[serde(default)]
    pub colocated_edge_node_id: String,
    #[serde(default)]
    pub is_colocated: bool,
    #[serde(default)]
    pub weight: i32,
    #[serde(default)]
    pub best_rtt_ms: i32,
    #[serde(default)]
    pub active_connectors: i32,
    #[serde(default)]
    pub active_streams: i32,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TunnelEdgeRoutingEntry {
    #[serde(default)]
    pub tunnel_id: u64,
    #[serde(default)]
    pub tunnel_public_id: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub quarantined: bool,
    #[serde(default)]
    pub preferred_transport: String,
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub relays: Vec<TunnelEdgeRelayEndpoint>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TunnelRoutingSnapshot {
    #[serde(default)]
    pub schema_version: u32,
    #[serde(default)]
    pub generated_at_unix: i64,
    #[serde(default, deserialize_with = "deserialize_null_default_vec")]
    pub tunnels: Vec<TunnelEdgeRoutingEntry>,
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

#[derive(Debug, Clone, Default, Deserialize, Serialize, PartialEq, Eq)]
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

#[derive(Debug, Clone, Default, Deserialize, Serialize, PartialEq, Eq)]
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

#[derive(Debug, Clone, Default, Deserialize, Serialize, PartialEq, Eq)]
pub struct PoWPathRules {
    #[serde(default)]
    pub mode: String,
    #[serde(default)]
    pub whitelist: Vec<PoWPathMatch>,
    #[serde(default)]
    pub blacklist: Vec<PoWPathMatch>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize, PartialEq, Eq)]
pub struct PoWPathMatch {
    #[serde(rename = "type")]
    pub match_type: String,
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

pub fn deserialize_null_default_vec<'de, D, T>(deserializer: D) -> Result<Vec<T>, D::Error>
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
    #[serde(default)]
    pub cert_sha256: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct OriginErrorPage {
    pub status_codes: Vec<u16>,
    pub html: String,
    pub get_only: bool,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ServiceWorker {
    pub domains: Vec<String>,
    pub script: String,
    pub offline_html: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct CachePolicy {
    pub stale_on_error: bool,
    pub stale_updating: bool,
    pub static_extensions: Vec<String>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct RouteCache {
    pub policy: String,
    pub rules: Vec<String>,
}
