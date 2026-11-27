use clap::{Args, Parser, Subcommand};

pub const DNS_VERSION: &str = match option_env!("OPENFLARE_DNS_VERSION") {
    Some(version) if !version.is_empty() => version,
    _ => "dev",
};

#[derive(Debug, Parser)]
#[command(name = "openflare-dns", version = DNS_VERSION, about = "OpenFlare SmartDNS authoritative DNS server")]
pub struct Cli {
    #[command(subcommand)]
    pub command: Command,
}

#[derive(Debug, Subcommand)]
pub enum Command {
    /// Run authoritative DNS server synchronized with the OpenFlare controller
    Serve(ServeArgs),
    /// Benchmark bare hickory-server vs full SmartDNS handler throughput
    Bench(BenchArgs),
}

#[derive(Debug, Clone, Args)]
pub struct ServeArgs {
    #[arg(long, env = "OPENFLARE_DNS_CONTROLLER_URL")]
    pub controller_url: String,

    #[arg(long, env = "OPENFLARE_DNS_TOKEN")]
    pub token: Option<String>,

    #[arg(long, env = "OPENFLARE_DNS_DISCOVERY_TOKEN")]
    pub discovery_token: Option<String>,

    #[arg(long, env = "OPENFLARE_DNS_TOKEN_FILE")]
    pub token_file: Option<String>,

    #[arg(long, env = "OPENFLARE_DNS_TOKEN_TYPE", default_value = "discovery")]
    pub token_type: String,

    #[arg(long, env = "OPENFLARE_DNS_INSTANCE_ID")]
    pub instance_id: Option<String>,

    #[arg(
        long,
        env = "OPENFLARE_DNS_NAME",
        default_value = "Standalone SmartDNS"
    )]
    pub name: String,

    #[arg(long, env = "OPENFLARE_DNS_LISTEN", default_value = "0.0.0.0:53")]
    pub listen: String,

    #[arg(long, env = "OPENFLARE_DNS_PUBLIC_ADDR", default_value = "")]
    pub public_addr: String,

    #[arg(long, env = "OPENFLARE_DNS_GEOIP_DIR", default_value = "data/netintel")]
    pub geoip_dir: String,

    #[arg(long, env = "OPENFLARE_DNS_CACHE_DIR", default_value = "data/smartdns")]
    pub cache_dir: String,

    #[arg(long, env = "OPENFLARE_DNS_WORKERS", default_value_t = 0)]
    pub workers: usize,

    #[arg(long, env = "OPENFLARE_DNS_RRL_QPS", default_value_t = 1000)]
    pub rrl_qps: u64,

    #[arg(long, env = "OPENFLARE_DNS_RRL_BURST", default_value_t = 2000)]
    pub rrl_burst: u64,

    #[arg(
        long,
        env = "OPENFLARE_DNS_SNAPSHOT_INTERVAL_SECS",
        default_value_t = 10
    )]
    pub snapshot_interval_secs: u64,
}

#[derive(Debug, Clone, Args)]
pub struct BenchArgs {
    #[arg(long, default_value_t = 40000)]
    pub queries: usize,

    #[arg(long, default_value_t = 32)]
    pub concurrency: usize,
}
