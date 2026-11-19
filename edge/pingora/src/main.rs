use anyhow::Result;
use clap::{Parser, Subcommand};
use std::path::PathBuf;

mod config;
mod control;
mod geoip;
mod pages;
mod policy;
mod protocol;
mod proxy;
mod runtime;
mod telemetry;

#[derive(Debug, Parser)]
#[command(name = "openflare-edge", version)]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
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
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .init();

    let cli = Cli::parse();
    match cli.command {
        Command::Run {
            upgrade,
            server_url,
            token_type,
            token_file,
            state_dir,
            http_listen,
            tls_listen,
        } => {
            std::fs::create_dir_all(&state_dir)?;
            let shared = config::SnapshotStore::open(&state_dir);
            if upgrade && state_dir.join("snapshot.json").exists() && shared.current().is_none() {
                anyhow::bail!("cannot upgrade without restoring an active configuration");
            }
            let listeners = runtime::Listeners::new(&http_listen, tls_listen.as_deref())?;
            let access_logs = telemetry::AccessLogBuffer::persistent(&state_dir);
            let control = control::Runtime::new(
                server_url,
                token_type,
                token_file,
                state_dir.clone(),
                shared.clone(),
                access_logs.clone(),
                listeners.clone(),
            )?;
            proxy::run_server(
                proxy::DataPlaneState {
                    shared,
                    state_dir,
                    listeners,
                    access_logs,
                },
                control,
                upgrade,
            )
        }
    }
}
