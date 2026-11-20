use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::{
    collections::HashSet,
    fs::{File, OpenOptions, Permissions},
    io::{BufRead, BufReader, Write},
    os::{
        fd::AsRawFd,
        unix::fs::{OpenOptionsExt, PermissionsExt},
    },
    path::PathBuf,
};

const MAX_ENTRIES: usize = 2048;
const MAX_BYTES: u64 = 4 * 1024 * 1024;

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

#[derive(Clone, Debug)]
pub struct AccessLogBuffer {
    state_dir: PathBuf,
}

#[derive(Serialize, Deserialize)]
pub struct LogEntry {
    id: u128,
    pub log: NodeAccessLog,
}

impl AccessLogBuffer {
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

    fn read_lines(&self) -> Result<Vec<String>> {
        match File::open(self.path()) {
            Ok(file) => BufReader::new(file)
                .lines()
                .collect::<std::io::Result<_>>()
                .map_err(Into::into),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(vec![]),
            Err(error) => Err(error.into()),
        }
    }

    fn replace_lines<'a>(&self, lines: impl Iterator<Item = &'a String>) -> Result<()> {
        let mut file = tempfile::NamedTempFile::new_in(&self.state_dir)?;
        for line in lines {
            writeln!(file, "{line}")?;
        }
        file.as_file().sync_data()?;
        file.persist(self.path())?;
        Ok(())
    }

    pub fn push(&self, log: NodeAccessLog) -> Result<()> {
        let _lock = self.lock()?;
        let line = serde_json::to_string(&LogEntry {
            id: rand::random(),
            log,
        })?;
        let mut file = OpenOptions::new()
            .create(true)
            .append(true)
            .mode(0o600)
            .open(self.path())?;
        if file.metadata()?.permissions().mode() & 0o077 != 0 {
            file.set_permissions(Permissions::from_mode(0o600))?;
        }
        writeln!(file, "{line}")?;
        file.sync_data()?;
        if file.metadata()?.len() > MAX_BYTES {
            let lines = self.read_lines()?;
            let mut retained = 0;
            let mut used = 0u64;
            for line in lines.iter().rev().take(MAX_ENTRIES) {
                used += line.len() as u64 + 1;
                if used > MAX_BYTES {
                    break;
                }
                retained += 1;
            }
            self.replace_lines(lines[lines.len() - retained..].iter())?;
        }
        Ok(())
    }

    pub fn peek(&self, limit: usize) -> Result<Vec<LogEntry>> {
        let _lock = self.lock()?;
        Ok(self
            .read_lines()?
            .iter()
            .filter_map(|line| serde_json::from_str(line).ok())
            .take(limit)
            .collect())
    }

    pub fn commit(&self, batch: &[LogEntry]) -> Result<()> {
        if batch.is_empty() {
            return Ok(());
        }
        let _lock = self.lock()?;
        let acknowledged: HashSet<_> = batch.iter().map(|entry| entry.id).collect();
        let lines = self.read_lines()?;
        self.replace_lines(lines.iter().filter(|line| {
            serde_json::from_str::<LogEntry>(line)
                .is_ok_and(|entry| !acknowledged.contains(&entry.id))
        }))
    }
}
