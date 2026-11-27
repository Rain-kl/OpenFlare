use crate::dns_probe::execute_dns_query;
use crate::ping::{execute_ping_batch, execute_ping_batch_with_policy, PingOptions};
use crate::safety::check_net_probe_budget;
use crate::traceroute::execute_traceroute;
use crate::types::{NetProbeRequest, NetProbeResult};
use time::format_description::well_known::Rfc3339;
use time::OffsetDateTime;

/// Unified dispatcher for all multi-node network probe operations (`ping_batch`, `traceroute`, `dns_query`).
pub async fn execute_net_probe(
    req: &NetProbeRequest,
    http_client: Option<&reqwest::Client>,
    ping_options: &PingOptions,
) -> NetProbeResult {
    execute_net_probe_with_policy(req, http_client, ping_options, true).await
}

pub(crate) async fn execute_net_probe_with_policy(
    req: &NetProbeRequest,
    http_client: Option<&reqwest::Client>,
    ping_options: &PingOptions,
    enforce_safety: bool,
) -> NetProbeResult {
    let kind = req.kind.trim().to_ascii_lowercase();
    match kind.as_str() {
        "ping_batch" | "ping" => match req.ping.as_ref() {
            Some(ping_req) => {
                if enforce_safety {
                    let pkt_cost = u32::from(ping_req.count.clamp(1, 10));
                    if let Err(err) = check_net_probe_budget(&ping_req.target, pkt_cost) {
                        return invalid_net_probe(req, &kind, &err);
                    }
                }
                let res = if enforce_safety {
                    execute_ping_batch(ping_req, http_client, ping_options).await
                } else {
                    execute_ping_batch_with_policy(ping_req, http_client, ping_options, false).await
                };
                let error = res.error.clone();
                NetProbeResult {
                    probe_id: req.probe_id.clone(),
                    kind,
                    ping: Some(res),
                    traceroute: None,
                    dns: None,
                    error,
                    finished_at: format_rfc3339_now(),
                }
            }
            None => invalid_net_probe(req, &kind, "missing ping payload"),
        },
        "traceroute" => match req.traceroute.as_ref() {
            Some(trace_req) => {
                if enforce_safety {
                    let hops = u32::from(trace_req.max_hops.clamp(1, 40));
                    let probes = u32::from(trace_req.probes_per_hop.clamp(1, 5));
                    if let Err(err) =
                        check_net_probe_budget(&trace_req.target, hops.saturating_mul(probes))
                    {
                        return invalid_net_probe(req, &kind, &err);
                    }
                }
                let res = execute_traceroute(trace_req).await;
                let error = res.error.clone();
                NetProbeResult {
                    probe_id: req.probe_id.clone(),
                    kind,
                    ping: None,
                    traceroute: Some(res),
                    dns: None,
                    error,
                    finished_at: format_rfc3339_now(),
                }
            }
            None => invalid_net_probe(req, &kind, "missing traceroute payload"),
        },
        "dns_query" | "dns" => match req.dns.as_ref() {
            Some(dns_req) => {
                if enforce_safety {
                    let budget_key =
                        format!("{}:{}", dns_req.resolver.trim(), dns_req.domain.trim());
                    if let Err(err) = check_net_probe_budget(&budget_key, 2) {
                        return invalid_net_probe(req, &kind, &err);
                    }
                }
                let res = execute_dns_query(dns_req).await;
                let error = res.error.clone();
                NetProbeResult {
                    probe_id: req.probe_id.clone(),
                    kind,
                    ping: None,
                    traceroute: None,
                    dns: Some(res),
                    error,
                    finished_at: format_rfc3339_now(),
                }
            }
            None => invalid_net_probe(req, &kind, "missing dns payload"),
        },
        other => invalid_net_probe(req, &kind, &format!("unsupported net probe kind: {other}")),
    }
}

fn invalid_net_probe(req: &NetProbeRequest, kind: &str, message: &str) -> NetProbeResult {
    NetProbeResult {
        probe_id: req.probe_id.clone(),
        kind: kind.to_owned(),
        ping: None,
        traceroute: None,
        dns: None,
        error: Some(message.to_owned()),
        finished_at: format_rfc3339_now(),
    }
}

pub(crate) fn format_rfc3339_now() -> String {
    OffsetDateTime::now_utc()
        .format(&Rfc3339)
        .unwrap_or_default()
}
