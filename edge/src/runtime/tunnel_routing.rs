use crate::config::persist_private_json;
use crate::protocol::config::{TunnelEdgeRelayEndpoint, TunnelEdgeRoutingEntry, TunnelRoutingSnapshot};
use anyhow::{Context, Result};
use std::{
    collections::HashMap,
    path::Path,
    sync::{Arc, RwLock},
};

const TUNNEL_ROUTING_FILE: &str = "tunnel-routing.json";

#[derive(Default)]
struct TunnelRoutingTable {
    sha256: String,
    by_tunnel: HashMap<u64, TunnelEdgeRoutingEntry>,
}

#[derive(Clone, Default)]
pub(crate) struct LiveTunnelRouting {
    active: Arc<RwLock<Arc<TunnelRoutingTable>>>,
}

impl LiveTunnelRouting {
    pub fn open(state_dir: &Path) -> Self {
        let path = state_dir.join(TUNNEL_ROUTING_FILE);
        let initial = match std::fs::read_to_string(&path) {
            Ok(raw) => match serde_json::from_str::<TunnelRoutingSnapshot>(&raw) {
                Ok(snapshot) => build_table(String::new(), snapshot),
                Err(error) => {
                    tracing::warn!(path = %path.display(), %error, "ignoring corrupted saved tunnel routing snapshot");
                    TunnelRoutingTable::default()
                }
            },
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                TunnelRoutingTable::default()
            }
            Err(error) => {
                tracing::warn!(path = %path.display(), %error, "could not load saved tunnel routing snapshot");
                TunnelRoutingTable::default()
            }
        };
        Self {
            active: Arc::new(RwLock::new(Arc::new(initial))),
        }
    }

    pub fn matches_sha256(&self, sha256: &str) -> bool {
        if sha256.is_empty() {
            return false;
        }
        let guard = self.active.read().expect("read active tunnel routing");
        guard.sha256 == sha256
    }

    pub fn apply(
        &self,
        state_dir: &Path,
        sha256: &str,
        snapshot: TunnelRoutingSnapshot,
    ) -> Result<bool> {
        if self.matches_sha256(sha256) {
            return Ok(false);
        }
        persist_private_json(&state_dir.join(TUNNEL_ROUTING_FILE), &snapshot)
            .context("persist tunnel-routing.v1 snapshot")?;
        let table = Arc::new(build_table(sha256.to_owned(), snapshot));
        let count = table.by_tunnel.len();
        *self
            .active
            .write()
            .expect("install live tunnel routing table") = table;
        tracing::info!(tunnels = count, "updated Pingora dynamic tunnel routing table");
        Ok(true)
    }

    pub fn endpoints_for_tunnel(&self, tunnel_id: u64) -> Vec<TunnelEdgeRelayEndpoint> {
        let guard = self.active.read().expect("read active tunnel routing");
        let Some(entry) = guard.by_tunnel.get(&tunnel_id) else {
            return Vec::new();
        };
        if !entry.enabled || entry.quarantined {
            return Vec::new();
        }
        entry.relays.clone()
    }
}

fn build_table(sha256: String, snapshot: TunnelRoutingSnapshot) -> TunnelRoutingTable {
    let mut by_tunnel = HashMap::with_capacity(snapshot.tunnels.len());
    for entry in snapshot.tunnels {
        by_tunnel.insert(entry.tunnel_id, entry);
    }
    TunnelRoutingTable { sha256, by_tunnel }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample_snapshot() -> TunnelRoutingSnapshot {
        TunnelRoutingSnapshot {
            schema_version: 1,
            generated_at_unix: 100,
            tunnels: vec![
                TunnelEdgeRoutingEntry {
                    tunnel_id: 7,
                    tunnel_public_id: "tun-abc".into(),
                    enabled: true,
                    quarantined: false,
                    preferred_transport: "noise".into(),
                    relays: vec![
                        TunnelEdgeRelayEndpoint {
                            relay_id: 1,
                            relay_public_id: "relay-1".into(),
                            gateway_url: "http://10.0.1.1:8443".into(),
                            gateway_secret: "sec-1".into(),
                            zone_tag: "hk-a".into(),
                            colocated_edge_node_id: "edge-1".into(),
                            is_colocated: true,
                            weight: 100,
                            best_rtt_ms: 20,
                            active_connectors: 2,
                            active_streams: 0,
                        },
                        TunnelEdgeRelayEndpoint {
                            relay_id: 2,
                            relay_public_id: "relay-2".into(),
                            gateway_url: "http://10.0.1.2:8443".into(),
                            gateway_secret: "sec-2".into(),
                            zone_tag: "hk-b".into(),
                            colocated_edge_node_id: String::new(),
                            is_colocated: false,
                            weight: 100,
                            best_rtt_ms: 40,
                            active_connectors: 1,
                            active_streams: 0,
                        },
                    ],
                },
                TunnelEdgeRoutingEntry {
                    tunnel_id: 8,
                    tunnel_public_id: "tun-disabled".into(),
                    enabled: false,
                    quarantined: false,
                    preferred_transport: "auto".into(),
                    relays: vec![TunnelEdgeRelayEndpoint {
                        relay_id: 1,
                        relay_public_id: "relay-1".into(),
                        gateway_url: "http://10.0.1.1:8443".into(),
                        gateway_secret: "sec-1".into(),
                        ..TunnelEdgeRelayEndpoint::default()
                    }],
                },
                TunnelEdgeRoutingEntry {
                    tunnel_id: 9,
                    tunnel_public_id: "tun-quar".into(),
                    enabled: true,
                    quarantined: true,
                    preferred_transport: "auto".into(),
                    relays: vec![TunnelEdgeRelayEndpoint::default()],
                },
            ],
        }
    }

    #[test]
    fn applies_persists_dedups_and_filters_unavailable_tunnels() {
        let dir = tempfile::tempdir().unwrap();
        let routing = LiveTunnelRouting::open(dir.path());
        assert!(routing.endpoints_for_tunnel(7).is_empty());

        let changed = routing
            .apply(dir.path(), "sha-1", sample_snapshot())
            .unwrap();
        assert!(changed);
        assert!(!routing
            .apply(dir.path(), "sha-1", sample_snapshot())
            .unwrap());

        let eps = routing.endpoints_for_tunnel(7);
        assert_eq!(eps.len(), 2);
        assert_eq!(eps[0].relay_public_id, "relay-1");
        assert_eq!(eps[1].relay_public_id, "relay-2");

        // Disabled or quarantined tunnels must not yield endpoints.
        assert!(routing.endpoints_for_tunnel(8).is_empty());
        assert!(routing.endpoints_for_tunnel(9).is_empty());

        // Cold restart loads saved snapshot from state_dir.
        let reloaded = LiveTunnelRouting::open(dir.path());
        assert_eq!(reloaded.endpoints_for_tunnel(7).len(), 2);
        assert!(!reloaded.matches_sha256("sha-1"));
    }
}
