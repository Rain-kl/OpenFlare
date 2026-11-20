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
    #[serde(rename = "threads_auto")]
    pub auto_configure: bool,
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

const MIB: u64 = 1024 * 1024;
const GIB: u64 = 1024 * MIB;

fn automatic_runtime(cores: usize, memory_bytes: u64) -> (usize, usize) {
    // Leave room for memory reserved by the OS on machines sold in whole GiB sizes.
    let memory_threads = (memory_bytes / (768 * MIB)).clamp(1, 256) as usize;
    let threads = cores.clamp(1, memory_threads);
    // Pingora multiplies the configured per-worker size by the number of workers.
    let memory_pool_per_worker = memory_bytes / (8 * MIB) / threads as u64;
    let pool_size = memory_pool_per_worker
        .min(64 * threads as u64)
        .clamp(1, 1024) as usize;
    (threads, pool_size)
}

fn node_memory_limit() -> u64 {
    let mut system = sysinfo::System::new();
    system.refresh_memory();
    let host_memory = match system.total_memory() {
        0 => GIB,
        bytes => bytes,
    };

    #[cfg(target_os = "linux")]
    if let Some(limit) = std::fs::read_to_string("/proc/self/cgroup")
        .ok()
        .and_then(|groups| cgroup_memory_limit(Path::new("/sys/fs/cgroup"), &groups))
    {
        return host_memory.min(limit);
    }
    host_memory
}

#[cfg(any(target_os = "linux", test))]
fn cgroup_memory_limit(mount: &Path, groups: &str) -> Option<u64> {
    groups
        .lines()
        .filter_map(|group| {
            let (_, rest) = group.split_once(':')?;
            let (controllers, directory) = rest.split_once(':')?;
            let (root, limit_file) = if controllers.is_empty() {
                (mount.to_path_buf(), "memory.max")
            } else if controllers
                .split(',')
                .any(|controller| controller == "memory")
            {
                (mount.join("memory"), "memory.limit_in_bytes")
            } else {
                return None;
            };

            let directory = Path::new(directory).strip_prefix("/").ok()?;
            if directory
                .components()
                .any(|component| !matches!(component, std::path::Component::Normal(_)))
            {
                return None;
            }

            directory
                .ancestors()
                .filter_map(|ancestor| {
                    std::fs::read_to_string(root.join(ancestor).join(limit_file))
                        .ok()?
                        .trim()
                        .parse::<u64>()
                        .ok()
                })
                .min()
        })
        .min()
}

impl Startup {
    pub fn configure_server(&self, conf: &mut ServerConf) {
        let (threads, keepalive_pool_size) = if self.auto_configure {
            let cores = std::thread::available_parallelism().map_or(1, std::num::NonZeroUsize::get);
            automatic_runtime(cores, node_memory_limit())
        } else {
            (self.threads, self.upstream_keepalive_pool_size)
        };
        conf.threads = threads;
        conf.upstream_keepalive_pool_size = keepalive_pool_size;
        conf.work_stealing = self.work_stealing;
        conf.listener_tasks_per_fd = self.listener_tasks_per_fd;
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
    fn sizes_workers_and_connection_pool_for_the_node() {
        assert_eq!(automatic_runtime(1, GIB), (1, 64));
        assert_eq!(automatic_runtime(2, 2 * GIB), (2, 128));
        assert_eq!(automatic_runtime(2, 2 * GIB - 128 * MIB), (2, 120));
        assert_eq!(automatic_runtime(4, 8 * GIB), (4, 256));
        assert_eq!(automatic_runtime(8, 2 * GIB), (2, 128));
        assert_eq!(automatic_runtime(4, 256 * MIB), (1, 32));
        assert_eq!(automatic_runtime(0, 0), (1, 1));
        assert_eq!(automatic_runtime(512, u64::MAX), (256, 1024));
    }

    #[test]
    fn reads_the_strictest_parent_cgroup_memory_limit() {
        let mount = tempfile::tempdir().unwrap();
        let root = mount.path();
        let service = root.join("system.slice/openflare-edge.service");
        std::fs::create_dir_all(&service).unwrap();
        std::fs::write(root.join("memory.max"), "max").unwrap();
        std::fs::write(root.join("system.slice/memory.max"), "1073741824").unwrap();
        std::fs::write(service.join("memory.max"), "2147483648").unwrap();
        assert_eq!(
            cgroup_memory_limit(root, "0::/system.slice/openflare-edge.service"),
            Some(GIB)
        );
        assert_eq!(cgroup_memory_limit(root, "0::/../../elsewhere"), None);

        let legacy = root.join("memory/workload");
        std::fs::create_dir_all(&legacy).unwrap();
        std::fs::write(legacy.join("memory.limit_in_bytes"), "536870912").unwrap();
        assert_eq!(
            cgroup_memory_limit(root, "3:memory:/workload"),
            Some(512 * MIB)
        );
    }

    #[test]
    fn applies_automatic_and_manual_settings_to_pingora() {
        let raw = include_str!("../../tests/performance_policy.json");
        let mut settings: Settings =
            serde_json::from_str(raw).expect("default performance settings");
        let mut conf = ServerConf::default();
        settings.startup.threads = 4;
        settings.startup.upstream_keepalive_pool_size = 123;
        settings.startup.configure_server(&mut conf);
        assert_eq!(conf.threads, 4);
        assert_eq!(conf.upstream_keepalive_pool_size, 123);

        settings.startup.auto_configure = true;
        settings.startup.configure_server(&mut conf);
        let cores = std::thread::available_parallelism().map_or(1, std::num::NonZeroUsize::get);
        let expected = automatic_runtime(cores, node_memory_limit());
        assert_eq!((conf.threads, conf.upstream_keepalive_pool_size), expected);
        assert_eq!(
            pingora::connectors::ConnectorOptions::from_server_conf(&conf).keepalive_pool_size,
            expected.0 * expected.1
        );
    }
}
