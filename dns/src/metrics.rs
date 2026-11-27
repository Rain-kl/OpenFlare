use crate::protocol::HeartbeatRequest;
use hdrhistogram::Histogram;
use std::array;
use std::sync::Mutex;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, Instant};
use sysinfo::System;

const HIST_SHARDS: usize = 16;

pub struct DnsMetrics {
    pub queries_total: AtomicU64,
    pub queries_success: AtomicU64,
    pub queries_nxdomain: AtomicU64,
    pub queries_servfail: AtomicU64,
    pub queries_ratelimited: AtomicU64,
    pub cache_lookups: AtomicU64,
    pub cache_hits: AtomicU64,
    hists: [Mutex<Histogram<u64>>; HIST_SHARDS],
    state: Mutex<FlushState>,
}

struct FlushState {
    last_flush: Instant,
    last_total: u64,
    sys: System,
}

fn new_histogram() -> Histogram<u64> {
    Histogram::<u64>::new_with_bounds(1, 10_000_000, 2)
        .unwrap_or_else(|_| Histogram::<u64>::new(2).expect("valid precision"))
}

impl DnsMetrics {
    pub fn new() -> Self {
        Self {
            queries_total: AtomicU64::new(0),
            queries_success: AtomicU64::new(0),
            queries_nxdomain: AtomicU64::new(0),
            queries_servfail: AtomicU64::new(0),
            queries_ratelimited: AtomicU64::new(0),
            cache_lookups: AtomicU64::new(0),
            cache_hits: AtomicU64::new(0),
            hists: array::from_fn(|_| Mutex::new(new_histogram())),
            state: Mutex::new(FlushState {
                last_flush: Instant::now(),
                last_total: 0,
                sys: System::new(),
            }),
        }
    }

    pub fn record_latency(&self, elapsed: Duration) {
        let micros = (elapsed.as_micros() as u64).clamp(1, 10_000_000);
        let idx = (self.queries_total.load(Ordering::Relaxed) as usize) & (HIST_SHARDS - 1);
        if let Ok(mut h) = self.hists[idx].try_lock() {
            let _ = h.record(micros);
        }
    }

    pub fn snapshot_heartbeat(
        &self,
        listen_addr: &str,
        public_addr: &str,
        snapshot_version: u64,
        snapshot_etag: &str,
        last_error: &str,
    ) -> HeartbeatRequest {
        let total = self.queries_total.load(Ordering::Relaxed);
        let success = self.queries_success.load(Ordering::Relaxed);
        let nxdomain = self.queries_nxdomain.load(Ordering::Relaxed);
        let servfail = self.queries_servfail.load(Ordering::Relaxed);
        let ratelimited = self.queries_ratelimited.load(Ordering::Relaxed);
        let lookups = self.cache_lookups.load(Ordering::Relaxed);
        let hits = self.cache_hits.load(Ordering::Relaxed);
        let cache_hit_ratio = if lookups > 0 {
            hits as f64 / lookups as f64
        } else {
            0.0
        };

        let mut merged = new_histogram();
        for shard in &self.hists {
            if let Ok(mut h) = shard.lock() {
                if !h.is_empty() {
                    let _ = merged.add(&*h);
                    h.reset();
                }
            }
        }
        let (p50_ms, p95_ms, p99_ms) = if merged.is_empty() {
            (0.0, 0.0, 0.0)
        } else {
            (
                merged.value_at_quantile(0.50) as f64 / 1000.0,
                merged.value_at_quantile(0.95) as f64 / 1000.0,
                merged.value_at_quantile(0.99) as f64 / 1000.0,
            )
        };

        let (qps, cpu, mem_pct) = if let Ok(mut st) = self.state.lock() {
            let now = Instant::now();
            let secs = now.saturating_duration_since(st.last_flush).as_secs_f64();
            let delta = total.saturating_sub(st.last_total);
            st.last_flush = now;
            st.last_total = total;
            let qps = if secs > 0.0 { delta as f64 / secs } else { 0.0 };

            st.sys.refresh_cpu_usage();
            st.sys.refresh_memory();
            let cpu = st.sys.global_cpu_usage() as f64;
            let total_mem = st.sys.total_memory();
            let mem_pct = if total_mem > 0 {
                (st.sys.used_memory() as f64 / total_mem as f64) * 100.0
            } else {
                0.0
            };
            (qps, cpu, mem_pct)
        } else {
            (0.0, 0.0, 0.0)
        };

        HeartbeatRequest {
            version: crate::config::DNS_VERSION.to_string(),
            public_addr: public_addr.to_string(),
            listen_addr: listen_addr.to_string(),
            snapshot_version,
            snapshot_etag: snapshot_etag.to_string(),
            qps: round4(qps),
            p50_ms: round4(p50_ms),
            p95_ms: round4(p95_ms),
            p99_ms: round4(p99_ms),
            queries_total: total,
            queries_success: success,
            queries_nxdomain: nxdomain,
            queries_servfail: servfail,
            queries_ratelimited: ratelimited,
            cache_hit_ratio: round4(cache_hit_ratio),
            cpu_usage_percent: round4(cpu),
            memory_usage_percent: round4(mem_pct),
            last_error: last_error.to_string(),
        }
    }
}

fn round4(v: f64) -> f64 {
    if v.is_nan() || v.is_infinite() {
        0.0
    } else {
        (v * 10000.0).round() / 10000.0
    }
}
