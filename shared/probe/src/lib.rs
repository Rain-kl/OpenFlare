//! `openflare-probe` is a reusable host monitoring, latency probing, and remote
//! command execution library extracted and modernized from `komari-monitor-rs`
//! for OpenFlare edge, relay, and future node runtimes.

pub mod collector;
pub mod dns_probe;
pub mod exec;
pub mod net_probe;
pub mod ping;
pub mod safety;
pub mod supervisor;
pub mod traceroute;
pub mod types;
pub mod worker;

pub use collector::{ProbeCollector, ProbeCollectorConfig};
pub use dns_probe::execute_dns_query;
pub use exec::{execute_command, ExecOptions};
pub use net_probe::execute_net_probe;
pub use ping::{execute_ping, execute_ping_batch, PingOptions};
pub use supervisor::{ProbeSupervisor, ProbeSupervisorConfig};
pub use traceroute::execute_traceroute;
pub use types::{
    BasicInfo, ConnectionsMetrics, CpuMetrics, DiskMetrics, DnsQueryRequest, DnsQueryResult,
    DnsRecord, ExecRequest, ExecResult, LoadMetrics, MemoryMetrics, NetProbeRequest,
    NetProbeResult, NetworkMetrics, PingBatchRequest, PingBatchResult, PingRequest, PingResult,
    PingSample, PingType, RealTimeMetrics, SwapMetrics, TracerouteHop, TracerouteProbe,
    TracerouteRequest, TracerouteResult, WorkerRequest, WorkerResponse,
};
pub use worker::{run_probe_worker, run_probe_worker_with_io, ProbeWorkerConfig};
