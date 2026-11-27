use crate::types::{PingBatchRequest, PingBatchResult, PingRequest, PingResult, PingSample, PingType};
use std::net::{IpAddr, Ipv4Addr, Ipv6Addr, SocketAddr};
use std::str::FromStr;
use std::sync::atomic::{AtomicU16, Ordering};
use std::sync::OnceLock;
use std::time::{Duration, Instant};
use time::format_description::well_known::Rfc3339;
use time::OffsetDateTime;
use tokio::net::{lookup_host, TcpStream};

/// Tunable timeouts for latency probes.
#[derive(Debug, Clone)]
pub struct PingOptions {
    pub icmp_timeout: Duration,
    pub tcp_timeout: Duration,
    pub http_timeout: Duration,
    pub user_agent: String,
}

impl Default for PingOptions {
    fn default() -> Self {
        Self {
            icmp_timeout: Duration::from_secs(3),
            tcp_timeout: Duration::from_secs(5),
            http_timeout: Duration::from_secs(5),
            user_agent: format!("openflare-probe/{}", env!("CARGO_PKG_VERSION")),
        }
    }
}

/// Executes a single ICMP, TCP, or HTTP latency probe and always returns a
/// [`PingResult`] (`value = -1` on timeout/failure).
pub async fn execute_ping(
    req: &PingRequest,
    http_client: Option<&reqwest::Client>,
    options: &PingOptions,
) -> PingResult {
    let ping_type = PingType::parse(&req.ping_type);
    let value = match ping_type {
        Some(PingType::Icmp) => measure_icmp(&req.ping_target, options.icmp_timeout).await,
        Some(PingType::Tcp) => measure_tcp(&req.ping_target, options.tcp_timeout).await,
        Some(PingType::Http) => {
            measure_http(
                &req.ping_target,
                http_client,
                options.http_timeout,
                &options.user_agent,
            )
            .await
        }
        None => {
            tracing::warn!(ping_type = %req.ping_type, task_id = req.ping_task_id, "unsupported ping type");
            -1
        }
    };

    PingResult {
        task_id: req.ping_task_id,
        ping_type: ping_type
            .map(|t| t.as_str().to_owned())
            .unwrap_or_else(|| req.ping_type.clone()),
        value,
        finished_at: format_rfc3339_now(),
    }
}

/// Executes a multi-sample ICMP, TCP, or HTTP latency probe batch by reusing
/// [`execute_ping`], returning packet loss and latency percentiles/jitter.
pub async fn execute_ping_batch(
    req: &PingBatchRequest,
    http_client: Option<&reqwest::Client>,
    options: &PingOptions,
) -> PingBatchResult {
    let trimmed_target = req.target.trim();
    let Some(ping_type) = PingType::parse(&req.protocol) else {
        return PingBatchResult {
            protocol: req.protocol.clone(),
            target: trimmed_target.to_owned(),
            resolved_ip: None,
            sent: 0,
            received: 0,
            loss_rate: 100.0,
            min_ms: 0.0,
            avg_ms: 0.0,
            max_ms: 0.0,
            p95_ms: 0.0,
            jitter_ms: 0.0,
            samples: Vec::new(),
            error: Some(format!("unsupported ping protocol: {}", req.protocol)),
        };
    };

    if trimmed_target.is_empty() {
        return PingBatchResult {
            protocol: ping_type.as_str().to_owned(),
            target: String::new(),
            resolved_ip: None,
            sent: 0,
            received: 0,
            loss_rate: 100.0,
            min_ms: 0.0,
            avg_ms: 0.0,
            max_ms: 0.0,
            p95_ms: 0.0,
            jitter_ms: 0.0,
            samples: Vec::new(),
            error: Some("ping target cannot be empty".to_owned()),
        };
    }

    let count = req.count.clamp(1, 10);
    let interval = Duration::from_millis(req.interval_ms.clamp(50, 2000));
    let mut effective_options = options.clone();
    if let Some(timeout_ms) = req.timeout_ms.filter(|&v| v > 0) {
        let clamped = Duration::from_millis(timeout_ms.clamp(200, 15_000));
        effective_options.icmp_timeout = clamped;
        effective_options.tcp_timeout = clamped;
        effective_options.http_timeout = clamped;
    }

    let (host, parsed_port) = split_host_port(trimmed_target);
    let resolved_ip = match ping_type {
        PingType::Http => resolve_ip(&host).await.ok().map(|ip| ip.to_string()),
        PingType::Icmp | PingType::Tcp => resolve_ip(&host).await.ok().map(|ip| ip.to_string()),
    };

    let ping_target = match ping_type {
        PingType::Tcp => {
            let port = req.port.unwrap_or(parsed_port);
            format_host_port(resolved_ip.as_deref().unwrap_or(&host), port)
        }
        PingType::Icmp => resolved_ip.clone().unwrap_or_else(|| host.clone()),
        PingType::Http => trimmed_target.to_owned(),
    };

    let single_req = PingRequest {
        ping_task_id: 0,
        ping_type: ping_type.as_str().to_owned(),
        ping_target,
    };

    let mut samples = Vec::with_capacity(usize::from(count));
    let mut successes = Vec::with_capacity(usize::from(count));
    for seq in 1..=count {
        let res = execute_ping(&single_req, http_client, &effective_options).await;
        let success = res.value >= 0;
        if success {
            successes.push(res.value as f64);
        }
        samples.push(PingSample {
            seq,
            rtt_ms: res.value,
            success,
        });
        if seq < count {
            tokio::time::sleep(interval).await;
        }
    }

    let received = successes.len() as u8;
    let loss_rate = if count > 0 {
        f64::from(count - received) / f64::from(count) * 100.0
    } else {
        0.0
    };

    let (min_ms, avg_ms, max_ms, p95_ms, jitter_ms) = summarize_latencies(&successes);
    let error = if received == 0 {
        Some("all ping probes timed out or failed".to_owned())
    } else {
        None
    };

    PingBatchResult {
        protocol: ping_type.as_str().to_owned(),
        target: trimmed_target.to_owned(),
        resolved_ip,
        sent: count,
        received,
        loss_rate,
        min_ms,
        avg_ms,
        max_ms,
        p95_ms,
        jitter_ms,
        samples,
        error,
    }
}

fn format_host_port(host: &str, port: u16) -> String {
    if host.contains(':') && !host.starts_with('[') {
        format!("[{host}]:{port}")
    } else {
        format!("{host}:{port}")
    }
}

fn summarize_latencies(values: &[f64]) -> (f64, f64, f64, f64, f64) {
    if values.is_empty() {
        return (0.0, 0.0, 0.0, 0.0, 0.0);
    }
    let mut jitter_sum = 0.0;
    for pair in values.windows(2) {
        jitter_sum += (pair[1] - pair[0]).abs();
    }
    let jitter = if values.len() > 1 {
        jitter_sum / (values.len() - 1) as f64
    } else {
        0.0
    };

    let mut sorted = values.to_vec();
    sorted.sort_by(f64::total_cmp);
    let min_ms = sorted[0];
    let max_ms = sorted[sorted.len() - 1];
    let avg_ms = sorted.iter().sum::<f64>() / sorted.len() as f64;
    let p95_idx = ((sorted.len() as f64) * 0.95).ceil() as usize;
    let p95_ms = sorted[p95_idx.saturating_sub(1).min(sorted.len() - 1)];
    (min_ms, avg_ms, max_ms, p95_ms, jitter)
}

async fn measure_tcp(target: &str, timeout: Duration) -> i64 {
    let (host, port) = split_host_port(target.trim());
    if host.is_empty() {
        return -1;
    }

    let addr = match resolve_socket_addr(&host, port).await {
        Ok(addr) => addr,
        Err(error) => {
            tracing::debug!(%error, target, "tcp ping DNS resolution failed");
            return -1;
        }
    };

    let started = Instant::now();
    match tokio::time::timeout(timeout, TcpStream::connect(addr)).await {
        Ok(Ok(_stream)) => elapsed_ms(started),
        Ok(Err(error)) => {
            tracing::debug!(%error, %addr, "tcp ping connect failed");
            -1
        }
        Err(_) => -1,
    }
}

async fn measure_http(
    target: &str,
    shared_client: Option<&reqwest::Client>,
    timeout: Duration,
    user_agent: &str,
) -> i64 {
    let trimmed = target.trim();
    if trimmed.is_empty() {
        return -1;
    }
    let url = if trimmed.starts_with("http://") || trimmed.starts_with("https://") {
        trimmed.to_owned()
    } else {
        format!("http://{trimmed}")
    };

    let owned_client;
    let client = if let Some(existing) = shared_client {
        existing
    } else {
        match reqwest::Client::builder()
            .timeout(timeout)
            .user_agent(user_agent)
            .build()
        {
            Ok(built) => {
                owned_client = built;
                &owned_client
            }
            Err(_) => return -1,
        }
    };

    let started = Instant::now();
    match client
        .get(&url)
        .timeout(timeout)
        .header(reqwest::header::USER_AGENT, user_agent)
        .send()
        .await
    {
        Ok(_resp) => elapsed_ms(started),
        Err(error) => {
            tracing::debug!(%error, %url, "http ping failed");
            -1
        }
    }
}

async fn measure_icmp(target: &str, timeout: Duration) -> i64 {
    let trimmed = target.trim();
    if trimmed.is_empty() {
        return -1;
    }

    let ip = match resolve_ip(trimmed).await {
        Ok(ip) => ip,
        Err(error) => {
            tracing::debug!(%error, target = trimmed, "icmp ping DNS resolution failed");
            return -1;
        }
    };

    #[cfg(unix)]
    {
        match tokio::task::spawn_blocking(move || unix_icmp_ping(ip, timeout)).await {
            Ok(Ok(ms)) => ms,
            Ok(Err(error)) => {
                tracing::debug!(%error, %ip, "icmp ping failed");
                -1
            }
            Err(_) => -1,
        }
    }

    #[cfg(not(unix))]
    {
        let _ = (ip, timeout);
        -1
    }
}

pub(crate) async fn resolve_ip(host_or_ip: &str) -> Result<IpAddr, String> {
    let cleaned = host_or_ip.trim_start_matches('[').trim_end_matches(']');
    if let Ok(ip) = IpAddr::from_str(cleaned) {
        return Ok(ip);
    }
    let mut addrs = lookup_host((cleaned, 80))
        .await
        .map_err(|err| err.to_string())?;
    addrs
        .next()
        .map(|addr| addr.ip())
        .ok_or_else(|| format!("no IP addresses found for {cleaned}"))
}

async fn resolve_socket_addr(host: &str, port: u16) -> Result<SocketAddr, String> {
    if let Ok(ip) = IpAddr::from_str(host) {
        return Ok(SocketAddr::new(ip, port));
    }
    let mut addrs = lookup_host((host, port))
        .await
        .map_err(|err| err.to_string())?;
    addrs
        .next()
        .ok_or_else(|| format!("no address found for {host}:{port}"))
}

/// Splits `host`, `host:port`, `ipv6`, or `[ipv6]:port` (default port 80).
pub fn split_host_port(raw: &str) -> (String, u16) {
    let trimmed = raw.trim();
    if let Ok(ip) = IpAddr::from_str(trimmed) {
        return (ip.to_string(), 80);
    }
    if let Some(stripped) = trimmed.strip_prefix('[') {
        if let Some((host, rest)) = stripped.split_once(']') {
            if let Some(port_str) = rest.strip_prefix(':') {
                if let Ok(port) = port_str.parse::<u16>() {
                    return (host.to_owned(), port);
                }
            }
            return (host.to_owned(), 80);
        }
    }
    if let Some((host, port_str)) = trimmed.rsplit_once(':') {
        if !host.contains(':') {
            if let Ok(port) = port_str.parse::<u16>() {
                return (host.to_owned(), port);
            }
        }
    }
    (trimmed.to_owned(), 80)
}

fn elapsed_ms(started: Instant) -> i64 {
    i64::try_from(started.elapsed().as_millis()).unwrap_or(i64::MAX)
}

fn format_rfc3339_now() -> String {
    OffsetDateTime::now_utc()
        .format(&Rfc3339)
        .unwrap_or_default()
}

fn next_echo_id() -> u16 {
    static COUNTER: OnceLock<AtomicU16> = OnceLock::new();
    let counter = COUNTER.get_or_init(|| {
        let seed = (std::process::id() as u16) ^ OffsetDateTime::now_utc().millisecond();
        AtomicU16::new(seed)
    });
    counter.fetch_add(1, Ordering::Relaxed)
}

fn internet_checksum(data: &[u8]) -> u16 {
    let mut sum = 0u32;
    let mut chunks = data.chunks_exact(2);
    for chunk in &mut chunks {
        let word = u16::from_be_bytes([chunk[0], chunk[1]]);
        sum = sum.wrapping_add(u32::from(word));
    }
    if let Some(&odd) = chunks.remainder().first() {
        sum = sum.wrapping_add(u32::from(u16::from_be_bytes([odd, 0])));
    }
    while (sum >> 16) != 0 {
        sum = (sum & 0xffff) + (sum >> 16);
    }
    !(sum as u16)
}

fn build_icmp_echo_packet(
    icmp_type: u8,
    identifier: u16,
    sequence: u16,
    with_checksum: bool,
) -> Vec<u8> {
    const PAYLOAD: &[u8] = b"openflare-probe-icmp-echo-request-payload-0123456789abcd";
    let mut packet = Vec::with_capacity(8 + PAYLOAD.len());
    packet.push(icmp_type);
    packet.push(0); // code
    packet.extend_from_slice(&[0, 0]); // checksum placeholder
    packet.extend_from_slice(&identifier.to_be_bytes());
    packet.extend_from_slice(&sequence.to_be_bytes());
    packet.extend_from_slice(PAYLOAD);
    if with_checksum {
        let csum = internet_checksum(&packet);
        packet[2..4].copy_from_slice(&csum.to_be_bytes());
    }
    packet
}

#[cfg(unix)]
struct SocketGuard(libc::c_int);

#[cfg(unix)]
impl Drop for SocketGuard {
    fn drop(&mut self) {
        unsafe { libc::close(self.0) };
    }
}

#[cfg(unix)]
fn unix_icmp_ping(ip: IpAddr, timeout: Duration) -> Result<i64, std::io::Error> {
    match ip {
        IpAddr::V4(v4) => unix_icmp_v4(v4, timeout),
        IpAddr::V6(v6) => unix_icmp_v6(v6, timeout),
    }
}

#[cfg(unix)]
fn open_icmp_socket(
    domain: libc::c_int,
    protocol: libc::c_int,
) -> Result<(SocketGuard, bool), std::io::Error> {
    // Prefer unprivileged SOCK_DGRAM ICMP first (works in rootless Linux containers when
    // ping_group_range permits it), then fall back to SOCK_RAW (root / CAP_NET_RAW).
    let dgram = unsafe { libc::socket(domain, libc::SOCK_DGRAM | libc::SOCK_CLOEXEC, protocol) };
    if dgram >= 0 {
        return Ok((SocketGuard(dgram), true));
    }
    #[cfg(target_os = "linux")]
    let _ = trippy_privilege::Privilege::acquire_privileges();
    let raw = unsafe { libc::socket(domain, libc::SOCK_RAW | libc::SOCK_CLOEXEC, protocol) };
    if raw >= 0 {
        return Ok((SocketGuard(raw), false));
    }
    Err(std::io::Error::last_os_error())
}

#[cfg(unix)]
fn set_recv_timeout(fd: libc::c_int, remaining: Duration) -> Result<(), std::io::Error> {
    let tv = libc::timeval {
        tv_sec: remaining.as_secs() as libc::time_t,
        tv_usec: remaining.subsec_micros() as libc::suseconds_t,
    };
    let rc = unsafe {
        libc::setsockopt(
            fd,
            libc::SOL_SOCKET,
            libc::SO_RCVTIMEO,
            (&raw const tv).cast::<libc::c_void>(),
            std::mem::size_of::<libc::timeval>() as libc::socklen_t,
        )
    };
    if rc < 0 {
        return Err(std::io::Error::last_os_error());
    }
    Ok(())
}

#[cfg(unix)]
fn unix_icmp_v4(target: Ipv4Addr, timeout: Duration) -> Result<i64, std::io::Error> {
    const ICMP_ECHO_REQUEST: u8 = 8;
    const ICMP_ECHO_REPLY: u8 = 0;

    let (sock, is_dgram) = open_icmp_socket(libc::AF_INET, libc::IPPROTO_ICMP)?;
    let id = next_echo_id();
    let seq = 1u16;
    let packet = build_icmp_echo_packet(ICMP_ECHO_REQUEST, id, seq, true);

    let mut addr: libc::sockaddr_in = unsafe { std::mem::zeroed() };
    addr.sin_family = libc::AF_INET as libc::sa_family_t;
    addr.sin_addr = libc::in_addr {
        s_addr: u32::from_ne_bytes(target.octets()),
    };

    let started = Instant::now();
    let sent = unsafe {
        libc::sendto(
            sock.0,
            packet.as_ptr().cast::<libc::c_void>(),
            packet.len(),
            0,
            (&raw const addr).cast::<libc::sockaddr>(),
            std::mem::size_of::<libc::sockaddr_in>() as libc::socklen_t,
        )
    };
    if sent < 0 {
        return Err(std::io::Error::last_os_error());
    }

    let mut buf = [0u8; 1500];
    loop {
        let Some(remaining) = timeout.checked_sub(started.elapsed()) else {
            return Ok(-1);
        };
        if remaining.is_zero() {
            return Ok(-1);
        }
        set_recv_timeout(sock.0, remaining)?;

        let nr = unsafe {
            libc::recvfrom(
                sock.0,
                buf.as_mut_ptr().cast::<libc::c_void>(),
                buf.len(),
                0,
                std::ptr::null_mut(),
                std::ptr::null_mut(),
            )
        };
        if nr < 0 {
            let err = std::io::Error::last_os_error();
            if err.kind() == std::io::ErrorKind::WouldBlock
                || err.kind() == std::io::ErrorKind::TimedOut
            {
                return Ok(-1);
            }
            return Err(err);
        }

        let data = &buf[..nr as usize];
        let icmp = if is_dgram {
            data
        } else {
            if data.is_empty() {
                continue;
            }
            let ihl = ((data[0] & 0x0f) as usize) * 4;
            if data.len() < ihl + 8 {
                continue;
            }
            &data[ihl..]
        };

        if icmp.len() < 8 || icmp[0] != ICMP_ECHO_REPLY {
            continue;
        }
        let reply_id = u16::from_be_bytes([icmp[4], icmp[5]]);
        let reply_seq = u16::from_be_bytes([icmp[6], icmp[7]]);
        // On SOCK_DGRAM the kernel assigns its own local ICMP port as identifier, so matching seq suffices.
        if (is_dgram && reply_seq == seq) || (!is_dgram && reply_id == id && reply_seq == seq) {
            return Ok(elapsed_ms(started));
        }
    }
}

#[cfg(unix)]
fn unix_icmp_v6(target: Ipv6Addr, timeout: Duration) -> Result<i64, std::io::Error> {
    const ICMPV6_ECHO_REQUEST: u8 = 128;
    const ICMPV6_ECHO_REPLY: u8 = 129;

    let (sock, is_dgram) = open_icmp_socket(libc::AF_INET6, libc::IPPROTO_ICMPV6)?;
    let id = next_echo_id();
    let seq = 1u16;
    // Kernel computes ICMPv6 pseudo-header checksum automatically.
    let packet = build_icmp_echo_packet(ICMPV6_ECHO_REQUEST, id, seq, false);

    let mut addr: libc::sockaddr_in6 = unsafe { std::mem::zeroed() };
    addr.sin6_family = libc::AF_INET6 as libc::sa_family_t;
    addr.sin6_addr = libc::in6_addr {
        s6_addr: target.octets(),
    };

    let started = Instant::now();
    let sent = unsafe {
        libc::sendto(
            sock.0,
            packet.as_ptr().cast::<libc::c_void>(),
            packet.len(),
            0,
            (&raw const addr).cast::<libc::sockaddr>(),
            std::mem::size_of::<libc::sockaddr_in6>() as libc::socklen_t,
        )
    };
    if sent < 0 {
        return Err(std::io::Error::last_os_error());
    }

    let mut buf = [0u8; 1500];
    loop {
        let Some(remaining) = timeout.checked_sub(started.elapsed()) else {
            return Ok(-1);
        };
        if remaining.is_zero() {
            return Ok(-1);
        }
        set_recv_timeout(sock.0, remaining)?;

        let nr = unsafe {
            libc::recvfrom(
                sock.0,
                buf.as_mut_ptr().cast::<libc::c_void>(),
                buf.len(),
                0,
                std::ptr::null_mut(),
                std::ptr::null_mut(),
            )
        };
        if nr < 0 {
            let err = std::io::Error::last_os_error();
            if err.kind() == std::io::ErrorKind::WouldBlock
                || err.kind() == std::io::ErrorKind::TimedOut
            {
                return Ok(-1);
            }
            return Err(err);
        }

        let icmp = &buf[..nr as usize];
        if icmp.len() < 8 || icmp[0] != ICMPV6_ECHO_REPLY {
            continue;
        }
        let reply_id = u16::from_be_bytes([icmp[4], icmp[5]]);
        let reply_seq = u16::from_be_bytes([icmp[6], icmp[7]]);
        if (is_dgram && reply_seq == seq) || (!is_dgram && reply_id == id && reply_seq == seq) {
            return Ok(elapsed_ms(started));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::net::TcpListener;

    #[test]
    fn split_host_port_handles_ipv4_ipv6_and_domains() {
        assert_eq!(split_host_port("1.1.1.1"), ("1.1.1.1".into(), 80));
        assert_eq!(split_host_port("1.1.1.1:443"), ("1.1.1.1".into(), 443));
        assert_eq!(
            split_host_port("example.com:8080"),
            ("example.com".into(), 8080)
        );
        assert_eq!(
            split_host_port("2606:4700:4700::1111"),
            ("2606:4700:4700::1111".into(), 80)
        );
        assert_eq!(
            split_host_port("[2606:4700:4700::1111]:443"),
            ("2606:4700:4700::1111".into(), 443)
        );
    }

    #[test]
    fn icmp_checksum_validates_to_zero() {
        let packet = build_icmp_echo_packet(8, 0x1234, 1, true);
        assert_eq!(internet_checksum(&packet), 0);
    }

    #[tokio::test]
    async fn tcp_ping_measures_local_listener() {
        let listener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let addr = listener.local_addr().expect("addr");
        let accept_task = tokio::spawn(async move {
            let _ = listener.accept().await;
        });

        let result = execute_ping(
            &PingRequest {
                ping_task_id: 42,
                ping_type: "tcp".into(),
                ping_target: addr.to_string(),
            },
            None,
            &PingOptions::default(),
        )
        .await;
        let _ = accept_task.await;

        assert_eq!(result.task_id, 42);
        assert_eq!(result.ping_type, "tcp");
        assert!(result.value >= 0);
        assert!(!result.finished_at.is_empty());
    }

    #[tokio::test]
    async fn tcp_ping_batch_reuses_execute_ping_and_computes_stats() {
        let listener = TcpListener::bind("127.0.0.1:0").await.expect("bind");
        let addr = listener.local_addr().expect("addr");
        let accept_task = tokio::spawn(async move {
            for _ in 0..3 {
                let _ = listener.accept().await;
            }
        });

        let result = execute_ping_batch(
            &PingBatchRequest {
                protocol: "tcp".into(),
                target: "127.0.0.1".into(),
                port: Some(addr.port()),
                count: 3,
                interval_ms: 50,
                timeout_ms: Some(1000),
            },
            None,
            &PingOptions::default(),
        )
        .await;
        let _ = accept_task.await;

        assert_eq!(result.protocol, "tcp");
        assert_eq!(result.sent, 3);
        assert_eq!(result.received, 3);
        assert_eq!(result.loss_rate, 0.0);
        assert_eq!(result.resolved_ip.as_deref(), Some("127.0.0.1"));
        assert!(result.error.is_none());
        assert_eq!(result.samples.len(), 3);
        assert!(result.samples.iter().all(|s| s.success && s.rtt_ms >= 0));
    }
}
