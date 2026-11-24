mod cpu;
mod memory;
#[cfg(target_os = "linux")]
mod netlink;
mod network;
mod os;
mod process;

use crate::types::{BasicInfo, LoadMetrics, RealTimeMetrics};
use std::path::PathBuf;
use std::time::{Instant, SystemTime, UNIX_EPOCH};
use sysinfo::{
    Components, CpuRefreshKind, DiskRefreshKind, Disks, MemoryRefreshKind, Networks, System,
};

/// Configuration for [`ProbeCollector`].
#[derive(Debug, Clone, Default)]
pub struct ProbeCollectorConfig {
    /// Optional path (e.g. node state directory) used as a disk mount fallback
    /// when standard filesystem filtering finds no host block devices (such as
    /// inside container overlayfs environments).
    pub fallback_disk_path: Option<PathBuf>,
    /// Optional explicit NIC exclude substrings (defaults to Komari's virtual NIC list).
    pub excluded_nic_keywords: Vec<String>,
}

/// Stateful host metrics collector. Each node runtime owns its own instance;
/// there is no process-global mutable state.
pub struct ProbeCollector {
    system: System,
    disks: Disks,
    networks: Networks,
    components: Components,
    config: ProbeCollectorConfig,
    virtualization: String,
    last_network_sample: Option<Instant>,
}

impl ProbeCollector {
    pub fn new(config: ProbeCollectorConfig) -> Self {
        let mut system = System::new();
        system.refresh_cpu_specifics(CpuRefreshKind::everything().without_frequency());
        system.refresh_memory_specifics(MemoryRefreshKind::everything());
        let disks = Disks::new_with_refreshed_list();
        let networks = Networks::new_with_refreshed_list();
        let components = Components::new_with_refreshed_list();
        let virtualization = os::detect_virtualization();

        Self {
            system,
            disks,
            networks,
            components,
            config,
            virtualization,
            last_network_sample: None,
        }
    }

    /// Collects static / slow-changing system inventory.
    pub fn collect_basic_info(
        &mut self,
        probe_version: &str,
        ipv4: Option<String>,
        ipv6: Option<String>,
    ) -> BasicInfo {
        self.system
            .refresh_memory_specifics(MemoryRefreshKind::everything());
        self.disks
            .refresh_specifics(true, DiskRefreshKind::nothing().with_storage());

        let cpu_info = cpu::collect_cpu_info(&self.system);
        let mem_total = self.system.total_memory();
        let swap_total = self.system.total_swap();
        let disk = memory::collect_disk(&self.disks, self.config.fallback_disk_path.as_deref());
        let os_info = os::collect_os_info();

        BasicInfo {
            hostname: os_info.hostname,
            arch: cpu_info.arch,
            cpu_cores: cpu_info.logical_cores,
            cpu_physical_cores: cpu_info.physical_cores,
            cpu_name: cpu_info.name,
            gpu_name: String::new(),
            disk_total: disk.total,
            swap_total,
            mem_total,
            ipv4,
            ipv6,
            os: os_info.display_name,
            os_name: os_info.os_name,
            os_version: os_info.os_version,
            kernel_version: os_info.kernel_version,
            virtualization: self.virtualization.clone(),
            version: probe_version.to_owned(),
            uptime: System::uptime(),
            reported_at_unix: unix_now(),
        }
    }

    /// Collects real-time performance metrics (Komari-aligned).
    pub fn collect_metrics(&mut self) -> RealTimeMetrics {
        self.system
            .refresh_cpu_specifics(CpuRefreshKind::everything().without_frequency());
        self.system
            .refresh_memory_specifics(MemoryRefreshKind::everything());
        self.disks
            .refresh_specifics(true, DiskRefreshKind::everything());
        self.networks.refresh(true);
        self.components.refresh(true);

        let now = Instant::now();
        let elapsed = self
            .last_network_sample
            .replace(now)
            .map(|prev| now.saturating_duration_since(prev));

        let cpu_metrics = cpu::collect_cpu_usage(&self.system);
        let ram = memory::collect_memory(&self.system);
        let swap = memory::collect_swap(&self.system);
        let disk = memory::collect_disk(&self.disks, self.config.fallback_disk_path.as_deref());
        let load = collect_load();
        let temp = collect_temperature(&self.components);
        let net =
            network::collect_network(&self.networks, elapsed, &self.config.excluded_nic_keywords);
        let connections = network::collect_connections();
        let proc_count = process::collect_process_count();

        RealTimeMetrics {
            captured_at_unix: unix_now(),
            cpu: cpu_metrics,
            gpu: 0.0,
            ram,
            swap,
            disk,
            load,
            temp,
            network: net,
            connections,
            uptime: System::uptime(),
            process: proc_count,
        }
    }
}

fn collect_load() -> LoadMetrics {
    #[cfg(target_os = "windows")]
    {
        LoadMetrics::default()
    }
    #[cfg(not(target_os = "windows"))]
    {
        let avg = System::load_average();
        LoadMetrics {
            load1: sanitize_non_negative(avg.one),
            load5: sanitize_non_negative(avg.five),
            load15: sanitize_non_negative(avg.fifteen),
        }
    }
}

fn collect_temperature(components: &Components) -> f64 {
    let mut sum = 0.0f64;
    let mut count = 0usize;
    for comp in components {
        if let Some(temp) = comp.temperature() {
            if temp.is_finite() && temp > 0.0 && temp < 200.0 {
                let label = comp.label().to_ascii_lowercase();
                if label.contains("cpu")
                    || label.contains("package")
                    || label.contains("core")
                    || label.contains("tctl")
                    || label.contains("tdie")
                {
                    sum += f64::from(temp);
                    count += 1;
                }
            }
        }
    }
    if count > 0 {
        return (sum / count as f64 * 10.0).round() / 10.0;
    }
    for comp in components {
        if let Some(temp) = comp.temperature() {
            if temp.is_finite() && temp > 0.0 && temp < 200.0 {
                return (f64::from(temp) * 10.0).round() / 10.0;
            }
        }
    }
    0.0
}

fn sanitize_non_negative(value: f64) -> f64 {
    if value.is_finite() && value >= 0.0 {
        value
    } else {
        0.0
    }
}

fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(0, |d| d.as_secs())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn collector_samples_basic_info_and_metrics() {
        let mut collector = ProbeCollector::new(ProbeCollectorConfig {
            fallback_disk_path: Some(PathBuf::from("/")),
            excluded_nic_keywords: vec![],
        });
        let basic = collector.collect_basic_info("openflare-probe-test", None, None);
        assert!(!basic.arch.is_empty());
        assert!(basic.cpu_cores >= 1);
        assert!(basic.mem_total > 0);

        let first = collector.collect_metrics();
        assert!(first.captured_at_unix > 0);
        assert!(first.ram.total > 0);
        // First sample must not report bogus network rate spike.
        assert_eq!(first.network.up, 0);
        assert_eq!(first.network.down, 0);
        assert!((0.0..=100.0).contains(&first.cpu.usage));
    }
}
