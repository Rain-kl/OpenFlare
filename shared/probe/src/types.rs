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
