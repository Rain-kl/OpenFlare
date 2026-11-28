use crate::config::{DNS_VERSION, ServeArgs};
use crate::metrics::DnsMetrics;
use crate::protocol::{
    ApiEnvelope, HeartbeatResponse, RegisterRequest, RegisterResponse, WsWakeMessage,
};
use crate::snapshot::{SnapshotStore, pull_snapshot};
use crate::update::RemoteUpdate;
use anyhow::{Context, Result};
use futures_util::StreamExt;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::Notify;
use tokio_tungstenite::connect_async;
use tokio_tungstenite::tungstenite::client::IntoClientRequest;
use tokio_tungstenite::tungstenite::http::HeaderValue;
use tracing::{info, warn};

const IDENTITY_FILE_NAME: &str = "identity.json";

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PersistedIdentity {
    pub instance_id: String,
    pub access_token: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BootstrapToken {
    Instance(String),
    Discovery(String),
}

pub fn resolve_bootstrap_token(args: &ServeArgs) -> Result<BootstrapToken> {
    if let Some(tok) = args
        .token
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
    {
        return Ok(BootstrapToken::Instance(tok.to_string()));
    }

    if let Some(path) = args
        .token_file
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
    {
        let raw = fs::read_to_string(path)
            .with_context(|| format!("failed to read token file {path:?}"))?;
        let trimmed = raw.trim().to_string();
        anyhow::ensure!(!trimmed.is_empty(), "token file {path:?} is empty");
        return match args.token_type.trim().to_ascii_lowercase().as_str() {
            "discovery" | "" => Ok(BootstrapToken::Discovery(trimmed)),
            "instance" | "access" => Ok(BootstrapToken::Instance(trimmed)),
            other => anyhow::bail!(
                "unsupported --token-type {other:?} (expected 'discovery' or 'instance')"
            ),
        };
    }

    if let Some(disc) = args
        .discovery_token
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
    {
        return Ok(BootstrapToken::Discovery(disc.to_string()));
    }

    anyhow::bail!("either --token, --token-file, or --discovery-token must be provided")
}

fn identity_path(cache_dir: &str) -> Option<PathBuf> {
    let trimmed = cache_dir.trim();
    if trimmed.is_empty() {
        return None;
    }
    Some(Path::new(trimmed).join(IDENTITY_FILE_NAME))
}

pub fn load_saved_identity(cache_dir: &str) -> Option<PersistedIdentity> {
    let path = identity_path(cache_dir)?;
    let raw = fs::read_to_string(&path).ok()?;
    let parsed: PersistedIdentity = serde_json::from_str(&raw).ok()?;
    if parsed.instance_id.trim().is_empty() || parsed.access_token.trim().is_empty() {
        return None;
    }
    Some(parsed)
}

pub fn save_identity(cache_dir: &str, identity: &PersistedIdentity) -> Result<()> {
    let Some(path) = identity_path(cache_dir) else {
        return Ok(());
    };
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create cache dir {:?}", parent))?;
    }
    let json = serde_json::to_vec_pretty(identity)?;
    let tmp = path.with_extension("json.tmp");
    fs::write(&tmp, &json).with_context(|| format!("write temp identity {:?}", tmp))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&tmp, fs::Permissions::from_mode(0o600))
            .with_context(|| format!("chmod identity temp {:?}", tmp))?;
    }
    fs::rename(&tmp, &path).with_context(|| format!("rename identity to {:?}", path))?;
    Ok(())
}

async fn send_register_request(
    client: &Client,
    args: &ServeArgs,
    instance_id: Option<String>,
    header_name: &str,
    header_value: &str,
) -> Result<RegisterResponse> {
    let base = args.controller_url.trim_end_matches('/');
    let url = format!("{base}/api/v1/dns-agent/register");
    let body = RegisterRequest {
        instance_id,
        name: args.name.clone(),
        public_addr: args.public_addr.clone(),
        listen_addr: args.listen.clone(),
        version: DNS_VERSION.to_string(),
    };
    let resp = client
        .post(&url)
        .header(header_name, header_value)
        .json(&body)
        .send()
        .await
        .context("register request failed")?;
    if !resp.status().is_success() {
        anyhow::bail!("register failed with HTTP {}", resp.status());
    }
    let env: ApiEnvelope<RegisterResponse> = resp.json().await?;
    env.data.context("empty registration data")
}

pub async fn register_if_needed(client: &Client, args: &ServeArgs) -> Result<String> {
    let bootstrap = resolve_bootstrap_token(args)?;
    let disc = match bootstrap {
        BootstrapToken::Instance(tok) => return Ok(tok),
        BootstrapToken::Discovery(disc) => disc,
    };

    if let Some(saved) = load_saved_identity(&args.cache_dir) {
        let preferred_id = args
            .instance_id
            .clone()
            .filter(|s| !s.trim().is_empty())
            .or_else(|| Some(saved.instance_id.clone()));
        match send_register_request(
            client,
            args,
            preferred_id,
            "X-DNS-Token",
            &saved.access_token,
        )
        .await
        {
            Ok(data) => {
                let updated = PersistedIdentity {
                    instance_id: data.instance_id.clone(),
                    access_token: data.access_token.clone(),
                };
                if let Err(err) = save_identity(&args.cache_dir, &updated) {
                    warn!("failed to persist refreshed SmartDNS identity: {}", err);
                }
                info!(
                    "resumed persisted SmartDNS instance_id={} (id={})",
                    data.instance_id, data.id
                );
                return Ok(data.access_token);
            }
            Err(err) => {
                warn!(
                    "persisted SmartDNS identity resume failed ({}); re-registering via discovery token",
                    err
                );
            }
        }
    }

    let data = send_register_request(
        client,
        args,
        args.instance_id.clone(),
        "X-DNS-Discovery-Token",
        &disc,
    )
    .await?;
    let persisted = PersistedIdentity {
        instance_id: data.instance_id.clone(),
        access_token: data.access_token.clone(),
    };
    if let Err(err) = save_identity(&args.cache_dir, &persisted) {
        warn!("failed to persist SmartDNS identity: {}", err);
    }
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
    remote_update: Option<RemoteUpdate>,
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
                    let mut hb = metrics.snapshot_heartbeat(&listen_addr, &public_addr, ver, &etag, &last_err);
                    if let Some(updater) = &remote_update {
                        hb.remote_update_supported = true;
                        hb.update_receipt = updater.receipt().await;
                    }
                    let base = controller_url.trim_end_matches('/');
                    let url = format!("{base}/api/v1/dns-agent/heartbeat");
                    match client.post(&url).header("X-DNS-Token", &token).json(&hb).send().await {
                        Ok(resp) if resp.status().is_success() => {
                            if let Ok(env) = resp.json::<ApiEnvelope<HeartbeatResponse>>().await {
                                if let Some(d) = env.data {
                                    if let (Some(updater), Some(command)) = (&remote_update, d.update) {
                                        if let Err(err) = updater.queue(command).await {
                                            warn!("could not queue remote update: {err}");
                                        }
                                    }
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
                                        warn!(
                                            "received dns_disconnect from controller: {:?}",
                                            ev.payload
                                        );
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
