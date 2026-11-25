use crate::types::{ConnectionsMetrics, NetworkMetrics};
use std::time::Duration;
use sysinfo::{MacAddr, Networks};

const DEFAULT_EXCLUDED_NIC_KEYWORDS: &[&str] = &[
    "br", "cni", "docker", "podman", "flannel", "lo", "veth", "virbr", "vmbr", "tap", "tun",
    "fwln", "fwpr",
];

pub(super) fn collect_network(
    networks: &Networks,
    elapsed: Option<Duration>,
    extra_excluded: &[String],
) -> NetworkMetrics {
    let mut delta_tx = 0u64;
    let mut delta_rx = 0u64;
    let mut total_tx = 0u64;
    let mut total_rx = 0u64;

    for (name, data) in networks {
        if is_excluded_nic(name, data.mac_address(), extra_excluded) {
            continue;
        }
        delta_tx = delta_tx.saturating_add(data.transmitted());
        delta_rx = delta_rx.saturating_add(data.received());
        total_tx = total_tx.saturating_add(data.total_transmitted());
        total_rx = total_rx.saturating_add(data.total_received());
    }

    // Align with komari-monitor-rs PR #27: skip initial rate sample and divide by real elapsed time.
    let (up, down) = match elapsed {
        Some(interval) if interval.as_secs_f64() > 0.0 => {
            let secs = interval.as_secs_f64();
            (
                (delta_tx as f64 / secs) as u64,
                (delta_rx as f64 / secs) as u64,
            )
        }
        _ => (0, 0),
    };

    NetworkMetrics {
        up,
        down,
        total_up: total_tx,
        total_down: total_rx,
    }
}

fn is_excluded_nic(name: &str, mac: MacAddr, extra_excluded: &[String]) -> bool {
    if mac == MacAddr([0, 0, 0, 0, 0, 0]) {
        return true;
    }
    let lower = name.to_ascii_lowercase();
    if DEFAULT_EXCLUDED_NIC_KEYWORDS
        .iter()
        .any(|keyword| lower.contains(keyword))
    {
        return true;
    }
    extra_excluded
        .iter()
        .any(|keyword| !keyword.is_empty() && lower.contains(&keyword.to_ascii_lowercase()))
}

pub(super) fn collect_connections() -> ConnectionsMetrics {
    #[cfg(target_os = "linux")]
    {
        let tcp4 = super::netlink::connections_count_with_protocol(
            libc::AF_INET as u8,
            libc::IPPROTO_TCP as u8,
        );
        let tcp6 = super::netlink::connections_count_with_protocol(
            libc::AF_INET6 as u8,
            libc::IPPROTO_TCP as u8,
        );
        let udp4 = super::netlink::connections_count_with_protocol(
            libc::AF_INET as u8,
            libc::IPPROTO_UDP as u8,
        );
        let udp6 = super::netlink::connections_count_with_protocol(
            libc::AF_INET6 as u8,
            libc::IPPROTO_UDP as u8,
        );

        let tcp = match (tcp4, tcp6) {
            (Ok(v4), Ok(v6)) => v4.saturating_add(v6),
            _ => count_proc_net_tcp(),
        };
        let udp = match (udp4, udp6) {
            (Ok(v4), Ok(v6)) => v4.saturating_add(v6),
            _ => count_proc_net_udp(),
        };
        ConnectionsMetrics { tcp, udp }
    }

    #[cfg(not(target_os = "linux"))]
    {
        ConnectionsMetrics::default()
    }
}

#[cfg(target_os = "linux")]
fn count_proc_net_tcp() -> u64 {
    let mut count = 0u64;
    for path in ["/proc/net/tcp", "/proc/net/tcp6"] {
        let Ok(content) = std::fs::read_to_string(path) else {
            continue;
        };
        for line in content.lines().skip(1) {
            if line.split_whitespace().nth(3) == Some("01") {
                count = count.saturating_add(1);
            }
        }
    }
    count
}

#[cfg(target_os = "linux")]
fn count_proc_net_udp() -> u64 {
    let mut count = 0u64;
    for path in ["/proc/net/udp", "/proc/net/udp6"] {
        let Ok(content) = std::fs::read_to_string(path) else {
            continue;
        };
        count = count.saturating_add(content.lines().skip(1).count() as u64);
    }
    count
}
