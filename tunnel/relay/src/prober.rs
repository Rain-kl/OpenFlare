use openflare_tunnel_common::protocol::PATH_CONNECTOR_HEALTH;
use std::net::SocketAddr;
use std::sync::atomic::Ordering;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;
use tokio::sync::broadcast;

use crate::gateway::RelayGatewayState;

const PROBE_INTERVAL: Duration = Duration::from_secs(5);
const PROBE_TIMEOUT: Duration = Duration::from_secs(2);

pub fn spawn_prober_loop(state: Arc<RelayGatewayState>, mut shutdown: broadcast::Receiver<bool>) {
    tokio::spawn(async move {
        let mut ticker = tokio::time::interval(PROBE_INTERVAL);
        ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
        loop {
            tokio::select! {
                _ = shutdown.recv() => return,
                _ = ticker.tick() => {
                    let entries = state.all_connectors().await;
                    for entry in entries {
                        let mut matched = None;
                        for &addr in &entry.candidate_addrs {
                            if let Some(rtt_ms) = probe_connector_endpoint(addr).await {
                                matched = Some((addr, rtt_ms));
                                break;
                            }
                        }
                        if let Some((addr, rtt_ms)) = matched {
                            *entry.active_addr.write().await = Some(addr);
                            entry.rtt_ms.store(rtt_ms, Ordering::Release);
                            entry.healthy.store(true, Ordering::Release);
                        } else {
                            entry.healthy.store(false, Ordering::Release);
                        }
                    }
                }
            }
        }
    });
}

async fn probe_connector_endpoint(addr: SocketAddr) -> Option<i64> {
    let start = Instant::now();
    let probe = async {
        let mut stream = TcpStream::connect(addr).await.ok()?;
        let req = format!(
            "GET {PATH_CONNECTOR_HEALTH} HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n"
        );
        stream.write_all(req.as_bytes()).await.ok()?;
        let mut buf = [0u8; 256];
        let n = stream.read(&mut buf).await.ok()?;
        let head = std::str::from_utf8(&buf[..n]).ok()?;
        if head.starts_with("HTTP/1.1 200") || head.starts_with("HTTP/1.0 200") {
            Some(start.elapsed().as_millis().max(1) as i64)
        } else {
            None
        }
    };
    tokio::time::timeout(PROBE_TIMEOUT, probe).await.ok().flatten()
}
