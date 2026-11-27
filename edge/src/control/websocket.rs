use anyhow::{Context, Result};
use futures_util::{SinkExt, StreamExt};
use openflare_probe::{
    execute_command, ExecOptions, ExecRequest, NetProbeRequest, NetProbeResult, PingRequest,
    PingResult, ProbeSupervisor, ProbeSupervisorConfig,
};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::{
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    },
    time::Duration,
};
use tokio::sync::{broadcast, mpsc, Notify, Semaphore};
use tokio::time::Instant;
use tokio_tungstenite::{
    connect_async,
    tungstenite::{client::IntoClientRequest, Message},
};

const MAX_CONCURRENT_PINGS: usize = 16;
const MAX_CONCURRENT_NET_PROBES: usize = 8;
const MAX_CONCURRENT_EXECS: usize = 4;
const WS_OUTBOUND_BUFFER: usize = 64;
const LIVE_LOG_BATCH: usize = 8;
const LIVE_LOG_TICK: Duration = Duration::from_millis(100);
const LIVE_LOG_LEASE: Duration = Duration::from_secs(45);

#[derive(Default)]
pub(super) struct Wake {
    notify: Notify,
    force: AtomicBool,
}

impl Wake {
    pub(super) async fn changed(&self) {
        self.notify.notified().await;
    }

    pub(super) fn requeue_force(&self) {
        self.force.store(true, Ordering::Release);
    }

    pub(super) fn take_force(&self) -> bool {
        self.force.swap(false, Ordering::AcqRel)
    }
}

pub(super) async fn run(
    base_url: &str,
    token: &str,
    client: &Client,
    wake: &Wake,
    keys: &crate::config::CertificateKeyRing,
) {
    let mut retry = super::RetryDelay::default();
    let ping_limiter = Arc::new(Semaphore::new(MAX_CONCURRENT_PINGS));
    let net_probe_limiter = Arc::new(Semaphore::new(MAX_CONCURRENT_NET_PROBES));
    let exec_limiter = Arc::new(Semaphore::new(MAX_CONCURRENT_EXECS));
    let probe_supervisor = ProbeSupervisorConfig::for_current_exe_subcommand("probe-worker")
        .map(|mut cfg| {
            cfg.queue_capacity = 64;
            ProbeSupervisor::start(cfg)
        });
    loop {
        if let Err(error) = connect(
            base_url,
            token,
            client,
            wake,
            keys,
            &mut retry,
            Arc::clone(&ping_limiter),
            Arc::clone(&net_probe_limiter),
            Arc::clone(&exec_limiter),
            probe_supervisor.clone(),
        )
        .await
        {
            tracing::warn!(%error, "control WebSocket disconnected");
        }
        retry.wait().await;
    }
}

#[derive(Deserialize)]
struct InboundMessage {
    #[serde(rename = "type")]
    kind: String,
    payload: Option<serde_json::Value>,
}

#[derive(Deserialize)]
struct ConfigChanged {
    #[serde(default)]
    force: bool,
}

#[derive(Serialize)]
struct OutboundMessage<T: Serialize> {
    #[serde(rename = "type")]
    kind: &'static str,
    payload: T,
}

#[derive(Deserialize)]
struct LogSubscription {
    stream_id: String,
}

#[derive(Serialize)]
struct LogFrame<'a> {
    stream_id: &'a str,
    ready: bool,
    lines: Vec<String>,
    dropped: u64,
}

struct LiveLogs {
    stream_id: String,
    expires: Instant,
    receiver: broadcast::Receiver<String>,
}

impl LiveLogs {
    fn start(stream_id: String) -> Self {
        Self {
            stream_id,
            expires: Instant::now() + LIVE_LOG_LEASE,
            receiver: crate::telemetry::runtime_logs::subscribe(),
        }
    }

    fn batch(&mut self) -> Option<LogFrame<'_>> {
        let mut lines = Vec::new();
        let mut dropped = 0;
        for _ in 0..LIVE_LOG_BATCH {
            match self.receiver.try_recv() {
                Ok(line) => lines.push(line),
                Err(broadcast::error::TryRecvError::Lagged(count)) => dropped += count,
                Err(_) => break,
            }
        }
        if lines.is_empty() && dropped == 0 {
            return None;
        }
        Some(LogFrame {
            stream_id: &self.stream_id,
            ready: false,
            lines,
            dropped,
        })
    }
}

#[allow(clippy::too_many_arguments)]
async fn connect(
    base_url: &str,
    token: &str,
    http_client: &Client,
    wake: &Wake,
    keys: &crate::config::CertificateKeyRing,
    retry: &mut super::RetryDelay,
    ping_limiter: Arc<Semaphore>,
    net_probe_limiter: Arc<Semaphore>,
    exec_limiter: Arc<Semaphore>,
    probe_supervisor: Option<ProbeSupervisor>,
) -> Result<()> {
    let mut url = url::Url::parse(&format!("{base_url}/api/v1/agent/ws"))?;
    let scheme = if url.scheme() == "https" { "wss" } else { "ws" };
    url.set_scheme(scheme)
        .map_err(|_| anyhow::anyhow!("invalid WebSocket URL"))?;
    let mut request = url.as_str().into_client_request()?;
    request
        .headers_mut()
        .insert("X-Agent-Token", token.parse()?);
    let (socket, _) =
        tokio::time::timeout(Duration::from_secs(15), connect_async(request)).await??;
    *retry = super::RetryDelay::default();
    wake.notify.notify_one();

    let (mut sink, mut stream) = socket.split();
    let (outbound_tx, mut outbound_rx) = mpsc::channel::<String>(WS_OUTBOUND_BUFFER);
    let mut keepalive = tokio::time::interval(Duration::from_secs(20));
    keepalive.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
    let mut live_logs: Option<LiveLogs> = None;
    let mut log_tick = tokio::time::interval(LIVE_LOG_TICK);
    log_tick.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);

    loop {
        tokio::select! {
            message = stream.next() => {
                match message.context("control WebSocket closed")?? {
                    Message::Text(text) => {
                        let Ok(message) = serde_json::from_str::<InboundMessage>(&text) else {
                            continue;
                        };
                        match message.kind.as_str() {
                            "config_changed" => {
                                if let Some(payload) = message.payload {
                                    if let Ok(changed) = serde_json::from_value::<ConfigChanged>(payload) {
                                        if changed.force {
                                            wake.requeue_force();
                                        }
                                    }
                                }
                                wake.notify.notify_one();
                            }
                            "quarantined" => {
                                tracing::warn!("received node quarantine notification over websocket; wiping memory TLS keys");
                                keys.clear();
                                return Ok(());
                            }
                            "ping" => {
                                let _ = outbound_tx.try_send(r#"{"type":"pong"}"#.to_owned());
                            }
                            "ping_task" => {
                                if let Some(payload) = message.payload {
                                    match serde_json::from_value::<PingRequest>(payload) {
                                        Ok(req) => {
                                            spawn_ping_task(
                                                req,
                                                base_url.to_owned(),
                                                token.to_owned(),
                                                http_client.clone(),
                                                outbound_tx.clone(),
                                                Arc::clone(&ping_limiter),
                                                probe_supervisor.clone(),
                                            );
                                        }
                                        Err(error) => {
                                            tracing::warn!(%error, "invalid ping_task payload");
                                        }
                                    }
                                }
                            }
                            "net_probe_task" => {
                                if let Some(payload) = message.payload {
                                    match serde_json::from_value::<NetProbeRequest>(payload) {
                                        Ok(req) => {
                                            spawn_net_probe_task(
                                                req,
                                                base_url.to_owned(),
                                                token.to_owned(),
                                                http_client.clone(),
                                                outbound_tx.clone(),
                                                Arc::clone(&net_probe_limiter),
                                                probe_supervisor.clone(),
                                            );
                                        }
                                        Err(error) => {
                                            tracing::warn!(%error, "invalid net_probe_task payload");
                                        }
                                    }
                                }
                            }
                            "runtime_log_subscribe" => {
                                if let Some(subscription) = message.payload.and_then(|p| serde_json::from_value::<LogSubscription>(p).ok()) {
                                    if let Some(live) = live_logs.as_mut().filter(|live| live.stream_id == subscription.stream_id) {
                                        live.expires = Instant::now() + LIVE_LOG_LEASE;
                                        continue;
                                    }
                                    let live = LiveLogs::start(subscription.stream_id);
                                    let frame = LogFrame { stream_id: &live.stream_id, ready: true, lines: Vec::new(), dropped: 0 };
                                    let text = serde_json::to_string(&OutboundMessage { kind: "runtime_log_frame", payload: frame })?;
                                    live_logs = Some(live);
                                    tokio::time::timeout(Duration::from_secs(5), sink.send(Message::Text(text.into()))).await??;
                                }
                            }
                            "runtime_log_unsubscribe" => {
                                if let Some(subscription) = message.payload.and_then(|p| serde_json::from_value::<LogSubscription>(p).ok()) {
                                    if live_logs.as_ref().is_some_and(|live| live.stream_id == subscription.stream_id) {
                                        live_logs = None;
                                    }
                                }
                            }
                            "remote_exec" => {
                                if let Some(payload) = message.payload {
                                    match serde_json::from_value::<ExecRequest>(payload) {
                                        Ok(req) => {
                                            spawn_exec_task(
                                                req,
                                                base_url.to_owned(),
                                                token.to_owned(),
                                                http_client.clone(),
                                                Arc::clone(&exec_limiter),
                                            );
                                        }
                                        Err(error) => {
                                            tracing::warn!(%error, "invalid remote_exec payload");
                                        }
                                    }
                                }
                            }
                            _ => {}
                        }
                    }
                    Message::Ping(_) => sink.flush().await?,
                    Message::Close(_) => return Ok(()),
                    _ => {}
                }
            }
            Some(outbound) = outbound_rx.recv() => {
                tokio::time::timeout(
                    Duration::from_secs(5),
                    sink.send(Message::Text(outbound.into())),
                ).await??;
            }
            _ = log_tick.tick(), if live_logs.is_some() => {
                if let Some(live) = live_logs.as_mut() {
                    if live.expires <= Instant::now() {
                        live_logs = None;
                    } else if let Some(frame) = live.batch() {
                        let text = serde_json::to_string(&OutboundMessage { kind: "runtime_log_frame", payload: frame })?;
                        tokio::time::timeout(Duration::from_secs(5), sink.send(Message::Text(text.into()))).await??;
                    }
                }
            }
            _ = keepalive.tick() => {
                tokio::time::timeout(
                    Duration::from_secs(5),
                    sink.send(Message::Text(r#"{"type":"ping"}"#.into())),
                ).await??;
            }
        }
    }
}

fn spawn_ping_task(
    req: PingRequest,
    base_url: String,
    token: String,
    http_client: Client,
    outbound_tx: mpsc::Sender<String>,
    limiter: Arc<Semaphore>,
    supervisor: Option<ProbeSupervisor>,
) {
    tokio::spawn(async move {
        let task_id = req.ping_task_id;
        let Ok(_permit) = limiter.try_acquire_owned() else {
            tracing::warn!(
                task_id,
                "dropping ping_task due to concurrency limit"
            );
            return;
        };
        let result = match supervisor {
            Some(sup) => sup.dispatch_ping(req).await,
            None => PingResult {
                task_id,
                ping_type: req.ping_type,
                value: -1,
                finished_at: String::new(),
            },
        };
        let frame = serde_json::to_string(&OutboundMessage {
            kind: "ping_result",
            payload: &result,
        });
        if let Ok(text) = frame {
            if outbound_tx.send(text).await.is_ok() {
                return;
            }
        }
        // Fallback to HTTP callback if the WebSocket session rotated while probing.
        if let Err(error) = http_client
            .post(format!("{base_url}/api/v1/agent/ping/result"))
            .header("X-Agent-Token", &token)
            .json(&result)
            .send()
            .await
        {
            tracing::warn!(task_id, %error, "failed to report ping result via HTTP fallback");
        }
    });
}

fn spawn_net_probe_task(
    req: NetProbeRequest,
    base_url: String,
    token: String,
    http_client: Client,
    outbound_tx: mpsc::Sender<String>,
    limiter: Arc<Semaphore>,
    supervisor: Option<ProbeSupervisor>,
) {
    tokio::spawn(async move {
        let probe_id = req.probe_id.clone();
        let kind = req.kind.clone();
        let Ok(_permit) = limiter.try_acquire_owned() else {
            tracing::warn!(probe_id = %probe_id, "dropping net_probe_task due to concurrency limit");
            return;
        };
        let result = match supervisor {
            Some(sup) => sup.dispatch_net_probe(req).await,
            None => NetProbeResult {
                probe_id: probe_id.clone(),
                kind,
                ping: None,
                traceroute: None,
                dns: None,
                error: Some("probe supervisor unavailable".to_owned()),
                finished_at: String::new(),
            },
        };
        let frame = serde_json::to_string(&OutboundMessage {
            kind: "net_probe_result",
            payload: &result,
        });
        if let Ok(text) = frame {
            if outbound_tx.send(text).await.is_ok() {
                return;
            }
        }
        if let Err(error) = http_client
            .post(format!("{base_url}/api/v1/agent/net-probe/result"))
            .header("X-Agent-Token", &token)
            .json(&result)
            .send()
            .await
        {
            tracing::warn!(probe_id = %probe_id, %error, "failed to report net probe result via HTTP fallback");
        }
    });
}

fn spawn_exec_task(
    req: ExecRequest,
    base_url: String,
    token: String,
    http_client: Client,
    limiter: Arc<Semaphore>,
) {
    tokio::spawn(async move {
        let Ok(_permit) = limiter.try_acquire_owned() else {
            tracing::warn!(task_id = %req.task_id, "rejecting remote_exec due to concurrency limit");
            let busy = openflare_probe::ExecResult {
                task_id: req.task_id.clone(),
                result: "node remote exec concurrency limit reached".to_owned(),
                exit_code: -1,
                finished_at: String::new(),
            };
            let _ = http_client
                .post(format!("{base_url}/api/v1/agent/exec/result"))
                .header("X-Agent-Token", &token)
                .json(&busy)
                .send()
                .await;
            return;
        };
        let result = execute_command(&req, &ExecOptions::default()).await;
        match http_client
            .post(format!("{base_url}/api/v1/agent/exec/result"))
            .header("X-Agent-Token", &token)
            .json(&result)
            .send()
            .await
        {
            Ok(resp) if resp.status().is_success() => {}
            Ok(resp) => {
                tracing::warn!(
                    task_id = %req.task_id,
                    status = %resp.status(),
                    "controller rejected remote exec result"
                );
            }
            Err(error) => {
                tracing::warn!(task_id = %req.task_id, %error, "failed to report remote exec result");
            }
        }
    });
}
