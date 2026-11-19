use anyhow::Result;
use clap::{Parser, Subcommand};
use std::path::PathBuf;

mod config;
mod control;
mod pages;
mod policy;
mod proxy;

#[derive(Debug, Parser)]
#[command(name = "openflare-edge", version)]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    Run {
        #[arg(long, hide = true)]
        data_plane: bool,

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
            data_plane,
            server_url,
            token_type,
            token_file,
            state_dir,
            http_listen,
            tls_listen,
        } => {
            if data_plane {
                let state = control::load_restored_state(state_dir, http_listen, tls_listen)?;
                proxy::run_server(state)
            } else {
                let tokio_runtime = tokio::runtime::Builder::new_multi_thread()
                    .enable_all()
                    .build()?;
                let runtime = control::Runtime::new(
                    server_url,
                    token_type,
                    token_file,
                    state_dir,
                    http_listen,
                    tls_listen,
                )?;
                tokio_runtime.block_on(runtime.run())
            }
        }
    }
}
