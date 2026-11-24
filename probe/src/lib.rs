//! `openflare-probe` is a reusable host monitoring, latency probing, and remote
//! command execution library extracted and modernized from `komari-monitor-rs`
//! for OpenFlare edge, relay, and future node runtimes.

pub mod collector;
pub mod exec;
pub mod ping;
pub mod types;

pub use collector::{ProbeCollector, ProbeCollectorConfig};
pub use exec::{execute_command, ExecOptions};
pub use ping::{execute_ping, PingOptions};
pub use types::{
    BasicInfo, ConnectionsMetrics, CpuMetrics, DiskMetrics, ExecRequest, ExecResult, LoadMetrics,
    MemoryMetrics, NetworkMetrics, PingRequest, PingResult, PingType, RealTimeMetrics, SwapMetrics,
};
