use super::ApiResponse;
use anyhow::Result;
use reqwest::Client;
use serde::Deserialize;
use std::{
    ffi::CStr,
    net::{IpAddr, Ipv4Addr, Ipv6Addr},
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, RwLock,
    },
    time::Duration,
};

pub(crate) const UNSPECIFIED_IP: &str = "0.0.0.0";
const IP_SB_URL: &str = "https://api.ip.sb/ip";
const IP_ME_URL: &str = "https://ip.me";
const IP2LOCATION_URL: &str = "https://api.ip2location.io/";
const LOOKUP_TIMEOUT: Duration = Duration::from_secs(3);
const REFRESH_INTERVAL: Duration = Duration::from_secs(60);
const MAX_IP_RESPONSE_BYTES: usize = 64 * 1024;
const CURL_COMPAT_USER_AGENT: &str = "curl/8.5.0";

#[derive(Clone)]
pub(crate) struct NodeIpResolver {
    current: Arc<RwLock<String>>,
    periodic_refresh: Arc<AtomicBool>,
    endpoints: ProbeEndpoints,
    v4_client: Client,
    v6_client: Client,
}

#[derive(Clone, Debug)]
struct ProbeEndpoints {
    ip_sb: String,
    ip_me: String,
    ip2location: String,
    controller_geoip: String,
}

impl ProbeEndpoints {
    fn for_controller(base_url: &str) -> Self {
        Self {
            ip_sb: IP_SB_URL.to_owned(),
            ip_me: IP_ME_URL.to_owned(),
            ip2location: IP2LOCATION_URL.to_owned(),
            controller_geoip: format!("{}/api/geoip", base_url.trim_end_matches('/')),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct InterfaceAddr {
    name: String,
    is_point_to_point: bool,
    ip: IpAddr,
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum InterfaceResolution {
    Found(IpAddr),
    NotFound {
        has_global_v6: bool,
        fallback_v6: Option<IpAddr>,
    },
}

#[derive(Debug, Deserialize)]
struct IpFieldResponse {
    ip: String,
}

impl NodeIpResolver {
    pub(crate) fn new(base_url: &str) -> Result<Self> {
        let user_agent = crate::edge_user_agent();
        let v4_client = Client::builder()
            .timeout(LOOKUP_TIMEOUT)
            .connect_timeout(LOOKUP_TIMEOUT)
            .redirect(reqwest::redirect::Policy::none())
            .local_address(IpAddr::V4(Ipv4Addr::UNSPECIFIED))
            .user_agent(user_agent.as_str())
            .build()?;
        let v6_client = Client::builder()
            .timeout(LOOKUP_TIMEOUT)
            .connect_timeout(LOOKUP_TIMEOUT)
            .redirect(reqwest::redirect::Policy::none())
            .local_address(IpAddr::V6(Ipv6Addr::UNSPECIFIED))
            .user_agent(user_agent)
            .build()?;
        Ok(Self {
            current: Arc::new(RwLock::new(UNSPECIFIED_IP.to_owned())),
            periodic_refresh: Arc::new(AtomicBool::new(false)),
            endpoints: ProbeEndpoints::for_controller(base_url),
            v4_client,
            v6_client,
        })
    }

    pub(crate) fn current(&self) -> String {
        self.current
            .read()
            .map_or_else(|_| UNSPECIFIED_IP.to_owned(), |value| value.clone())
    }

    pub(crate) async fn initialize(&self) {
        let probe = select_interface_ip(&read_interface_addrs());
        match probe {
            InterfaceResolution::Found(ip) => {
                self.periodic_refresh.store(false, Ordering::Release);
                let resolved = ip.to_string();
                self.store_ip(&resolved);
                tracing::info!(ip = %resolved, source = "interface", "resolved edge node IP");
            }
            InterfaceResolution::NotFound {
                has_global_v6,
                fallback_v6,
            } => {
                self.periodic_refresh.store(true, Ordering::Release);
                let (resolved, source) = self.resolve_http(has_global_v6, fallback_v6).await;
                self.store_ip(&resolved);
                if resolved == UNSPECIFIED_IP {
                    tracing::warn!(
                        ip = %resolved,
                        source,
                        "failed to resolve public edge node IP; reporting fallback IP"
                    );
                } else {
                    tracing::info!(ip = %resolved, source, "resolved edge node IP");
                }
            }
        }
    }

    pub(crate) async fn refresh_loop_if_needed(&self) {
        if !self.periodic_refresh.load(Ordering::Acquire) {
            std::future::pending::<()>().await;
            return;
        }
        let start = tokio::time::Instant::now() + REFRESH_INTERVAL;
        let mut ticker = tokio::time::interval_at(start, REFRESH_INTERVAL);
        ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
        loop {
            ticker.tick().await;
            self.refresh_once().await;
        }
    }

    async fn refresh_once(&self) {
        let (resolved, source) = match select_interface_ip(&read_interface_addrs()) {
            InterfaceResolution::Found(ip) => (ip.to_string(), "interface"),
            InterfaceResolution::NotFound {
                has_global_v6,
                fallback_v6,
            } => self.resolve_http(has_global_v6, fallback_v6).await,
        };
        let previous = self.current();
        self.store_ip(&resolved);
        if resolved != previous {
            if resolved == UNSPECIFIED_IP {
                tracing::warn!(
                    previous = %previous,
                    ip = %resolved,
                    source,
                    "edge node public IP unavailable; reporting fallback IP"
                );
            } else {
                tracing::info!(
                    previous = %previous,
                    ip = %resolved,
                    source,
                    "updated edge node IP"
                );
            }
        } else {
            tracing::debug!(ip = %resolved, source, "edge node IP unchanged");
        }
    }

    fn store_ip(&self, ip: &str) {
        if let Ok(mut guard) = self.current.write() {
            *guard = ip.to_owned();
        }
    }

    async fn resolve_http(
        &self,
        has_global_v6: bool,
        fallback_v6: Option<IpAddr>,
    ) -> (String, &'static str) {
        if let Some((ip, source)) = self.probe_external(&self.v4_client).await {
            return (ip.to_string(), source);
        }
        if let Some(ip) = self.probe_controller_geoip(&self.v4_client).await {
            return (ip.to_string(), "controller:/api/geoip");
        }
        if has_global_v6 {
            if let Some((ip, source)) = self.probe_external(&self.v6_client).await {
                return (ip.to_string(), source);
            }
        }
        if let Some(ip) = fallback_v6.and_then(normalize_public_ip) {
            return (ip.to_string(), "interface_v6");
        }
        if has_global_v6 || controller_may_use_ipv6(&self.endpoints.controller_geoip) {
            if let Some(ip) = self.probe_controller_geoip(&self.v6_client).await {
                return (ip.to_string(), "controller:/api/geoip");
            }
        }
        (UNSPECIFIED_IP.to_owned(), "unspecified")
    }

    async fn probe_external(&self, client: &Client) -> Option<(IpAddr, &'static str)> {
        if let Some(ip) = fetch_plain_ip(client, &self.endpoints.ip_sb, None).await {
            return Some((ip, "ip.sb"));
        }
        if let Some(ip) =
            fetch_plain_ip(client, &self.endpoints.ip_me, Some(CURL_COMPAT_USER_AGENT)).await
        {
            return Some((ip, "ip.me"));
        }
        if let Some(ip) = fetch_ip2location(client, &self.endpoints.ip2location).await {
            return Some((ip, "api.ip2location.io"));
        }
        None
    }

    async fn probe_controller_geoip(&self, client: &Client) -> Option<IpAddr> {
        let response = client
            .get(&self.endpoints.controller_geoip)
            .send()
            .await
            .ok()?;
        let body = read_bounded_body(response).await.ok()?;
        let envelope: ApiResponse<IpFieldResponse> = serde_json::from_slice(&body).ok()?;
        if !envelope.error_msg.is_empty() {
            return None;
        }
        let data = envelope.data?;
        parse_public_ip(&data.ip)
    }
}

fn controller_may_use_ipv6(url_str: &str) -> bool {
    let Ok(parsed) = url::Url::parse(url_str) else {
        return false;
    };
    match parsed.host() {
        Some(url::Host::Ipv4(_)) => false,
        Some(url::Host::Ipv6(_)) | Some(url::Host::Domain(_)) => true,
        None => false,
    }
}

async fn fetch_plain_ip(client: &Client, url: &str, user_agent: Option<&str>) -> Option<IpAddr> {
    let mut request = client.get(url);
    if let Some(agent) = user_agent {
        request = request.header(reqwest::header::USER_AGENT, agent);
    }
    let response = request.send().await.ok()?;
    let body = read_bounded_body(response).await.ok()?;
    let text = std::str::from_utf8(&body).ok()?;
    parse_public_ip(text)
}

async fn fetch_ip2location(client: &Client, url: &str) -> Option<IpAddr> {
    let response = client.get(url).send().await.ok()?;
    let body = read_bounded_body(response).await.ok()?;
    let decoded: IpFieldResponse = serde_json::from_slice(&body).ok()?;
    parse_public_ip(&decoded.ip)
}

async fn read_bounded_body(response: reqwest::Response) -> Result<Vec<u8>> {
    let mut response = response.error_for_status()?;
    if response
        .content_length()
        .is_some_and(|len| len > MAX_IP_RESPONSE_BYTES as u64)
    {
        anyhow::bail!("IP probe response exceeds size limit");
    }
    let mut body = Vec::new();
    while let Some(chunk) = response.chunk().await? {
        anyhow::ensure!(
            body.len().saturating_add(chunk.len()) <= MAX_IP_RESPONSE_BYTES,
            "IP probe response exceeds size limit"
        );
        body.extend_from_slice(&chunk);
    }
    Ok(body)
}

pub(crate) fn parse_public_ip(raw: &str) -> Option<IpAddr> {
    let parsed: IpAddr = raw.trim().parse().ok()?;
    normalize_public_ip(parsed)
}

pub(crate) fn normalize_public_ip(ip: IpAddr) -> Option<IpAddr> {
    match ip {
        IpAddr::V4(v4) => is_public_routable_v4(v4).then_some(IpAddr::V4(v4)),
        IpAddr::V6(v6) => {
            if let Some(mapped) = v6.to_ipv4_mapped() {
                return is_public_routable_v4(mapped).then_some(IpAddr::V4(mapped));
            }
            is_public_routable_v6(v6).then_some(IpAddr::V6(v6))
        }
    }
}

fn is_public_routable_v4(ip: Ipv4Addr) -> bool {
    let addr = u32::from(ip);
    const EXCLUDED_V4: &[(u32, u8)] = &[
        (u32::from_be_bytes([0, 0, 0, 0]), 8), // 0.0.0.0/8 (unspecified / "this network")
        (u32::from_be_bytes([10, 0, 0, 0]), 8), // 10.0.0.0/8 (RFC 1918 private)
        (u32::from_be_bytes([100, 64, 0, 0]), 10), // 100.64.0.0/10 (RFC 6598 CGNAT)
        (u32::from_be_bytes([127, 0, 0, 0]), 8), // 127.0.0.0/8 (loopback)
        (u32::from_be_bytes([169, 254, 0, 0]), 16), // 169.254.0.0/16 (link-local)
        (u32::from_be_bytes([172, 16, 0, 0]), 12), // 172.16.0.0/12 (RFC 1918 private)
        (u32::from_be_bytes([192, 0, 0, 0]), 24), // 192.0.0.0/24 (IETF protocol assignments)
        (u32::from_be_bytes([192, 0, 2, 0]), 24), // 192.0.2.0/24 (TEST-NET-1)
        (u32::from_be_bytes([192, 88, 99, 0]), 24), // 192.88.99.0/24 (deprecated 6to4 relay)
        (u32::from_be_bytes([192, 168, 0, 0]), 16), // 192.168.0.0/16 (RFC 1918 private)
        (u32::from_be_bytes([198, 18, 0, 0]), 15), // 198.18.0.0/15 (benchmarking)
        (u32::from_be_bytes([198, 51, 100, 0]), 24), // 198.51.100.0/24 (TEST-NET-2)
        (u32::from_be_bytes([203, 0, 113, 0]), 24), // 203.0.113.0/24 (TEST-NET-3)
        (u32::from_be_bytes([224, 0, 0, 0]), 4), // 224.0.0.0/4 (multicast)
        (u32::from_be_bytes([240, 0, 0, 0]), 4), // 240.0.0.0/4 (reserved + broadcast)
    ];
    !EXCLUDED_V4
        .iter()
        .any(|&(network, prefix)| ipv4_in_cidr(addr, network, prefix))
}

fn ipv4_in_cidr(addr: u32, network: u32, prefix: u8) -> bool {
    let mask = if prefix == 0 {
        0
    } else {
        u32::MAX << (32 - prefix)
    };
    (addr & mask) == (network & mask)
}

fn is_public_routable_v6(ip: Ipv6Addr) -> bool {
    let addr = u128::from(ip);
    // Global unicast IPv6 is currently allocated solely inside 2000::/3,
    // which naturally excludes ::/128, ::1/128, ::/96, ::ffff:0:0/96,
    // 64:ff9b::/96, 100::/64, fc00::/7, fe80::/10, fec0::/10, ff00::/8, etc.
    let global_unicast_net = u128::from(Ipv6Addr::new(0x2000, 0, 0, 0, 0, 0, 0, 0));
    if !ipv6_in_cidr(addr, global_unicast_net, 3) {
        return false;
    }
    const EXCLUDED_GLOBAL_V6: &[(Ipv6Addr, u8)] = &[
        (Ipv6Addr::new(0x2001, 0x0000, 0, 0, 0, 0, 0, 0), 32), // 2001::/32 (Teredo)
        (Ipv6Addr::new(0x2001, 0x0002, 0, 0, 0, 0, 0, 0), 48), // 2001:2::/48 (Benchmarking)
        (Ipv6Addr::new(0x2001, 0x0003, 0, 0, 0, 0, 0, 0), 32), // 2001:3::/32 (AMT)
        (Ipv6Addr::new(0x2001, 0x0004, 0x0112, 0, 0, 0, 0, 0), 48), // 2001:4:112::/48 (AS112-v6)
        (Ipv6Addr::new(0x2001, 0x0010, 0, 0, 0, 0, 0, 0), 28), // 2001:10::/28 (ORCHID)
        (Ipv6Addr::new(0x2001, 0x0020, 0, 0, 0, 0, 0, 0), 28), // 2001:20::/28 (ORCHIDv2)
        (Ipv6Addr::new(0x2001, 0x0db8, 0, 0, 0, 0, 0, 0), 32), // 2001:db8::/32 (Documentation)
        (Ipv6Addr::new(0x2002, 0, 0, 0, 0, 0, 0, 0), 16),      // 2002::/16 (6to4)
        (Ipv6Addr::new(0x3fff, 0, 0, 0, 0, 0, 0, 0), 20),      // 3fff::/20 (Documentation)
    ];
    !EXCLUDED_GLOBAL_V6
        .iter()
        .any(|&(network, prefix)| ipv6_in_cidr(addr, u128::from(network), prefix))
}

fn ipv6_in_cidr(addr: u128, network: u128, prefix: u8) -> bool {
    let mask = if prefix == 0 {
        0
    } else {
        u128::MAX << (128 - prefix)
    };
    (addr & mask) == (network & mask)
}

fn is_link_local_or_unspecified_v4(ip: Ipv4Addr) -> bool {
    let addr = u32::from(ip);
    ipv4_in_cidr(addr, u32::from_be_bytes([0, 0, 0, 0]), 8)
        || ipv4_in_cidr(addr, u32::from_be_bytes([169, 254, 0, 0]), 16)
}

fn select_interface_ip(addrs: &[InterfaceAddr]) -> InterfaceResolution {
    let mut public_v4: Vec<&InterfaceAddr> = Vec::new();
    let mut public_v6: Vec<&InterfaceAddr> = Vec::new();
    let mut has_non_loopback_v4 = false;

    for item in addrs {
        match item.ip {
            IpAddr::V4(v4) => {
                if !is_link_local_or_unspecified_v4(v4) && !v4.is_loopback() && !v4.is_multicast() {
                    has_non_loopback_v4 = true;
                }
                if let Some(normalized) = normalize_public_ip(IpAddr::V4(v4)) {
                    if matches!(normalized, IpAddr::V4(_)) {
                        public_v4.push(item);
                    }
                }
            }
            IpAddr::V6(v6) => {
                if let Some(normalized) = normalize_public_ip(IpAddr::V6(v6)) {
                    match normalized {
                        IpAddr::V4(mapped) => {
                            if !is_link_local_or_unspecified_v4(mapped) {
                                has_non_loopback_v4 = true;
                            }
                            public_v4.push(item);
                        }
                        IpAddr::V6(_) => {
                            public_v6.push(item);
                        }
                    }
                }
            }
        }
    }

    public_v4.sort_by(|left, right| {
        (left.is_point_to_point, &left.name, left.ip).cmp(&(
            right.is_point_to_point,
            &right.name,
            right.ip,
        ))
    });
    public_v6.sort_by(|left, right| {
        (left.is_point_to_point, &left.name, left.ip).cmp(&(
            right.is_point_to_point,
            &right.name,
            right.ip,
        ))
    });

    if let Some(best_v4) = public_v4
        .first()
        .and_then(|item| normalize_public_ip(item.ip))
    {
        return InterfaceResolution::Found(best_v4);
    }

    let best_v6 = public_v6
        .first()
        .and_then(|item| normalize_public_ip(item.ip));
    if !has_non_loopback_v4 {
        if let Some(v6) = best_v6 {
            return InterfaceResolution::Found(v6);
        }
    }

    InterfaceResolution::NotFound {
        has_global_v6: best_v6.is_some(),
        fallback_v6: best_v6,
    }
}

struct IfAddrsGuard(*mut libc::ifaddrs);

impl Drop for IfAddrsGuard {
    fn drop(&mut self) {
        if !self.0.is_null() {
            unsafe {
                libc::freeifaddrs(self.0);
            }
        }
    }
}

fn read_interface_addrs() -> Vec<InterfaceAddr> {
    let mut raw: *mut libc::ifaddrs = std::ptr::null_mut();
    if unsafe { libc::getifaddrs(&mut raw) } != 0 {
        return Vec::new();
    }
    let _guard = IfAddrsGuard(raw);
    let mut results = Vec::new();
    let mut cursor = raw;
    while !cursor.is_null() {
        let entry = unsafe { &*cursor };
        cursor = entry.ifa_next;

        let flags = u64::from(entry.ifa_flags);
        let up = (flags & (libc::IFF_UP as u64)) != 0;
        let loopback = (flags & (libc::IFF_LOOPBACK as u64)) != 0;
        if !up || loopback || entry.ifa_addr.is_null() {
            continue;
        }
        let is_point_to_point = (flags & (libc::IFF_POINTOPOINT as u64)) != 0;
        let name = if entry.ifa_name.is_null() {
            String::new()
        } else {
            unsafe { CStr::from_ptr(entry.ifa_name) }
                .to_string_lossy()
                .into_owned()
        };

        let family = unsafe { (*entry.ifa_addr).sa_family as i32 };
        let ip = match family {
            libc::AF_INET => {
                let sin =
                    unsafe { std::ptr::read_unaligned(entry.ifa_addr.cast::<libc::sockaddr_in>()) };
                IpAddr::V4(Ipv4Addr::from(sin.sin_addr.s_addr.to_ne_bytes()))
            }
            libc::AF_INET6 => {
                let sin6 = unsafe {
                    std::ptr::read_unaligned(entry.ifa_addr.cast::<libc::sockaddr_in6>())
                };
                IpAddr::V6(Ipv6Addr::from(sin6.sin6_addr.s6_addr))
            }
            _ => continue,
        };
        results.push(InterfaceAddr {
            name,
            is_point_to_point,
            ip,
        });
    }
    results
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::{
        io::{AsyncReadExt, AsyncWriteExt},
        net::TcpListener,
    };

    #[test]
    fn excludes_private_and_reserved_cidrs() {
        let public_cases = [
            "1.1.1.1",
            "8.8.8.8",
            "82.22.26.203",
            "172.32.0.1",
            "100.128.0.1",
            "::ffff:8.8.8.8",
            "2606:4700:4700::1111",
            "2001:4860:4860::8888",
        ];
        for raw in public_cases {
            assert!(
                parse_public_ip(raw).is_some(),
                "expected {raw} to be accepted as public"
            );
        }

        let excluded_cases = [
            "",
            "<html>not-an-ip</html>",
            "0.0.0.0",
            "0.10.20.30",
            "10.0.0.1",
            "100.64.0.1",
            "100.127.255.254",
            "127.0.0.1",
            "169.254.169.254",
            "172.16.0.1",
            "172.27.0.1",
            "172.31.255.254",
            "192.0.0.1",
            "192.0.2.1",
            "192.88.99.1",
            "192.168.1.1",
            "198.18.0.1",
            "198.19.255.254",
            "198.51.100.1",
            "203.0.113.10",
            "224.0.0.1",
            "240.0.0.1",
            "255.255.255.255",
            "::",
            "::1",
            "::ffff:172.27.0.1",
            "64:ff9b::1.2.3.4",
            "100::1",
            "2001::1",
            "2001:2::1",
            "2001:3::1",
            "2001:4:112::1",
            "2001:10::1",
            "2001:20::1",
            "2001:db8::1",
            "2002::1",
            "3fff::1",
            "fc00::1",
            "fd12:3456::1",
            "fe80::1",
            "ff02::1",
        ];
        for raw in excluded_cases {
            assert!(
                parse_public_ip(raw).is_none(),
                "expected {raw:?} to be excluded"
            );
        }
    }

    #[test]
    fn interface_selection_prioritizes_public_ipv4_and_excludes_internal() {
        let only_internal = vec![
            InterfaceAddr {
                name: "eth0".into(),
                is_point_to_point: false,
                ip: "172.27.0.1".parse().unwrap(),
            },
            InterfaceAddr {
                name: "eth1".into(),
                is_point_to_point: false,
                ip: "100.64.0.5".parse().unwrap(),
            },
            InterfaceAddr {
                name: "eth0".into(),
                is_point_to_point: false,
                ip: "fe80::1".parse().unwrap(),
            },
        ];
        assert_eq!(
            select_interface_ip(&only_internal),
            InterfaceResolution::NotFound {
                has_global_v6: false,
                fallback_v6: None
            }
        );

        let mixed = vec![
            InterfaceAddr {
                name: "docker0".into(),
                is_point_to_point: false,
                ip: "172.17.0.1".parse().unwrap(),
            },
            InterfaceAddr {
                name: "wg0".into(),
                is_point_to_point: true,
                ip: "1.1.1.1".parse().unwrap(),
            },
            InterfaceAddr {
                name: "eth0".into(),
                is_point_to_point: false,
                ip: "82.22.26.203".parse().unwrap(),
            },
        ];
        assert_eq!(
            select_interface_ip(&mixed),
            InterfaceResolution::Found("82.22.26.203".parse().unwrap())
        );

        let nat_v4_with_v6 = vec![
            InterfaceAddr {
                name: "eth0".into(),
                is_point_to_point: false,
                ip: "10.0.0.5".parse().unwrap(),
            },
            InterfaceAddr {
                name: "eth0".into(),
                is_point_to_point: false,
                ip: "2606:4700:4700::1111".parse().unwrap(),
            },
        ];
        assert_eq!(
            select_interface_ip(&nat_v4_with_v6),
            InterfaceResolution::NotFound {
                has_global_v6: true,
                fallback_v6: Some("2606:4700:4700::1111".parse().unwrap())
            }
        );

        let ipv6_only = vec![InterfaceAddr {
            name: "eth0".into(),
            is_point_to_point: false,
            ip: "2606:4700:4700::1111".parse().unwrap(),
        }];
        assert_eq!(
            select_interface_ip(&ipv6_only),
            InterfaceResolution::Found("2606:4700:4700::1111".parse().unwrap())
        );
    }

    async fn spawn_mock_http<F>(handler: F) -> String
    where
        F: Fn(String) -> (u16, &'static str, String) + Send + Sync + 'static,
    {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let handler = Arc::new(handler);
        tokio::spawn(async move {
            loop {
                let Ok((mut stream, _)) = listener.accept().await else {
                    break;
                };
                let handler = handler.clone();
                tokio::spawn(async move {
                    let mut buf = [0u8; 4096];
                    let n = stream.read(&mut buf).await.unwrap_or(0);
                    let req = String::from_utf8_lossy(&buf[..n]).into_owned();
                    let (status, content_type, body) = handler(req);
                    let response = format!(
                        "HTTP/1.1 {status} OK\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                        body.len()
                    );
                    let _ = stream.write_all(response.as_bytes()).await;
                });
            }
        });
        format!("http://{addr}")
    }

    fn resolver_with_endpoints(endpoints: ProbeEndpoints) -> NodeIpResolver {
        let mut resolver = NodeIpResolver::new("http://127.0.0.1:1").unwrap();
        resolver.endpoints = endpoints;
        resolver
    }

    #[tokio::test]
    async fn http_fallback_chain_and_unspecified_fallback() {
        // 1. Primary ip.sb works -> returns ip.sb without falling back.
        let ip_sb = spawn_mock_http(|_| (200, "text/plain", "82.22.26.203\n".into())).await;
        let ip_me = spawn_mock_http(|_| (500, "text/plain", "fail".into())).await;
        let ip2loc = spawn_mock_http(|_| (500, "application/json", "{}".into())).await;
        let ctrl = spawn_mock_http(|_| (500, "application/json", "{}".into())).await;
        let resolver = resolver_with_endpoints(ProbeEndpoints {
            ip_sb,
            ip_me,
            ip2location: ip2loc,
            controller_geoip: format!("{ctrl}/api/geoip"),
        });
        assert_eq!(
            resolver.resolve_http(false, None).await,
            ("82.22.26.203".into(), "ip.sb")
        );

        // 2. ip.sb returns private IP -> falls back to ip.me (verifying curl UA).
        let ip_sb = spawn_mock_http(|_| (200, "text/plain", "172.27.0.1\n".into())).await;
        let ip_me = spawn_mock_http(|req| {
            let has_curl_ua = req
                .lines()
                .any(|line| line.to_ascii_lowercase().starts_with("user-agent: curl/"));
            if has_curl_ua {
                (200, "text/plain", "138.2.50.10\n".into())
            } else {
                (200, "text/html", "<!DOCTYPE html>".into())
            }
        })
        .await;
        let ip2loc = spawn_mock_http(|_| (500, "application/json", "{}".into())).await;
        let ctrl = spawn_mock_http(|_| (500, "application/json", "{}".into())).await;
        let resolver = resolver_with_endpoints(ProbeEndpoints {
            ip_sb,
            ip_me,
            ip2location: ip2loc,
            controller_geoip: format!("{ctrl}/api/geoip"),
        });
        assert_eq!(
            resolver.resolve_http(false, None).await,
            ("138.2.50.10".into(), "ip.me")
        );

        // 3. ip.sb and ip.me fail -> falls back to api.ip2location.io JSON.
        let ip_sb = spawn_mock_http(|_| (503, "text/plain", "down".into())).await;
        let ip_me = spawn_mock_http(|_| (200, "text/html", "<html>blocked</html>".into())).await;
        let ip2loc =
            spawn_mock_http(|_| (200, "application/json", r#"{"ip":"1.1.1.1"}"#.into())).await;
        let ctrl = spawn_mock_http(|_| (500, "application/json", "{}".into())).await;
        let resolver = resolver_with_endpoints(ProbeEndpoints {
            ip_sb,
            ip_me,
            ip2location: ip2loc,
            controller_geoip: format!("{ctrl}/api/geoip"),
        });
        assert_eq!(
            resolver.resolve_http(false, None).await,
            ("1.1.1.1".into(), "api.ip2location.io")
        );

        // 4. External APIs all fail -> falls back to controller /api/geoip.
        let ip_sb = spawn_mock_http(|_| (500, "text/plain", "err".into())).await;
        let ip_me = spawn_mock_http(|_| (500, "text/plain", "err".into())).await;
        let ip2loc = spawn_mock_http(|_| (500, "application/json", "{}".into())).await;
        let ctrl = spawn_mock_http(|_| {
            (
                200,
                "application/json",
                r#"{"error_msg":"","data":{"ip":"8.8.8.8"}}"#.into(),
            )
        })
        .await;
        let resolver = resolver_with_endpoints(ProbeEndpoints {
            ip_sb,
            ip_me,
            ip2location: ip2loc,
            controller_geoip: format!("{ctrl}/api/geoip"),
        });
        assert_eq!(
            resolver.resolve_http(false, None).await,
            ("8.8.8.8".into(), "controller:/api/geoip")
        );

        // 5. All fail or return internal IP -> falls back to 0.0.0.0.
        let ip_sb = spawn_mock_http(|_| (500, "text/plain", "err".into())).await;
        let ip_me = spawn_mock_http(|_| (500, "text/plain", "err".into())).await;
        let ip2loc = spawn_mock_http(|_| (500, "application/json", "{}".into())).await;
        let ctrl = spawn_mock_http(|_| {
            (
                200,
                "application/json",
                r#"{"error_msg":"","data":{"ip":"172.27.0.1"}}"#.into(),
            )
        })
        .await;
        let resolver = resolver_with_endpoints(ProbeEndpoints {
            ip_sb,
            ip_me,
            ip2location: ip2loc,
            controller_geoip: format!("{ctrl}/api/geoip"),
        });
        assert_eq!(
            resolver.resolve_http(false, None).await,
            ("0.0.0.0".into(), "unspecified")
        );
    }
}
