use serde::{Deserialize, Serialize};
use std::fmt;

/// Static / slow-changing node hardware and OS inventory (Komari `BasicInfo` aligned).
#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq, Eq)]
pub struct BasicInfo {
    pub hostname: String,
    pub arch: String,
    pub cpu_cores: u64,
    pub cpu_physical_cores: u64,
    pub cpu_name: String,
    pub gpu_name: String,
    pub disk_total: u64,
    pub swap_total: u64,
    pub mem_total: u64,
    pub ipv4: Option<String>,
    pub ipv6: Option<String>,
    pub os: String,
    pub os_name: String,
    pub os_version: String,
    pub kernel_version: String,
    pub virtualization: String,
    pub version: String,
    pub uptime: u64,
    pub reported_at_unix: u64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
pub struct CpuMetrics {
    pub usage: f64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq, Eq)]
pub struct MemoryMetrics {
    pub used: u64,
    pub total: u64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq, Eq)]
pub struct SwapMetrics {
    pub used: u64,
    pub total: u64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq, Eq)]
pub struct DiskMetrics {
    pub used: u64,
    pub total: u64,
    pub read_bytes: u64,
    pub write_bytes: u64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
pub struct LoadMetrics {
    pub load1: f64,
    pub load5: f64,
    pub load15: f64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq, Eq)]
pub struct NetworkMetrics {
    /// Realtime upload speed in bytes/sec.
    pub up: u64,
    /// Realtime download speed in bytes/sec.
    pub down: u64,
    /// Cumulative upload bytes.
    #[serde(rename = "totalUp")]
    pub total_up: u64,
    /// Cumulative download bytes.
    #[serde(rename = "totalDown")]
    pub total_down: u64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq, Eq)]
pub struct ConnectionsMetrics {
    pub tcp: u64,
    pub udp: u64,
}

/// Real-time host telemetry snapshot (Komari `RealTimeInfo` aligned).
#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
pub struct RealTimeMetrics {
    pub captured_at_unix: u64,
    pub cpu: CpuMetrics,
    pub gpu: f64,
    pub ram: MemoryMetrics,
    pub swap: SwapMetrics,
    pub disk: DiskMetrics,
    pub load: LoadMetrics,
    pub temp: f64,
    pub network: NetworkMetrics,
    pub connections: ConnectionsMetrics,
    pub uptime: u64,
    pub process: u64,
}

/// Supported latency probe protocols.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PingType {
    Icmp,
    Tcp,
    Http,
}

impl PingType {
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Icmp => "icmp",
            Self::Tcp => "tcp",
            Self::Http => "http",
        }
    }

    pub fn parse(value: &str) -> Option<Self> {
        match value.trim().to_ascii_lowercase().as_str() {
            "icmp" => Some(Self::Icmp),
            "tcp" => Some(Self::Tcp),
            "http" | "https" => Some(Self::Http),
            _ => None,
        }
    }
}

impl fmt::Display for PingType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Inbound latency probe request dispatched by the controller.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct PingRequest {
    pub ping_task_id: u64,
    pub ping_type: String,
    pub ping_target: String,
}

/// Outbound latency probe measurement.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct PingResult {
    pub task_id: u64,
    pub ping_type: String,
    /// Latency in milliseconds, or `-1` on timeout/failure.
    pub value: i64,
    pub finished_at: String,
}

/// Inbound remote shell command request dispatched by the controller.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ExecRequest {
    pub task_id: String,
    pub command: String,
    #[serde(default)]
    pub timeout_seconds: Option<u64>,
}

/// Outbound remote command execution outcome.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ExecResult {
    pub task_id: String,
    pub result: String,
    pub exit_code: i32,
    pub finished_at: String,
}

fn default_ping_count() -> u8 {
    4
}

fn default_ping_interval_ms() -> u64 {
    200
}

fn default_trace_protocol() -> String {
    "icmp".to_owned()
}

fn default_trace_max_hops() -> u8 {
    30
}

fn default_trace_probes() -> u8 {
    3
}

fn default_dns_qtype() -> String {
    "A".to_owned()
}

fn default_dns_resolver() -> String {
    "1.1.1.1:53".to_owned()
}

/// Multi-sample ping / TCPing / HTTP latency probe request.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct PingBatchRequest {
    pub protocol: String,
    pub target: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub port: Option<u16>,
    #[serde(default = "default_ping_count")]
    pub count: u8,
    #[serde(default = "default_ping_interval_ms")]
    pub interval_ms: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
}

/// Individual latency attempt within a batch ping result.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct PingSample {
    pub seq: u8,
    pub rtt_ms: i64,
    pub success: bool,
}

/// Aggregated multi-sample ping / TCPing result.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PingBatchResult {
    pub protocol: String,
    pub target: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resolved_ip: Option<String>,
    pub sent: u8,
    pub received: u8,
    pub loss_rate: f64,
    pub min_ms: f64,
    pub avg_ms: f64,
    pub max_ms: f64,
    pub p95_ms: f64,
    pub jitter_ms: f64,
    pub samples: Vec<PingSample>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Traceroute probe request executed via `trippy-core`.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct TracerouteRequest {
    pub target: String,
    #[serde(default = "default_trace_protocol")]
    pub protocol: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub port: Option<u16>,
    #[serde(default = "default_trace_max_hops")]
    pub max_hops: u8,
    #[serde(default = "default_trace_probes")]
    pub probes_per_hop: u8,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
}

/// Single round measurement observed at a traceroute hop.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct TracerouteProbe {
    pub seq: u8,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ip: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rtt_ms: Option<f64>,
}

/// Per-TTL traceroute hop summary.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct TracerouteHop {
    pub ttl: u8,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ip: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub alt_ips: Vec<String>,
    pub sent: u16,
    pub recv: u16,
    pub loss_pct: f64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_ms: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub avg_ms: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_ms: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub jitter_ms: Option<f64>,
    #[serde(default)]
    pub probes: Vec<TracerouteProbe>,
}

/// Complete traceroute measurement output.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct TracerouteResult {
    pub target: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resolved_ip: Option<String>,
    pub protocol: String,
    pub hop_count: u8,
    pub reached: bool,
    pub total_rtt_ms: f64,
    pub hops: Vec<TracerouteHop>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// DNS query probe request supporting optional EDNS0 Client Subnet (ECS).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct DnsQueryRequest {
    pub domain: String,
    #[serde(default = "default_dns_qtype")]
    pub record_type: String,
    #[serde(default = "default_dns_resolver")]
    pub resolver: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ecs_subnet: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
}

/// Single DNS answer record returned by `execute_dns_query`.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct DnsRecord {
    pub name: String,
    pub record_type: String,
    pub ttl: u32,
    pub value: String,
}

/// DNS query probe output.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DnsQueryResult {
    pub domain: String,
    pub record_type: String,
    pub resolver: String,
    pub rcode: String,
    pub authoritative: bool,
    pub truncated: bool,
    pub used_tcp: bool,
    pub latency_ms: f64,
    pub records: Vec<DnsRecord>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Unified network probe task envelope dispatched from the controller to nodes.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct NetProbeRequest {
    pub probe_id: String,
    pub kind: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ping: Option<PingBatchRequest>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub traceroute: Option<TracerouteRequest>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dns: Option<DnsQueryRequest>,
}

/// Unified network probe response emitted by node probe workers.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct NetProbeResult {
    pub probe_id: String,
    pub kind: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ping: Option<PingBatchResult>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub traceroute: Option<TracerouteResult>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dns: Option<DnsQueryResult>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    pub finished_at: String,
}

/// NDJSON IPC request sent from parent supervisor to isolated `probe-worker` subprocess.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "op", rename_all = "snake_case")]
pub enum WorkerRequest {
    Ping {
        request_id: String,
        payload: PingRequest,
    },
    NetProbe {
        request_id: String,
        payload: NetProbeRequest,
    },
}

/// NDJSON IPC response returned by isolated `probe-worker` subprocess.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "op", rename_all = "snake_case")]
pub enum WorkerResponse {
    Ping {
        request_id: String,
        payload: PingResult,
    },
    NetProbe {
        request_id: String,
        payload: NetProbeResult,
    },
}
