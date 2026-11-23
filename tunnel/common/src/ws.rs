use futures_util::{SinkExt, StreamExt};
use http::HeaderName;
use std::time::Duration;
use tokio::sync::{broadcast, mpsc};
use tokio_tungstenite::tungstenite::client::IntoClientRequest;
use tokio_tungstenite::tungstenite::http::HeaderValue;
use tokio_tungstenite::tungstenite::Message;

use crate::hardening::WakeMessage;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WakeSignal {
    ConfigSync,
    ForceDisconnect,
}

pub fn spawn_wake_loop(
    ws_url: String,
    headers: Vec<(String, String)>,
    tx: mpsc::Sender<WakeSignal>,
    mut shutdown: broadcast::Receiver<bool>,
) {
    tokio::spawn(async move {
        let mut backoff_secs = 1u64;
        loop {
            if shutdown.try_recv().is_ok() {
                return;
            }
            match run_ws_session(&ws_url, &headers, &tx, &mut shutdown).await {
                Ok(should_stop) => {
                    backoff_secs = 1;
                    if should_stop {
                        return;
                    }
                }
                Err(err) => {
                    tracing::debug!(%err, url = %ws_url, "control plane wake websocket disconnected");
                }
            }
            tokio::select! {
                _ = shutdown.recv() => return,
                _ = tokio::time::sleep(Duration::from_secs(backoff_secs)) => {}
            }
            backoff_secs = (backoff_secs * 2).min(30);
        }
    });
}

async fn run_ws_session(
    ws_url: &str,
    headers: &[(String, String)],
    tx: &mpsc::Sender<WakeSignal>,
    shutdown: &mut broadcast::Receiver<bool>,
) -> anyhow::Result<bool> {
    let mut request = ws_url.into_client_request()?;
    {
        let req_headers = request.headers_mut();
        for (name, value) in headers {
            if let (Ok(k), Ok(v)) = (
                HeaderName::from_bytes(name.as_bytes()),
                HeaderValue::from_str(value),
            ) {
                req_headers.insert(k, v);
            }
        }
    }

    let (ws_stream, _) = tokio_tungstenite::connect_async(request).await?;
    let (mut write, mut read) = ws_stream.split();
    let mut ping_interval = tokio::time::interval(Duration::from_secs(30));

    loop {
        tokio::select! {
            _ = shutdown.recv() => {
                let _ = write.send(Message::Close(None)).await;
                return Ok(true);
            }
            _ = ping_interval.tick() => {
                write.send(Message::Ping(Vec::new())).await?;
            }
            frame = read.next() => {
                match frame {
                    Some(Ok(Message::Text(text))) => {
                        let signal = parse_wake_signal(&text);
                        let _ = tx.send(signal).await;
                    }
                    Some(Ok(Message::Binary(bin))) => {
                        if let Ok(text) = std::str::from_utf8(&bin) {
                            let _ = tx.send(parse_wake_signal(text)).await;
                        }
                    }
                    Some(Ok(Message::Ping(payload))) => {
                        write.send(Message::Pong(payload)).await?;
                    }
                    Some(Ok(Message::Close(_))) | None => {
                        return Ok(false);
                    }
                    Some(Err(err)) => return Err(err.into()),
                    Some(Ok(_)) => {}
                }
            }
        }
    }
}

pub fn parse_wake_signal(raw: &str) -> WakeSignal {
    let parsed: Result<WakeMessage, _> = serde_json::from_str(raw);
    match parsed {
        Ok(msg)
            if matches!(
                msg.kind.as_str(),
                "quarantined" | "revoked" | "disconnect" | "force_disconnect"
            ) =>
        {
            WakeSignal::ForceDisconnect
        }
        Ok(_) | Err(_) => WakeSignal::ConfigSync,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_wake_message_variants() {
        assert_eq!(
            parse_wake_signal(r#"{"type":"config_changed","config_revision":12}"#),
            WakeSignal::ConfigSync
        );
        assert_eq!(
            parse_wake_signal(r#"{"type":"force_sync"}"#),
            WakeSignal::ConfigSync
        );
        assert_eq!(
            parse_wake_signal(r#"{"type":"drain"}"#),
            WakeSignal::ConfigSync
        );
        assert_eq!(
            parse_wake_signal(r#"{"type":"revoked"}"#),
            WakeSignal::ForceDisconnect
        );
        assert_eq!(
            parse_wake_signal(r#"{"type":"quarantined","reason":"manual"}"#),
            WakeSignal::ForceDisconnect
        );
        assert_eq!(parse_wake_signal("sync"), WakeSignal::ConfigSync);
    }
}
