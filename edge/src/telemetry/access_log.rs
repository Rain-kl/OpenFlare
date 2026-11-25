use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::{
    collections::HashSet,
    fs::{File, OpenOptions, Permissions},
    io::{ErrorKind, Write},
    os::{
        fd::AsRawFd,
        unix::fs::{FileExt, OpenOptionsExt, PermissionsExt},
    },
    path::PathBuf,
    sync::{
        atomic::{AtomicUsize, Ordering},
        Arc,
    },
    thread::{self, JoinHandle},
    time::Duration,
};
use tokio::{
    sync::{mpsc, oneshot, OwnedSemaphorePermit, Semaphore},
    time::{Instant, MissedTickBehavior},
};

const MAX_ENTRIES: usize = 2048;
const MAX_BYTES: u64 = 4 * 1024 * 1024;
const QUEUE_CAPACITY: usize = 2048;
const QUEUE_BYTES: usize = 16 * 1024 * 1024;
const BATCH_SIZE: usize = 256;
const FLUSH_INTERVAL: Duration = Duration::from_millis(100);
const REPORT_INTERVAL: Duration = Duration::from_secs(5);

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeAccessLog {
    pub logged_at_unix: i64,
    pub remote_addr: String,
    pub host: String,
    pub path: String,
    pub user_agent: String,
    pub cache_status: String,
    pub status_code: i32,
    pub bytes_sent: i64,
    pub request_length: i64,
    pub request_time_ms: i64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub waf: Option<crate::policy::WafAudit>,
}

impl NodeAccessLog {
    fn heap_bytes(&self) -> usize {
        self.remote_addr.capacity()
            + self.host.capacity()
            + self.path.capacity()
            + self.user_agent.capacity()
            + self.cache_status.capacity()
            + self.waf.as_ref().map_or(0, |waf| waf.heap_bytes())
    }
}

pub struct AccessLogBuffer {
    sender: mpsc::Sender<QueuedLog>,
    budget: Arc<Semaphore>,
    dropped: Arc<AtomicUsize>,
}

struct QueuedLog {
    log: NodeAccessLog,
    _budget: OwnedSemaphorePermit,
}

pub struct AccessLogWriter {
    shutdown: Option<(oneshot::Sender<()>, JoinHandle<()>)>,
}

#[derive(Clone, Debug)]
pub struct AccessLogSpool {
    state_dir: PathBuf,
}

#[derive(Serialize, Deserialize)]
pub struct LogEntry<Log = NodeAccessLog> {
    pub id: u128,
    pub log: Log,
}

impl AccessLogBuffer {
    pub fn start(spool: AccessLogSpool) -> Result<(Self, AccessLogWriter)> {
        let (buffer, receiver) = Self::channel(QUEUE_CAPACITY, QUEUE_BYTES);
        let (stop, shutdown) = oneshot::channel();
        let dropped = buffer.dropped.clone();
        let runtime = tokio::runtime::Builder::new_current_thread()
            .enable_time()
            .build()?;
        let thread = thread::Builder::new()
            .name("edge-access-log".into())
            .spawn(move || runtime.block_on(write_logs(spool, receiver, shutdown, dropped)))
            .context("start access log writer")?;
        Ok((
            buffer,
            AccessLogWriter {
                shutdown: Some((stop, thread)),
            },
        ))
    }

    fn channel(capacity: usize, bytes: usize) -> (Self, mpsc::Receiver<QueuedLog>) {
        let (sender, receiver) = mpsc::channel(capacity);
        let buffer = Self {
            sender,
            budget: Arc::new(Semaphore::new(bytes)),
            dropped: Arc::new(AtomicUsize::new(0)),
        };
        (buffer, receiver)
    }

    pub fn push(&self, log: NodeAccessLog) {
        let queued = u32::try_from(log.heap_bytes()).ok().is_some_and(|bytes| {
            self.budget
                .clone()
                .try_acquire_many_owned(bytes)
                .is_ok_and(|budget| {
                    self.sender
                        .try_send(QueuedLog {
                            log,
                            _budget: budget,
                        })
                        .is_ok()
                })
        });
        if !queued {
            self.dropped.fetch_add(1, Ordering::Relaxed);
        }
    }
}

impl Drop for AccessLogWriter {
    fn drop(&mut self) {
        if let Some((stop, thread)) = self.shutdown.take() {
            let _ = stop.send(());
            if thread.join().is_err() {
                tracing::error!("access log writer stopped unexpectedly");
            }
        }
    }
}

async fn write_logs(
    spool: AccessLogSpool,
    mut receiver: mpsc::Receiver<QueuedLog>,
    mut shutdown: oneshot::Receiver<()>,
    dropped: Arc<AtomicUsize>,
) {
    let mut batch = Vec::with_capacity(BATCH_SIZE);
    let mut flush_clock = tokio::time::interval_at(Instant::now() + FLUSH_INTERVAL, FLUSH_INTERVAL);
    flush_clock.set_missed_tick_behavior(MissedTickBehavior::Skip);
    let mut report_clock =
        tokio::time::interval_at(Instant::now() + REPORT_INTERVAL, REPORT_INTERVAL);
    report_clock.set_missed_tick_behavior(MissedTickBehavior::Skip);
    let mut disk_failures = DiskFailures::default();
    loop {
        tokio::select! {
            biased;
            _ = &mut shutdown => {
                receiver.close();
                break;
            }
            _ = report_clock.tick() => disk_failures.report(&dropped),
            _ = flush_clock.tick() => flush(&spool, &mut batch, &mut disk_failures),
            log = receiver.recv() => {
                let Some(log) = log else { break };
                batch.push(log);
                if batch.len() == BATCH_SIZE {
                    flush(&spool, &mut batch, &mut disk_failures);
                }
            }
        }
    }
    while let Some(log) = receiver.recv().await {
        batch.push(log);
        if batch.len() == BATCH_SIZE {
            flush(&spool, &mut batch, &mut disk_failures);
        }
    }
    flush(&spool, &mut batch, &mut disk_failures);
    disk_failures.report(&dropped);
}

#[derive(Default)]
struct DiskFailures {
    entries: usize,
    last_error: Option<anyhow::Error>,
}

impl DiskFailures {
    fn report(&mut self, dropped: &AtomicUsize) {
        let rejected = dropped.swap(0, Ordering::Relaxed);
        if rejected > 0 {
            tracing::warn!(rejected, "access logs dropped at memory queue admission");
        }
        if let Some(error) = self.last_error.take() {
            let entries = std::mem::take(&mut self.entries);
            tracing::warn!(entries, error = %format_args!("{error:#}"), "access log batches could not be persisted");
        }
    }
}

fn flush(spool: &AccessLogSpool, batch: &mut Vec<QueuedLog>, failures: &mut DiskFailures) {
    if !batch.is_empty() {
        let entries = batch.len();
        if let Err(error) = spool.write_batch(batch.iter().map(|entry| &entry.log)) {
            failures.entries += entries;
            failures.last_error = Some(error);
        }
        batch.clear();
    }
}

impl AccessLogSpool {
    pub fn persistent(state_dir: impl Into<PathBuf>) -> Self {
        Self {
            state_dir: state_dir.into(),
        }
    }

    fn lock(&self) -> Result<File> {
        let file = OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(self.state_dir.join("access-logs.lock"))?;
        // The lock also serializes writes while old and new processes overlap during FD handoff.
        if unsafe { libc::flock(file.as_raw_fd(), libc::LOCK_EX) } != 0 {
            return Err(std::io::Error::last_os_error()).context("lock access log spool");
        }
        Ok(file)
    }

    fn path(&self) -> PathBuf {
        self.state_dir.join("access-logs.jsonl")
    }

    fn read(&self) -> Result<Vec<u8>> {
        match std::fs::read(self.path()) {
            Ok(bytes) => Ok(bytes),
            Err(error) if error.kind() == ErrorKind::NotFound => Ok(vec![]),
            Err(error) => Err(error.into()),
        }
    }

    fn replace_lines<'a>(&self, lines: impl Iterator<Item = &'a [u8]>) -> Result<()> {
        let mut file = tempfile::NamedTempFile::new_in(&self.state_dir)?;
        {
            let mut writer = std::io::BufWriter::new(file.as_file_mut());
            for line in lines {
                writer.write_all(line)?;
                writer.write_all(b"\n")?;
            }
            writer.flush()?;
        }
        file.as_file().sync_data()?;
        file.persist(self.path())?;
        Ok(())
    }

    fn write_batch<'a>(&self, logs: impl IntoIterator<Item = &'a NodeAccessLog>) -> Result<()> {
        let mut encoded = Vec::new();
        for log in logs {
            serde_json::to_writer(
                &mut encoded,
                &LogEntry {
                    id: rand::random(),
                    log,
                },
            )?;
            encoded.push(b'\n');
        }
        let _lock = self.lock()?;
        let file = OpenOptions::new()
            .create(true)
            .read(true)
            .append(true)
            .mode(0o600)
            .open(self.path())?;
        let metadata = file.metadata()?;
        if metadata.permissions().mode() & 0o077 != 0 {
            file.set_permissions(Permissions::from_mode(0o600))?;
        }
        let previous_len = metadata.len();
        let mut last_byte = [b'\n'];
        if previous_len > 0 {
            file.read_exact_at(&mut last_byte, previous_len - 1)?;
        }
        let separator = last_byte != [b'\n'];
        if previous_len + encoded.len() as u64 + u64::from(separator) > MAX_BYTES {
            let previous = self.read()?;
            let mut used = 0;
            let retained = lines(&previous)
                .chain(lines(&encoded))
                .rev()
                .filter(|line| (line.len() as u64) < MAX_BYTES)
                .take(MAX_ENTRIES)
                .take_while(|line| {
                    used += line.len() as u64 + 1;
                    used <= MAX_BYTES
                })
                .collect::<Vec<_>>();
            return self.replace_lines(retained.into_iter().rev());
        }
        let append = |mut file: &File| {
            if separator {
                file.write_all(b"\n")?;
            }
            file.write_all(&encoded)?;
            file.sync_data()
        };
        for attempt in 0..2 {
            if let Err(error) = append(&file) {
                file.set_len(previous_len)
                    .with_context(|| format!("roll back access log append after {error}"))?;
                if attempt == 1 {
                    return Err(error).context("persist access log batch");
                }
            } else {
                break;
            }
        }
        Ok(())
    }

    pub fn peek(&self, limit: usize) -> Result<Vec<LogEntry>> {
        let bytes = {
            let _lock = self.lock()?;
            self.read()?
        };
        Ok(lines(&bytes)
            .filter_map(|line| serde_json::from_slice(line).ok())
            .take(limit)
            .collect())
    }

    pub fn commit(&self, acknowledged: Vec<u128>) -> Result<()> {
        if acknowledged.is_empty() {
            return Ok(());
        }
        #[derive(Deserialize)]
        struct EntryId {
            id: u128,
        }
        let acknowledged: HashSet<_> = acknowledged.into_iter().collect();
        let _lock = self.lock()?;
        let bytes = self.read()?;
        self.replace_lines(lines(&bytes).filter(|line| {
            serde_json::from_slice::<EntryId>(line)
                .is_ok_and(|entry| !acknowledged.contains(&entry.id))
        }))
    }
}

fn lines(bytes: &[u8]) -> impl DoubleEndedIterator<Item = &[u8]> {
    bytes
        .split(|&byte| byte == b'\n')
        .filter(|line| !line.is_empty())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn log(path: &str) -> NodeAccessLog {
        NodeAccessLog {
            logged_at_unix: 100,
            remote_addr: "192.0.2.1".into(),
            host: "example.com".into(),
            path: path.into(),
            user_agent: "curl/8".into(),
            cache_status: "-".into(),
            status_code: 200,
            bytes_sent: 123,
            request_length: 45,
            request_time_ms: 6,
            waf: None,
        }
    }

    #[test]
    fn writer_flushes_at_low_load_and_drains_on_shutdown() {
        let state_dir = tempfile::tempdir().unwrap();
        let spool = AccessLogSpool::persistent(state_dir.path());
        let (buffer, writer) = AccessLogBuffer::start(spool.clone()).unwrap();
        buffer.push(log("/first"));
        let deadline = std::time::Instant::now() + Duration::from_secs(3);
        let first = loop {
            if let Some(entry) = spool.peek(1).unwrap().pop() {
                break entry;
            }
            assert!(
                std::time::Instant::now() < deadline,
                "low-volume batch was not flushed"
            );
            thread::sleep(Duration::from_millis(10));
        };
        assert_eq!(first.log.path, "/first");
        for index in 0..=BATCH_SIZE {
            buffer.push(log(&format!("/next-{index}")));
        }
        drop(writer);

        let reopened = AccessLogSpool::persistent(state_dir.path());
        reopened.commit(vec![first.id]).unwrap();
        let remaining = reopened.peek(MAX_ENTRIES).unwrap();
        assert_eq!(remaining.len(), BATCH_SIZE + 1);
        for (index, entry) in remaining.iter().enumerate() {
            assert_eq!(entry.log.path, format!("/next-{index}"));
        }
        reopened
            .commit(remaining.into_iter().map(|entry| entry.id).collect())
            .unwrap();
        assert!(reopened.peek(1).unwrap().is_empty());
    }

    #[test]
    fn queue_drops_new_entries_at_count_or_byte_limit() {
        let first = log("/first");
        let (buffer, mut receiver) = AccessLogBuffer::channel(1, QUEUE_BYTES);
        buffer.push(first.clone());
        buffer.push(log("/count-limited"));
        assert_eq!(buffer.dropped.load(Ordering::Relaxed), 1);
        assert_eq!(receiver.try_recv().unwrap().log.path, "/first");
        assert!(receiver.try_recv().is_err());

        let (buffer, mut receiver) = AccessLogBuffer::channel(2, first.heap_bytes());
        buffer.push(first.clone());
        let in_flight = receiver.try_recv().unwrap();
        buffer.push(first.clone());
        assert_eq!(buffer.dropped.load(Ordering::Relaxed), 1);
        assert!(receiver.try_recv().is_err());
        drop(in_flight);
        buffer.push(first);
        assert_eq!(receiver.try_recv().unwrap().log.path, "/first");
    }

    #[test]
    fn compaction_retains_newest_entries_and_spool_permissions() {
        let state_dir = tempfile::tempdir().unwrap();
        let spool = AccessLogSpool::persistent(state_dir.path());
        let old = log(&"x".repeat(4096));
        for _ in 0..4 {
            spool
                .write_batch(std::iter::repeat_n(&old, BATCH_SIZE))
                .unwrap();
        }
        spool.write_batch([&log("/newest")]).unwrap();
        let retained = spool.peek(MAX_ENTRIES + 1).unwrap();
        let metadata = std::fs::metadata(spool.path()).unwrap();
        assert!(retained.len() < BATCH_SIZE * 4 + 1);
        assert_eq!(retained.last().unwrap().log.path, "/newest");
        assert!(metadata.len() <= MAX_BYTES);
        assert_eq!(metadata.permissions().mode() & 0o777, 0o600);
    }

    #[test]
    fn oversized_line_does_not_evict_the_existing_spool() {
        let state_dir = tempfile::tempdir().unwrap();
        let spool = AccessLogSpool::persistent(state_dir.path());
        let oversized = log(&"x".repeat(MAX_BYTES as usize));
        spool.write_batch([&log("/first")]).unwrap();
        spool.write_batch([&oversized, &log("/last")]).unwrap();
        let batch = spool.peek(MAX_ENTRIES).unwrap();
        assert_eq!(batch.len(), 2);
        assert_eq!(batch[0].log.path, "/first");
        assert_eq!(batch[1].log.path, "/last");
    }

    #[test]
    fn append_does_not_join_legacy_incomplete_line() {
        let state_dir = tempfile::tempdir().unwrap();
        let spool = AccessLogSpool::persistent(state_dir.path());
        let old = LogEntry {
            id: 1,
            log: log("/old"),
        };
        let mut partial = serde_json::to_vec(&old).unwrap();
        partial.extend_from_slice(b"\n{\"id\":2,");
        std::fs::write(spool.path(), partial).unwrap();
        spool.write_batch([&log("/new")]).unwrap();
        let batch = spool.peek(2).unwrap();
        assert_eq!(batch.len(), 2);
        assert_eq!(batch[0].log.path, "/old");
        assert_eq!(batch[1].log.path, "/new");
    }
}
