use serde::Serialize;
use std::{
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};
use sysinfo::{Disks, System};

pub(crate) struct HostCollector {
    system: System,
    disks: Disks,
    root: PathBuf,
}

impl HostCollector {
    pub fn new(root: PathBuf) -> Self {
        let mut system = System::new();
        system.refresh_cpu_all();
        Self {
            system,
            disks: Disks::new_with_refreshed_list(),
            root,
        }
    }

    pub fn sample(
        &mut self,
        status: &'static str,
        message: &'static str,
        last_error: &str,
    ) -> HostTelemetry {
        self.system.refresh_cpu_usage();
        self.system.refresh_memory();
        self.disks.refresh(true);
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_or(0, |time| time.as_secs());
        let disk = self
            .disks
            .iter()
            .filter(|disk| self.root.starts_with(disk.mount_point()))
            .max_by_key(|disk| disk.mount_point().as_os_str().len());
        let total_disk = disk.map_or(0, |disk| disk.total_space());
        let free_disk = disk.map_or(0, |disk| disk.available_space());
        let (disk_read, disk_write) =
            self.disks.iter().fold((0u64, 0u64), |(read, write), disk| {
                let usage = disk.usage();
                (
                    read.saturating_add(usage.total_read_bytes),
                    write.saturating_add(usage.total_written_bytes),
                )
            });
        let profile = SystemProfile {
            hostname: System::host_name().unwrap_or_default(),
            os_name: System::name().unwrap_or_default(),
            os_version: System::os_version().unwrap_or_default(),
            kernel_version: System::kernel_version().unwrap_or_default(),
            architecture: std::env::consts::ARCH,
            cpu_model: self
                .system
                .cpus()
                .first()
                .map_or("", |cpu| cpu.brand())
                .to_owned(),
            cpu_cores: self.system.cpus().len(),
            total_memory_bytes: self.system.total_memory(),
            total_disk_bytes: total_disk,
            uptime_seconds: System::uptime(),
            reported_at_unix: timestamp,
        };
        let metric = MetricSnapshot {
            captured_at_unix: timestamp,
            cpu_usage_percent: self.system.global_cpu_usage(),
            memory_used_bytes: self.system.used_memory(),
            memory_total_bytes: self.system.total_memory(),
            storage_used_bytes: total_disk.saturating_sub(free_disk),
            storage_total_bytes: total_disk,
            disk_read_bytes: disk_read,
            disk_write_bytes: disk_write,
        };
        let health = EdgeHealth {
            captured_at_unix: timestamp,
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
                triggered_at_unix: timestamp,
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
    architecture: &'static str,
    cpu_model: String,
    cpu_cores: usize,
    total_memory_bytes: u64,
    total_disk_bytes: u64,
    uptime_seconds: u64,
    reported_at_unix: u64,
}

#[derive(Serialize)]
struct MetricSnapshot {
    captured_at_unix: u64,
    cpu_usage_percent: f32,
    memory_used_bytes: u64,
    memory_total_bytes: u64,
    storage_used_bytes: u64,
    storage_total_bytes: u64,
    disk_read_bytes: u64,
    disk_write_bytes: u64,
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
