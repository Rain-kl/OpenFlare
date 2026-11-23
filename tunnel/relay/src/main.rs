use anyhow::{Context, Result};
use clap::Parser;
use openflare_relay::{control, gateway, prober, rathole_mgr};
use openflare_relay::control::RelayRuntimeSettings;
use openflare_tunnel_common::bootstrap::{resolve_launch_auth, BootstrapKind};
use openflare_tunnel_common::identity::{load_json, save_json_secret, token_sha256, RelayIdentity};
use openflare_tunnel_common::node_ip::NodeIpResolver;
use openflare_tunnel_common::protocol::{
    ApiResponse, RelayRegisterRequest, RelayRegisterResponse, HEADER_RELAY_BOOTSTRAP_TOKEN,
    HEADER_RELAY_TOKEN, PATH_RELAY_REGISTER,
};
use openflare_tunnel_common::updater::{
    confirm_trial_if_running, launch_staged_if_supported, spawn_update_loop, TunnelUpdatePlan,
};
use openflare_tunnel_common::{user_agent, CONTRACT_VERSION, TUNNEL_VERSION};
use std::path::PathBuf;
use std::sync::Arc;
use tokio::sync::{broadcast, mpsc, watch};

const BINARY_NAME: &str = "openflare-relay";

#[derive(Parser, Debug, Clone)]
#[command(name = "openflare-relay", about = "OpenFlare Native Tunnel Relay")]
struct Args {
    #[arg(long, env = "OPENFLARE_SERVER_URL")]
    server_url: Option<String>,

    #[arg(long, env = "OPENFLARE_RELAY_TOKEN")]
    token: Option<String>,

    #[arg(long, env = "OPENFLARE_RELAY_TOKEN_FILE")]
    token_file: Option<PathBuf>,

    #[arg(long, env = "OPENFLARE_RELAY_BOOTSTRAP_TOKEN")]
    bootstrap_token: Option<String>,

    #[arg(
        long,
        env = "OPENFLARE_RELAY_STATE_DIR",
        default_value = "/var/lib/openflare-relay"
    )]
    state_dir: PathBuf,

    #[arg(long, env = "OPENFLARE_RELAY_NAME")]
    name: Option<String>,

    #[arg(long, alias = "endpoint-host", env = "OPENFLARE_RELAY_PUBLIC_ADDR")]
    public_addr: Option<String>,

    #[arg(long, env = "OPENFLARE_RELAY_GATEWAY_ADDR")]
    gateway_addr: Option<String>,

    #[arg(long, env = "OPENFLARE_RELAY_ZONE_TAG")]
    zone_tag: Option<String>,

    #[arg(long, env = "OPENFLARE_RELAY_COLOCATED_EDGE_NODE_ID")]
    colocated_edge_node_id: Option<String>,

    #[arg(long, env = "OPENFLARE_RELAY_TRANSPORT_PORT", default_value_t = 7443)]
    transport_port: u16,

    #[arg(long, env = "OPENFLARE_RELAY_WS_PORT", default_value_t = 7444)]
    ws_port: u16,

    #[arg(long, env = "OPENFLARE_RELAY_GATEWAY_PORT", default_value_t = 8443)]
    gateway_port: u16,

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

    let client = reqwest::Client::builder()
        .user_agent(user_agent(BINARY_NAME))
        .timeout(std::time::Duration::from_secs(15))
        .build()?;

    let identity = ensure_identity(&client, &args).await?;
    let gateway_state = Arc::new(gateway::RelayGatewayState::new(
        identity.relay_id.clone(),
        identity.gateway_secret.clone(),
    ));
    let rathole_mgr = Arc::new(rathole_mgr::RatholeRelayManager::new(
        args.state_dir.join("rathole"),
        gateway_state.clone(),
    ));

    let ip_resolver = Arc::new(NodeIpResolver::new(
        &identity.server_url,
        &user_agent(BINARY_NAME),
    )?);
    let _ = ip_resolver.resolve_now().await;

    let (shutdown_tx, shutdown_rx) = broadcast::channel(4);
    let (restart_tx, mut restart_rx) = mpsc::channel(2);
    let (update_tx, update_rx) = watch::channel::<Option<TunnelUpdatePlan>>(None);

    spawn_update_loop(
        args.state_dir.clone(),
        BINARY_NAME.to_owned(),
        update_rx,
        restart_tx,
    );

    gateway::spawn_gateway_server(
        format!("0.0.0.0:{}", args.gateway_port),
        gateway_state.clone(),
        shutdown_tx.subscribe(),
    );
    prober::spawn_prober_loop(gateway_state.clone(), shutdown_tx.subscribe());

    let (wake_tx, wake_rx) = mpsc::channel(16);
    control::spawn_wake_client(&identity, wake_tx, shutdown_tx.subscribe());

    confirm_trial_if_running(&args.state_dir, BINARY_NAME)?;
    if let Err(err) = openflare_tunnel_common::updater::notify_systemd_ready("OpenFlare Relay active")
    {
        tracing::warn!(%err, "failed to notify systemd readiness");
    }

    let runtime_settings = RelayRuntimeSettings {
        state_dir: args.state_dir.clone(),
        public_addr: args.public_addr.clone(),
        gateway_addr: args.gateway_addr.clone(),
        transport_port: args.transport_port,
        ws_port: args.ws_port,
        gateway_port: args.gateway_port,
    };

    tokio::select! {
        res = control::run_control_loop(
            client,
            identity,
            runtime_settings,
            ip_resolver,
            rathole_mgr.clone(),
            gateway_state,
            update_tx,
            wake_rx,
            shutdown_tx.subscribe(),
        ) => {
            if let Err(err) = res {
                tracing::error!(%err, "relay control loop exited with error");
                let _ = shutdown_tx.send(true);
                rathole_mgr.shutdown().await;
                return Err(err);
            }
        }
        _ = restart_rx.recv() => {
            tracing::info!("restarting relay process for staged update");
        }
        _ = shutdown_signal() => {
            tracing::info!("received shutdown signal");
        }
    }

    let _ = shutdown_tx.send(true);
    rathole_mgr.shutdown().await;
    drop(shutdown_rx);
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

async fn ensure_identity(client: &reqwest::Client, args: &Args) -> Result<RelayIdentity> {
    let identity_path = args.state_dir.join("relay_identity.json");
    let existing = load_json::<RelayIdentity>(&identity_path)?;
    if let Some(ref existing_id) = existing {
        if args.token.is_none() && args.bootstrap_token.is_none() && args.token_file.is_none() {
            return Ok(existing_id.clone());
        }
    }

    let auth = resolve_launch_auth(
        args.server_url.clone(),
        args.token.clone(),
        args.token_file.as_deref(),
        args.bootstrap_token.clone(),
        BootstrapKind::Relay,
    )?;
    let Some(auth) = auth else {
        if let Some(existing_id) = existing {
            return Ok(existing_id);
        }
        anyhow::bail!(
            "relay requires --token, --token-file, --bootstrap-token, or existing identity state"
        );
    };

    let fp = token_sha256(&auth.raw_token);
    if let Some(existing_id) = existing {
        if existing_id.server_url == auth.server_url
            && !existing_id.source_token_sha256.is_empty()
            && existing_id.source_token_sha256 == fp
        {
            return Ok(existing_id);
        }
    }

    let ip_resolver = NodeIpResolver::new(&auth.server_url, &user_agent(BINARY_NAME))?;
    let resolved_ip = ip_resolver.resolve_now().await;

    let reg = if auth.used_bootstrap {
        register_relay(
            client,
            &auth.server_url,
            Some(&auth.raw_token),
            None,
            auth.pool_key.as_deref(),
            args,
            &resolved_ip,
        )
        .await?
    } else {
        // Pre-created relay tokens or discovery tokens passed cleanly via X-Relay-Token are both
        // accepted by POST /api/v1/relay/register.
        register_relay(
            client,
            &auth.server_url,
            None,
            Some(&auth.raw_token),
            auth.pool_key.as_deref(),
            args,
            &resolved_ip,
        )
        .await?
    };

    let identity = RelayIdentity {
        server_url: auth.server_url,
        relay_id: reg.relay_id,
        pool_id: reg.pool_id,
        access_token: if reg.access_token.is_empty() {
            auth.raw_token
        } else {
            reg.access_token
        },
        noise_private_key: reg.noise_private_key,
        noise_public_key: reg.noise_public_key,
        gateway_secret: reg.gateway_secret,
        source_token_sha256: fp,
    };
    save_json_secret(&identity_path, &identity)?;
    Ok(identity)
}

async fn register_relay(
    client: &reqwest::Client,
    server_url: &str,
    bootstrap_token: Option<&str>,
    relay_token: Option<&str>,
    pool_key: Option<&str>,
    args: &Args,
    resolved_ip: &str,
) -> Result<RelayRegisterResponse> {
    let hostname = std::env::var("HOSTNAME")
        .ok()
        .map(|s| s.trim().to_owned())
        .filter(|s| !s.is_empty());
    let name = args
        .name
        .clone()
        .or_else(|| hostname.clone())
        .unwrap_or_else(|| "openflare-relay".to_owned());
    let public_addr = args
        .public_addr
        .clone()
        .filter(|s| !s.trim().is_empty())
        .unwrap_or_else(|| resolved_ip.to_owned());

    let req_body = RelayRegisterRequest {
        relay_id: None,
        name,
        pool_key: pool_key.map(|v| v.to_owned()),
        public_addr,
        transport_port: args.transport_port,
        ws_port: args.ws_port,
        gateway_addr: args.gateway_addr.clone(),
        gateway_port: args.gateway_port,
        gateway_tls: false,
        supported_transports: vec!["noise".into(), "websocket".into(), "tcp".into()],
        noise_public_key: None,
        geo_name: None,
        zone_tag: args.zone_tag.clone(),
        colocated_edge_node_id: args.colocated_edge_node_id.clone(),
        weight: None,
        version: Some(TUNNEL_VERSION.to_owned()),
        engine_version: Some("rathole-0.5".to_owned()),
        contract_version: CONTRACT_VERSION,
        capabilities: vec![
            "noise".into(),
            "websocket".into(),
            "tcp".into(),
            "http-gateway".into(),
            "hot-reload".into(),
        ],
    };

    let url = format!("{}{PATH_RELAY_REGISTER}", server_url.trim_end_matches('/'));
    let mut builder = client.post(url).json(&req_body);
    if let Some(btok) = bootstrap_token {
        builder = builder.header(HEADER_RELAY_BOOTSTRAP_TOKEN, btok);
    }
    if let Some(rtok) = relay_token {
        builder = builder.header(HEADER_RELAY_TOKEN, rtok);
    }
    let resp = builder.send().await?;
    if !resp.status().is_success() {
        let status = resp.status();
        let text = resp.text().await.unwrap_or_default();
        anyhow::bail!("relay registration failed ({status}): {text}");
    }
    let api_resp: ApiResponse<RelayRegisterResponse> = resp.json().await?;
    api_resp.into_result()
}
