use anyhow::{bail, Result};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

pub const HEADER_RELAY_TOKEN: &str = "X-Relay-Token";
pub const HEADER_RELAY_BOOTSTRAP_TOKEN: &str = "X-Relay-Bootstrap-Token";
pub const HEADER_TUNNEL_TOKEN: &str = "X-Tunnel-Token";
pub const HEADER_CONNECTOR_ID: &str = "X-Connector-ID";
pub const HEADER_OPENFLARE_TUNNEL_ID: &str = "X-OpenFlare-Tunnel-ID";
pub const HEADER_OPENFLARE_ROUTE_ID: &str = "X-OpenFlare-Route-ID";
pub const HEADER_OPENFLARE_RELAY_AUTH: &str = "X-OpenFlare-Relay-Auth";

pub const HEADER_QUARANTINED: &str = "X-OpenFlare-Quarantined";
pub const HEADER_CONNECTOR_REVOKED: &str = "X-OpenFlare-Connector-Revoked";
pub const HEADER_AUTH_ERROR: &str = "X-OpenFlare-Auth-Error";

pub const PATH_RELAY_REGISTER: &str = "/api/v1/relay/register";
pub const PATH_RELAY_HEARTBEAT: &str = "/api/v1/relay/heartbeat";
pub const PATH_RELAY_CONFIG: &str = "/api/v1/relay/config";
pub const PATH_RELAY_WS: &str = "/api/v1/relay/ws";

pub const PATH_TUNNEL_HEARTBEAT: &str = "/api/v1/tunnel/heartbeat";
pub const PATH_TUNNEL_CONFIG: &str = "/api/v1/tunnel/config";
pub const PATH_TUNNEL_ATTACHMENTS: &str = "/api/v1/tunnel/attachments";
pub const PATH_TUNNEL_WS: &str = "/api/v1/tunnel/ws";

pub const PATH_RELAY_HEALTH: &str = "/_openflare/health";
pub const PATH_CONNECTOR_HEALTH: &str = "/_openflare/connector/health";

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ApiResponse<T> {
    #[serde(default)]
    pub error_msg: String,
    pub data: Option<T>,
}

impl<T> ApiResponse<T> {
    pub fn into_result(self) -> Result<T> {
        if !self.error_msg.is_empty() {
            bail!("control plane error: {}", self.error_msg);
        }
        match self.data {
            Some(data) => Ok(data),
            None => bail!("control plane returned empty data payload"),
        }
    }
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Deserialize, Serialize)]
pub struct UpdateSettings {
    #[serde(default)]
    pub repo: String,
    #[serde(default)]
    pub auto_update: bool,
    #[serde(default)]
    pub channel: Option<String>,
    #[serde(default)]
    pub tag: Option<String>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct RelayRegisterRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub relay_id: Option<String>,
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pool_key: Option<String>,
    pub public_addr: String,
    pub transport_port: u16,
    #[serde(default)]
    pub ws_port: u16,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gateway_addr: Option<String>,
    pub gateway_port: u16,
    #[serde(default)]
    pub gateway_tls: bool,
    #[serde(default)]
    pub supported_transports: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub noise_public_key: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub geo_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub zone_tag: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub colocated_edge_node_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub weight: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub engine_version: Option<String>,
    #[serde(default)]
    pub contract_version: u32,
    #[serde(default)]
    pub capabilities: Vec<String>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct RelayRegisterResponse {
    #[serde(default)]
    pub id: u64,
    #[serde(default)]
    pub relay_id: String,
    #[serde(default)]
    pub pool_id: u64,
    #[serde(default)]
    pub access_token: String,
    #[serde(default)]
    pub public_addr: String,
    #[serde(default)]
    pub gateway_secret: String,
    #[serde(default)]
    pub noise_private_key: String,
    #[serde(default)]
    pub noise_public_key: String,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct RelayVerifiedAttachment {
    pub connector_id: String,
    pub service_name: String,
    #[serde(default)]
    pub rtt_ms: i64,
    #[serde(default)]
    pub active_streams: i64,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct RelayHeartbeatPayload {
    pub version: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub engine_version: Option<String>,
    #[serde(default)]
    pub contract_version: u32,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub public_addr: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gateway_addr: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub transport_port: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ws_port: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gateway_port: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gateway_tls: Option<bool>,
    #[serde(default)]
    pub supported_transports: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub runtime_status: Option<String>,
    #[serde(default)]
    pub draining: bool,
    #[serde(default)]
    pub active_connectors: i64,
    #[serde(default)]
    pub active_streams: i64,
    #[serde(default)]
    pub cpu_usage_percent: f64,
    #[serde(default)]
    pub memory_usage_percent: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub applied_update_tag: Option<String>,
    #[serde(default)]
    pub verified_attachments: Vec<RelayVerifiedAttachment>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct RelayHeartbeatResponse {
    #[serde(default)]
    pub relay_id: String,
    #[serde(default)]
    pub pool_id: u64,
    #[serde(default)]
    pub heartbeat_interval: u64,
    #[serde(default)]
    pub lease_ttl_seconds: i64,
    #[serde(default)]
    pub draining: bool,
    #[serde(default)]
    pub quarantined: bool,
    #[serde(default)]
    pub quarantine_reason: String,
    #[serde(default)]
    pub force_sync: bool,
    #[serde(default)]
    pub update_settings: Option<UpdateSettings>,
}

#[derive(Debug, Clone, PartialEq, Eq, Deserialize, Serialize)]
pub struct RelayServiceSpec {
    pub service_name: String,
    #[serde(default = "default_http_ingress_service_type")]
    pub service_type: String,
    #[serde(default)]
    pub tunnel_id: u64,
    #[serde(default)]
    pub tunnel_public_id: String,
    #[serde(default)]
    pub connector_id: String,
    #[serde(default)]
    pub token: String,
    #[serde(default)]
    pub transport: String,
    #[serde(default)]
    pub listen_port: u16,
    #[serde(default)]
    pub state: String,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct RelayRuntimeConfigResponse {
    #[serde(default)]
    pub id: u64,
    #[serde(default)]
    pub relay_id: String,
    #[serde(default)]
    pub pool_id: u64,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub public_addr: String,
    #[serde(default)]
    pub transport_port: u16,
    #[serde(default)]
    pub ws_port: u16,
    #[serde(default)]
    pub gateway_addr: String,
    #[serde(default)]
    pub gateway_port: u16,
    #[serde(default)]
    pub gateway_tls: bool,
    #[serde(default)]
    pub gateway_secret: String,
    #[serde(default)]
    pub supported_transports: Vec<String>,
    #[serde(default)]
    pub noise_private_key: String,
    #[serde(default)]
    pub noise_public_key: String,
    #[serde(default)]
    pub draining: bool,
    #[serde(default)]
    pub lease_ttl_seconds: i64,
    #[serde(default)]
    pub updated_at_unix: i64,
    #[serde(default)]
    pub services: Vec<RelayServiceSpec>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Deserialize, Serialize)]
pub struct TunnelConnectedRelayReport {
    pub relay_id: u64,
    #[serde(default)]
    pub relay_public_id: String,
    pub service_name: String,
    #[serde(default)]
    pub transport: String,
    #[serde(default)]
    pub state: String,
    #[serde(default)]
    pub rtt_ms: i64,
    #[serde(default)]
    pub active_streams: i64,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TunnelConnectorHeartbeatPayload {
    pub connector_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hostname: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub os_arch: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub public_ip: Option<String>,
    pub version: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub engine_version: Option<String>,
    #[serde(default)]
    pub contract_version: u32,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(default)]
    pub applied_revision: i64,
    #[serde(default)]
    pub draining: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub applied_update_tag: Option<String>,
    #[serde(default)]
    pub connected_relays: Vec<TunnelConnectedRelayReport>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TunnelConnectorHeartbeatResponse {
    #[serde(default)]
    pub tunnel_id: u64,
    #[serde(default)]
    pub tunnel_public_id: String,
    #[serde(default)]
    pub connector_id: String,
    #[serde(default)]
    pub config_revision: i64,
    #[serde(default)]
    pub heartbeat_interval: u64,
    #[serde(default)]
    pub force_sync: bool,
    #[serde(default)]
    pub disabled: bool,
    #[serde(default)]
    pub quarantined: bool,
    #[serde(default)]
    pub quarantine_reason: String,
    #[serde(default)]
    pub revoked: bool,
    #[serde(default)]
    pub update_settings: Option<UpdateSettings>,
}

#[derive(Debug, Clone, PartialEq, Deserialize, Serialize)]
pub struct TunnelRelayCandidate {
    pub id: u64,
    pub relay_id: String,
    #[serde(default)]
    pub pool_id: u64,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub public_addr: String,
    #[serde(default)]
    pub transport_port: u16,
    #[serde(default)]
    pub ws_port: u16,
    #[serde(default)]
    pub gateway_addr: String,
    #[serde(default)]
    pub gateway_port: u16,
    #[serde(default)]
    pub gateway_tls: bool,
    #[serde(default = "default_health_path")]
    pub health_path: String,
    #[serde(default)]
    pub supported_transports: Vec<String>,
    #[serde(default)]
    pub noise_public_key: String,
    #[serde(default)]
    pub geo_name: String,
    #[serde(default)]
    pub geo_latitude: Option<f64>,
    #[serde(default)]
    pub geo_longitude: Option<f64>,
    #[serde(default)]
    pub zone_tag: String,
    #[serde(default = "default_weight")]
    pub weight: i32,
    #[serde(default)]
    pub draining: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Deserialize, Serialize)]
pub struct TunnelConnectorAttachmentGrant {
    pub relay_id: u64,
    #[serde(default)]
    pub relay_public_id: String,
    pub service_name: String,
    pub session_token: String,
    #[serde(default)]
    pub transport: String,
    #[serde(default)]
    pub state: String,
    #[serde(default)]
    pub lease_expires_at_unix: i64,
}

#[derive(Debug, Clone, PartialEq, Eq, Deserialize, Serialize)]
pub struct TunnelHTTPRouteRule {
    #[serde(default)]
    pub route_id: u64,
    #[serde(default)]
    pub site_name: String,
    #[serde(default)]
    pub domains: Vec<String>,
    pub target_addr: String,
    #[serde(default = "default_http_protocol")]
    pub target_protocol: String,
    #[serde(default)]
    pub origin_host: String,
    #[serde(default)]
    pub options_json: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Deserialize, Serialize)]
pub struct TunnelL4ServiceRule {
    pub id: u64,
    #[serde(default)]
    pub name: String,
    #[serde(default = "default_tcp_protocol")]
    pub service_type: String,
    #[serde(default)]
    pub relay_id: u64,
    #[serde(default)]
    pub listen_port: u16,
    pub target_addr: String,
    #[serde(default)]
    pub options_json: String,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TunnelConnectorConfigResponse {
    #[serde(default)]
    pub tunnel_id: u64,
    #[serde(default)]
    pub tunnel_public_id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub config_revision: i64,
    #[serde(default = "default_true")]
    pub enabled: bool,
    #[serde(default)]
    pub pool_id: u64,
    #[serde(default = "default_ha_relay_count")]
    pub ha_relay_count: usize,
    #[serde(default = "default_preferred_transport")]
    pub preferred_transport: String,
    #[serde(default)]
    pub fallback_transports: Vec<String>,
    #[serde(default)]
    pub client_proxy_url: String,
    #[serde(default)]
    pub allowed_target_cidrs: Vec<String>,
    #[serde(default)]
    pub blocked_target_cidrs: Vec<String>,
    #[serde(default)]
    pub candidate_relays: Vec<TunnelRelayCandidate>,
    #[serde(default)]
    pub attachments: Vec<TunnelConnectorAttachmentGrant>,
    #[serde(default)]
    pub routes: Vec<TunnelHTTPRouteRule>,
    #[serde(default)]
    pub services: Vec<TunnelL4ServiceRule>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct TunnelDesiredAttachment {
    pub relay_id: u64,
    pub transport: String,
    #[serde(default)]
    pub rtt_ms: i64,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TunnelNegotiateAttachmentsRequest {
    pub connector_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hostname: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub os_arch: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub public_ip: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub engine_version: Option<String>,
    #[serde(default)]
    pub contract_version: u32,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(default)]
    pub desired: Vec<TunnelDesiredAttachment>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct TunnelNegotiateAttachmentsResponse {
    #[serde(default)]
    pub tunnel_id: u64,
    #[serde(default)]
    pub config_revision: i64,
    #[serde(default)]
    pub lease_ttl_seconds: i64,
    #[serde(default)]
    pub attachments: Vec<TunnelConnectorAttachmentGrant>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct HealthCheckResponse {
    pub status: String,
    #[serde(default)]
    pub version: String,
    #[serde(default)]
    pub role: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub relay_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub draining: Option<bool>,
}

/// Computes the deterministic Rathole service name for an L4 TCP/UDP rule,
/// matching `l4ServiceName` in `internal/apps/openflare/tunnel/protocol_relay.go`.
pub fn derive_l4_service_name(tunnel_id: u64, service_id: u64) -> String {
    format!("l4_t{tunnel_id}_s{service_id}")
}

/// Computes the deterministic Rathole service token for an L4 TCP/UDP rule,
/// matching `deriveL4ServiceToken` in `internal/apps/openflare/tunnel/protocol_relay.go`.
pub fn derive_l4_service_token(tunnel_access_token: &str, tunnel_id: u64, service_id: u64) -> String {
    let input = format!("{tunnel_access_token}:{tunnel_id}:{service_id}");
    let digest = Sha256::digest(input.as_bytes());
    let mut out = String::with_capacity(35);
    out.push_str("l4_");
    for byte in &digest[..16] {
        use std::fmt::Write;
        let _ = write!(&mut out, "{byte:02x}");
    }
    out
}

fn default_http_ingress_service_type() -> String {
    "http_ingress".to_owned()
}

fn default_http_protocol() -> String {
    "http".to_owned()
}

fn default_tcp_protocol() -> String {
    "tcp".to_owned()
}

fn default_health_path() -> String {
    PATH_RELAY_HEALTH.to_owned()
}

fn default_weight() -> i32 {
    100
}

fn default_true() -> bool {
    true
}

fn default_ha_relay_count() -> usize {
    2
}

fn default_preferred_transport() -> String {
    "auto".to_owned()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn derives_deterministic_l4_service_name_and_token() {
        assert_eq!(derive_l4_service_name(7, 19), "l4_t7_s19");
        let token = derive_l4_service_token("tun_secret", 7, 19);
        assert!(token.starts_with("l4_"));
        assert_eq!(token.len(), 35);
        assert_eq!(token, derive_l4_service_token("tun_secret", 7, 19));
    }
}
