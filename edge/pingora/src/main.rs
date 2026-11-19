use anyhow::Result;
use clap::{Parser, Subcommand};
use std::path::PathBuf;

mod config;
mod control;
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
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()?;
    runtime.block_on(async move {
        match cli.command {
            Command::Run {
                server_url,
                token_type,
                token_file,
                state_dir,
                http_listen,
                tls_listen,
            } => {
                let runtime = control::Runtime::new(
                    server_url,
                    token_type,
                    token_file,
                    state_dir,
                    http_listen,
                    tls_listen,
                )?;
                runtime.run().await
            }
        }
    })
}
