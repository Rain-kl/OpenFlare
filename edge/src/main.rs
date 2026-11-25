use anyhow::Result;
use clap::{Parser, Subcommand};
use std::path::PathBuf;
use tracing_subscriber::{filter::dynamic_filter_fn, prelude::*};

mod config;
mod control;
mod geoip;
mod pages;
mod policy;
mod protocol;
mod proxy;
mod runtime;
mod telemetry;

const EDGE_VERSION: &str = match option_env!("OPENFLARE_EDGE_VERSION") {
    Some(version) => version,
    None => "dev",
};

fn edge_user_agent() -> String {
    format!("openflare-edge/{EDGE_VERSION}")
}

#[derive(Debug, Parser)]
#[command(name = "openflare-edge", version = EDGE_VERSION)]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    /// Print the supported controller configuration sync protocol version.
    ConfigSyncVersion,
    /// Print the supported managed update protocol version.
    SelfUpdateVersion,
    /// Print the attack rules available for configuring protection groups.
    AttackRuleCatalog,
    Run {
        #[arg(long)]
        upgrade: bool,

        #[arg(long)]
        server_url: String,
        #[arg(long, default_value = "discovery")]
        token_type: String,
        #[arg(long)]
        token_file: PathBuf,
        #[arg(long)]
        state_dir: PathBuf,
        #[arg(long, default_value = "127.0.0.1:8080")]
        http_listen: String,
        #[arg(long)]
        tls_listen: Option<String>,
    },
}

fn main() -> Result<()> {
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .with(tracing_subscriber::fmt::layer())
        .with(
            tracing_subscriber::fmt::layer()
                .with_ansi(false)
                .with_writer(telemetry::runtime_logs::LiveLogWriter::default)
                .with_filter(dynamic_filter_fn(|_, _| {
                    telemetry::runtime_logs::has_subscribers()
                })),
        )
        .init();

    let cli = Cli::parse();
    match cli.command {
        Command::ConfigSyncVersion => {
            println!("{}", control::config_sync::VERSION);
            Ok(())
        }
        Command::SelfUpdateVersion => {
            println!("{}", control::update::VERSION);
            Ok(())
        }
        Command::AttackRuleCatalog => {
            println!(
                "{}",
                serde_json::to_string_pretty(&policy::attack_rule_catalog()?)?
            );
            Ok(())
        }
        Command::Run {
            upgrade,
            server_url,
            token_type,
            token_file,
            state_dir,
            http_listen,
            tls_listen,
        } => {
            #[cfg(target_os = "linux")]
            harden_process();
            std::fs::create_dir_all(&state_dir)?;
            control::update::launch(&state_dir);
            let shared = config::SnapshotStore::open(&state_dir);
            if upgrade && state_dir.join("snapshot.json").exists() && shared.current().is_none() {
                anyhow::bail!("cannot upgrade without restoring an active configuration");
            }
            let keys = config::CertificateKeyRing::new();
            let listeners = runtime::Listeners::new(&http_listen, tls_listen.as_deref())?;
            let spool = telemetry::AccessLogSpool::persistent(&state_dir);
            let control = control::Runtime::new(
                server_url,
                token_type,
                token_file,
                state_dir.clone(),
                shared.clone(),
                spool.clone(),
                listeners.clone(),
            )?
            .with_key_ring(keys.clone());
            let (access_logs, log_writer) = telemetry::AccessLogBuffer::start(spool)?;
            let reload = proxy::run_server(
                proxy::DataPlaneState {
                    shared,
                    keys,
                    state_dir: state_dir.clone(),
                    listeners,
                    access_logs,
                },
                control,
                upgrade,
            )?;
            drop(log_writer);
            if reload && !control::update::restart_pending(&state_dir) {
                use std::os::unix::process::CommandExt;
                let mut args = std::env::args_os();
                let executable = std::env::current_exe()?;
                args.next();
                // An inherited listener retains its old socket options. Rebind it on reload.
                let error = std::process::Command::new(executable)
                    .args(args.filter(|arg| arg != "--upgrade"))
                    .exec();
                return Err(error.into());
            }
            Ok(())
        }
    }
}

#[cfg(target_os = "linux")]
fn harden_process() {
    // SAFETY: prctl(PR_SET_DUMPABLE, 0) sets the process dumpable flag without pointers.
    let rc = unsafe { libc::prctl(libc::PR_SET_DUMPABLE, 0, 0, 0, 0) };
    if rc != 0 {
        let err = std::io::Error::last_os_error();
        tracing::warn!(%err, "failed to mark process non-dumpable");
    }
    let limit = libc::rlimit {
        rlim_cur: 0,
        rlim_max: 0,
    };
    // SAFETY: `limit` is a valid initialized struct passed by shared pointer to setrlimit.
    let rlimit_rc = unsafe { libc::setrlimit(libc::RLIMIT_CORE, &limit) };
    if rlimit_rc != 0 {
        let err = std::io::Error::last_os_error();
        tracing::warn!(%err, "failed to disable core dumps via RLIMIT_CORE");
    }
}
