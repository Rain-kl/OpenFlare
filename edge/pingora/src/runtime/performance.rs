use super::Listeners;
use crate::config::persist_private_json;
use anyhow::{Context, Result};
use async_trait::async_trait;
use pingora::{
    listeners::TcpSocketOptions,
    protocols::{
        l4::ext::{set_dscp, set_tcp_fastopen_backlog},
        tls::ALPN,
        TcpKeepalive,
    },
    server::{
        configuration::ServerConf, ShutdownSignal, ShutdownSignalWatch, UnixShutdownSignalWatch,
    },
    upstreams::peer::HttpPeer,
};
use serde::{Deserialize, Serialize};
use std::{
    io::ErrorKind,
    os::fd::AsRawFd,
    path::Path,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, RwLock,
    },
    time::Duration,
};
use tokio::sync::Notify;

#[derive(Clone, Debug, Default, PartialEq, Eq, Deserialize, Serialize)]
pub(crate) struct NodePolicy {
    pub settings: Option<Settings>,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub(crate) struct Settings {
    #[serde(flatten)]
    pub startup: Startup,
    #[serde(flatten)]
    pub upstream: Upstream,
    #[serde(flatten)]
    pub cache: Cache,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub(crate) struct Startup {
    pub threads: usize,
    pub threads_auto: bool,
    pub work_stealing: bool,
    pub listener_tasks_per_fd: usize,
    pub upstream_keepalive_pool_size: usize,
    pub max_retries: usize,
    pub runtime_enable_alt_timer: bool,
    pub fast_timeout_to_tokio_threshold_seconds: u64,
    pub runtime_metrics_poll_time_histogram: bool,
    pub max_concurrent_streams: u32,
    pub initial_window_size: u32,
    pub initial_connection_window_size: u32,
    pub max_frame_size: u32,
    pub max_header_list_size: u32,
    pub header_table_size: u32,
    pub max_send_buffer_size: usize,
    pub upstream_connect_offload_threadpools: usize,
    pub upstream_connect_offload_thread_per_pool: usize,
    pub downstream_tls_offload_threadpools: usize,
    pub downstream_tls_offload_thread_per_pool: usize,
    pub keepalive_request_limit: u32,
    pub h2_idle_timeout: u64,
    pub h2c: bool,
    pub so_reuseport: bool,
    pub tcp_fastopen: usize,
    pub listener_tcp_keepalive: bool,
    pub tcp_snd_buf: usize,
    pub tcp_recv_buf: usize,
    pub ipv6_only: bool,
    pub dscp: u8,
    pub cache_capacity_mib: usize,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub(crate) struct Upstream {
    connection_timeout: u64,
    total_connection_timeout: u64,
    read_timeout: u64,
    write_timeout: u64,
    idle_timeout: u64,
    upstream_tcp_keepalive: bool,
    upstream_tcp_recv_buf: usize,
    verify_cert: bool,
    verify_hostname: bool,
    upstream_alpn: OriginProtocol,
    max_h2_streams: usize,
    h2_ping_interval: u64,
    h2_stream_window_size: u32,
    h2_connection_window_size: u32,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub(crate) struct Cache {
    pub cache_enabled: bool,
    pub cache_max_object_mib: usize,
    pub cache_lock_wait_timeout: u64,
    pub cache_max_lock_retries: usize,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Deserialize, Serialize)]
#[serde(rename_all = "lowercase")]
enum OriginProtocol {
    H1,
    H2h1,
    H2,
}

impl NodePolicy {
    const FILE: &'static str = "performance-policy.json";

    pub fn load(state_dir: &Path) -> Self {
        let result = std::fs::read(state_dir.join(Self::FILE));
        let result = match result {
            Ok(contents) => serde_json::from_slice::<Self>(&contents)
                .context("decode saved Pingora performance settings")
                .and_then(|policy| {
                    policy.validate()?;
                    Ok(policy)
                }),
            Err(error) if error.kind() == ErrorKind::NotFound => return Self::default(),
            Err(error) => Err(error).context("read saved Pingora performance settings"),
        };
        result.unwrap_or_else(|error| {
            tracing::error!(%error, "using default Pingora performance settings");
            Self::default()
        })
    }

    fn validate(&self) -> Result<()> {
        let Some(settings) = &self.settings else {
            return Ok(());
        };
        let startup = &settings.startup;
        anyhow::ensure!(
            (1..=256).contains(&startup.threads)
                && (1..=32).contains(&startup.listener_tasks_per_fd)
                && (1..=16).contains(&startup.max_retries)
                && (1..=65536).contains(&startup.upstream_keepalive_pool_size)
                && startup.upstream_connect_offload_threadpools <= 16
                && (1..=16).contains(&startup.upstream_connect_offload_thread_per_pool)
                && startup.downstream_tls_offload_threadpools <= 16
                && (1..=16).contains(&startup.downstream_tls_offload_thread_per_pool)
                && startup.initial_window_size <= i32::MAX as u32
                && startup.initial_connection_window_size <= i32::MAX as u32
                && (16384..=16777215).contains(&startup.max_frame_size)
                && startup.max_send_buffer_size <= 16777216
                && (16..=4096).contains(&startup.cache_capacity_mib)
                && settings.cache.cache_max_object_mib <= startup.cache_capacity_mib
                && (1..=1024).contains(&settings.upstream.max_h2_streams)
                && settings.upstream.h2_stream_window_size <= i32::MAX as u32
                && settings.upstream.h2_connection_window_size <= i32::MAX as u32
                && (!settings.upstream.verify_hostname || settings.upstream.verify_cert),
            "invalid Pingora performance settings"
        );
        Ok(())
    }
}

impl Startup {
    pub fn configure_server(&self, conf: &mut ServerConf) {
        conf.threads = if self.threads_auto {
            std::thread::available_parallelism().map_or(1, std::num::NonZeroUsize::get)
        } else {
            self.threads
        };
        conf.work_stealing = self.work_stealing;
        conf.listener_tasks_per_fd = self.listener_tasks_per_fd;
        conf.upstream_keepalive_pool_size = self.upstream_keepalive_pool_size;
        conf.max_retries = self.max_retries;
        conf.runtime_enable_alt_timer = self.runtime_enable_alt_timer;
        conf.fast_timeout_to_tokio_threshold_seconds =
            Some(self.fast_timeout_to_tokio_threshold_seconds);
        conf.runtime_metrics_poll_time_histogram = self.runtime_metrics_poll_time_histogram;
        conf.upstream_connect_offload_threadpools =
            nonzero(self.upstream_connect_offload_threadpools);
        conf.upstream_connect_offload_thread_per_pool =
            Some(self.upstream_connect_offload_thread_per_pool);
        conf.downstream_tls_offload_threadpools = nonzero(self.downstream_tls_offload_threadpools);
        conf.downstream_tls_offload_thread_per_pool =
            Some(self.downstream_tls_offload_thread_per_pool);
    }

    fn validate_listener_options(&self, listeners: &Listeners) -> Result<()> {
        if self.tcp_fastopen == 0 && self.dscp == 0 {
            return Ok(());
        }
        for address in [Some(listeners.http), listeners.tls].into_iter().flatten() {
            let socket = if address.is_ipv6() {
                tokio::net::TcpSocket::new_v6()?
            } else {
                tokio::net::TcpSocket::new_v4()?
            };
            if self.tcp_fastopen > 0 {
                set_tcp_fastopen_backlog(socket.as_raw_fd(), self.tcp_fastopen)
                    .context("validate listener TCP Fast Open")?;
            }
            if self.dscp > 0 {
                set_dscp(socket.as_raw_fd(), self.dscp).context("validate listener DSCP")?;
            }
        }
        Ok(())
    }

    pub fn socket_options(&self, ipv6: bool) -> TcpSocketOptions {
        let mut options = TcpSocketOptions::default();
        options.so_reuseport = Some(self.so_reuseport);
        options.ipv6_only = ipv6.then_some(self.ipv6_only);
        options.tcp_fastopen = nonzero(self.tcp_fastopen);
        options.tcp_keepalive = keepalive(self.listener_tcp_keepalive);
        options.tcp_snd_buf = nonzero(self.tcp_snd_buf);
        options.tcp_recv_buf = nonzero(self.tcp_recv_buf);
        options.dscp = (self.dscp != 0).then_some(self.dscp);
        options
    }
}

impl Upstream {
    pub fn configure(&self, peer: &mut HttpPeer) {
        let tls = peer.is_tls();
        let options = &mut peer.options;
        options.connection_timeout = seconds(self.connection_timeout);
        options.total_connection_timeout = seconds(self.total_connection_timeout);
        options.read_timeout = seconds(self.read_timeout);
        options.write_timeout = seconds(self.write_timeout);
        options.idle_timeout = seconds(self.idle_timeout);
        options.verify_cert = self.verify_cert;
        options.verify_hostname = self.verify_hostname;
        options.tcp_keepalive = keepalive(self.upstream_tcp_keepalive);
        options.tcp_recv_buf = nonzero(self.upstream_tcp_recv_buf);
        // ALPN negotiation applies only to TLS connections.
        if tls {
            options.alpn = match self.upstream_alpn {
                OriginProtocol::H1 => ALPN::H1,
                OriginProtocol::H2h1 => ALPN::H2H1,
                OriginProtocol::H2 => ALPN::H2,
            };
        }
        options.max_h2_streams = self.max_h2_streams;
        options.h2_ping_interval = seconds(self.h2_ping_interval);
        options.h2_stream_window_size = Some(self.h2_stream_window_size);
        options.h2_connection_window_size = Some(self.h2_connection_window_size);
    }
}

pub(crate) fn seconds(value: u64) -> Option<Duration> {
    (value != 0).then(|| Duration::from_secs(value))
}

fn nonzero(value: usize) -> Option<usize> {
    (value != 0).then_some(value)
}

fn keepalive(enabled: bool) -> Option<TcpKeepalive> {
    enabled.then(|| TcpKeepalive {
        idle: Duration::from_secs(60),
        interval: Duration::from_secs(15),
        count: 4,
        #[cfg(target_os = "linux")]
        user_timeout: Duration::ZERO,
    })
}

#[derive(Clone)]
pub(crate) struct LivePolicy {
    active: Arc<RwLock<Arc<AppliedPolicy>>>,
    reload: Arc<Notify>,
}

pub(crate) struct AppliedPolicy {
    pub policy: NodePolicy,
    pub upstream_generation: u64,
}

impl LivePolicy {
    pub fn new(policy: NodePolicy) -> Self {
        Self {
            active: Arc::new(RwLock::new(Arc::new(AppliedPolicy {
                policy,
                upstream_generation: 0,
            }))),
            reload: Arc::new(Notify::new()),
        }
    }

    pub fn current(&self) -> Arc<AppliedPolicy> {
        self.active
            .read()
            .expect("read active performance policy")
            .clone()
    }

    pub fn reconcile(
        &self,
        state_dir: &Path,
        listeners: &Listeners,
        desired: NodePolicy,
    ) -> Result<bool> {
        let current = self.current();
        if desired == current.policy {
            return Ok(false);
        }
        desired.validate()?;
        let before = current.policy.settings.as_ref();
        let after = desired.settings.as_ref();
        let needs_reload = before.map(|s| &s.startup) != after.map(|s| &s.startup);
        if let Some(settings) = after.filter(|_| needs_reload) {
            settings.startup.validate_listener_options(listeners)?;
        }
        persist_private_json(&state_dir.join(NodePolicy::FILE), &desired)?;
        if needs_reload {
            tracing::info!("reloading Pingora startup parameters");
            self.reload.notify_one();
            return Ok(true);
        }
        // Different peer settings must not reuse pre-change origin connections.
        let upstream_changed = before.map(|s| &s.upstream) != after.map(|s| &s.upstream);
        *self.active.write().expect("install performance policy") = Arc::new(AppliedPolicy {
            policy: desired,
            upstream_generation: current.upstream_generation + u64::from(upstream_changed),
        });
        tracing::info!("updated Pingora performance parameters");
        Ok(false)
    }

    pub fn shutdown_watch(&self, reloading: Arc<AtomicBool>) -> ReloadWatch {
        ReloadWatch {
            reload: self.reload.clone(),
            reloading,
        }
    }
}

pub(crate) struct ReloadWatch {
    reload: Arc<Notify>,
    reloading: Arc<AtomicBool>,
}

#[async_trait]
impl ShutdownSignalWatch for ReloadWatch {
    async fn recv(&self) -> ShutdownSignal {
        tokio::select! {
            biased;
            signal = UnixShutdownSignalWatch.recv() => signal,
            _ = self.reload.notified() => {
                self.reloading.store(true, Ordering::Release);
                ShutdownSignal::GracefulTerminate
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn automatically_resolves_threads_from_available_cpus() {
        let raw = include_str!("../../tests/performance_policy.json");
        let mut settings: Settings =
            serde_json::from_str(raw).expect("default performance settings");
        let mut conf = ServerConf::default();
        settings.startup.threads = 4;
        settings.startup.configure_server(&mut conf);
        assert_eq!(conf.threads, 4);

        settings.startup.threads_auto = true;
        settings.startup.configure_server(&mut conf);
        assert_eq!(
            conf.threads,
            std::thread::available_parallelism().map_or(1, std::num::NonZeroUsize::get),
        );
    }
}
