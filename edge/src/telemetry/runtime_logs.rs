use std::{
    io::{self, Write},
    sync::OnceLock,
};
use tokio::sync::broadcast;

const BUFFER_LINES: usize = 128;
const MAX_LINE_BYTES: usize = 4096;

static LIVE_LOGS: OnceLock<broadcast::Sender<String>> = OnceLock::new();

pub(crate) fn subscribe() -> broadcast::Receiver<String> {
    LIVE_LOGS
        .get_or_init(|| broadcast::channel(BUFFER_LINES).0)
        .subscribe()
}

pub(crate) fn has_subscribers() -> bool {
    LIVE_LOGS
        .get()
        .is_some_and(|sender| sender.receiver_count() > 0)
}

#[derive(Default)]
pub(crate) struct LiveLogWriter(Vec<u8>);

impl Write for LiveLogWriter {
    fn write(&mut self, bytes: &[u8]) -> io::Result<usize> {
        let len = bytes.len().min(MAX_LINE_BYTES - self.0.len());
        self.0.extend_from_slice(&bytes[..len]);
        Ok(bytes.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

impl Drop for LiveLogWriter {
    fn drop(&mut self) {
        if let Some(sender) = LIVE_LOGS.get() {
            let text = String::from_utf8_lossy(&self.0);
            let line = text.trim_end_matches(['\r', '\n']);
            if !line.is_empty() {
                let _ = sender.send(line.to_owned());
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tracing_subscriber::{filter::dynamic_filter_fn, prelude::*};

    #[test]
    fn streams_only_events_after_subscribing() {
        let subscriber = tracing_subscriber::registry()
            .with(tracing_subscriber::EnvFilter::new("info"))
            .with(tracing_subscriber::fmt::layer().with_writer(std::io::sink))
            .with(
                tracing_subscriber::fmt::layer()
                    .with_ansi(false)
                    .with_writer(LiveLogWriter::default)
                    .with_filter(dynamic_filter_fn(|_, _| has_subscribers())),
            );
        fn log_event(message: &str) {
            tracing::info!("{message}");
        }

        tracing::subscriber::with_default(subscriber, || {
            log_event("before subscription");
            let mut receiver = subscribe();
            log_event("streamed event");
            assert!(receiver.try_recv().unwrap().contains("streamed event"));
            assert!(matches!(
                receiver.try_recv(),
                Err(broadcast::error::TryRecvError::Empty)
            ));
            drop(receiver);
            assert!(!has_subscribers());
        });
    }
}
