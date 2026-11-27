use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Deserialize)]
pub struct ApiEnvelope<T> {
    #[serde(default)]
    #[allow(dead_code)]
    pub error_msg: String,
    pub data: Option<T>,
}

#[derive(Debug, Clone, Serialize)]
pub struct RegisterRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub instance_id: Option<String>,
    pub name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub public_addr: String,
    pub listen_addr: String,
    pub version: String,
}

#[derive(Debug, Clone, Deserialize)]
pub struct RegisterResponse {
    pub id: u64,
    pub instance_id: String,
    #[allow(dead_code)]
    pub name: String,
    pub access_token: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct HeartbeatRequest {
    pub version: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub public_addr: String,
    pub listen_addr: String,
    pub snapshot_version: u64,
    pub snapshot_etag: String,
    pub qps: f64,
    pub p50_ms: f64,
    pub p95_ms: f64,
    pub p99_ms: f64,
    pub queries_total: u64,
    pub queries_success: u64,
    pub queries_nxdomain: u64,
    pub queries_servfail: u64,
    pub queries_ratelimited: u64,
    pub cache_hit_ratio: f64,
    pub cpu_usage_percent: f64,
    pub memory_usage_percent: f64,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub last_error: String,
}

#[derive(Debug, Clone, Deserialize)]
pub struct HeartbeatResponse {
    #[allow(dead_code)]
    pub ok: bool,
    pub snapshot_version: u64,
    #[serde(default)]
    pub snapshot_etag: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotEnvelope {
    pub version: u64,
    #[serde(default)]
    pub etag: String,
    #[serde(default)]
    pub generated_at: String,
    #[serde(default)]
    pub delegation_domain: String,
    #[serde(default)]
    pub ns_hosts: Vec<String>,
    #[serde(default = "default_ttl")]
    pub default_ttl: i32,
    #[serde(default)]
    pub nodes: Vec<SnapshotNode>,
    #[serde(default)]
    pub policies: Vec<SnapshotPolicy>,
    #[serde(default)]
    pub domain_routes: Vec<SnapshotDomainRoute>,
    #[serde(default)]
    pub quality: Option<CompactQualitySnapshot>,
}

fn default_ttl() -> i32 {
    60
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotTag {
    pub slug: String,
    #[serde(default)]
    pub name: String,
    #[serde(default = "default_weight")]
    pub effective_weight: f64,
}

fn default_weight() -> f64 {
    1.0
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotNode {
    pub node_id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub ipv4: String,
    #[serde(default)]
    pub ipv6: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub quarantined: bool,
    #[serde(default)]
    pub network_group: String,
    #[serde(default)]
    pub asn: u32,
    #[serde(default)]
    pub country: String,
    #[serde(default)]
    pub geo_latitude: Option<f64>,
    #[serde(default)]
    pub geo_longitude: Option<f64>,
    #[serde(default)]
    pub capacity_bandwidth_mbps: i64,
    #[serde(default)]
    pub capacity_connections: i64,
    #[serde(default = "default_cap_weight")]
    pub capacity_weight: i32,
    #[serde(default)]
    pub cpu_usage_percent: f64,
    #[serde(default)]
    pub memory_usage_percent: f64,
    #[serde(default)]
    pub bandwidth_mbps: f64,
    #[serde(default)]
    pub active_connections: i64,
    #[serde(default)]
    pub tags: Vec<SnapshotTag>,
}

fn default_cap_weight() -> i32 {
    100
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotPolicy {
    pub id: u64,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub scope_type: String,
    #[serde(default)]
    pub scope_domains: Vec<String>,
    #[serde(default)]
    pub strategy_type: String,
    #[serde(default)]
    pub priority: i32,
    #[serde(default = "default_ttl")]
    pub ttl: i32,
    #[serde(default = "default_max_answers")]
    pub max_answers: i32,
    #[serde(default)]
    pub is_default: bool,
    #[serde(default)]
    pub fallback_strategy: String,
    #[serde(default)]
    pub fallback_node_ids: Vec<String>,
    #[serde(default)]
    pub jdm_json: String,
    #[serde(default)]
    pub revision: i32,
}

fn default_max_answers() -> i32 {
    1
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotDomainRoute {
    pub target_host: String,
    pub policy_id: u64,
    #[serde(default)]
    pub group_id: u64,
    #[serde(default)]
    pub member_domains: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct CompactQualitySnapshot {
    #[serde(default)]
    pub schema_version: i32,
    #[serde(default)]
    pub generation: u64,
    #[serde(default)]
    pub generated_at: String,
    #[serde(default)]
    pub provider_name: String,
    #[serde(default)]
    pub entries: Vec<CompactQualityEntry>,
    #[serde(default)]
    pub node_defaults: HashMap<String, CompactNodePrior>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompactQualityEntry {
    #[serde(rename = "n")]
    pub node_id: String,
    #[serde(rename = "a")]
    pub client_asn: u32,
    #[serde(rename = "c")]
    pub client_country: String,
    #[serde(rename = "r", default)]
    pub client_region: String,
    #[serde(rename = "f")]
    pub ip_family: String,
    #[serde(rename = "l")]
    pub level: String,
    #[serde(rename = "rtt")]
    pub effective_rtt_ms: f64,
    #[serde(rename = "p95", default)]
    pub rtt_p95_ms: f64,
    #[serde(rename = "disp", default)]
    pub dispersion_ms: f64,
    #[serde(rename = "loss", default)]
    pub loss_rate: f64,
    #[serde(rename = "conf")]
    pub confidence: f64,
    #[serde(rename = "geo_rtt", default)]
    pub geo_base_rtt_ms: f64,
    #[serde(rename = "geo_ok", default)]
    pub geo_base_known: bool,
    #[serde(rename = "tier")]
    pub source_tier: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct CompactNodePrior {
    #[serde(default)]
    pub network_group: String,
    #[serde(default)]
    pub asn: u32,
    #[serde(default)]
    pub lat: Option<f64>,
    #[serde(default)]
    pub lon: Option<f64>,
    #[serde(default)]
    pub median_extra: f64,
}

#[derive(Debug, Clone, Deserialize)]
pub struct WsWakeMessage {
    #[serde(rename = "type")]
    pub msg_type: String,
    #[serde(default)]
    pub payload: Option<serde_json::Value>,
}
