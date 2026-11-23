use anyhow::Result;
use openflare_tunnel_common::hardening::write_file_in_place_0600;
use openflare_tunnel_common::protocol::{
    derive_l4_service_name, derive_l4_service_token, TunnelConnectedRelayReport,
    TunnelConnectorConfigResponse, TunnelRelayCandidate,
};
use serde::Serialize;
use std::collections::{BTreeMap, HashMap};
use std::net::SocketAddr;
use std::path::PathBuf;
use std::time::Duration;
use tokio::sync::{broadcast, Mutex};

pub struct RatholeClientPool {
    dir: PathBuf,
    ingress_addr: SocketAddr,
    tunnel_token: String,
    tasks: Mutex<HashMap<u64, RunningClientTask>>,
}

struct RunningClientTask {
    relay_public_id: String,
    service_name: String,
    transport: String,
    signature: String,
    toml_path: PathBuf,
    shutdown_tx: broadcast::Sender<bool>,
    handle: tokio::task::JoinHandle<()>,
}

#[derive(Serialize)]
struct RatholeClientDoc {
    client: RatholeClientConfig,
}

#[derive(Serialize)]
struct RatholeClientConfig {
    remote_addr: String,
    transport: RatholeClientTransport,
    services: BTreeMap<String, RatholeClientService>,
}

#[derive(Serialize)]
struct RatholeClientTransport {
    #[serde(rename = "type")]
    transport_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    tcp: Option<RatholeTcpSection>,
    #[serde(skip_serializing_if = "Option::is_none")]
    noise: Option<RatholeClientNoise>,
    #[serde(skip_serializing_if = "Option::is_none")]
    websocket: Option<RatholeClientWebsocket>,
}

#[derive(Serialize)]
struct RatholeTcpSection {
    proxy: String,
}

#[derive(Serialize)]
struct RatholeClientNoise {
    pattern: &'static str,
    remote_public_key: String,
}

#[derive(Serialize)]
struct RatholeClientWebsocket {
    tls: bool,
}

#[derive(Serialize)]
struct RatholeClientService {
    #[serde(rename = "type")]
    service_type: String,
    local_addr: String,
    token: String,
    nodelay: bool,
}

impl RatholeClientPool {
    pub fn new(dir: PathBuf, ingress_addr: SocketAddr, tunnel_token: String) -> Self {
        Self {
            dir,
            ingress_addr,
            tunnel_token,
            tasks: Mutex::new(HashMap::new()),
        }
    }

    pub async fn reconcile(&self, cfg: &TunnelConnectorConfigResponse) -> Result<()> {
        let mut guard = self.tasks.lock().await;
        let relay_by_id: HashMap<u64, &TunnelRelayCandidate> =
            cfg.candidate_relays.iter().map(|r| (r.id, r)).collect();

        let mut desired_relays = HashMap::new();
        for att in &cfg.attachments {
            if att.state == "stale" {
                continue;
            }
            if let Some(&relay) = relay_by_id.get(&att.relay_id) {
                desired_relays.insert(att.relay_id, (att, relay));
            }
        }

        // Phase 1 (Make-Before-Break): bring up or hot-reload all desired relay attachments FIRST.
        for (&relay_id, &(att, relay)) in &desired_relays {
            let transport = if att.transport.is_empty() {
                "noise".to_owned()
            } else {
                att.transport.to_ascii_lowercase()
            };
            let port = if transport == "websocket" {
                relay.ws_port
            } else {
                relay.transport_port
            };
            if port == 0 || relay.public_addr.trim().is_empty() {
                continue;
            }
            let remote_host = if relay.public_addr.contains(':') && !relay.public_addr.starts_with('[') {
                format!("[{}]", relay.public_addr.trim())
            } else {
                relay.public_addr.trim().to_owned()
            };
            let remote_addr = format!("{remote_host}:{port}");

            let mut services = BTreeMap::new();
            services.insert(
                att.service_name.clone(),
                RatholeClientService {
                    service_type: "tcp".to_owned(),
                    local_addr: self.ingress_addr.to_string(),
                    token: att.session_token.clone(),
                    nodelay: true,
                },
            );

            for svc in &cfg.services {
                if svc.relay_id != 0 && svc.relay_id != relay_id {
                    continue;
                }
                let svc_name = derive_l4_service_name(cfg.tunnel_id, svc.id);
                let svc_token = derive_l4_service_token(&self.tunnel_token, cfg.tunnel_id, svc.id);
                let svc_type = if svc.service_type.eq_ignore_ascii_case("udp") {
                    "udp".to_owned()
                } else {
                    "tcp".to_owned()
                };
                services.insert(
                    svc_name,
                    RatholeClientService {
                        service_type: svc_type.clone(),
                        local_addr: svc.target_addr.clone(),
                        token: svc_token,
                        nodelay: svc_type == "tcp",
                    },
                );
            }

            let doc = build_client_doc(
                remote_addr.clone(),
                &transport,
                &relay.noise_public_key,
                cfg.client_proxy_url.trim(),
                services,
            );
            let toml_text = toml::to_string_pretty(&doc)?;
            let signature = format!(
                "{remote_addr}:{transport}:{}:{}",
                relay.noise_public_key,
                cfg.client_proxy_url.trim()
            );

            let needs_spawn = match guard.get(&relay_id) {
                Some(existing) => existing.signature != signature || existing.handle.is_finished(),
                None => true,
            };

            if needs_spawn {
                if let Some(old) = guard.remove(&relay_id) {
                    let _ = old.shutdown_tx.send(true);
                    old.handle.abort();
                }
                let path = self.dir.join(format!("client-relay-{relay_id}.toml"));
                write_file_in_place_0600(&path, &toml_text)?;
                let running = spawn_rathole_client(
                    relay.relay_id.clone(),
                    att.service_name.clone(),
                    transport,
                    path,
                    signature,
                );
                guard.insert(relay_id, running);
            } else if let Some(existing) = guard.get_mut(&relay_id) {
                existing.service_name = att.service_name.clone();
                write_file_in_place_0600(&existing.toml_path, &toml_text)?;
            }
        }

        // Phase 2: drain obsolete relay attachments gracefully after newly added attachments start.
        let obsolete_ids: Vec<u64> = guard
            .keys()
            .filter(|id| !desired_relays.contains_key(id))
            .copied()
            .collect();
        let mut retiring = Vec::new();
        for id in obsolete_ids {
            if let Some(old) = guard.remove(&id) {
                retiring.push(old);
            }
        }
        drop(guard);

        if !retiring.is_empty() {
            tokio::spawn(async move {
                tokio::time::sleep(Duration::from_millis(1500)).await;
                for old in retiring {
                    let _ = old.shutdown_tx.send(true);
                    let _ = tokio::time::timeout(Duration::from_secs(2), old.handle).await;
                }
            });
        }

        Ok(())
    }

    pub async fn connected_reports(
        &self,
        rtts: &HashMap<u64, i64>,
    ) -> Vec<TunnelConnectedRelayReport> {
        let guard = self.tasks.lock().await;
        guard
            .iter()
            .map(|(&relay_id, task)| TunnelConnectedRelayReport {
                relay_id,
                relay_public_id: task.relay_public_id.clone(),
                service_name: task.service_name.clone(),
                transport: task.transport.clone(),
                state: if task.handle.is_finished() {
                    "connecting".to_owned()
                } else {
                    "active".to_owned()
                },
                rtt_ms: rtts.get(&relay_id).copied().unwrap_or(0),
                active_streams: 0,
            })
            .collect()
    }

    pub async fn shutdown_all(&self) {
        let mut guard = self.tasks.lock().await;
        for (_, old) in guard.drain() {
            let _ = old.shutdown_tx.send(true);
            old.handle.abort();
        }
    }
}

fn spawn_rathole_client(
    relay_public_id: String,
    service_name: String,
    transport: String,
    toml_path: PathBuf,
    signature: String,
) -> RunningClientTask {
    let (shutdown_tx, shutdown_rx) = broadcast::channel(1);
    let cli_path = toml_path.clone();
    let cli = rathole::Cli {
        config_path: Some(cli_path.clone()),
        server: false,
        client: true,
        genkey: None,
    };
    let handle = tokio::spawn(async move {
        if let Err(err) = rathole::run(cli, shutdown_rx).await {
            tracing::error!(%err, path = %cli_path.display(), "rathole client task exited");
        }
    });
    RunningClientTask {
        relay_public_id,
        service_name,
        transport,
        signature,
        toml_path,
        shutdown_tx,
        handle,
    }
}

fn build_client_doc(
    remote_addr: String,
    transport: &str,
    noise_public_key: &str,
    proxy_url: &str,
    services: BTreeMap<String, RatholeClientService>,
) -> RatholeClientDoc {
    let tcp = (!proxy_url.is_empty()).then(|| RatholeTcpSection {
        proxy: proxy_url.to_owned(),
    });
    let transport_cfg = match transport {
        "noise" => RatholeClientTransport {
            transport_type: "noise".to_owned(),
            tcp,
            noise: Some(RatholeClientNoise {
                pattern: "Noise_NK_25519_ChaChaPoly_BLAKE2s",
                remote_public_key: noise_public_key.to_owned(),
            }),
            websocket: None,
        },
        "websocket" => RatholeClientTransport {
            transport_type: "websocket".to_owned(),
            tcp,
            noise: None,
            websocket: Some(RatholeClientWebsocket { tls: false }),
        },
        _ => RatholeClientTransport {
            transport_type: "tcp".to_owned(),
            tcp,
            noise: None,
            websocket: None,
        },
    };

    RatholeClientDoc {
        client: RatholeClientConfig {
            remote_addr,
            transport: transport_cfg,
            services,
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generates_valid_rathole_client_toml_with_noise_and_proxy() {
        let mut services = BTreeMap::new();
        services.insert(
            "http_t1_c1".to_owned(),
            RatholeClientService {
                service_type: "tcp".to_owned(),
                local_addr: "127.0.0.1:45123".to_owned(),
                token: "ses_abc".to_owned(),
                nodelay: true,
            },
        );
        let doc = build_client_doc(
            "relay.example.com:7443".into(),
            "noise",
            "pubkey==",
            "socks5://127.0.0.1:1080",
            services,
        );
        let rendered = toml::to_string_pretty(&doc).expect("serialize");
        assert!(rendered.contains("type = \"noise\""));
        assert!(rendered.contains("remote_public_key = \"pubkey==\""));
        assert!(rendered.contains("proxy = \"socks5://127.0.0.1:1080\""));
    }
}
