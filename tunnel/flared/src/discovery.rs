use openflare_tunnel_common::protocol::{TunnelDesiredAttachment, TunnelRelayCandidate};
use std::collections::{HashMap, HashSet};
use std::time::{Duration, Instant};
use tokio::net::TcpStream;

const PROBE_TIMEOUT: Duration = Duration::from_millis(2500);

#[derive(Debug, Clone)]
pub struct ProbedRelay {
    pub relay: TunnelRelayCandidate,
    pub healthy: bool,
    pub rtt_ms: Option<i64>,
    pub picked_transport: Option<String>,
}

pub async fn probe_candidates(
    client: &reqwest::Client,
    candidates: &[TunnelRelayCandidate],
    preferred_transport: &str,
    fallback_transports: &[String],
    client_proxy_url: Option<&str>,
) -> Vec<ProbedRelay> {
    let mut set = tokio::task::JoinSet::new();
    let ordered_prefs = resolve_transport_order(preferred_transport, fallback_transports);
    let has_proxy = client_proxy_url.is_some();

    for candidate in candidates.iter().cloned() {
        let client = client.clone();
        let prefs = ordered_prefs.clone();
        set.spawn(async move {
            if candidate.draining {
                return ProbedRelay {
                    relay: candidate,
                    healthy: false,
                    rtt_ms: None,
                    picked_transport: None,
                };
            }
            let rtt_ms = probe_gateway_health(&client, &candidate).await;
            let picked_transport = if rtt_ms.is_some() {
                pick_reachable_transport(&candidate, &prefs, has_proxy).await
            } else {
                None
            };
            let healthy = rtt_ms.is_some() && picked_transport.is_some();
            ProbedRelay {
                relay: candidate,
                healthy,
                rtt_ms,
                picked_transport,
            }
        });
    }

    let mut out = Vec::with_capacity(candidates.len());
    while let Some(res) = set.join_next().await {
        if let Ok(item) = res {
            out.push(item);
        }
    }
    out
}

pub fn resolve_transport_order(preferred: &str, fallbacks: &[String]) -> Vec<String> {
    let mut order = Vec::new();
    let pref = preferred.trim().to_ascii_lowercase();
    if !pref.is_empty() && pref != "auto" {
        order.push(pref);
    }
    let defaults = ["noise", "websocket", "tcp"];
    let fb_source: Vec<String> = if fallbacks.is_empty() {
        defaults.iter().map(|s| (*s).to_owned()).collect()
    } else {
        fallbacks.iter().map(|s| s.trim().to_ascii_lowercase()).collect()
    };
    for item in fb_source {
        if !item.is_empty() && !order.contains(&item) {
            order.push(item);
        }
    }
    if order.is_empty() {
        order.push("noise".to_owned());
    }
    order
}

async fn probe_gateway_health(client: &reqwest::Client, relay: &TunnelRelayCandidate) -> Option<i64> {
    if relay.gateway_port == 0 {
        return None;
    }
    let host = if !relay.gateway_addr.trim().is_empty() {
        relay.gateway_addr.trim()
    } else {
        relay.public_addr.trim()
    };
    if host.is_empty() {
        return None;
    }
    let scheme = if relay.gateway_tls { "https" } else { "http" };
    let path = if relay.health_path.starts_with('/') {
        relay.health_path.as_str()
    } else {
        "/_openflare/health"
    };
    let formatted_host = if host.contains(':') && !host.starts_with('[') {
        format!("[{host}]")
    } else {
        host.to_owned()
    };
    let url = format!("{scheme}://{formatted_host}:{}{path}", relay.gateway_port);
    let started = Instant::now();
    let resp = tokio::time::timeout(PROBE_TIMEOUT, client.get(url).send())
        .await
        .ok()?
        .ok()?;
    if !resp.status().is_success() {
        return None;
    }
    Some(started.elapsed().as_millis().max(1) as i64)
}

async fn pick_reachable_transport(
    relay: &TunnelRelayCandidate,
    prefs: &[String],
    has_proxy: bool,
) -> Option<String> {
    for transport in prefs {
        if !relay.supported_transports.is_empty()
            && !relay
                .supported_transports
                .iter()
                .any(|t| t.eq_ignore_ascii_case(transport))
        {
            continue;
        }
        let port = match transport.as_str() {
            "websocket" => relay.ws_port,
            "noise" | "tcp" => relay.transport_port,
            _ => continue,
        };
        if port == 0 {
            continue;
        }
        if has_proxy {
            return Some(transport.clone());
        }
        let host = relay.public_addr.trim();
        if host.is_empty() {
            continue;
        }
        if let Ok(Ok(_)) =
            tokio::time::timeout(PROBE_TIMEOUT, TcpStream::connect((host, port))).await
        {
            return Some(transport.clone());
        }
    }
    None
}

/// Ranks healthy, non-draining probed relays by RTT -> weight -> id, selecting up to `k`
/// relays while maximizing failure-domain (`zone_tag`) diversity first.
pub fn select_closest_k(probed: &[ProbedRelay], k: usize) -> Vec<TunnelDesiredAttachment> {
    if k == 0 {
        return Vec::new();
    }
    let mut eligible: Vec<&ProbedRelay> = probed
        .iter()
        .filter(|p| p.healthy && !p.relay.draining && p.picked_transport.is_some())
        .collect();
    eligible.sort_by(|a, b| {
        a.rtt_ms
            .unwrap_or(i64::MAX)
            .cmp(&b.rtt_ms.unwrap_or(i64::MAX))
            .then_with(|| b.relay.weight.cmp(&a.relay.weight))
            .then_with(|| a.relay.id.cmp(&b.relay.id))
    });

    let mut chosen: Vec<&ProbedRelay> = Vec::with_capacity(k);
    let mut used_ids = HashSet::new();
    let mut used_zones = HashSet::new();

    for item in &eligible {
        if chosen.len() >= k {
            break;
        }
        let zone = item.relay.zone_tag.trim();
        if !zone.is_empty() && used_zones.insert(zone.to_owned()) {
            used_ids.insert(item.relay.id);
            chosen.push(*item);
        }
    }
    for item in &eligible {
        if chosen.len() >= k {
            break;
        }
        if used_ids.insert(item.relay.id) {
            chosen.push(*item);
        }
    }

    chosen
        .into_iter()
        .map(|item| TunnelDesiredAttachment {
            relay_id: item.relay.id,
            transport: item.picked_transport.clone().unwrap_or_else(|| "noise".into()),
            rtt_ms: item.rtt_ms.unwrap_or(1),
        })
        .collect()
}

/// Applies Make-Before-Break anti-flapping hysteresis when switching attached relays:
/// retains healthy active relays unless a candidate beats the slowest active relay by
/// >= 25% AND >= 15ms across 2 consecutive probe rounds.
#[derive(Debug, Default)]
pub struct RebalanceTracker {
    consecutive_wins: HashMap<u64, u8>,
}

impl RebalanceTracker {
    pub fn decide(
        &mut self,
        probed: &[ProbedRelay],
        current_attached: &HashMap<u64, String>,
        closest_k: &[TunnelDesiredAttachment],
        k: usize,
    ) -> Vec<TunnelDesiredAttachment> {
        if k == 0 {
            return Vec::new();
        }
        if current_attached.is_empty() {
            self.consecutive_wins.clear();
            return closest_k.to_vec();
        }

        let probe_map: HashMap<u64, &ProbedRelay> =
            probed.iter().map(|p| (p.relay.id, p)).collect();

        // Keep currently attached relays that remain healthy, non-draining, and transport-compatible.
        let mut retained: Vec<TunnelDesiredAttachment> = current_attached
            .iter()
            .filter_map(|(&relay_id, _)| {
                let p = probe_map.get(&relay_id)?;
                if !p.healthy || p.relay.draining {
                    return None;
                }
                Some(TunnelDesiredAttachment {
                    relay_id,
                    transport: p.picked_transport.clone()?,
                    rtt_ms: p.rtt_ms.unwrap_or(1),
                })
            })
            .collect();

        // Fill vacant slots immediately if capacity grew or an attached relay became unhealthy.
        for cand in closest_k {
            if retained.len() >= k {
                break;
            }
            if !retained.iter().any(|r| r.relay_id == cand.relay_id) {
                retained.push(cand.clone());
            }
        }

        if retained.len() < k {
            return retained;
        }

        // Check hysteresis replacement against the slowest currently retained relay.
        retained.sort_by_key(|r| r.rtt_ms);
        let Some(worst) = retained.last().cloned() else {
            return retained;
        };
        let challenger = closest_k
            .iter()
            .find(|c| !retained.iter().any(|r| r.relay_id == c.relay_id))
            .cloned();

        let Some(best_new) = challenger else {
            self.consecutive_wins.clear();
            return retained;
        };

        let beats_threshold = best_new.rtt_ms > 0
            && worst.rtt_ms > best_new.rtt_ms
            && (worst.rtt_ms - best_new.rtt_ms) >= 15
            && (best_new.rtt_ms * 4 <= worst.rtt_ms * 3);

        if beats_threshold {
            let count = self.consecutive_wins.entry(best_new.relay_id).or_insert(0);
            *count = count.saturating_add(1);
            let current_wins = *count;
            self.consecutive_wins.retain(|id, _| *id == best_new.relay_id);
            if current_wins >= 2 {
                self.consecutive_wins.clear();
                retained.pop();
                retained.push(best_new);
                return retained;
            }
        } else {
            self.consecutive_wins.clear();
        }

        retained
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn mk_probe(id: u64, zone: &str, rtt: i64, draining: bool) -> ProbedRelay {
        ProbedRelay {
            relay: TunnelRelayCandidate {
                id,
                relay_id: format!("relay-{id}"),
                pool_id: 1,
                name: format!("r{id}"),
                public_addr: "127.0.0.1".into(),
                transport_port: 7443,
                ws_port: 7444,
                gateway_addr: "127.0.0.1".into(),
                gateway_port: 8443,
                gateway_tls: false,
                health_path: "/_openflare/health".into(),
                supported_transports: vec!["noise".into()],
                noise_public_key: "pub".into(),
                geo_name: String::new(),
                geo_latitude: None,
                geo_longitude: None,
                zone_tag: zone.into(),
                weight: 100,
                draining,
            },
            healthy: !draining,
            rtt_ms: (!draining).then_some(rtt),
            picked_transport: (!draining).then(|| "noise".into()),
        }
    }

    #[test]
    fn selects_closest_k_across_zone_failure_domains_excluding_draining() {
        let probed = vec![
            mk_probe(1, "az-a", 10, false),
            mk_probe(2, "az-a", 12, false),
            mk_probe(3, "az-b", 25, false),
            mk_probe(4, "az-c", 5, true), // draining must be excluded
        ];
        let picked = select_closest_k(&probed, 2);
        let ids: Vec<u64> = picked.iter().map(|p| p.relay_id).collect();
        assert_eq!(ids, vec![1, 3]);
    }

    #[test]
    fn rebalance_hysteresis_requires_two_consecutive_wins() {
        let mut tracker = RebalanceTracker::default();
        let current = HashMap::from([(1u64, "noise".to_owned()), (2u64, "noise".to_owned())]);
        let probed = vec![
            mk_probe(1, "az-a", 40, false),
            mk_probe(2, "az-b", 100, false),
            mk_probe(3, "az-c", 20, false), // beats relay 2 (100ms) by 80ms & 80%
        ];
        let closest = select_closest_k(&probed, 2);
        let round1 = tracker.decide(&probed, &current, &closest, 2);
        assert!(round1.iter().any(|r| r.relay_id == 2));

        let round2 = tracker.decide(&probed, &current, &closest, 2);
        assert!(!round2.iter().any(|r| r.relay_id == 2));
        assert!(round2.iter().any(|r| r.relay_id == 3));
    }
}
