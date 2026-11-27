use std::collections::{HashMap, VecDeque};
use std::net::{IpAddr, Ipv4Addr, Ipv6Addr, SocketAddr};
use std::str::FromStr;
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant};
use tokio::net::lookup_host;

const BUDGET_WINDOW: Duration = Duration::from_secs(60);
const MAX_GLOBAL_DISPATCHES_PER_WINDOW: usize = 180;
const MAX_GLOBAL_PACKETS_PER_WINDOW: u32 = 2_400;
const MAX_TARGET_DISPATCHES_PER_WINDOW: usize = 16;
const MAX_TARGET_PACKETS_PER_WINDOW: u32 = 600;
const MAX_TRACKED_TARGETS: usize = 2048;

/// Returns `true` when `ip` falls into any non-public, loopback, private, link-local,
/// CGNAT (`100.64.0.0/10`), benchmark (`198.18.0.0/15`), multicast, or unspecified range.
pub fn is_blocked_ip(ip: IpAddr) -> bool {
    match ip {
        IpAddr::V4(v4) => is_blocked_ipv4(v4),
        IpAddr::V6(v6) => {
            if let Some(mapped) = v6.to_ipv4_mapped() {
                return is_blocked_ipv4(mapped);
            }
            is_blocked_ipv6(v6)
        }
    }
}

fn is_blocked_ipv4(v4: Ipv4Addr) -> bool {
    let octets = v4.octets();
    // 0.0.0.0/8 ("this host on this network")
    if octets[0] == 0 {
        return true;
    }
    if v4.is_loopback()
        || v4.is_private()
        || v4.is_link_local()
        || v4.is_multicast()
        || v4.is_broadcast()
        || v4.is_unspecified()
    {
        return true;
    }
    // 100.64.0.0/10 (RFC 6598 CGNAT)
    if octets[0] == 100 && (octets[1] & 0b1100_0000) == 64 {
        return true;
    }
    // 198.18.0.0/15 (RFC 2544 benchmarking / internal lab allocation)
    if octets[0] == 198 && (octets[1] == 18 || octets[1] == 19) {
        return true;
    }
    false
}

fn is_blocked_ipv6(v6: Ipv6Addr) -> bool {
    if v6.is_loopback() || v6.is_unspecified() || v6.is_multicast() {
        return true;
    }
    let first = v6.segments()[0];
    // fc00::/7 Unique Local Addresses
    if (first & 0xfe00) == 0xfc00 {
        return true;
    }
    // fe80::/10 Unicast Link-Local
    if (first & 0xffc0) == 0xfe80 {
        return true;
    }
    false
}

/// Returns `true` if `host` is a local or reserved internal DNS suffix.
pub fn is_internal_hostname(host: &str) -> bool {
    let h = host.trim().trim_end_matches('.').to_ascii_lowercase();
    if h.is_empty() || h == "localhost" {
        return true;
    }
    [
        ".localhost",
        ".local",
        ".internal",
        ".localdomain",
        ".home.arpa",
    ]
    .iter()
    .any(|suffix| h.ends_with(suffix))
}

/// Validates a resolved literal IP against SSRF policy.
pub fn validate_target_ip(ip: IpAddr) -> Result<IpAddr, String> {
    if is_blocked_ip(ip) {
        return Err(format!(
            "target address {ip} is blocked by SSRF security policy"
        ));
    }
    Ok(ip)
}

/// Resolves a host or literal IP and ensures NO returned address is blocked by SSRF policy.
pub async fn resolve_public_ip(host_or_ip: &str) -> Result<IpAddr, String> {
    let sock = resolve_public_socket_addr(host_or_ip, 80).await?;
    Ok(sock.ip())
}

/// Resolves a host/IP + port and ensures ALL returned DNS addresses satisfy SSRF policy.
pub async fn resolve_public_socket_addr(host_or_ip: &str, port: u16) -> Result<SocketAddr, String> {
    let cleaned = host_or_ip
        .trim()
        .trim_start_matches('[')
        .trim_end_matches(']');
    if cleaned.is_empty() {
        return Err("target host cannot be empty".to_owned());
    }
    if let Ok(ip) = IpAddr::from_str(cleaned) {
        validate_target_ip(ip)?;
        return Ok(SocketAddr::new(ip, port));
    }
    if is_internal_hostname(cleaned) {
        return Err(format!(
            "internal hostname {cleaned} is blocked by SSRF security policy"
        ));
    }
    let addrs: Vec<SocketAddr> = lookup_host((cleaned, port))
        .await
        .map_err(|err| format!("DNS resolution failed for {cleaned}: {err}"))?
        .collect();
    if addrs.is_empty() {
        return Err(format!("no IP addresses found for {cleaned}"));
    }
    for addr in &addrs {
        validate_target_ip(addr.ip())?;
    }
    Ok(addrs[0])
}

/// Extracts hostname/authority from an HTTP(S) target URL safely before probing.
pub fn extract_http_target_host(raw: &str) -> Result<String, String> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return Err("HTTP target cannot be empty".to_owned());
    }
    let rest = if let Some(after) = trimmed.strip_prefix("http://") {
        after
    } else if let Some(after) = trimmed.strip_prefix("https://") {
        after
    } else if trimmed.contains("://") {
        return Err("only http and https URLs are permitted for HTTP probes".to_owned());
    } else {
        trimmed
    };
    let authority = rest.split(['/', '?', '#']).next().unwrap_or("").trim();
    if authority.is_empty() || authority.contains('@') {
        return Err("invalid HTTP probe target authority".to_owned());
    }
    let (host, _) = crate::ping::split_host_port(authority);
    let cleaned = host.trim_start_matches('[').trim_end_matches(']').trim();
    if cleaned.is_empty() {
        return Err("HTTP probe target host cannot be empty".to_owned());
    }
    Ok(cleaned.to_owned())
}

#[derive(Debug, Clone)]
struct BudgetEntry {
    ts: Instant,
    packets: u32,
}

#[derive(Debug, Default)]
struct BudgetState {
    global: VecDeque<BudgetEntry>,
    per_target: HashMap<String, VecDeque<BudgetEntry>>,
}

/// Sliding-window dispatch and packet rate budget guard preventing edge probe storms.
#[derive(Debug)]
pub struct ProbeBudgetGuard {
    window: Duration,
    max_global_dispatches: usize,
    max_global_packets: u32,
    max_target_dispatches: usize,
    max_target_packets: u32,
    state: Mutex<BudgetState>,
}

impl ProbeBudgetGuard {
    pub fn new(
        window: Duration,
        max_global_dispatches: usize,
        max_global_packets: u32,
        max_target_dispatches: usize,
        max_target_packets: u32,
    ) -> Self {
        Self {
            window,
            max_global_dispatches,
            max_global_packets,
            max_target_dispatches,
            max_target_packets,
            state: Mutex::new(BudgetState::default()),
        }
    }

    pub fn check_and_record(&self, target_key: &str, packet_cost: u32) -> Result<(), String> {
        let now = Instant::now();
        self.check_and_record_at(target_key, packet_cost.max(1), now)
    }

    fn check_and_record_at(
        &self,
        target_key: &str,
        packet_cost: u32,
        now: Instant,
    ) -> Result<(), String> {
        let key = target_key.trim().to_ascii_lowercase();
        let mut guard = self
            .state
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner());

        prune_queue(&mut guard.global, self.window, now);
        let global_dispatches = guard.global.len();
        let global_packets: u32 = guard.global.iter().map(|e| e.packets).sum();
        if global_dispatches >= self.max_global_dispatches
            || global_packets.saturating_add(packet_cost) > self.max_global_packets
        {
            return Err("edge global probe packet budget exceeded; try again shortly".to_owned());
        }

        if let Some(target_q) = guard.per_target.get_mut(&key) {
            prune_queue(target_q, self.window, now);
            let target_dispatches = target_q.len();
            let target_packets: u32 = target_q.iter().map(|e| e.packets).sum();
            if target_dispatches >= self.max_target_dispatches
                || target_packets.saturating_add(packet_cost) > self.max_target_packets
            {
                return Err(format!(
                    "edge per-target probe rate budget exceeded for {target_key}"
                ));
            }
        }

        if guard.per_target.len() >= MAX_TRACKED_TARGETS && !guard.per_target.contains_key(&key) {
            let window = self.window;
            guard.per_target.retain(|_, q| {
                prune_queue(q, window, now);
                !q.is_empty()
            });
        }

        let entry = BudgetEntry {
            ts: now,
            packets: packet_cost,
        };
        guard.global.push_back(entry.clone());
        guard.per_target.entry(key).or_default().push_back(entry);
        Ok(())
    }
}

fn prune_queue(q: &mut VecDeque<BudgetEntry>, window: Duration, now: Instant) {
    while let Some(front) = q.front() {
        if now.saturating_duration_since(front.ts) > window {
            q.pop_front();
        } else {
            break;
        }
    }
}

/// Checks global and per-target rate/packet budgets in the process-wide probe worker guard.
pub fn check_net_probe_budget(target: &str, packet_cost: u32) -> Result<(), String> {
    static GLOBAL_BUDGET: OnceLock<ProbeBudgetGuard> = OnceLock::new();
    let guard = GLOBAL_BUDGET.get_or_init(|| {
        ProbeBudgetGuard::new(
            BUDGET_WINDOW,
            MAX_GLOBAL_DISPATCHES_PER_WINDOW,
            MAX_GLOBAL_PACKETS_PER_WINDOW,
            MAX_TARGET_DISPATCHES_PER_WINDOW,
            MAX_TARGET_PACKETS_PER_WINDOW,
        )
    });
    guard.check_and_record(target, packet_cost)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn blocked_ip_rejects_all_internal_and_reserved_ranges() {
        let blocked = [
            "0.0.0.0",
            "0.10.0.1",
            "127.0.0.1",
            "127.255.0.1",
            "10.0.0.1",
            "172.16.0.1",
            "172.31.255.254",
            "192.168.1.1",
            "169.254.169.254",
            "100.64.0.1",
            "100.127.255.254",
            "198.18.0.1",
            "198.19.255.254",
            "224.0.0.1",
            "255.255.255.255",
            "::",
            "::1",
            "::ffff:127.0.0.1",
            "::ffff:169.254.169.254",
            "::ffff:100.64.1.1",
            "fc00::1",
            "fd12:3456::1",
            "fe80::1",
            "ff02::1",
        ];
        for raw in blocked {
            let ip = IpAddr::from_str(raw).expect(raw);
            assert!(is_blocked_ip(ip), "expected {raw} to be blocked");
            assert!(validate_target_ip(ip).is_err());
        }

        let allowed = [
            "1.1.1.1",
            "8.8.8.8",
            "9.9.9.9",
            "100.128.0.1",
            "198.20.0.1",
            "2606:4700:4700::1111",
            "2001:4860:4860::8888",
        ];
        for raw in allowed {
            let ip = IpAddr::from_str(raw).expect(raw);
            assert!(!is_blocked_ip(ip), "expected {raw} to be allowed");
            assert!(validate_target_ip(ip).is_ok());
        }
    }

    #[test]
    fn internal_hostnames_and_http_authority_are_validated() {
        assert!(is_internal_hostname("localhost"));
        assert!(is_internal_hostname("metadata.google.internal."));
        assert!(is_internal_hostname("router.home.arpa"));
        assert!(!is_internal_hostname("example.com"));

        assert_eq!(
            extract_http_target_host("https://1.1.1.1:8443/dns-query?a=1").unwrap(),
            "1.1.1.1"
        );
        assert!(extract_http_target_host("file:///etc/passwd").is_err());
        assert!(extract_http_target_host("http://user@127.0.0.1/").is_err());
    }

    #[test]
    fn budget_guard_enforces_per_target_and_global_limits() {
        let guard = ProbeBudgetGuard::new(Duration::from_secs(60), 5, 30, 2, 10);
        let t0 = Instant::now();
        assert!(guard.check_and_record_at("1.1.1.1", 4, t0).is_ok());
        assert!(guard.check_and_record_at("1.1.1.1", 4, t0).is_ok());
        // Third dispatch hits per-target dispatch limit (2)
        assert!(guard.check_and_record_at("1.1.1.1", 2, t0).is_err());
        // Different target allowed until global packet/dispatch budget hits
        assert!(guard.check_and_record_at("8.8.8.8", 8, t0).is_ok());
        assert!(guard.check_and_record_at("9.9.9.9", 8, t0).is_ok());
        assert!(guard.check_and_record_at("9.9.9.9", 8, t0).is_err());
        // After window passes, budget recovers
        let t1 = t0 + Duration::from_secs(61);
        assert!(guard.check_and_record_at("1.1.1.1", 4, t1).is_ok());
    }

    #[test]
    fn default_budget_supports_consecutive_traceroutes_while_staying_bounded() {
        let guard = ProbeBudgetGuard::new(
            BUDGET_WINDOW,
            MAX_GLOBAL_DISPATCHES_PER_WINDOW,
            MAX_GLOBAL_PACKETS_PER_WINDOW,
            MAX_TARGET_DISPATCHES_PER_WINDOW,
            MAX_TARGET_PACKETS_PER_WINDOW,
        );
        let t0 = Instant::now();

        // Default traceroute (30 hops * 3 probes = 90 packets) can run several times in 60s
        for _ in 0..6 {
            assert!(guard.check_and_record_at("example.com", 90, t0).is_ok());
        }
        // 7th default traceroute (630 > 600) is still blocked by conservative per-target cap
        assert!(guard.check_and_record_at("example.com", 90, t0).is_err());

        // Max-parameter traceroute (40 hops * 5 probes = 200 packets) allows 3 runs on another target
        for _ in 0..3 {
            assert!(guard.check_and_record_at("1.1.1.1", 200, t0).is_ok());
        }
        assert!(guard.check_and_record_at("1.1.1.1", 200, t0).is_err());
    }
}
