use anyhow::{Context, Result};
use futures_util::{SinkExt, StreamExt};
use serde::Deserialize;
use std::{
    sync::atomic::{AtomicBool, Ordering},
    time::Duration,
};
use tokio::sync::Notify;
use tokio_tungstenite::{
    connect_async,
    tungstenite::{client::IntoClientRequest, Message},
};

#[derive(Default)]
pub(super) struct Wake {
    notify: Notify,
    force: AtomicBool,
}

impl Wake {
    pub(super) async fn changed(&self) {
        self.notify.notified().await;
    }

    pub(super) fn requeue_force(&self) {
        self.force.store(true, Ordering::Release);
    }

    pub(super) fn take_force(&self) -> bool {
        self.force.swap(false, Ordering::AcqRel)
    }
}

pub(super) async fn run(base_url: &str, token: &str, wake: &Wake) {
    let mut retry = super::RetryDelay::default();
    loop {
        if let Err(error) = connect(base_url, token, wake, &mut retry).await {
            tracing::warn!(%error, "control WebSocket disconnected");
        }
        retry.wait().await;
    }
}

#[derive(Deserialize)]
struct MessageKind {
    #[serde(rename = "type")]
    kind: String,
    payload: Option<ConfigChanged>,
}

#[derive(Deserialize)]
struct ConfigChanged {
    #[serde(default)]
    force: bool,
}

async fn connect(
    base_url: &str,
    token: &str,
    wake: &Wake,
    retry: &mut super::RetryDelay,
) -> Result<()> {
    let mut url = url::Url::parse(&format!("{base_url}/api/v1/agent/ws"))?;
    let scheme = if url.scheme() == "https" { "wss" } else { "ws" };
    url.set_scheme(scheme)
        .map_err(|_| anyhow::anyhow!("invalid WebSocket URL"))?;
    let mut request = url.as_str().into_client_request()?;
    request
        .headers_mut()
        .insert("X-Agent-Token", token.parse()?);
    let (mut socket, _) =
        tokio::time::timeout(Duration::from_secs(15), connect_async(request)).await??;
    *retry = super::RetryDelay::default();
    wake.notify.notify_one();
    let mut keepalive = tokio::time::interval(Duration::from_secs(20));
    loop {
        tokio::select! {
            message = socket.next() => {
                match message.context("control WebSocket closed")?? {
                    Message::Text(text) => {
                        let Ok(message) = serde_json::from_str::<MessageKind>(&text) else {
                            continue;
                        };
                        match message.kind.as_str() {
                            "config_changed" => {
                                if message.payload.is_some_and(|payload| payload.force) {
                                    wake.requeue_force();
                                }
                                wake.notify.notify_one();
                            }
                            "ping" => socket.send(Message::Text(r#"{"type":"pong"}"#.into())).await?,
                            _ => {}
                        }
                    }
                    Message::Ping(_) => socket.flush().await?,
                    Message::Close(_) => return Ok(()),
                    _ => {}
                }
            }
            _ = keepalive.tick() => {
                tokio::time::timeout(
                    Duration::from_secs(5), socket.send(Message::Text(r#"{"type":"ping"}"#.into())),
                ).await??;
            }
        }
    }
}
