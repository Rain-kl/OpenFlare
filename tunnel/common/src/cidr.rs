use anyhow::{Context, Result};
use ipnet::IpNet;
use std::net::{IpAddr, SocketAddr};
use std::time::Duration;
use tokio::net::TcpStream;

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct CidrPolicy {
    allowed: Vec<IpNet>,
    blocked: Vec<IpNet>,
}

#[derive(Debug)]
pub enum ConnectTargetError {
    Blocked(String),
    Io(anyhow::Error),
}

impl CidrPolicy {
    pub fn parse(allowed: &[String], blocked: &[String]) -> Result<Self> {
        let allowed = allowed
            .iter()
            .map(|raw| raw.trim())
            .filter(|raw| !raw.is_empty())
            .map(|raw| {
                raw.parse::<IpNet>()
                    .or_else(|_| raw.parse::<IpAddr>().map(IpNet::from))
                    .with_context(|| format!("invalid allowed CIDR: {raw}"))
            })
            .collect::<Result<Vec<_>>>()?;
        let blocked = blocked
            .iter()
            .map(|raw| raw.trim())
            .filter(|raw| !raw.is_empty())
            .map(|raw| {
                raw.parse::<IpNet>()
                    .or_else(|_| raw.parse::<IpAddr>().map(IpNet::from))
                    .with_context(|| format!("invalid blocked CIDR: {raw}"))
            })
            .collect::<Result<Vec<_>>>()?;
        Ok(Self { allowed, blocked })
    }

    pub fn is_ip_allowed(&self, ip: IpAddr) -> bool {
        let normalized = normalize_ip(ip);
        if self.blocked.iter().any(|net| net.contains(&normalized)) {
            return false;
        }
        if self.allowed.is_empty() {
            return true;
        }
        self.allowed.iter().any(|net| net.contains(&normalized))
    }

    pub fn filter_addresses(&self, addrs: Vec<SocketAddr>) -> Result<Vec<SocketAddr>, String> {
        if addrs.is_empty() {
            return Err("target host did not resolve to any address".to_owned());
        }
        let permitted: Vec<SocketAddr> = addrs
            .into_iter()
            .filter(|addr| self.is_ip_allowed(addr.ip()))
            .collect();
        if permitted.is_empty() {
            return Err("target IP rejected by tunnel CIDR policy".to_owned());
        }
        Ok(permitted)
    }
}

pub async fn connect_validated_tcp(
    host: &str,
    port: u16,
    policy: &CidrPolicy,
    connect_timeout: Duration,
) -> Result<(TcpStream, SocketAddr), ConnectTargetError> {
    let resolved: Vec<SocketAddr> = if let Ok(ip) = host.parse::<IpAddr>() {
        vec![SocketAddr::new(ip, port)]
    } else {
        tokio::net::lookup_host((host, port))
            .await
            .map_err(|err| ConnectTargetError::Io(anyhow::anyhow!("resolve target {host}: {err}")))?
            .collect()
    };

    // Security: require every DNS-resolved address to pass the blocklist when block rules are active
    // so dual-A/AAAA DNS rebinding cannot smuggle a blocked address.
    for addr in &resolved {
        let normalized = normalize_ip(addr.ip());
        if policy.blocked.iter().any(|net| net.contains(&normalized)) {
            return Err(ConnectTargetError::Blocked(format!(
                "target resolved IP {} matches blocked CIDR rule",
                addr.ip()
            )));
        }
    }

    let permitted = policy
        .filter_addresses(resolved)
        .map_err(ConnectTargetError::Blocked)?;

    let mut last_err = None;
    for addr in permitted {
        match tokio::time::timeout(connect_timeout, TcpStream::connect(addr)).await {
            Ok(Ok(stream)) => {
                let _ = stream.set_nodelay(true);
                return Ok((stream, addr));
            }
            Ok(Err(err)) => last_err = Some(anyhow::anyhow!("connect {addr}: {err}")),
            Err(_) => {
                last_err = Some(anyhow::anyhow!(
                    "connect {addr} timed out after {}ms",
                    connect_timeout.as_millis()
                ))
            }
        }
    }

    Err(ConnectTargetError::Io(last_err.unwrap_or_else(|| {
        anyhow::anyhow!("no reachable target address for {host}:{port}")
    })))
}

fn normalize_ip(ip: IpAddr) -> IpAddr {
    match ip {
        IpAddr::V6(v6) => match v6.to_ipv4_mapped() {
            Some(v4) => IpAddr::V4(v4),
            None => IpAddr::V6(v6),
        },
        other => other,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn enforces_allowed_and_blocked_cidrs_including_ipv4_mapped_v6() {
        let policy = CidrPolicy::parse(
            &["10.0.0.0/8".into(), "127.0.0.1".into()],
            &["169.254.0.0/16".into(), "10.99.0.0/16".into()],
        )
        .expect("parse policy");

        assert!(policy.is_ip_allowed("10.1.2.3".parse().unwrap()));
        assert!(policy.is_ip_allowed("127.0.0.1".parse().unwrap()));
        assert!(!policy.is_ip_allowed("10.99.1.5".parse().unwrap()));
        assert!(!policy.is_ip_allowed("169.254.169.254".parse().unwrap()));
        assert!(!policy.is_ip_allowed("::ffff:169.254.169.254".parse().unwrap()));
        assert!(!policy.is_ip_allowed("192.168.1.10".parse().unwrap()));
    }

    #[tokio::test]
    async fn connect_validated_tcp_blocks_disallowed_loopback_target() {
        let policy = CidrPolicy::parse(&[], &["127.0.0.0/8".into()]).expect("parse");
        let err = connect_validated_tcp("127.0.0.1", 80, &policy, Duration::from_millis(200))
            .await
            .expect_err("loopback should be blocked");
        assert!(matches!(err, ConnectTargetError::Blocked(_)));
    }
}
