pub(crate) mod performance;
pub(crate) mod tunnel_routing;

use crate::{
    config::{self, SnapshotStore},
    telemetry::tcp_sockets,
    EDGE_VERSION,
};
use anyhow::{Context, Result};
use serde::Serialize;
use std::{net::SocketAddr, path::Path, time::Duration};

#[derive(Clone)]
pub struct Listeners {
    pub http: SocketAddr,
    pub tls: Option<SocketAddr>,
}

impl Listeners {
    pub fn new(http: &str, tls: Option<&str>) -> Result<Self> {
        let parse = |value: &str| {
            let address = value
                .parse::<SocketAddr>()
                .with_context(|| format!("invalid listener address {value}"))?;
            anyhow::ensure!(address.port() != 0, "listener must use a fixed port");
            Ok(address)
        };
        let http = parse(http)?;
        let tls = tls.map(parse).transpose()?;
        anyhow::ensure!(
            tls != Some(http),
            "HTTP and HTTPS listeners need distinct addresses"
        );
        Ok(Self { http, tls })
    }

    pub async fn ready(&self) -> bool {
        if cfg!(target_os = "linux") {
            let sockets = tcp_sockets();
            return [Some(self.http), self.tls]
                .into_iter()
                .flatten()
                .all(|address| {
                    sockets
                        .iter()
                        .any(|socket| socket.listening && socket.port == address.port())
                });
        }
        for mut address in [Some(self.http), self.tls].into_iter().flatten() {
            if address.ip().is_unspecified() {
                address.set_ip(if address.is_ipv4() {
                    std::net::Ipv4Addr::LOCALHOST.into()
                } else {
                    std::net::Ipv6Addr::LOCALHOST.into()
                });
            }
            if !tokio::time::timeout(
                Duration::from_millis(150),
                tokio::net::TcpStream::connect(address),
            )
            .await
            .is_ok_and(|result| result.is_ok())
            {
                return false;
            }
        }
        true
    }
}

pub(crate) async fn claim_control(state_dir: &Path) -> Result<std::fs::File> {
    use std::os::fd::AsRawFd;

    let lock = std::fs::OpenOptions::new()
        .create(true)
        .truncate(false)
        .read(true)
        .write(true)
        .open(state_dir.join("control.lock"))?;
    loop {
        // Durably writing snapshots requires exclusive ownership while listeners overlap.
        if unsafe { libc::flock(lock.as_raw_fd(), libc::LOCK_EX | libc::LOCK_NB) } == 0 {
            return Ok(lock);
        }
        let error = std::io::Error::last_os_error();
        if error.kind() != std::io::ErrorKind::WouldBlock {
            return Err(error).context("lock Edge control loop");
        }
        tokio::time::sleep(Duration::from_millis(250)).await;
    }
}

pub async fn announce_ready(
    listeners: &Listeners,
    state_dir: &Path,
    state: &SnapshotStore,
) -> Result<()> {
    tokio::time::timeout(Duration::from_secs(15), async {
        while !listeners.ready().await {
            tokio::time::sleep(Duration::from_millis(100)).await;
        }
    })
    .await
    .context("configured listeners did not become ready")?;
    #[derive(Serialize)]
    struct Ready<'a> {
        pid: u32,
        binary_version: &'static str,
        applied_version: &'a str,
        payload_checksum: &'a str,
    }
    let current = loop {
        if let Some(snapshot) = state.current() {
            break snapshot;
        }
        tokio::time::sleep(Duration::from_millis(100)).await;
    };
    config::persist_private_json(
        &state_dir.join("ready.json"),
        &Ready {
            pid: std::process::id(),
            binary_version: EDGE_VERSION,
            applied_version: &current.version,
            payload_checksum: &current.payload_checksum,
        },
    )?;
    crate::control::update::confirm(state_dir)?;
    notify_systemd()?;
    Ok(())
}

#[cfg(target_os = "linux")]
fn notify_systemd() -> Result<()> {
    use std::os::linux::net::SocketAddrExt;
    use std::os::unix::{
        ffi::OsStrExt,
        net::{SocketAddr, UnixDatagram},
    };
    let Some(socket) = std::env::var_os("NOTIFY_SOCKET") else {
        return Ok(());
    };
    let addr = match socket.as_bytes().strip_prefix(b"@") {
        Some(name) => SocketAddr::from_abstract_name(name),
        None => SocketAddr::from_pathname(&socket),
    }?;
    UnixDatagram::unbound()?
        .send_to_addr(b"READY=1\nSTATUS=Pingora configuration active", &addr)?;
    Ok(())
}

#[cfg(not(target_os = "linux"))]
fn notify_systemd() -> Result<()> {
    Ok(())
}

// Pingora handles SIGTERM by draining listeners. The service manager restarts the node.
pub(crate) fn request_restart() {
    let status = unsafe { libc::raise(libc::SIGTERM) };
    if status != 0 {
        tracing::error!(error = %std::io::Error::last_os_error(), "could not signal Edge restart");
    }
}
