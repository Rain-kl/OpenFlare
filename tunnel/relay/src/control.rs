use anyhow::{bail, Context, Result};
use openflare_tunnel_common::hardening::http_to_ws_url;
use openflare_tunnel_common::identity::{save_json_secret, RelayIdentity};
use openflare_tunnel_common::node_ip::{NodeIpResolver, UNSPECIFIED_IP};
use openflare_tunnel_common::protocol::{
    ApiResponse, RelayHeartbeatPayload, RelayHeartbeatResponse, RelayRuntimeConfigResponse,
    HEADER_QUARANTINED, HEADER_RELAY_TOKEN, PATH_RELAY_CONFIG, PATH_RELAY_HEARTBEAT,
    PATH_RELAY_WS,
};
use openflare_tunnel_common::updater::TunnelUpdatePlan;
use openflare_tunnel_common::ws::{spawn_wake_loop, WakeSignal};
use openflare_tunnel_common::{CONTRACT_VERSION, TUNNEL_VERSION};
use std::path::PathBuf;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::{broadcast, mpsc, watch};

use crate::gateway::RelayGatewayState;
use crate::rathole_mgr::RatholeRelayManager;

const DEFAULT_HEARTBEAT_INTERVAL: Duration = Duration::from_secs(15);
const PERIODIC_CONFIG_REFRESH: Duration = Duration::from_secs(60);

#[derive(Debug, Clone)]
pub struct RelayRuntimeSettings {
    pub state_dir: PathBuf,
    pub public_addr: Option<String>,
    pub gateway_addr: Option<String>,
    pub transport_port: u16,
    pub ws_port: u16,
    pub gateway_port: u16,
}

pub fn spawn_wake_client(
    identity: &RelayIdentity,
    tx: mpsc::Sender<WakeSignal>,
    shutdown: broadcast::Receiver<bool>,
) {
    let Ok(ws_url) = http_to_ws_url(&identity.server_url, PATH_RELAY_WS) else {
        return;
    };
    spawn_wake_loop(
        ws_url,
        vec![(HEADER_RELAY_TOKEN.to_owned(), identity.access_token.clone())],
        tx,
        shutdown,
    );
}

#[allow(clippy::too_many_arguments)]
pub async fn run_control_loop(
    client: reqwest::Client,
    mut identity: RelayIdentity,
    settings: RelayRuntimeSettings,
    ip_resolver: Arc<NodeIpResolver>,
    rathole_mgr: Arc<RatholeRelayManager>,
    gateway_state: Arc<RelayGatewayState>,
    update_tx: watch::Sender<Option<TunnelUpdatePlan>>,
    mut wake_rx: mpsc::Receiver<WakeSignal>,
    mut shutdown: broadcast::Receiver<bool>,
) -> Result<()> {
    sync_runtime_config(
        &client,
        &mut identity,
        &settings,
        &rathole_mgr,
        &gateway_state,
    )
    .await?;

    let mut hb_interval = DEFAULT_HEARTBEAT_INTERVAL;
    let mut last_sync_at = Instant::now();

    loop {
        tokio::select! {
            _ = shutdown.recv() => return Ok(()),
            wake = wake_rx.recv() => {
                match wake {
                    Some(WakeSignal::ForceDisconnect) => {
                        bail!("relay received remote quarantine/disconnect signal");
                    }
                    Some(WakeSignal::ConfigSync) => {
                        if let Err(err) = sync_runtime_config(
                            &client,
                            &mut identity,
                            &settings,
                            &rathole_mgr,
                            &gateway_state,
                        ).await {
                            tracing::warn!(%err, "relay wake config sync failed");
                        } else {
                            last_sync_at = Instant::now();
                        }
                    }
                    None => {}
                }
            }
            _ = tokio::time::sleep(hb_interval) => {
                let ip = ip_resolver.resolve_now().await;
                match send_heartbeat(&client, &identity, &settings, &ip, &gateway_state).await {
                    Ok(hb) => {
                        gateway_state.set_draining(hb.draining);
                        if hb.quarantined {
                            bail!("relay node is quarantined by control plane: {}", hb.quarantine_reason);
                        }
                        if hb.heartbeat_interval > 0 {
                            let ms = hb.heartbeat_interval.clamp(5_000, 120_000);
                            hb_interval = Duration::from_millis(ms);
                        }
                        if let Some(upd) = hb.update_settings {
                            let next_plan = (upd.auto_update || upd.tag.is_some())
                                .then_some(TunnelUpdatePlan {
                                    repo: upd.repo,
                                    tag: upd.tag,
                                    binary_name: "openflare-relay".to_owned(),
                                });
                            update_tx.send_if_modified(|current| {
                                if *current != next_plan {
                                    *current = next_plan;
                                    true
                                } else {
                                    false
                                }
                            });
                        }
                        if hb.force_sync || last_sync_at.elapsed() >= PERIODIC_CONFIG_REFRESH {
                            if let Err(err) = sync_runtime_config(
                                &client,
                                &mut identity,
                                &settings,
                                &rathole_mgr,
                                &gateway_state,
                            ).await {
                                tracing::warn!(%err, "relay periodic config sync failed");
                            } else {
                                last_sync_at = Instant::now();
                            }
                        }
                    }
                    Err(err) => {
                        let msg = err.to_string();
                        if msg.contains("quarantined") {
                            return Err(err);
                        }
                        tracing::warn!(%err, "relay heartbeat failed");
                    }
                }
            }
        }
    }
}

async fn send_heartbeat(
    client: &reqwest::Client,
    identity: &RelayIdentity,
    settings: &RelayRuntimeSettings,
    ip: &str,
    gateway_state: &RelayGatewayState,
) -> Result<RelayHeartbeatResponse> {
    let url = format!(
        "{}{PATH_RELAY_HEARTBEAT}",
        identity.server_url.trim_end_matches('/')
    );
    let verified = gateway_state.verified_attachments().await;
    let (active_connectors, active_streams) = gateway_state.connector_stats(&verified);
    let public_addr = settings
        .public_addr
        .clone()
        .filter(|s| !s.trim().is_empty())
        .or_else(|| (ip != UNSPECIFIED_IP).then(|| ip.to_owned()));

    let payload = RelayHeartbeatPayload {
        version: TUNNEL_VERSION.to_owned(),
        engine_version: Some("rathole-0.5".to_owned()),
        contract_version: CONTRACT_VERSION,
        capabilities: vec![
            "noise".into(),
            "websocket".into(),
            "tcp".into(),
            "http-gateway".into(),
            "hot-reload".into(),
        ],
        public_addr,
        gateway_addr: settings.gateway_addr.clone(),
        transport_port: Some(settings.transport_port),
        ws_port: Some(settings.ws_port),
        gateway_port: Some(settings.gateway_port),
        gateway_tls: Some(false),
        supported_transports: vec!["noise".into(), "websocket".into(), "tcp".into()],
        runtime_status: Some(if gateway_state.is_draining() {
            "draining".to_owned()
        } else {
            "healthy".to_owned()
        }),
        draining: gateway_state.is_draining(),
        active_connectors,
        active_streams,
        applied_update_tag: Some(TUNNEL_VERSION.to_owned()),
        verified_attachments: verified,
        ..Default::default()
    };
    let resp = client
        .post(url)
        .header(HEADER_RELAY_TOKEN, &identity.access_token)
        .json(&payload)
        .send()
        .await?;
    if resp.status() == reqwest::StatusCode::FORBIDDEN
        && resp
            .headers()
            .get(HEADER_QUARANTINED)
            .and_then(|v| v.to_str().ok())
            == Some("true")
    {
        bail!("relay is quarantined by control plane");
    }
    let resp = resp.error_for_status().context("relay heartbeat HTTP status")?;
    let api_resp: ApiResponse<RelayHeartbeatResponse> = resp.json().await?;
    api_resp.into_result()
}

async fn sync_runtime_config(
    client: &reqwest::Client,
    identity: &mut RelayIdentity,
    settings: &RelayRuntimeSettings,
    rathole_mgr: &RatholeRelayManager,
    gateway_state: &RelayGatewayState,
) -> Result<()> {
    let url = format!(
        "{}{PATH_RELAY_CONFIG}",
        identity.server_url.trim_end_matches('/')
    );
    let resp = client
        .get(url)
        .header(HEADER_RELAY_TOKEN, &identity.access_token)
        .send()
        .await?;
    if resp.status() == reqwest::StatusCode::FORBIDDEN
        && resp
            .headers()
            .get(HEADER_QUARANTINED)
            .and_then(|v| v.to_str().ok())
            == Some("true")
    {
        bail!("relay is quarantined by control plane");
    }
    let resp = resp.error_for_status().context("fetch relay runtime config")?;
    let api_resp: ApiResponse<RelayRuntimeConfigResponse> = resp.json().await?;
    let mut cfg = api_resp.into_result()?;
    gateway_state.set_draining(cfg.draining);
    if cfg.noise_private_key.is_empty() {
        cfg.noise_private_key = identity.noise_private_key.clone();
    }
    if cfg.gateway_secret.is_empty() {
        cfg.gateway_secret = identity.gateway_secret.clone();
    } else if cfg.gateway_secret != identity.gateway_secret {
        identity.gateway_secret = cfg.gateway_secret.clone();
        let _ = save_json_secret(&settings.state_dir.join("relay_identity.json"), identity);
    }
    if cfg.transport_port == 0 {
        cfg.transport_port = settings.transport_port;
    }
    if cfg.ws_port == 0 {
        cfg.ws_port = settings.ws_port;
    }
    if !cfg.gateway_secret.is_empty() {
        gateway_state.set_gateway_secret(cfg.gateway_secret.clone()).await;
    }
    rathole_mgr.apply_config(&cfg).await?;
    Ok(())
}
