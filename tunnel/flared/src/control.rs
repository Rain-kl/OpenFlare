use anyhow::{Context, Result};
use openflare_tunnel_common::hardening::http_to_ws_url;
use openflare_tunnel_common::identity::ConnectorIdentity;
use openflare_tunnel_common::node_ip::{NodeIpResolver, UNSPECIFIED_IP};
use openflare_tunnel_common::protocol::{
    ApiResponse, TunnelConnectorConfigResponse, TunnelConnectorHeartbeatPayload,
    TunnelConnectorHeartbeatResponse, TunnelDesiredAttachment, TunnelNegotiateAttachmentsRequest,
    TunnelNegotiateAttachmentsResponse, HEADER_CONNECTOR_ID, HEADER_CONNECTOR_REVOKED,
    HEADER_QUARANTINED, HEADER_TUNNEL_TOKEN, PATH_TUNNEL_ATTACHMENTS, PATH_TUNNEL_CONFIG,
    PATH_TUNNEL_HEARTBEAT, PATH_TUNNEL_WS,
};
use openflare_tunnel_common::updater::TunnelUpdatePlan;
use openflare_tunnel_common::ws::{spawn_wake_loop, WakeSignal};
use openflare_tunnel_common::{CONTRACT_VERSION, TUNNEL_VERSION};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::{broadcast, mpsc, watch};

use crate::discovery::{probe_candidates, select_closest_k, RebalanceTracker};
use crate::ingress::IngressState;
use crate::rathole_pool::RatholeClientPool;

const DEFAULT_HEARTBEAT_INTERVAL: Duration = Duration::from_secs(15);
const PROBE_AND_RENEW_INTERVAL: Duration = Duration::from_secs(25);

pub fn spawn_wake_client(
    identity: &ConnectorIdentity,
    tx: mpsc::Sender<WakeSignal>,
    shutdown: broadcast::Receiver<bool>,
) {
    let Ok(ws_url) = http_to_ws_url(&identity.server_url, PATH_TUNNEL_WS) else {
        return;
    };
    spawn_wake_loop(
        ws_url,
        vec![
            (HEADER_TUNNEL_TOKEN.to_owned(), identity.tunnel_token.clone()),
            (HEADER_CONNECTOR_ID.to_owned(), identity.connector_id.clone()),
        ],
        tx,
        shutdown,
    );
}

#[allow(clippy::too_many_arguments)]
pub async fn run_control_loop(
    client: reqwest::Client,
    identity: ConnectorIdentity,
    ip_resolver: Arc<NodeIpResolver>,
    ingress_state: Arc<IngressState>,
    rathole_pool: Arc<RatholeClientPool>,
    update_tx: watch::Sender<Option<TunnelUpdatePlan>>,
    mut wake_rx: mpsc::Receiver<WakeSignal>,
    mut shutdown: broadcast::Receiver<bool>,
) -> Result<()> {
    let mut rebalance = RebalanceTracker::default();
    let mut last_cfg: Option<TunnelConnectorConfigResponse> = None;
    let mut applied_revision = 0i64;
    let mut lease_ttl_secs = 90i64;
    let mut last_negotiate_at = Instant::now() - PROBE_AND_RENEW_INTERVAL;
    let mut last_probe_rtts: HashMap<u64, i64> = HashMap::new();
    let mut last_error = String::new();

    if let Err(err) = sync_and_reconcile(
        &client,
        &identity,
        &ingress_state,
        &rathole_pool,
        &mut rebalance,
        &mut last_cfg,
        &mut applied_revision,
        &mut lease_ttl_secs,
        &mut last_probe_rtts,
        &mut last_negotiate_at,
    )
    .await
    {
        if is_terminal_error(&err) {
            return Err(err);
        }
        last_error = err.to_string();
        tracing::warn!(%err, "initial tunnel config sync deferred to retry loop");
    }

    let mut hb_interval = DEFAULT_HEARTBEAT_INTERVAL;

    loop {
        tokio::select! {
            _ = shutdown.recv() => return Ok(()),
            wake = wake_rx.recv() => {
                match wake {
                    Some(WakeSignal::ForceDisconnect) => {
                        anyhow::bail!("connector received remote revoke/quarantine disconnect signal");
                    }
                    Some(WakeSignal::ConfigSync) => {
                        match sync_and_reconcile(
                            &client,
                            &identity,
                            &ingress_state,
                            &rathole_pool,
                            &mut rebalance,
                            &mut last_cfg,
                            &mut applied_revision,
                            &mut lease_ttl_secs,
                            &mut last_probe_rtts,
                            &mut last_negotiate_at,
                        ).await {
                            Ok(()) => last_error.clear(),
                            Err(err) => {
                                if is_terminal_error(&err) {
                                    return Err(err);
                                }
                                last_error = err.to_string();
                                tracing::warn!(%err, "wake config sync failed");
                            }
                        }
                    }
                    None => {}
                }
            }
            _ = tokio::time::sleep(hb_interval) => {
                let ip = ip_resolver.resolve_now().await;
                let connected_relays = rathole_pool.connected_reports(&last_probe_rtts).await;
                match send_heartbeat(
                    &client,
                    &identity,
                    &ip,
                    applied_revision,
                    &last_error,
                    connected_relays,
                ).await {
                    Ok(hb) => {
                        if hb.revoked {
                            anyhow::bail!("connector has been revoked by control plane");
                        }
                        if hb.quarantined {
                            anyhow::bail!("tunnel is quarantined by control plane: {}", hb.quarantine_reason);
                        }
                        if hb.heartbeat_interval > 0 {
                            let ms = hb.heartbeat_interval.clamp(5_000, 120_000);
                            hb_interval = Duration::from_millis(ms);
                        }
                        if let Some(settings) = hb.update_settings {
                            let next_plan = (settings.auto_update || settings.tag.is_some())
                                .then_some(TunnelUpdatePlan {
                                    repo: settings.repo,
                                    tag: settings.tag,
                                    binary_name: "openflared".to_owned(),
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
                        if hb.disabled {
                            rathole_pool.shutdown_all().await;
                        }
                        let lease_renew_due = last_negotiate_at.elapsed().as_secs() >= (lease_ttl_secs.max(20) as u64 / 2);
                        let should_sync = hb.force_sync
                            || applied_revision != hb.config_revision
                            || last_negotiate_at.elapsed() >= PROBE_AND_RENEW_INTERVAL
                            || lease_renew_due;
                        if should_sync {
                            match sync_and_reconcile(
                                &client,
                                &identity,
                                &ingress_state,
                                &rathole_pool,
                                &mut rebalance,
                                &mut last_cfg,
                                &mut applied_revision,
                                &mut lease_ttl_secs,
                                &mut last_probe_rtts,
                                &mut last_negotiate_at,
                            ).await {
                                Ok(()) => last_error.clear(),
                                Err(err) => {
                                    if is_terminal_error(&err) {
                                        return Err(err);
                                    }
                                    last_error = err.to_string();
                                    tracing::warn!(%err, "periodic config reconcile failed");
                                }
                            }
                        }
                    }
                    Err(err) => {
                        if is_terminal_error(&err) {
                            return Err(err);
                        }
                        tracing::warn!(%err, "connector heartbeat failed");
                    }
                }
            }
        }
    }
}

#[allow(clippy::too_many_arguments)]
async fn sync_and_reconcile(
    client: &reqwest::Client,
    identity: &ConnectorIdentity,
    ingress_state: &IngressState,
    rathole_pool: &RatholeClientPool,
    rebalance: &mut RebalanceTracker,
    last_cfg: &mut Option<TunnelConnectorConfigResponse>,
    applied_revision: &mut i64,
    lease_ttl_secs: &mut i64,
    last_probe_rtts: &mut HashMap<u64, i64>,
    last_negotiate_at: &mut Instant,
) -> Result<()> {
    let mut cfg = fetch_connector_config(client, identity).await?;
    ingress_state
        .update_config(
            &cfg.routes,
            &cfg.allowed_target_cidrs,
            &cfg.blocked_target_cidrs,
        )
        .await?;

    if !cfg.enabled {
        rathole_pool.shutdown_all().await;
        *applied_revision = cfg.config_revision;
        *last_cfg = Some(cfg);
        return Ok(());
    }

    if !cfg.candidate_relays.is_empty() {
        let proxy_opt = (!cfg.client_proxy_url.trim().is_empty()).then_some(cfg.client_proxy_url.as_str());
        let probed = probe_candidates(
            client,
            &cfg.candidate_relays,
            &cfg.preferred_transport,
            &cfg.fallback_transports,
            proxy_opt,
        )
        .await;
        for sample in &probed {
            if let Some(rtt) = sample.rtt_ms {
                last_probe_rtts.insert(sample.relay.id, rtt);
            }
        }
        let current_attached: HashMap<u64, String> = cfg
            .attachments
            .iter()
            .filter(|a| a.state != "stale")
            .map(|a| (a.relay_id, a.transport.clone()))
            .collect();
        let target_k = cfg.ha_relay_count.clamp(1, 5);
        let desired_selection = select_closest_k(&probed, target_k);
        let final_desired = rebalance.decide(&probed, &current_attached, &desired_selection, target_k);

        let lease_renew_due = last_negotiate_at.elapsed().as_secs() >= (*lease_ttl_secs as u64 / 2);
        let set_changed = !same_attachment_set(&current_attached, &final_desired);

        if !final_desired.is_empty() && (set_changed || lease_renew_due || cfg.attachments.is_empty()) {
            let negotiated = negotiate_attachments(client, identity, final_desired).await?;
            *lease_ttl_secs = negotiated.lease_ttl_seconds.max(20);
            cfg.attachments = negotiated.attachments;
            *last_negotiate_at = Instant::now();
        }
    }

    rathole_pool.reconcile(&cfg).await?;
    *applied_revision = cfg.config_revision;
    *last_cfg = Some(cfg);
    Ok(())
}

fn same_attachment_set(
    current: &HashMap<u64, String>,
    desired: &[TunnelDesiredAttachment],
) -> bool {
    if current.len() != desired.len() {
        return false;
    }
    desired
        .iter()
        .all(|item| current.get(&item.relay_id) == Some(&item.transport))
}

fn check_terminal_status(resp: &reqwest::Response) -> Result<()> {
    if resp.status() == reqwest::StatusCode::FORBIDDEN {
        let headers = resp.headers();
        if headers.get(HEADER_QUARANTINED).and_then(|v| v.to_str().ok()) == Some("true") {
            anyhow::bail!("TERMINAL: tunnel is quarantined");
        }
        if headers
            .get(HEADER_CONNECTOR_REVOKED)
            .and_then(|v| v.to_str().ok())
            == Some("true")
        {
            anyhow::bail!("TERMINAL: connector is revoked");
        }
    }
    Ok(())
}

fn is_terminal_error(err: &anyhow::Error) -> bool {
    err.to_string().contains("TERMINAL:")
}

async fn fetch_connector_config(
    client: &reqwest::Client,
    identity: &ConnectorIdentity,
) -> Result<TunnelConnectorConfigResponse> {
    let url = format!(
        "{}{PATH_TUNNEL_CONFIG}",
        identity.server_url.trim_end_matches('/')
    );
    let resp = client
        .get(url)
        .query(&[("connector_id", identity.connector_id.as_str())])
        .header(HEADER_TUNNEL_TOKEN, &identity.tunnel_token)
        .header(HEADER_CONNECTOR_ID, &identity.connector_id)
        .send()
        .await?;
    check_terminal_status(&resp)?;
    let resp = resp.error_for_status().context("fetch connector config")?;
    let api_resp: ApiResponse<TunnelConnectorConfigResponse> = resp.json().await?;
    api_resp.into_result()
}

async fn negotiate_attachments(
    client: &reqwest::Client,
    identity: &ConnectorIdentity,
    desired: Vec<TunnelDesiredAttachment>,
) -> Result<TunnelNegotiateAttachmentsResponse> {
    let url = format!(
        "{}{PATH_TUNNEL_ATTACHMENTS}",
        identity.server_url.trim_end_matches('/')
    );
    let hostname = std::env::var("HOSTNAME").ok().filter(|v| !v.trim().is_empty());
    let req_body = TunnelNegotiateAttachmentsRequest {
        connector_id: identity.connector_id.clone(),
        hostname,
        os_arch: Some(format!("{}/{}", std::env::consts::OS, std::env::consts::ARCH)),
        version: Some(TUNNEL_VERSION.to_owned()),
        engine_version: Some("rathole-0.5".to_owned()),
        contract_version: CONTRACT_VERSION,
        capabilities: vec!["noise".into(), "websocket".into(), "tcp".into(), "http-ingress".into()],
        desired,
        ..Default::default()
    };
    let resp = client
        .post(url)
        .header(HEADER_TUNNEL_TOKEN, &identity.tunnel_token)
        .header(HEADER_CONNECTOR_ID, &identity.connector_id)
        .json(&req_body)
        .send()
        .await?;
    check_terminal_status(&resp)?;
    let resp = resp.error_for_status().context("negotiate tunnel attachments")?;
    let api_resp: ApiResponse<TunnelNegotiateAttachmentsResponse> = resp.json().await?;
    api_resp.into_result()
}

async fn send_heartbeat(
    client: &reqwest::Client,
    identity: &ConnectorIdentity,
    ip: &str,
    applied_revision: i64,
    last_error: &str,
    connected_relays: Vec<openflare_tunnel_common::protocol::TunnelConnectedRelayReport>,
) -> Result<TunnelConnectorHeartbeatResponse> {
    let url = format!(
        "{}{PATH_TUNNEL_HEARTBEAT}",
        identity.server_url.trim_end_matches('/')
    );
    let hostname = std::env::var("HOSTNAME").ok().filter(|v| !v.trim().is_empty());
    let req_body = TunnelConnectorHeartbeatPayload {
        connector_id: identity.connector_id.clone(),
        hostname,
        os_arch: Some(format!("{}/{}", std::env::consts::OS, std::env::consts::ARCH)),
        public_ip: (ip != UNSPECIFIED_IP).then(|| ip.to_owned()),
        version: TUNNEL_VERSION.to_owned(),
        engine_version: Some("rathole-0.5".to_owned()),
        contract_version: CONTRACT_VERSION,
        capabilities: vec!["noise".into(), "websocket".into(), "tcp".into(), "http-ingress".into()],
        applied_revision,
        draining: false,
        last_error: (!last_error.is_empty()).then(|| last_error.to_owned()),
        applied_update_tag: Some(TUNNEL_VERSION.to_owned()),
        connected_relays,
    };
    let resp = client
        .post(url)
        .header(HEADER_TUNNEL_TOKEN, &identity.tunnel_token)
        .header(HEADER_CONNECTOR_ID, &identity.connector_id)
        .json(&req_body)
        .send()
        .await?;
    check_terminal_status(&resp)?;
    let resp = resp.error_for_status().context("connector heartbeat")?;
    let api_resp: ApiResponse<TunnelConnectorHeartbeatResponse> = resp.json().await?;
    api_resp.into_result()
}
