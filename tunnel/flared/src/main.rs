use anyhow::{Context, Result};
use clap::Parser;
use openflare_tunnel_common::bootstrap::{resolve_launch_auth, BootstrapKind};
use openflare_tunnel_common::identity::{
    ensure_connector_id, load_json, save_json_secret, token_sha256, ConnectorIdentity,
};
use openflare_tunnel_common::node_ip::NodeIpResolver;
use openflare_tunnel_common::updater::{
    confirm_trial_if_running, launch_staged_if_supported, notify_systemd_ready, spawn_update_loop,
    TunnelUpdatePlan,
};
use openflare_tunnel_common::{user_agent, TUNNEL_VERSION};
use openflared::{control, ingress, rathole_pool};
use std::path::PathBuf;
use std::sync::Arc;
use tokio::sync::{broadcast, mpsc, watch};

const BINARY_NAME: &str = "openflared";

#[derive(Parser, Debug, Clone)]
#[command(name = "openflared", about = "OpenFlare Native Tunnel Connector")]
struct Args {
    #[arg(long, env = "OPENFLARE_SERVER_URL")]
    server_url: Option<String>,

    #[arg(long, env = "OPENFLARE_TUNNEL_TOKEN")]
    token: Option<String>,

    #[arg(long, env = "OPENFLARE_TUNNEL_TOKEN_FILE")]
    token_file: Option<PathBuf>,

    #[arg(long, env = "OPENFLARE_TUNNEL_BOOTSTRAP_TOKEN")]
    bootstrap_token: Option<String>,

    #[arg(
        long,
        env = "OPENFLARE_TUNNEL_STATE_DIR",
        default_value = "/var/lib/openflared"
    )]
    state_dir: PathBuf,

    #[arg(long, env = "OPENFLARE_CONNECTOR_ID")]
    connector_id: Option<String>,

    #[arg(long)]
    version: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();
    if args.version {
        println!("{BINARY_NAME} {TUNNEL_VERSION}");
        return Ok(());
    }

    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .init();

    launch_staged_if_supported(&args.state_dir, BINARY_NAME);
    std::fs::create_dir_all(&args.state_dir)
        .with_context(|| format!("create state dir {}", args.state_dir.display()))?;

    let identity = ensure_connector_identity(&args)?;
    let (shutdown_tx, _shutdown_rx) = broadcast::channel(4);
    let (restart_tx, mut restart_rx) = mpsc::channel(2);
    let (update_tx, update_rx) = watch::channel::<Option<TunnelUpdatePlan>>(None);

    spawn_update_loop(
        args.state_dir.clone(),
        BINARY_NAME.to_owned(),
        update_rx,
        restart_tx,
    );

    let (ingress_state, ingress_addr) = ingress::spawn_ingress_server(shutdown_tx.subscribe()).await?;
    tracing::info!(%ingress_addr, connector_id = %identity.connector_id, "embedded tunnel HTTP/WS ingress listening");

    let rathole_pool = Arc::new(rathole_pool::RatholeClientPool::new(
        args.state_dir.join("rathole"),
        ingress_addr,
        identity.tunnel_token.clone(),
    ));
    let client = reqwest::Client::builder()
        .user_agent(user_agent(BINARY_NAME))
        .timeout(std::time::Duration::from_secs(15))
        .build()?;
    let ip_resolver = Arc::new(NodeIpResolver::new(
        &identity.server_url,
        &user_agent(BINARY_NAME),
    )?);
    let _ = ip_resolver.resolve_now().await;

    let (wake_tx, wake_rx) = mpsc::channel(16);
    control::spawn_wake_client(&identity, wake_tx, shutdown_tx.subscribe());

    confirm_trial_if_running(&args.state_dir, BINARY_NAME)?;
    if let Err(err) = notify_systemd_ready("OpenFlared active") {
        tracing::warn!(%err, "failed to notify systemd readiness");
    }

    tokio::select! {
        res = control::run_control_loop(
            client,
            identity,
            ip_resolver,
            ingress_state,
            rathole_pool.clone(),
            update_tx,
            wake_rx,
            shutdown_tx.subscribe(),
        ) => {
            if let Err(err) = res {
                tracing::error!(%err, "openflared control loop exited with error");
                let _ = shutdown_tx.send(true);
                rathole_pool.shutdown_all().await;
                return Err(err);
            }
        }
        _ = restart_rx.recv() => {
            tracing::info!("restarting openflared for staged update");
        }
        _ = shutdown_signal() => {
            tracing::info!("received shutdown signal");
        }
    }

    let _ = shutdown_tx.send(true);
    rathole_pool.shutdown_all().await;
    Ok(())
}

async fn shutdown_signal() {
    #[cfg(unix)]
    {
        match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
            Ok(mut sigterm) => {
                tokio::select! {
                    _ = tokio::signal::ctrl_c() => {}
                    _ = sigterm.recv() => {}
                }
            }
            Err(_) => {
                let _ = tokio::signal::ctrl_c().await;
            }
        }
    }
    #[cfg(not(unix))]
    {
        let _ = tokio::signal::ctrl_c().await;
    }
}

fn ensure_connector_identity(args: &Args) -> Result<ConnectorIdentity> {
    let identity_path = args.state_dir.join("connector_identity.json");
    let persistent_id = match args.connector_id.as_deref().map(str::trim).filter(|s| !s.is_empty()) {
        Some(explicit) => explicit.to_owned(),
        None => ensure_connector_id(&args.state_dir)?,
    };

    let existing = load_json::<ConnectorIdentity>(&identity_path)?;
    if let Some(mut existing_id) = existing.clone() {
        if existing_id.connector_id.is_empty() {
            existing_id.connector_id = persistent_id.clone();
        }
        if args.token.is_none() && args.bootstrap_token.is_none() && args.token_file.is_none() {
            return Ok(existing_id);
        }
    }

    let auth = resolve_launch_auth(
        args.server_url.clone(),
        args.token.clone(),
        args.token_file.as_deref(),
        args.bootstrap_token.clone(),
        BootstrapKind::Tunnel,
    )?;
    let Some(auth) = auth else {
        if let Some(mut existing_id) = existing {
            if existing_id.connector_id.is_empty() {
                existing_id.connector_id = persistent_id;
            }
            return Ok(existing_id);
        }
        anyhow::bail!(
            "openflared requires --token, --token-file, --bootstrap-token, or existing identity state"
        );
    };

    let fp = token_sha256(&auth.raw_token);
    if let Some(mut existing_id) = existing {
        if existing_id.connector_id.is_empty() {
            existing_id.connector_id = persistent_id.clone();
        }
        if existing_id.server_url == auth.server_url
            && !existing_id.source_token_sha256.is_empty()
            && existing_id.source_token_sha256 == fp
        {
            return Ok(existing_id);
        }
    }

    let identity = ConnectorIdentity {
        server_url: auth.server_url,
        connector_id: persistent_id,
        tunnel_token: auth.raw_token,
        source_token_sha256: fp,
    };
    save_json_secret(&identity_path, &identity)?;
    Ok(identity)
}
