use anyhow::Result;
use reqwest::Client;
use serde::Deserialize;
use std::{
    ffi::CStr,
    net::{IpAddr, Ipv4Addr, Ipv6Addr},
    sync::{Arc, RwLock},
    time::Duration,
};

use crate::protocol::ApiResponse;

pub const UNSPECIFIED_IP: &str = "0.0.0.0";
const IP_SB_URL: &str = "https://api.ip.sb/ip";
const LOOKUP_TIMEOUT: Duration = Duration::from_secs(3);
const MAX_IP_RESPONSE_BYTES: usize = 64 * 1024;

#[derive(Clone)]
pub struct NodeIpResolver {
    current: Arc<RwLock<String>>,
    controller_geoip: String,
    v4_client: Client,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct InterfaceAddr {
    name: String,
    is_point_to_point: bool,
    ip: IpAddr,
}

#[derive(Debug, Deserialize)]
struct IpFieldResponse {
    ip: String,
}

impl NodeIpResolver {
    pub fn new(base_url: &str, user_agent: &str) -> Result<Self> {
        let v4_client = Client::builder()
            .timeout(LOOKUP_TIMEOUT)
            .connect_timeout(LOOKUP_TIMEOUT)
            .redirect(reqwest::redirect::Policy::none())
            .local_address(IpAddr::V4(Ipv4Addr::UNSPECIFIED))
            .user_agent(user_agent)
            .build()?;
        Ok(Self {
            current: Arc::new(RwLock::new(UNSPECIFIED_IP.to_owned())),
            controller_geoip: format!("{}/api/geoip", base_url.trim_end_matches('/')),
            v4_client,
        })
    }

    pub fn current(&self) -> String {
        self.current
            .read()
            .map_or_else(|_| UNSPECIFIED_IP.to_owned(), |value| value.clone())
    }

    pub async fn resolve_now(&self) -> String {
        if let Some(ip) = select_interface_ip(&read_interface_addrs()) {
            let s = ip.to_string();
            self.store(&s);
            return s;
        }
        if let Some(ip) = self.probe_geoip().await {
            let s = ip.to_string();
            self.store(&s);
            return s;
        }
        if let Some(ip) = fetch_plain_ip(&self.v4_client, IP_SB_URL).await {
            let s = ip.to_string();
            self.store(&s);
            return s;
        }
        self.current()
    }

    fn store(&self, ip: &str) {
        if let Ok(mut guard) = self.current.write() {
            *guard = ip.to_owned();
        }
    }

    async fn probe_geoip(&self) -> Option<IpAddr> {
        let resp = self.v4_client.get(&self.controller_geoip).send().await.ok()?;
        let body = read_bounded_body(resp).await.ok()?;
        let envelope: ApiResponse<IpFieldResponse> = serde_json::from_slice(&body).ok()?;
        if !envelope.error_msg.is_empty() {
            return None;
        }
        parse_public_ip(&envelope.data?.ip)
    }
}

async fn fetch_plain_ip(client: &Client, url: &str) -> Option<IpAddr> {
    let resp = client.get(url).send().await.ok()?;
    let body = read_bounded_body(resp).await.ok()?;
    let text = std::str::from_utf8(&body).ok()?;
    parse_public_ip(text)
}

async fn read_bounded_body(response: reqwest::Response) -> Result<Vec<u8>> {
    let mut response = response.error_for_status()?;
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

pub fn parse_public_ip(raw: &str) -> Option<IpAddr> {
    let parsed: IpAddr = raw.trim().parse().ok()?;
    normalize_public_ip(parsed)
}

pub fn normalize_public_ip(ip: IpAddr) -> Option<IpAddr> {
    match ip {
        IpAddr::V4(v4) => is_public_routable_v4(v4).then_some(IpAddr::V4(v4)),
        IpAddr::V6(v6) => match v6.to_ipv4_mapped() {
            Some(mapped) => is_public_routable_v4(mapped).then_some(IpAddr::V4(mapped)),
            None => is_public_routable_v6(v6).then_some(IpAddr::V6(v6)),
        },
    }
}

fn is_public_routable_v4(ip: Ipv4Addr) -> bool {
    let addr = u32::from(ip);
    const EXCLUDED_V4: &[(u32, u8)] = &[
        (u32::from_be_bytes([0, 0, 0, 0]), 8),
        (u32::from_be_bytes([10, 0, 0, 0]), 8),
        (u32::from_be_bytes([100, 64, 0, 0]), 10),
        (u32::from_be_bytes([127, 0, 0, 0]), 8),
        (u32::from_be_bytes([169, 254, 0, 0]), 16),
        (u32::from_be_bytes([172, 16, 0, 0]), 12),
        (u32::from_be_bytes([192, 0, 0, 0]), 24),
        (u32::from_be_bytes([192, 0, 2, 0]), 24),
        (u32::from_be_bytes([192, 88, 99, 0]), 24),
        (u32::from_be_bytes([192, 168, 0, 0]), 16),
        (u32::from_be_bytes([198, 18, 0, 0]), 15),
        (u32::from_be_bytes([198, 51, 100, 0]), 24),
        (u32::from_be_bytes([203, 0, 113, 0]), 24),
        (u32::from_be_bytes([224, 0, 0, 0]), 4),
        (u32::from_be_bytes([240, 0, 0, 0]), 4),
    ];
    !EXCLUDED_V4
        .iter()
        .any(|&(net, prefix)| ipv4_in_cidr(addr, net, prefix))
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
    let global_unicast_net = u128::from(Ipv6Addr::new(0x2000, 0, 0, 0, 0, 0, 0, 0));
    let mask = u128::MAX << 125;
    (addr & mask) == (global_unicast_net & mask) && !ip.is_loopback() && !ip.is_unspecified()
}

fn select_interface_ip(addrs: &[InterfaceAddr]) -> Option<IpAddr> {
    let mut public_v4: Vec<&InterfaceAddr> = Vec::new();
    let mut public_v6: Vec<&InterfaceAddr> = Vec::new();
    for item in addrs {
        if let Some(ip) = normalize_public_ip(item.ip) {
            match ip {
                IpAddr::V4(_) => public_v4.push(item),
                IpAddr::V6(_) => public_v6.push(item),
            }
        }
    }
    public_v4.sort_by(|a, b| (a.is_point_to_point, &a.name, a.ip).cmp(&(b.is_point_to_point, &b.name, b.ip)));
    public_v6.sort_by(|a, b| (a.is_point_to_point, &a.name, a.ip).cmp(&(b.is_point_to_point, &b.name, b.ip)));
    public_v4
        .first()
        .or_else(|| public_v6.first())
        .and_then(|item| normalize_public_ip(item.ip))
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
                let sin = unsafe { std::ptr::read_unaligned(entry.ifa_addr.cast::<libc::sockaddr_in>()) };
                IpAddr::V4(Ipv4Addr::from(sin.sin_addr.s_addr.to_ne_bytes()))
            }
            libc::AF_INET6 => {
                let sin6 = unsafe { std::ptr::read_unaligned(entry.ifa_addr.cast::<libc::sockaddr_in6>()) };
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

    #[test]
    fn filters_private_and_reserved_ips() {
        assert_eq!(parse_public_ip("8.8.8.8"), Some("8.8.8.8".parse().unwrap()));
        assert_eq!(parse_public_ip("10.0.0.1"), None);
        assert_eq!(parse_public_ip("169.254.169.254"), None);
        assert_eq!(parse_public_ip("::ffff:192.168.1.1"), None);
    }
}
