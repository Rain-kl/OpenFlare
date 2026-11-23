use anyhow::Result;
use openflare_tunnel_common::hardening::write_file_in_place_0600;
use openflare_tunnel_common::protocol::RelayRuntimeConfigResponse;
use serde::Serialize;
use std::collections::{BTreeMap, HashMap};
use std::net::{SocketAddr, TcpListener};
use std::path::PathBuf;
use std::sync::Arc;
use tokio::sync::{broadcast, Mutex};

use crate::gateway::{ConnectorEntry, RelayGatewayState};

pub struct RatholeRelayManager {
    dir: PathBuf,
    gateway_state: Arc<RelayGatewayState>,
    state: Mutex<ManagerState>,
}

struct ManagerState {
    port_allocations: HashMap<(String, String), SocketAddr>,
    instances: HashMap<String, RunningServer>,
}

struct RunningServer {
    global_signature: String,
    toml_path: PathBuf,
    shutdown_tx: broadcast::Sender<bool>,
    handle: tokio::task::JoinHandle<()>,
}

#[derive(Serialize)]
struct RatholeServerDoc {
    server: RatholeServerConfig,
}

#[derive(Serialize)]
struct RatholeServerConfig {
    bind_addr: String,
    transport: RatholeTransport,
    services: BTreeMap<String, RatholeServerService>,
}

#[derive(Serialize)]
struct RatholeTransport {
    #[serde(rename = "type")]
    transport_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    noise: Option<RatholeNoise>,
    #[serde(skip_serializing_if = "Option::is_none")]
    websocket: Option<RatholeWebsocket>,
}

#[derive(Serialize)]
struct RatholeNoise {
    pattern: &'static str,
    local_private_key: String,
}

#[derive(Serialize)]
struct RatholeWebsocket {
    tls: bool,
}

#[derive(Serialize)]
struct RatholeServerService {
    #[serde(rename = "type")]
    service_type: String,
    bind_addr: String,
    token: String,
    nodelay: bool,
}

impl RatholeRelayManager {
    pub fn new(dir: PathBuf, gateway_state: Arc<RelayGatewayState>) -> Self {
        Self {
            dir,
            gateway_state,
            state: Mutex::new(ManagerState {
                port_allocations: HashMap::new(),
                instances: HashMap::new(),
            }),
        }
    }

    pub async fn apply_config(&self, cfg: &RelayRuntimeConfigResponse) -> Result<()> {
        let mut guard = self.state.lock().await;
        let transports = resolve_active_server_transports(cfg);
        let mut connector_endpoints: HashMap<u64, Vec<Arc<ConnectorEntry>>> = HashMap::new();

        // Remove stale allocations for services no longer present.
        guard.port_allocations.retain(|(svc, _), _| {
            cfg.services.iter().any(|s| &s.service_name == svc)
        });

        let mut per_transport_services: HashMap<String, BTreeMap<String, RatholeServerService>> =
            HashMap::new();
        for (kind, _) in &transports {
            per_transport_services.insert(kind.clone(), BTreeMap::new());
        }

        for svc in &cfg.services {
            if svc.state == "stale" {
                continue;
            }
            let is_l4 = matches!(svc.service_type.as_str(), "tcp" | "udp") && svc.listen_port > 0;
            let mut candidate_addrs = Vec::new();
            for (kind, _) in &transports {
                let bind_addr = if is_l4 {
                    format!("0.0.0.0:{}", svc.listen_port)
                } else {
                    let key = (svc.service_name.clone(), kind.clone());
                    let addr = match guard.port_allocations.get(&key).copied() {
                        Some(existing) => existing,
                        None => {
                            let allocated = allocate_free_loopback_port()?;
                            guard.port_allocations.insert(key, allocated);
                            allocated
                        }
                    };
                    candidate_addrs.push(addr);
                    addr.to_string()
                };

                let service_type = if svc.service_type == "udp" {
                    "udp".to_owned()
                } else {
                    "tcp".to_owned()
                };

                per_transport_services
                    .get_mut(kind)
                    .expect("transport present")
                    .insert(
                        svc.service_name.clone(),
                        RatholeServerService {
                            service_type,
                            bind_addr,
                            token: svc.token.clone(),
                            nodelay: !is_l4 || svc.service_type == "tcp",
                        },
                    );
            }

            if !is_l4 {
                connector_endpoints
                    .entry(svc.tunnel_id)
                    .or_default()
                    .push(Arc::new(ConnectorEntry::new(
                        svc.connector_id.clone(),
                        svc.service_name.clone(),
                        100,
                        candidate_addrs,
                    )));
            }
        }

        self.gateway_state
            .replace_connectors(connector_endpoints)
            .await;

        // Shut down transports that are no longer active.
        let active_keys: std::collections::HashSet<String> =
            transports.iter().map(|(k, _)| k.clone()).collect();
        let removed_keys: Vec<String> = guard
            .instances
            .keys()
            .filter(|k| !active_keys.contains(*k))
            .cloned()
            .collect();
        for key in removed_keys {
            if let Some(old) = guard.instances.remove(&key) {
                let _ = old.shutdown_tx.send(true);
                old.handle.abort();
            }
        }

        for (kind, port) in transports {
            let doc = build_server_doc(
                &kind,
                port,
                &cfg.noise_private_key,
                per_transport_services.remove(&kind).unwrap_or_default(),
            );
            let global_sig = format!("{kind}:{port}:{}", cfg.noise_private_key);
            let toml_text = toml::to_string_pretty(&doc)?;

            let needs_restart = match guard.instances.get(&kind) {
                Some(running) => running.global_signature != global_sig || running.handle.is_finished(),
                None => true,
            };

            if needs_restart {
                if let Some(old) = guard.instances.remove(&kind) {
                    let _ = old.shutdown_tx.send(true);
                    let _ = tokio::time::timeout(std::time::Duration::from_secs(2), old.handle).await;
                }
                let path = self.dir.join(format!("relay-{kind}.toml"));
                write_file_in_place_0600(&path, &toml_text)?;
                let running = spawn_rathole_server(path, global_sig);
                guard.instances.insert(kind, running);
            } else if let Some(running) = guard.instances.get(&kind) {
                write_file_in_place_0600(&running.toml_path, &toml_text)?;
            }
        }

        Ok(())
    }

    pub async fn shutdown(&self) {
        let mut guard = self.state.lock().await;
        for (_, old) in guard.instances.drain() {
            let _ = old.shutdown_tx.send(true);
            old.handle.abort();
        }
    }
}

fn spawn_rathole_server(toml_path: PathBuf, global_signature: String) -> RunningServer {
    let (shutdown_tx, shutdown_rx) = broadcast::channel(1);
    let cli_path = toml_path.clone();
    let cli = rathole::Cli {
        config_path: Some(cli_path.clone()),
        server: true,
        client: false,
        genkey: None,
    };
    let handle = tokio::spawn(async move {
        if let Err(err) = rathole::run(cli, shutdown_rx).await {
            tracing::error!(%err, path = %cli_path.display(), "rathole relay task exited");
        }
    });
    RunningServer {
        global_signature,
        toml_path,
        shutdown_tx,
        handle,
    }
}

fn resolve_active_server_transports(cfg: &RelayRuntimeConfigResponse) -> Vec<(String, u16)> {
    let mut out = Vec::new();
    let supports = |name: &str| cfg.supported_transports.iter().any(|t| t == name);
    if cfg.transport_port > 0 {
        if supports("noise") || cfg.supported_transports.is_empty() {
            out.push(("noise".to_owned(), cfg.transport_port));
        } else if supports("tcp") {
            out.push(("tcp".to_owned(), cfg.transport_port));
        }
    }
    if cfg.ws_port > 0 && cfg.ws_port != cfg.transport_port && supports("websocket") {
        out.push(("websocket".to_owned(), cfg.ws_port));
    }
    if out.is_empty() && cfg.transport_port > 0 {
        out.push(("tcp".to_owned(), cfg.transport_port));
    }
    out
}

fn build_server_doc(
    kind: &str,
    port: u16,
    noise_private_key: &str,
    services: BTreeMap<String, RatholeServerService>,
) -> RatholeServerDoc {
    let transport = match kind {
        "noise" => RatholeTransport {
            transport_type: "noise".to_owned(),
            noise: Some(RatholeNoise {
                pattern: "Noise_NK_25519_ChaChaPoly_BLAKE2s",
                local_private_key: noise_private_key.to_owned(),
            }),
            websocket: None,
        },
        "websocket" => RatholeTransport {
            transport_type: "websocket".to_owned(),
            noise: None,
            websocket: Some(RatholeWebsocket { tls: false }),
        },
        _ => RatholeTransport {
            transport_type: "tcp".to_owned(),
            noise: None,
            websocket: None,
        },
    };
    RatholeServerDoc {
        server: RatholeServerConfig {
            bind_addr: format!("0.0.0.0:{port}"),
            transport,
            services,
        },
    }
}

fn allocate_free_loopback_port() -> Result<SocketAddr> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    Ok(listener.local_addr()?)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generates_valid_rathole_server_toml_with_empty_and_active_services() {
        let mut services = BTreeMap::new();
        services.insert(
            "tun_1_c1".to_owned(),
            RatholeServerService {
                service_type: "tcp".to_owned(),
                bind_addr: "127.0.0.1:19001".to_owned(),
                token: "tok-123".to_owned(),
                nodelay: true,
            },
        );
        let doc = build_server_doc("noise", 2333, "privkey==", services);
        let rendered = toml::to_string_pretty(&doc).expect("serialize toml");
        assert!(rendered.contains("type = \"noise\""));
        assert!(rendered.contains("tun_1_c1"));
    }
}
