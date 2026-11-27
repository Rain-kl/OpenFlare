use crate::config::ServeArgs;
use crate::metrics::DnsMetrics;
use crate::protocol::{
    ApiEnvelope, HeartbeatResponse, RegisterRequest, RegisterResponse, WsWakeMessage,
};
use crate::snapshot::{SnapshotStore, pull_snapshot};
use anyhow::{Context, Result};
use futures_util::StreamExt;
use reqwest::Client;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::Notify;
use tokio_tungstenite::connect_async;
use tokio_tungstenite::tungstenite::client::IntoClientRequest;
use tokio_tungstenite::tungstenite::http::HeaderValue;
use tracing::{info, warn};

pub async fn register_if_needed(client: &Client, args: &ServeArgs) -> Result<String> {
    if let Some(ref tok) = args.token {
        let trimmed = tok.trim();
        if !trimmed.is_empty() {
            return Ok(trimmed.to_string());
        }
    }
    let disc = args
        .discovery_token
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .context("either --token or --discovery-token must be provided")?;

    let base = args.controller_url.trim_end_matches('/');
    let url = format!("{base}/api/v1/dns-agent/register");
    let body = RegisterRequest {
        instance_id: args.instance_id.clone(),
        name: args.name.clone(),
        public_addr: args.public_addr.clone(),
        listen_addr: args.listen.clone(),
        version: env!("CARGO_PKG_VERSION").to_string(),
    };
    let resp = client
        .post(&url)
        .header("X-DNS-Discovery-Token", disc)
        .json(&body)
        .send()
        .await
        .context("register request failed")?;
    if !resp.status().is_success() {
        anyhow::bail!("register failed with HTTP {}", resp.status());
    }
    let env: ApiEnvelope<RegisterResponse> = resp.json().await?;
    let data = env.data.context("empty registration data")?;
    info!(
        "registered SmartDNS instance_id={} (id={})",
        data.instance_id, data.id
    );
    Ok(data.access_token)
}

pub fn spawn_control_loops(
    client: Client,
    controller_url: String,
    token: String,
    listen_addr: String,
    public_addr: String,
    interval_secs: u64,
    store: Arc<SnapshotStore>,
    metrics: Arc<DnsMetrics>,
) {
    let wake = Arc::new(Notify::new());

    // 1. WebSocket wake listener for instant snapshot updates & disconnect commands.
    {
        let ws_controller = controller_url.clone();
        let ws_token = token.clone();
        let ws_wake = wake.clone();
        tokio::spawn(async move {
            run_ws_loop(&ws_controller, &ws_token, ws_wake).await;
        });
    }

    // 2. Periodic snapshot polling + instant wake + 10s heartbeat loop.
    tokio::spawn(async move {
        let poll_secs = interval_secs.clamp(3, 300);
        let mut snap_tick = tokio::time::interval(Duration::from_secs(poll_secs));
        let mut hb_tick = tokio::time::interval(Duration::from_secs(10));
        let mut last_err = String::new();

        loop {
            tokio::select! {
                _ = wake.notified() => {
                    if let Err(err) = pull_snapshot(&client, &controller_url, &token, &store).await {
                        warn!("wake snapshot pull failed: {}", err);
                        last_err = err.to_string();
                    } else {
                        last_err.clear();
                    }
                }
                _ = snap_tick.tick() => {
                    match pull_snapshot(&client, &controller_url, &token, &store).await {
                        Ok(_) => last_err.clear(),
                        Err(err) => {
                            warn!("periodic snapshot pull failed (continuing with local cache): {}", err);
                            last_err = err.to_string();
                        }
                    }
                }
                _ = hb_tick.tick() => {
                    let (ver, etag) = store.current_version_and_etag();
                    let hb = metrics.snapshot_heartbeat(&listen_addr, &public_addr, ver, &etag, &last_err);
                    let base = controller_url.trim_end_matches('/');
                    let url = format!("{base}/api/v1/dns-agent/heartbeat");
                    match client.post(&url).header("X-DNS-Token", &token).json(&hb).send().await {
                        Ok(resp) if resp.status().is_success() => {
                            if let Ok(env) = resp.json::<ApiEnvelope<HeartbeatResponse>>().await {
                                if let Some(d) = env.data {
                                    if d.snapshot_version > ver || (!d.snapshot_etag.is_empty() && d.snapshot_etag != etag) {
                                        ws_wake_notify(&wake);
                                    }
                                }
                            }
                        }
                        Ok(resp) => {
                            warn!("heartbeat returned HTTP {}", resp.status());
                        }
                        Err(err) => {
                            warn!("heartbeat failed: {}", err);
                        }
                    }
                }
            }
        }
    });
}

fn ws_wake_notify(wake: &Notify) {
    wake.notify_one();
}

async fn run_ws_loop(controller_url: &str, token: &str, wake: Arc<Notify>) {
    let base = controller_url.trim_end_matches('/');
    let ws_base = if let Some(rest) = base.strip_prefix("https://") {
        format!("wss://{rest}")
    } else if let Some(rest) = base.strip_prefix("http://") {
        format!("ws://{rest}")
    } else {
        format!("ws://{base}")
    };
    let ws_url = format!("{ws_base}/api/v1/dns-agent/ws");
    let mut backoff = Duration::from_secs(1);

    loop {
        let mut req = match ws_url.as_str().into_client_request() {
            Ok(r) => r,
            Err(err) => {
                warn!("invalid ws url {}: {}", ws_url, err);
                tokio::time::sleep(Duration::from_secs(10)).await;
                continue;
            }
        };
        if let Ok(val) = HeaderValue::from_str(token) {
            req.headers_mut().insert("X-DNS-Token", val);
        }

        match connect_async(req).await {
            Ok((mut stream, _)) => {
                info!("connected SmartDNS control WebSocket");
                backoff = Duration::from_secs(1);
                while let Some(msg_res) = stream.next().await {
                    match msg_res {
                        Ok(msg) if msg.is_text() => {
                            if let Ok(text) = msg.to_text() {
                                if let Ok(ev) = serde_json::from_str::<WsWakeMessage>(text) {
                                    if ev.msg_type == "dns_snapshot_changed" {
                                        wake.notify_one();
                                    } else if ev.msg_type == "dns_disconnect" {
                                        warn!("received dns_disconnect from controller: {:?}", ev.payload);
                                    }
                                }
                            }
                        }
                        Ok(_) => {}
                        Err(err) => {
                            warn!("ws read error: {}", err);
                            break;
                        }
                    }
                }
            }
            Err(err) => {
                warn!("ws connect error: {}", err);
            }
        }
        tokio::time::sleep(backoff).await;
        backoff = (backoff * 2).min(Duration::from_secs(30));
    }
}
