use crate::net_probe::{execute_net_probe, format_rfc3339_now};
use crate::ping::{execute_ping, PingOptions};
use crate::types::{
    NetProbeRequest, NetProbeResult, PingRequest, PingResult, WorkerRequest, WorkerResponse,
};
use std::io::{BufRead, BufReader, BufWriter, Read, Write};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::{mpsc, Semaphore};

const MAX_IPC_LINE_BYTES: usize = 1024 * 1024;
const WORKER_INBOUND_QUEUE: usize = 128;
const WORKER_OUTBOUND_QUEUE: usize = 128;

/// Runtime configuration for the isolated OS subprocess worker (`run_probe_worker`).
#[derive(Debug, Clone)]
pub struct ProbeWorkerConfig {
    pub user_agent: String,
    pub idle_timeout: Duration,
    pub max_concurrent_pings: usize,
    pub max_concurrent_net_probes: usize,
    pub max_task_timeout: Duration,
}

impl Default for ProbeWorkerConfig {
    fn default() -> Self {
        Self {
            user_agent: format!("openflare-probe/{}", env!("CARGO_PKG_VERSION")),
            idle_timeout: Duration::from_secs(300),
            max_concurrent_pings: 16,
            max_concurrent_net_probes: 8,
            max_task_timeout: Duration::from_secs(45),
        }
    }
}

/// Runs the isolated probe worker process reading NDJSON [`WorkerRequest`] lines from `stdin`
/// and emitting NDJSON [`WorkerResponse`] lines to `stdout`.
/// Logs MUST be routed to `stderr` by the caller before invoking this function.
pub fn run_probe_worker(config: ProbeWorkerConfig) -> Result<(), String> {
    #[cfg(target_os = "linux")]
    {
        harden_and_deprioritize_worker();
        acquire_probe_raw_socket_privilege();
    }

    run_probe_worker_with_io(std::io::stdin(), std::io::stdout(), config)
}

#[cfg(target_os = "linux")]
fn acquire_probe_raw_socket_privilege() {
    if let Err(err) = trippy_privilege::Privilege::acquire_privileges() {
        tracing::debug!(%err, "failed to raise CAP_NET_RAW for probe worker");
    }
}

/// Testable core loop taking explicit `Read`/`Write` IPC streams.
pub fn run_probe_worker_with_io<R, W>(
    reader: R,
    writer: W,
    config: ProbeWorkerConfig,
) -> Result<(), String>
where
    R: Read + Send + 'static,
    W: Write + Send + 'static,
{
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(2)
        .thread_name("openflare-probe-worker")
        .enable_all()
        .build()
        .map_err(|err| format!("failed to build probe worker tokio runtime: {err}"))?;

    let (inbound_tx, mut inbound_rx) = mpsc::channel::<WorkerRequest>(WORKER_INBOUND_QUEUE);
    let (outbound_tx, mut outbound_rx) = mpsc::channel::<String>(WORKER_OUTBOUND_QUEUE);

    let reader_handle = std::thread::Builder::new()
        .name("probe-worker-stdin".into())
        .spawn(move || {
            let mut buf_reader = BufReader::new(reader);
            let mut raw = Vec::with_capacity(1024);
            loop {
                raw.clear();
                match read_bounded_line(&mut buf_reader, &mut raw, MAX_IPC_LINE_BYTES) {
                    Ok(0) => break, // EOF
                    Ok(_) => {
                        let line = String::from_utf8_lossy(&raw);
                        let trimmed = line.trim();
                        if trimmed.is_empty() {
                            continue;
                        }
                        match serde_json::from_str::<WorkerRequest>(trimmed) {
                            Ok(req) => {
                                if inbound_tx.blocking_send(req).is_err() {
                                    break;
                                }
                            }
                            Err(err) => {
                                tracing::warn!(%err, "probe worker ignored malformed IPC frame");
                            }
                        }
                    }
                    Err(err) => {
                        tracing::warn!(%err, "probe worker stdin read stopped");
                        break;
                    }
                }
            }
        })
        .map_err(|err| format!("failed to spawn probe stdin thread: {err}"))?;

    let writer_handle = std::thread::Builder::new()
        .name("probe-worker-stdout".into())
        .spawn(move || {
            let mut buf_writer = BufWriter::new(writer);
            while let Some(line) = outbound_rx.blocking_recv() {
                if buf_writer.write_all(line.as_bytes()).is_err()
                    || buf_writer.write_all(b"\n").is_err()
                    || buf_writer.flush().is_err()
                {
                    break;
                }
            }
        })
        .map_err(|err| format!("failed to spawn probe stdout thread: {err}"))?;

    runtime.block_on(async move {
        let http_client = reqwest::Client::builder()
            .timeout(Duration::from_secs(15))
            .user_agent(&config.user_agent)
            .build()
            .ok();
        let ping_options = PingOptions {
            user_agent: config.user_agent.clone(),
            ..PingOptions::default()
        };
        let ping_limiter = Arc::new(Semaphore::new(config.max_concurrent_pings.max(1)));
        let net_limiter = Arc::new(Semaphore::new(config.max_concurrent_net_probes.max(1)));
        let inflight = Arc::new(AtomicUsize::new(0));
        let mut last_activity = Instant::now();
        let mut idle_tick = tokio::time::interval(Duration::from_secs(5));
        idle_tick.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);

        loop {
            tokio::select! {
                maybe_req = inbound_rx.recv() => {
                    let Some(req) = maybe_req else {
                        break;
                    };
                    last_activity = Instant::now();
                    dispatch_worker_request(
                        req,
                        http_client.clone(),
                        ping_options.clone(),
                        Arc::clone(&ping_limiter),
                        Arc::clone(&net_limiter),
                        outbound_tx.clone(),
                        Arc::clone(&inflight),
                        config.max_task_timeout,
                    );
                }
                _ = idle_tick.tick() => {
                    if !config.idle_timeout.is_zero()
                        && inflight.load(Ordering::Acquire) == 0
                        && last_activity.elapsed() >= config.idle_timeout
                    {
                        tracing::info!("probe worker idle timeout reached; exiting cleanly");
                        break;
                    }
                }
            }
        }

        // Wait briefly for in-flight probe tasks before closing stdout.
        let deadline = Instant::now() + Duration::from_secs(5);
        while inflight.load(Ordering::Acquire) > 0 && Instant::now() < deadline {
            tokio::time::sleep(Duration::from_millis(50)).await;
        }
        drop(outbound_tx);
    });

    drop(reader_handle);
    let _ = writer_handle.join();
    Ok(())
}

#[allow(clippy::too_many_arguments)]
fn dispatch_worker_request(
    req: WorkerRequest,
    http_client: Option<reqwest::Client>,
    ping_options: PingOptions,
    ping_limiter: Arc<Semaphore>,
    net_limiter: Arc<Semaphore>,
    outbound_tx: mpsc::Sender<String>,
    inflight: Arc<AtomicUsize>,
    max_task_timeout: Duration,
) {
    inflight.fetch_add(1, Ordering::AcqRel);
    tokio::spawn(async move {
        let _guard = InflightGuard(Arc::clone(&inflight));
        match req {
            WorkerRequest::Ping {
                request_id,
                payload,
            } => {
                let response = run_isolated_ping(
                    payload,
                    http_client,
                    ping_options,
                    ping_limiter,
                    max_task_timeout,
                )
                .await;
                send_response(
                    &outbound_tx,
                    WorkerResponse::Ping {
                        request_id,
                        payload: response,
                    },
                )
                .await;
            }
            WorkerRequest::NetProbe {
                request_id,
                payload,
            } => {
                let response = run_isolated_net_probe(
                    payload,
                    http_client,
                    ping_options,
                    net_limiter,
                    max_task_timeout,
                )
                .await;
                send_response(
                    &outbound_tx,
                    WorkerResponse::NetProbe {
                        request_id,
                        payload: response,
                    },
                )
                .await;
            }
        }
    });
}

struct InflightGuard(Arc<AtomicUsize>);

impl Drop for InflightGuard {
    fn drop(&mut self) {
        self.0.fetch_sub(1, Ordering::AcqRel);
    }
}

async fn run_isolated_ping(
    req: PingRequest,
    http_client: Option<reqwest::Client>,
    ping_options: PingOptions,
    limiter: Arc<Semaphore>,
    max_timeout: Duration,
) -> PingResult {
    let Ok(_permit) = limiter.try_acquire_owned() else {
        return PingResult {
            task_id: req.ping_task_id,
            ping_type: req.ping_type,
            value: -1,
            finished_at: format_rfc3339_now(),
        };
    };
    let req_clone = req.clone();
    let handle = tokio::spawn(async move {
        tokio::time::timeout(
            max_timeout,
            execute_ping(&req_clone, http_client.as_ref(), &ping_options),
        )
        .await
    });
    match handle.await {
        Ok(Ok(result)) => result,
        Ok(Err(_)) | Err(_) => PingResult {
            task_id: req.ping_task_id,
            ping_type: req.ping_type,
            value: -1,
            finished_at: format_rfc3339_now(),
        },
    }
}

async fn run_isolated_net_probe(
    req: NetProbeRequest,
    http_client: Option<reqwest::Client>,
    ping_options: PingOptions,
    limiter: Arc<Semaphore>,
    max_timeout: Duration,
) -> NetProbeResult {
    let Ok(_permit) = limiter.try_acquire_owned() else {
        return failed_net_probe(
            &req,
            "node net probe concurrency limit reached".to_owned(),
        );
    };
    let req_clone = req.clone();
    let handle = tokio::spawn(async move {
        tokio::time::timeout(
            max_timeout,
            execute_net_probe(&req_clone, http_client.as_ref(), &ping_options),
        )
        .await
    });
    match handle.await {
        Ok(Ok(result)) => result,
        Ok(Err(_)) => failed_net_probe(
            &req,
            format!("probe timed out after {}s", max_timeout.as_secs()),
        ),
        Err(join_err) => failed_net_probe(&req, format!("probe task failed cleanly: {join_err}")),
    }
}

fn failed_net_probe(req: &NetProbeRequest, msg: String) -> NetProbeResult {
    NetProbeResult {
        probe_id: req.probe_id.clone(),
        kind: req.kind.clone(),
        ping: None,
        traceroute: None,
        dns: None,
        error: Some(msg),
        finished_at: format_rfc3339_now(),
    }
}

async fn send_response(tx: &mpsc::Sender<String>, response: WorkerResponse) {
    if let Ok(line) = serde_json::to_string(&response) {
        let _ = tx.send(line).await;
    }
}

fn read_bounded_line<B: BufRead>(
    reader: &mut B,
    out: &mut Vec<u8>,
    max_bytes: usize,
) -> std::io::Result<usize> {
    let mut total = 0usize;
    loop {
        let available = reader.fill_buf()?;
        if available.is_empty() {
            return Ok(total);
        }
        if let Some(newline_pos) = available.iter().position(|&b| b == b'\n') {
            let chunk = &available[..=newline_pos];
            if out.len() + chunk.len() <= max_bytes {
                out.extend_from_slice(chunk);
            } else {
                out.clear();
            }
            let len = chunk.len();
            reader.consume(len);
            total += len;
            return Ok(total);
        }
        let len = available.len();
        if out.len() + len <= max_bytes {
            out.extend_from_slice(available);
        } else {
            out.clear();
        }
        reader.consume(len);
        total += len;
        if out.is_empty() && total > max_bytes {
            return Err(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                "IPC line exceeded max length",
            ));
        }
    }
}

#[cfg(target_os = "linux")]
fn harden_and_deprioritize_worker() {
    unsafe {
        // Ensure parent exit/reload terminates the probe child automatically via kernel PDEATHSIG.
        let _ = libc::prctl(libc::PR_SET_PDEATHSIG, libc::SIGTERM, 0, 0, 0);
        let _ = libc::prctl(libc::PR_SET_DUMPABLE, 0, 0, 0, 0);
        let limit = libc::rlimit {
            rlim_cur: 0,
            rlim_max: 0,
        };
        let _ = libc::setrlimit(libc::RLIMIT_CORE, &limit);
        // Run active probes at lower OS scheduling priority (nice +10) than the reverse proxy.
        let _ = libc::setpriority(libc::PRIO_PROCESS, 0, 10);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn worker_processes_ping_and_net_probe_over_pipes_and_recovers_from_bad_json() {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("bind");
        let port = listener.local_addr().unwrap().port();
        std::thread::spawn(move || {
            for _ in 0..2 {
                let _ = listener.accept();
            }
        });

        let req1 = WorkerRequest::Ping {
            request_id: "p1".into(),
            payload: PingRequest {
                ping_task_id: 9,
                ping_type: "tcp".into(),
                ping_target: format!("127.0.0.1:{port}"),
            },
        };
        let req2 = WorkerRequest::NetProbe {
            request_id: "np1".into(),
            payload: NetProbeRequest {
                probe_id: "np1".into(),
                kind: "ping_batch".into(),
                ping: Some(crate::types::PingBatchRequest {
                    protocol: "tcp".into(),
                    target: "127.0.0.1".into(),
                    port: Some(port),
                    count: 1,
                    interval_ms: 50,
                    timeout_ms: Some(1000),
                }),
                traceroute: None,
                dns: None,
            },
        };

        let mut input = String::new();
        input.push_str("{broken-json\n");
        input.push_str(&serde_json::to_string(&req1).unwrap());
        input.push('\n');
        input.push_str(&serde_json::to_string(&req2).unwrap());
        input.push('\n');

        let writer = SharedBuf::default();
        let reader_cursor = std::io::Cursor::new(input.into_bytes());
        run_probe_worker_with_io(
            reader_cursor,
            writer.clone(),
            ProbeWorkerConfig {
                idle_timeout: Duration::from_secs(10),
                ..ProbeWorkerConfig::default()
            },
        )
        .expect("worker");

        let output = writer.contents();
        let lines: Vec<WorkerResponse> = output
            .lines()
            .map(|l| serde_json::from_str::<WorkerResponse>(l).expect("valid json line"))
            .collect();
        assert_eq!(lines.len(), 2);
        assert!(lines
            .iter()
            .any(|r| matches!(r, WorkerResponse::Ping { request_id, payload } if request_id == "p1" && payload.value >= 0)));
        assert!(lines
            .iter()
            .any(|r| matches!(r, WorkerResponse::NetProbe { request_id, payload } if request_id == "np1" && payload.ping.as_ref().is_some_and(|p| p.received == 1))));
    }

    #[derive(Clone, Default)]
    struct SharedBuf(Arc<std::sync::Mutex<Vec<u8>>>);

    impl SharedBuf {
        fn contents(&self) -> String {
            String::from_utf8_lossy(&self.0.lock().unwrap()).into_owned()
        }
    }

    impl Write for SharedBuf {
        fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
            self.0.lock().unwrap().extend_from_slice(buf);
            Ok(buf.len())
        }
        fn flush(&mut self) -> std::io::Result<()> {
            Ok(())
        }
    }
}
