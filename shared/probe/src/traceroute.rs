use crate::ping::{resolve_ip, split_host_port};
use crate::types::{TracerouteHop, TracerouteProbe, TracerouteRequest, TracerouteResult};
use std::sync::atomic::{AtomicU16, Ordering};
use std::sync::OnceLock;
use std::time::Duration;
use trippy_core::{
    Builder as TrippyBuilder, Port, PortDirection, PrivilegeMode, Protocol as TrippyProtocol, State,
};
use trippy_privilege::Privilege;

const MIN_MAX_HOPS: u8 = 1;
const MAX_ALLOWED_HOPS: u8 = 40;
const MIN_PROBES_PER_HOP: u8 = 1;
const MAX_PROBES_PER_HOP: u8 = 5;
const DEFAULT_TRACE_TIMEOUT_MS: u64 = 25_000;
const MIN_TRACE_TIMEOUT_MS: u64 = 3_000;
const MAX_TRACE_TIMEOUT_MS: u64 = 40_000;

fn next_trace_id() -> u16 {
    static TRACE_COUNTER: OnceLock<AtomicU16> = OnceLock::new();
    let counter = TRACE_COUNTER.get_or_init(|| AtomicU16::new(std::process::id() as u16));
    counter.fetch_add(1, Ordering::Relaxed)
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ParsedTraceProtocol {
    Icmp,
    Udp,
    Tcp,
}

impl ParsedTraceProtocol {
    fn parse(raw: &str) -> Option<Self> {
        match raw.trim().to_ascii_lowercase().as_str() {
            "" | "icmp" => Some(Self::Icmp),
            "udp" => Some(Self::Udp),
            "tcp" => Some(Self::Tcp),
            _ => None,
        }
    }

    fn as_str(self) -> &'static str {
        match self {
            Self::Icmp => "icmp",
            Self::Udp => "udp",
            Self::Tcp => "tcp",
        }
    }
}

/// Executes a multi-round traceroute probe backed by `trippy-core`.
/// Never panics and always returns a structured [`TracerouteResult`].
pub async fn execute_traceroute(req: &TracerouteRequest) -> TracerouteResult {
    let trimmed = req.target.trim();
    let Some(protocol) = ParsedTraceProtocol::parse(&req.protocol) else {
        return error_result(
            trimmed,
            None,
            &req.protocol,
            format!("unsupported traceroute protocol: {}", req.protocol),
        );
    };

    let (host, parsed_port) = split_host_port(trimmed);
    if host.is_empty() {
        return error_result(
            trimmed,
            None,
            protocol.as_str(),
            "traceroute target cannot be empty".to_owned(),
        );
    }

    let target_ip = match resolve_ip(&host).await {
        Ok(ip) => ip,
        Err(err) => {
            return error_result(
                trimmed,
                None,
                protocol.as_str(),
                format!("DNS resolution failed for {host}: {err}"),
            );
        }
    };
    let resolved_ip = Some(target_ip.to_string());

    let privilege_mode = match discover_privilege_mode() {
        Ok(mode) => mode,
        Err(msg) => return error_result(trimmed, resolved_ip, protocol.as_str(), msg),
    };

    let max_hops = req.max_hops.clamp(MIN_MAX_HOPS, MAX_ALLOWED_HOPS);
    let probes_per_hop = req
        .probes_per_hop
        .clamp(MIN_PROBES_PER_HOP, MAX_PROBES_PER_HOP);
    let timeout_ms = req
        .timeout_ms
        .unwrap_or(DEFAULT_TRACE_TIMEOUT_MS)
        .clamp(MIN_TRACE_TIMEOUT_MS, MAX_TRACE_TIMEOUT_MS);
    let trace_id = next_trace_id();
    let dest_port = req.port.unwrap_or(parsed_port);

    let mut builder = TrippyBuilder::new(target_ip)
        .privilege_mode(privilege_mode)
        .trace_identifier(trace_id)
        .first_ttl(1)
        .max_ttl(max_hops)
        .max_rounds(Some(usize::from(probes_per_hop)))
        .min_round_duration(Duration::from_millis(250))
        .max_round_duration(Duration::from_millis(1000))
        .grace_duration(Duration::from_millis(100))
        .read_timeout(Duration::from_millis(10))
        .tcp_connect_timeout(Duration::from_millis(1000))
        .drop_privileges(false);

    builder = match protocol {
        ParsedTraceProtocol::Icmp => builder
            .protocol(TrippyProtocol::Icmp)
            .port_direction(PortDirection::None),
        ParsedTraceProtocol::Udp => builder
            .protocol(TrippyProtocol::Udp)
            .port_direction(PortDirection::FixedSrc(Port(33434))),
        ParsedTraceProtocol::Tcp => builder
            .protocol(TrippyProtocol::Tcp)
            .port_direction(PortDirection::FixedDest(Port(dest_port))),
    };

    let tracer = match builder.build() {
        Ok(tracer) => tracer,
        Err(err) => {
            return error_result(
                trimmed,
                resolved_ip,
                protocol.as_str(),
                format!("failed to initialize trippy tracer: {err}"),
            );
        }
    };

    let trace_future = tokio::task::spawn_blocking(move || {
        #[cfg(target_os = "linux")]
        let _ = Privilege::acquire_privileges();
        tracer.run()?;
        Ok::<State, trippy_core::Error>(tracer.snapshot())
    });

    match tokio::time::timeout(Duration::from_millis(timeout_ms), trace_future).await {
        Ok(Ok(Ok(state))) => state_to_result(trimmed, target_ip, protocol.as_str(), &state),
        Ok(Ok(Err(err))) => error_result(
            trimmed,
            resolved_ip,
            protocol.as_str(),
            format!("traceroute failed: {err}"),
        ),
        Ok(Err(join_err)) => error_result(
            trimmed,
            resolved_ip,
            protocol.as_str(),
            format!("traceroute worker aborted: {join_err}"),
        ),
        Err(_) => error_result(
            trimmed,
            resolved_ip,
            protocol.as_str(),
            format!("traceroute timed out after {timeout_ms}ms"),
        ),
    }
}

fn discover_privilege_mode() -> Result<PrivilegeMode, String> {
    match Privilege::acquire_privileges() {
        Ok(privilege) => {
            if privilege.has_privileges() {
                Ok(PrivilegeMode::Privileged)
            } else if !privilege.needs_privileges() {
                Ok(PrivilegeMode::Unprivileged)
            } else {
                Err("traceroute requires CAP_NET_RAW on Linux (ensure openflare-edge unit includes CAP_NET_RAW or Docker runs with --cap-add=NET_RAW)".to_owned())
            }
        }
        Err(err) => Err(format!("failed to inspect traceroute privileges: {err}")),
    }
}

fn state_to_result(
    target: &str,
    target_ip: std::net::IpAddr,
    protocol: &str,
    state: &State,
) -> TracerouteResult {
    let raw_hops = state.hops();
    let mut hops = Vec::with_capacity(raw_hops.len());
    let mut reached = false;
    let mut total_rtt_ms = 0.0;

    for hop in raw_hops {
        let mut addrs_iter = hop.addrs();
        let primary_ip = addrs_iter.next().map(|ip| ip.to_string());
        let alt_ips: Vec<String> = addrs_iter.map(|ip| ip.to_string()).collect();
        let hop_reaches_target = hop.addrs().any(|ip| *ip == target_ip);
        if hop_reaches_target {
            reached = true;
            if hop.total_recv() > 0 {
                total_rtt_ms = hop.avg_ms();
            } else if let Some(best) = hop.best_ms() {
                total_rtt_ms = best;
            }
        }

        let probes = hop
            .samples()
            .iter()
            .enumerate()
            .map(|(idx, duration)| TracerouteProbe {
                seq: u8::try_from(idx + 1).unwrap_or(u8::MAX),
                ip: primary_ip.clone(),
                rtt_ms: Some(duration.as_secs_f64() * 1000.0),
            })
            .collect();

        hops.push(TracerouteHop {
            ttl: hop.ttl(),
            ip: primary_ip,
            alt_ips,
            sent: u16::try_from(hop.total_sent()).unwrap_or(u16::MAX),
            recv: u16::try_from(hop.total_recv()).unwrap_or(u16::MAX),
            loss_pct: hop.loss_pct(),
            min_ms: hop.best_ms(),
            avg_ms: (hop.total_recv() > 0).then(|| hop.avg_ms()),
            max_ms: hop.worst_ms(),
            jitter_ms: hop.jitter_ms(),
            probes,
        });
    }

    if total_rtt_ms <= 0.0 {
        if let Some(last_with_rtt) = hops.iter().rev().find_map(|hop| hop.avg_ms) {
            total_rtt_ms = last_with_rtt;
        }
    }

    let hop_count = u8::try_from(hops.len()).unwrap_or(u8::MAX);
    TracerouteResult {
        target: target.to_owned(),
        resolved_ip: Some(target_ip.to_string()),
        protocol: protocol.to_owned(),
        hop_count,
        reached,
        total_rtt_ms,
        hops,
        error: state.error().map(str::to_owned),
    }
}

fn error_result(
    target: &str,
    resolved_ip: Option<String>,
    protocol: &str,
    message: String,
) -> TracerouteResult {
    TracerouteResult {
        target: target.to_owned(),
        resolved_ip,
        protocol: protocol.to_owned(),
        hop_count: 0,
        reached: false,
        total_rtt_ms: 0.0,
        hops: Vec::new(),
        error: Some(message),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::{IpAddr, Ipv4Addr};

    #[tokio::test]
    async fn traceroute_rejects_empty_target_and_bad_protocol() {
        let bad_proto = execute_traceroute(&TracerouteRequest {
            target: "1.1.1.1".into(),
            protocol: "sctp".into(),
            port: None,
            max_hops: 10,
            probes_per_hop: 1,
            timeout_ms: None,
        })
        .await;
        assert!(bad_proto
            .error
            .as_deref()
            .is_some_and(|e| e.contains("unsupported")));

        let empty = execute_traceroute(&TracerouteRequest {
            target: "   ".into(),
            protocol: "icmp".into(),
            port: None,
            max_hops: 10,
            probes_per_hop: 1,
            timeout_ms: None,
        })
        .await;
        assert!(empty.error.as_deref().is_some_and(|e| e.contains("empty")));
    }

    #[test]
    fn trippy_builder_builds_all_protocols_and_converts_empty_state() {
        let target = IpAddr::V4(Ipv4Addr::new(1, 1, 1, 1));
        for proto in [
            TrippyProtocol::Icmp,
            TrippyProtocol::Udp,
            TrippyProtocol::Tcp,
        ] {
            let port_dir = match proto {
                TrippyProtocol::Icmp => PortDirection::None,
                TrippyProtocol::Udp => PortDirection::FixedSrc(Port(33434)),
                TrippyProtocol::Tcp => PortDirection::FixedDest(Port(443)),
            };
            let tracer = TrippyBuilder::new(target)
                .protocol(proto)
                .port_direction(port_dir)
                .privilege_mode(PrivilegeMode::Unprivileged)
                .max_ttl(20)
                .max_rounds(Some(2))
                .build();
            let _ = tracer;
        }

        let tracer = TrippyBuilder::new(target)
            .protocol(TrippyProtocol::Tcp)
            .port_direction(PortDirection::FixedDest(Port(443)))
            .privilege_mode(PrivilegeMode::Unprivileged)
            .max_ttl(20)
            .max_rounds(Some(2))
            .build()
            .expect("build trippy tracer");
        let snapshot = tracer.snapshot();
        let converted = state_to_result("1.1.1.1", target, "tcp", &snapshot);
        assert_eq!(converted.resolved_ip.as_deref(), Some("1.1.1.1"));
        assert!(!converted.reached);
        assert!(converted.hops.is_empty());
    }
}
