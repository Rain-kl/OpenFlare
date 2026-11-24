use openflare_probe::{ProbeCollector, ProbeCollectorConfig};
use serde::Serialize;
use std::path::PathBuf;

pub(crate) struct HostCollector {
    probe: ProbeCollector,
    probe_version: String,
}

impl HostCollector {
    pub fn new(root: PathBuf) -> Self {
        Self {
            probe: ProbeCollector::new(ProbeCollectorConfig {
                fallback_disk_path: Some(root),
                excluded_nic_keywords: Vec::new(),
            }),
            probe_version: format!("openflare-edge/{}", crate::EDGE_VERSION),
        }
    }

    pub fn sample(
        &mut self,
        status: &'static str,
        message: &'static str,
        last_error: &str,
        node_ip: &str,
    ) -> HostTelemetry {
        let ipv4 = if node_ip.contains('.') && node_ip != "0.0.0.0" {
            Some(node_ip.to_owned())
        } else {
            None
        };
        let ipv6 = if node_ip.contains(':') {
            Some(node_ip.to_owned())
        } else {
            None
        };

        let basic = self
            .probe
            .collect_basic_info(&self.probe_version, ipv4, ipv6);
        let metrics = self.probe.collect_metrics();

        let profile = SystemProfile {
            hostname: basic.hostname,
            os_name: basic.os_name,
            os_version: basic.os_version,
            kernel_version: basic.kernel_version,
            architecture: basic.arch,
            cpu_model: basic.cpu_name,
            cpu_cores: basic.cpu_cores,
            cpu_physical_cores: basic.cpu_physical_cores,
            virtualization: basic.virtualization,
            gpu_name: basic.gpu_name,
            total_memory_bytes: basic.mem_total,
            swap_total_bytes: basic.swap_total,
            total_disk_bytes: basic.disk_total,
            ipv4: basic.ipv4.unwrap_or_default(),
            ipv6: basic.ipv6.unwrap_or_default(),
            probe_version: basic.version,
            uptime_seconds: basic.uptime,
            reported_at_unix: basic.reported_at_unix,
        };

        let metric = MetricSnapshot {
            captured_at_unix: metrics.captured_at_unix,
            cpu_usage_percent: metrics.cpu.usage,
            gpu_usage_percent: metrics.gpu,
            memory_used_bytes: metrics.ram.used,
            memory_total_bytes: metrics.ram.total,
            swap_used_bytes: metrics.swap.used,
            swap_total_bytes: metrics.swap.total,
            load1: metrics.load.load1,
            load5: metrics.load.load5,
            load15: metrics.load.load15,
            temp: metrics.temp,
            storage_used_bytes: metrics.disk.used,
            storage_total_bytes: metrics.disk.total,
            disk_read_bytes: metrics.disk.read_bytes,
            disk_write_bytes: metrics.disk.write_bytes,
            net_in: metrics.network.down,
            net_out: metrics.network.up,
            net_total_up: metrics.network.total_up,
            net_total_down: metrics.network.total_down,
            process_count: metrics.process,
            tcp_connections: metrics.connections.tcp,
            udp_connections: metrics.connections.udp,
            uptime_seconds: metrics.uptime,
        };

        let health = EdgeHealth {
            captured_at_unix: metrics.captured_at_unix,
            status,
            message,
            connections: tcp_sockets()
                .into_iter()
                .filter(|socket| socket.connected)
                .count(),
        };

        let health_events = if last_error.is_empty() {
            vec![]
        } else {
            vec![HealthEvent {
                event_type: "sync_error",
                severity: "warning",
                message: last_error.into(),
                triggered_at_unix: metrics.captured_at_unix,
            }]
        };

        HostTelemetry {
            profile,
            host_metrics: metric,
            edge_health: health,
            health_events,
        }
    }
}

#[derive(Serialize)]
pub(crate) struct HostTelemetry {
    profile: SystemProfile,
    host_metrics: MetricSnapshot,
    edge_health: EdgeHealth,
    health_events: Vec<HealthEvent>,
}

#[derive(Serialize)]
struct SystemProfile {
    hostname: String,
    os_name: String,
    os_version: String,
    kernel_version: String,
    architecture: String,
    cpu_model: String,
    cpu_cores: u64,
    cpu_physical_cores: u64,
    virtualization: String,
    gpu_name: String,
    total_memory_bytes: u64,
    swap_total_bytes: u64,
    total_disk_bytes: u64,
    ipv4: String,
    ipv6: String,
    probe_version: String,
    uptime_seconds: u64,
    reported_at_unix: u64,
}

#[derive(Serialize)]
struct MetricSnapshot {
    captured_at_unix: u64,
    cpu_usage_percent: f64,
    gpu_usage_percent: f64,
    memory_used_bytes: u64,
    memory_total_bytes: u64,
    swap_used_bytes: u64,
    swap_total_bytes: u64,
    load1: f64,
    load5: f64,
    load15: f64,
    temp: f64,
    storage_used_bytes: u64,
    storage_total_bytes: u64,
    disk_read_bytes: u64,
    disk_write_bytes: u64,
    net_in: u64,
    net_out: u64,
    net_total_up: u64,
    net_total_down: u64,
    process_count: u64,
    tcp_connections: u64,
    udp_connections: u64,
    uptime_seconds: u64,
}

#[derive(Serialize)]
struct EdgeHealth {
    captured_at_unix: u64,
    status: &'static str,
    message: &'static str,
    connections: usize,
}

#[derive(Serialize)]
struct HealthEvent {
    event_type: &'static str,
    severity: &'static str,
    message: String,
    triggered_at_unix: u64,
}

pub(crate) struct TcpSocket {
    pub(crate) port: u16,
    pub(crate) listening: bool,
    connected: bool,
}

#[cfg(target_os = "linux")]
pub(crate) fn tcp_sockets() -> Vec<TcpSocket> {
    let Ok(fds) = std::fs::read_dir("/proc/self/fd") else {
        return vec![];
    };
    let mut inodes = std::collections::HashSet::new();
    for entry in fds.flatten() {
        if let Ok(target) = std::fs::read_link(entry.path()) {
            if let Some(inode) = target
                .to_str()
                .and_then(|value| value.strip_prefix("socket:["))
                .and_then(|value| value.strip_suffix(']'))
            {
                inodes.insert(inode.to_owned());
            }
        }
    }
    let mut sockets = Vec::new();
    for path in ["/proc/net/tcp", "/proc/net/tcp6"] {
        let Ok(table) = std::fs::read_to_string(path) else {
            continue;
        };
        for line in table.lines().skip(1) {
            let fields = line.split_whitespace().collect::<Vec<_>>();
            if fields.len() < 10 || !inodes.contains(fields[9]) {
                continue;
            }
            let Some(port) = fields[1]
                .split_once(':')
                .and_then(|(_, value)| u16::from_str_radix(value, 16).ok())
            else {
                continue;
            };
            sockets.push(TcpSocket {
                port,
                listening: fields[3] == "0A",
                connected: fields[3] == "01",
            });
        }
    }
    sockets
}

#[cfg(not(target_os = "linux"))]
pub(crate) fn tcp_sockets() -> Vec<TcpSocket> {
    vec![]
}
