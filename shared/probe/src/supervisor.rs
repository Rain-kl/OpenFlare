use crate::net_probe::format_rfc3339_now;
use crate::types::{
    NetProbeRequest, NetProbeResult, PingRequest, PingResult, WorkerRequest, WorkerResponse,
};
use std::collections::HashMap;
use std::path::PathBuf;
use std::process::Stdio;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};
use tokio::process::{Child, ChildStdin, ChildStdout, Command};
use tokio::sync::{mpsc, oneshot, Mutex};

const DEFAULT_QUEUE_CAPACITY: usize = 64;
const DEFAULT_TASK_TIMEOUT: Duration = Duration::from_secs(50);
const DEFAULT_BACKOFF_MIN: Duration = Duration::from_secs(1);
const DEFAULT_BACKOFF_MAX: Duration = Duration::from_secs(30);
const HEALTHY_RESET_AGE: Duration = Duration::from_secs(60);

/// Configuration for [`ProbeSupervisor`], which offloads all active probing
/// to a supervised child OS process so proxy threads stay completely isolated.
#[derive(Debug, Clone)]
pub struct ProbeSupervisorConfig {
    pub program: PathBuf,
    pub args: Vec<String>,
    pub queue_capacity: usize,
    pub task_timeout: Duration,
    pub backoff_min: Duration,
    pub backoff_max: Duration,
}

impl ProbeSupervisorConfig {
    pub fn for_current_exe_subcommand(subcommand: &str) -> Option<Self> {
        let program = std::env::current_exe().ok()?;
        Some(Self {
            program,
            args: vec![subcommand.to_owned()],
            queue_capacity: DEFAULT_QUEUE_CAPACITY,
            task_timeout: DEFAULT_TASK_TIMEOUT,
            backoff_min: DEFAULT_BACKOFF_MIN,
            backoff_max: DEFAULT_BACKOFF_MAX,
        })
    }
}

enum DispatchItem {
    Ping {
        request_id: String,
        payload: PingRequest,
        reply: oneshot::Sender<PingResult>,
    },
    NetProbe {
        request_id: String,
        payload: NetProbeRequest,
        reply: oneshot::Sender<NetProbeResult>,
    },
}

enum PendingReply {
    Ping {
        ping_task_id: u64,
        ping_type: String,
        reply: oneshot::Sender<PingResult>,
        deadline: tokio::time::Instant,
    },
    NetProbe {
        probe_id: String,
        kind: String,
        reply: oneshot::Sender<NetProbeResult>,
        deadline: tokio::time::Instant,
    },
}

impl PendingReply {
    fn deadline(&self) -> tokio::time::Instant {
        match self {
            Self::Ping { deadline, .. } | Self::NetProbe { deadline, .. } => *deadline,
        }
    }

    fn resolve_with_error(self, message: &str) {
        match self {
            Self::Ping {
                ping_task_id,
                ping_type,
                reply,
                ..
            } => {
                let _ = reply.send(PingResult {
                    task_id: ping_task_id,
                    ping_type,
                    value: -1,
                    finished_at: format_rfc3339_now(),
                });
            }
            Self::NetProbe {
                probe_id,
                kind,
                reply,
                ..
            } => {
                let _ = reply.send(NetProbeResult {
                    probe_id,
                    kind,
                    ping: None,
                    traceroute: None,
                    dns: None,
                    error: Some(message.to_owned()),
                    finished_at: format_rfc3339_now(),
                });
            }
        }
    }
}

struct CommandSpec {
    program: PathBuf,
    args: Vec<String>,
}

/// Non-blocking supervisor managing an isolated `probe-worker` subprocess with lazy spawn,
/// crash recovery, exponential backoff, and bounded non-blocking dispatch.
#[derive(Clone)]
pub struct ProbeSupervisor {
    tx: mpsc::Sender<DispatchItem>,
    next_id: Arc<AtomicU64>,
    spec: Arc<Mutex<CommandSpec>>,
}

impl ProbeSupervisor {
    /// Starts the supervisor background loop inside the current Tokio runtime.
    pub fn start(config: ProbeSupervisorConfig) -> Self {
        let cap = config.queue_capacity.max(1);
        let (tx, rx) = mpsc::channel::<DispatchItem>(cap);
        let spec = Arc::new(Mutex::new(CommandSpec {
            program: config.program.clone(),
            args: config.args.clone(),
        }));
        let loop_spec = Arc::clone(&spec);
        tokio::spawn(async move {
            supervisor_loop(rx, config, loop_spec).await;
        });
        Self {
            tx,
            next_id: Arc::new(AtomicU64::new(1)),
            spec,
        }
    }

    /// Replaces the worker executable/args used on next child spawn (primarily for tests).
    pub async fn set_command(&self, program: PathBuf, args: Vec<String>) {
        let mut guard = self.spec.lock().await;
        guard.program = program;
        guard.args = args;
    }

    /// Dispatches a latency ping request to the isolated probe child without blocking the caller
    /// if the queue is full.
    pub async fn dispatch_ping(&self, req: PingRequest) -> PingResult {
        let req_id = format!("ping-{}", self.next_id.fetch_add(1, Ordering::Relaxed));
        let fallback_task_id = req.ping_task_id;
        let fallback_type = req.ping_type.clone();
        let (reply_tx, reply_rx) = oneshot::channel();
        if self
            .tx
            .try_send(DispatchItem::Ping {
                request_id: req_id,
                payload: req,
                reply: reply_tx,
            })
            .is_err()
        {
            return PingResult {
                task_id: fallback_task_id,
                ping_type: fallback_type,
                value: -1,
                finished_at: format_rfc3339_now(),
            };
        }

        match reply_rx.await {
            Ok(result) => result,
            Err(_) => PingResult {
                task_id: fallback_task_id,
                ping_type: fallback_type,
                value: -1,
                finished_at: format_rfc3339_now(),
            },
        }
    }

    /// Dispatches a network probe task (`ping_batch`, `traceroute`, `dns_query`) to the
    /// isolated probe child without blocking if the queue is full.
    pub async fn dispatch_net_probe(&self, req: NetProbeRequest) -> NetProbeResult {
        let req_id = format!("net-{}", self.next_id.fetch_add(1, Ordering::Relaxed));
        let fallback_id = req.probe_id.clone();
        let fallback_kind = req.kind.clone();
        let (reply_tx, reply_rx) = oneshot::channel();
        if self
            .tx
            .try_send(DispatchItem::NetProbe {
                request_id: req_id,
                payload: req,
                reply: reply_tx,
            })
            .is_err()
        {
            return NetProbeResult {
                probe_id: fallback_id,
                kind: fallback_kind,
                ping: None,
                traceroute: None,
                dns: None,
                error: Some("probe supervisor queue is full; request dropped".to_owned()),
                finished_at: format_rfc3339_now(),
            };
        }

        match reply_rx.await {
            Ok(result) => result,
            Err(_) => NetProbeResult {
                probe_id: fallback_id,
                kind: fallback_kind,
                ping: None,
                traceroute: None,
                dns: None,
                error: Some("probe supervisor channel closed unexpectedly".to_owned()),
                finished_at: format_rfc3339_now(),
            },
        }
    }
}

struct LiveWorker {
    child: Child,
    stdin: ChildStdin,
    lines: tokio::io::Lines<BufReader<ChildStdout>>,
    started_at: Instant,
}

async fn spawn_worker(spec: &Mutex<CommandSpec>) -> Result<LiveWorker, String> {
    let (program, args) = {
        let guard = spec.lock().await;
        (guard.program.clone(), guard.args.clone())
    };
    let mut cmd = Command::new(&program);
    cmd.args(&args)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit())
        .kill_on_drop(true);
    let mut child = cmd
        .spawn()
        .map_err(|err| format!("failed to spawn probe worker {}: {err}", program.display()))?;
    let stdin = child
        .stdin
        .take()
        .ok_or_else(|| "probe worker stdin handle missing".to_owned())?;
    let stdout = child
        .stdout
        .take()
        .ok_or_else(|| "probe worker stdout handle missing".to_owned())?;
    Ok(LiveWorker {
        child,
        stdin,
        lines: BufReader::new(stdout).lines(),
        started_at: Instant::now(),
    })
}

async fn supervisor_loop(
    mut rx: mpsc::Receiver<DispatchItem>,
    config: ProbeSupervisorConfig,
    spec: Arc<Mutex<CommandSpec>>,
) {
    let mut worker: Option<LiveWorker> = None;
    let mut pending: HashMap<String, PendingReply> = HashMap::new();
    let mut backoff = config.backoff_min.max(Duration::from_millis(50));
    let mut cooldown_until: Option<Instant> = None;
    let mut sweep = tokio::time::interval(Duration::from_secs(1));
    sweep.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);

    loop {
        if let Some(live) = worker.as_mut() {
            tokio::select! {
                maybe_item = rx.recv() => {
                    let Some(item) = maybe_item else { break };
                    if let Err(err) = send_item_to_worker(live, item, &mut pending, config.task_timeout).await {
                        tracing::warn!(%err, "failed writing task to probe worker; resetting child");
                        fail_all_pending(&mut pending, "probe worker write failed");
                        shutdown_worker(&mut worker, &mut backoff, &mut cooldown_until, &config).await;
                    }
                }
                line_res = live.lines.next_line() => {
                    match line_res {
                        Ok(Some(line)) => {
                            handle_worker_line(&line, &mut pending);
                        }
                        Ok(None) => {
                            tracing::warn!("probe worker stdout closed unexpectedly");
                            fail_all_pending(&mut pending, "probe worker exited unexpectedly");
                            shutdown_worker(&mut worker, &mut backoff, &mut cooldown_until, &config).await;
                        }
                        Err(err) => {
                            tracing::warn!(%err, "probe worker stdout read error");
                            fail_all_pending(&mut pending, "probe worker stream error");
                            shutdown_worker(&mut worker, &mut backoff, &mut cooldown_until, &config).await;
                        }
                    }
                }
                _ = sweep.tick() => {
                    expire_timed_out_pending(&mut pending);
                    if let Ok(Some(status)) = live.child.try_wait() {
                        tracing::warn!(?status, "probe worker exited prematurely");
                        fail_all_pending(&mut pending, &format!("probe worker exited ({status})"));
                        shutdown_worker(&mut worker, &mut backoff, &mut cooldown_until, &config).await;
                    }
                }
            }
        } else {
            let Some(item) = rx.recv().await else { break };
            if cooldown_until.is_some_and(|until| Instant::now() < until) {
                resolve_item_with_error(item, "probe worker is recovering from crash cooldown");
                continue;
            }
            match spawn_worker(&spec).await {
                Ok(mut spawned) => {
                    if let Err(err) =
                        send_item_to_worker(&mut spawned, item, &mut pending, config.task_timeout)
                            .await
                    {
                        tracing::warn!(%err, "newly spawned probe worker failed on initial write");
                        fail_all_pending(&mut pending, "probe worker startup write failed");
                        let _ = spawned.child.kill().await;
                        let _ = spawned.child.wait().await;
                        cooldown_until = Some(Instant::now() + backoff);
                        backoff = (backoff * 2).min(config.backoff_max.max(config.backoff_min));
                    } else {
                        worker = Some(spawned);
                    }
                }
                Err(err) => {
                    tracing::warn!(%err, "could not start probe worker subprocess");
                    resolve_item_with_error(item, &err);
                    cooldown_until = Some(Instant::now() + backoff);
                    backoff = (backoff * 2).min(config.backoff_max.max(config.backoff_min));
                }
            }
        }
    }

    fail_all_pending(&mut pending, "probe supervisor shutting down");
    if let Some(mut live) = worker.take() {
        let _ = live.child.kill().await;
        let _ = live.child.wait().await;
    }
}

async fn shutdown_worker(
    worker: &mut Option<LiveWorker>,
    backoff: &mut Duration,
    cooldown_until: &mut Option<Instant>,
    config: &ProbeSupervisorConfig,
) {
    if let Some(mut live) = worker.take() {
        let ran_healthy = live.started_at.elapsed() >= HEALTHY_RESET_AGE;
        let _ = live.child.kill().await;
        let _ = live.child.wait().await;
        if ran_healthy {
            *backoff = config.backoff_min.max(Duration::from_millis(50));
        } else {
            *cooldown_until = Some(Instant::now() + *backoff);
            *backoff = (*backoff * 2).min(config.backoff_max.max(config.backoff_min));
        }
    }
}

async fn send_item_to_worker(
    live: &mut LiveWorker,
    item: DispatchItem,
    pending: &mut HashMap<String, PendingReply>,
    timeout: Duration,
) -> Result<(), String> {
    let deadline = tokio::time::Instant::now() + timeout;
    let (request_id, wire_req, entry) = match item {
        DispatchItem::Ping {
            request_id,
            payload,
            reply,
        } => {
            let entry = PendingReply::Ping {
                ping_task_id: payload.ping_task_id,
                ping_type: payload.ping_type.clone(),
                reply,
                deadline,
            };
            let wire = WorkerRequest::Ping {
                request_id: request_id.clone(),
                payload,
            };
            (request_id, wire, entry)
        }
        DispatchItem::NetProbe {
            request_id,
            payload,
            reply,
        } => {
            let entry = PendingReply::NetProbe {
                probe_id: payload.probe_id.clone(),
                kind: payload.kind.clone(),
                reply,
                deadline,
            };
            let wire = WorkerRequest::NetProbe {
                request_id: request_id.clone(),
                payload,
            };
            (request_id, wire, entry)
        }
    };

    let json = serde_json::to_string(&wire_req)
        .map_err(|err| format!("serialize worker request: {err}"))?;
    pending.insert(request_id.clone(), entry);
    if let Err(err) = live.stdin.write_all(json.as_bytes()).await {
        if let Some(pending_entry) = pending.remove(&request_id) {
            pending_entry.resolve_with_error(&format!("stdin write failed: {err}"));
        }
        return Err(err.to_string());
    }
    if let Err(err) = live.stdin.write_all(b"\n").await {
        if let Some(pending_entry) = pending.remove(&request_id) {
            pending_entry.resolve_with_error(&format!("stdin newline write failed: {err}"));
        }
        return Err(err.to_string());
    }
    if let Err(err) = live.stdin.flush().await {
        if let Some(pending_entry) = pending.remove(&request_id) {
            pending_entry.resolve_with_error(&format!("stdin flush failed: {err}"));
        }
        return Err(err.to_string());
    }
    Ok(())
}

fn handle_worker_line(line: &str, pending: &mut HashMap<String, PendingReply>) {
    let trimmed = line.trim();
    if trimmed.is_empty() {
        return;
    }
    match serde_json::from_str::<WorkerResponse>(trimmed) {
        Ok(WorkerResponse::Ping {
            request_id,
            payload,
        }) => {
            if let Some(PendingReply::Ping { reply, .. }) = pending.remove(&request_id) {
                let _ = reply.send(payload);
            }
        }
        Ok(WorkerResponse::NetProbe {
            request_id,
            payload,
        }) => {
            if let Some(PendingReply::NetProbe { reply, .. }) = pending.remove(&request_id) {
                let _ = reply.send(payload);
            }
        }
        Err(err) => {
            tracing::warn!(%err, "ignored unparseable probe worker response line");
        }
    }
}

fn expire_timed_out_pending(pending: &mut HashMap<String, PendingReply>) {
    let now = tokio::time::Instant::now();
    let expired: Vec<String> = pending
        .iter()
        .filter(|(_, v)| v.deadline() <= now)
        .map(|(k, _)| k.clone())
        .collect();
    for key in expired {
        if let Some(entry) = pending.remove(&key) {
            entry.resolve_with_error("probe task timed out in supervisor");
        }
    }
}

fn fail_all_pending(pending: &mut HashMap<String, PendingReply>, reason: &str) {
    for (_, entry) in pending.drain() {
        entry.resolve_with_error(reason);
    }
}

fn resolve_item_with_error(item: DispatchItem, reason: &str) {
    match item {
        DispatchItem::Ping {
            payload, reply, ..
        } => {
            let _ = reply.send(PingResult {
                task_id: payload.ping_task_id,
                ping_type: payload.ping_type,
                value: -1,
                finished_at: format_rfc3339_now(),
            });
        }
        DispatchItem::NetProbe {
            payload, reply, ..
        } => {
            let _ = reply.send(NetProbeResult {
                probe_id: payload.probe_id,
                kind: payload.kind,
                ping: None,
                traceroute: None,
                dns: None,
                error: Some(reason.to_owned()),
                finished_at: format_rfc3339_now(),
            });
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::DnsQueryRequest;

    fn script_cmd(script: &str) -> (PathBuf, Vec<String>) {
        (
            PathBuf::from("/bin/sh"),
            vec!["-c".to_owned(), script.to_owned()],
        )
    }

    #[tokio::test]
    async fn supervisor_executes_in_separate_child_process_and_isolates_crashes() {
        let parent_pid = std::process::id();
        // Echoes a valid WorkerResponse containing the child's own PID in `finished_at` so we can verify separate OS process execution.
        let healthy_script = r#"
while IFS= read -r line; do
  case "$line" in
    *"\"op\":\"ping\""*)
      req_id=$(printf '%s' "$line" | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
      printf '{"op":"ping","request_id":"%s","payload":{"task_id":7,"ping_type":"tcp","value":12,"finished_at":"pid:%s"}}\n' "$req_id" "$$"
      ;;
    *"\"op\":\"net_probe\""*)
      req_id=$(printf '%s' "$line" | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
      printf '{"op":"net_probe","request_id":"%s","payload":{"probe_id":"probe-1","kind":"dns_query","error":null,"finished_at":"pid:%s"}}\n' "$req_id" "$$"
      ;;
  esac
done
"#;
        let (prog, args) = script_cmd(healthy_script);
        let sup = ProbeSupervisor::start(ProbeSupervisorConfig {
            program: prog,
            args,
            queue_capacity: 8,
            task_timeout: Duration::from_secs(5),
            backoff_min: Duration::from_millis(50),
            backoff_max: Duration::from_millis(200),
        });

        let ping_res = sup
            .dispatch_ping(PingRequest {
                ping_task_id: 7,
                ping_type: "tcp".into(),
                ping_target: "127.0.0.1:80".into(),
            })
            .await;
        assert_eq!(ping_res.value, 12);
        let child_pid: u32 = ping_res
            .finished_at
            .strip_prefix("pid:")
            .and_then(|v| v.parse().ok())
            .expect("child pid encoded");
        assert_ne!(child_pid, parent_pid, "worker must run in a different OS process");

        // Now switch command to one that immediately crashes on input (simulating segfault/kill),
        // wait by crashing current worker via bad script after exit: test a crashing supervisor from fresh start.
        let (crash_prog, crash_args) = script_cmd("exit 137");
        let crash_sup = ProbeSupervisor::start(ProbeSupervisorConfig {
            program: crash_prog,
            args: crash_args,
            queue_capacity: 4,
            task_timeout: Duration::from_secs(5),
            backoff_min: Duration::from_millis(60),
            backoff_max: Duration::from_millis(200),
        });

        let first = crash_sup
            .dispatch_net_probe(NetProbeRequest {
                probe_id: "np-crash".into(),
                kind: "dns_query".into(),
                ping: None,
                traceroute: None,
                dns: Some(DnsQueryRequest {
                    domain: "example.com".into(),
                    record_type: "A".into(),
                    resolver: "1.1.1.1:53".into(),
                    ecs_subnet: None,
                    timeout_ms: None,
                }),
            })
            .await;
        assert!(first.error.is_some(), "crash must yield error result without panicking supervisor");

        // Switch to healthy script and wait out short test backoff -> must auto-recover.
        let (ok_prog, ok_args) = script_cmd(healthy_script);
        crash_sup.set_command(ok_prog, ok_args).await;
        tokio::time::sleep(Duration::from_millis(120)).await;

        let recovered = crash_sup
            .dispatch_ping(PingRequest {
                ping_task_id: 7,
                ping_type: "tcp".into(),
                ping_target: "127.0.0.1:80".into(),
            })
            .await;
        assert_eq!(recovered.value, 12);
    }

    #[tokio::test]
    async fn supervisor_drops_nonblockingly_when_queue_full() {
        let (prog, args) = script_cmd("sleep 1");
        let sup = ProbeSupervisor::start(ProbeSupervisorConfig {
            program: prog,
            args,
            queue_capacity: 1,
            task_timeout: Duration::from_millis(300),
            backoff_min: Duration::from_millis(50),
            backoff_max: Duration::from_millis(100),
        });
        let sup_bg = sup.clone();
        let h1 = tokio::spawn(async move {
            sup_bg
                .dispatch_net_probe(NetProbeRequest {
                    probe_id: "slow-1".into(),
                    kind: "dns_query".into(),
                    ping: None,
                    traceroute: None,
                    dns: None,
                })
                .await
        });
        // Give background spawn a moment to pop slow-1 and saturate queue_capacity=1
        tokio::time::sleep(Duration::from_millis(20)).await;
        let mut hit_busy = false;
        let mut handles = Vec::new();
        for i in 0..8 {
            let s = sup.clone();
            handles.push(tokio::spawn(async move {
                s.dispatch_net_probe(NetProbeRequest {
                    probe_id: format!("q-{i}"),
                    kind: "dns_query".into(),
                    ping: None,
                    traceroute: None,
                    dns: None,
                })
                .await
            }));
        }
        // Check quick non-blocking dispatch on another call once saturated
        for h in handles {
            if let Ok(Ok(res)) = tokio::time::timeout(Duration::from_secs(2), h).await {
                if res.error.as_deref().is_some_and(|e| e.contains("queue is full")) {
                    hit_busy = true;
                }
            }
        }
        let _ = h1.await;
        assert!(hit_busy, "queue_capacity=1 must drop overflow cleanly without blocking");
    }
}
