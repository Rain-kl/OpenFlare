use crate::dns_probe::execute_dns_query;
use crate::ping::{execute_ping_batch, PingOptions};
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
    let kind = req.kind.trim().to_ascii_lowercase();
    match kind.as_str() {
        "ping_batch" | "ping" => match req.ping.as_ref() {
            Some(ping_req) => {
                let res = execute_ping_batch(ping_req, http_client, ping_options).await;
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
