mod release;

use anyhow::{Context, Result};
use serde::Deserialize;
use std::fs;
use std::io::ErrorKind;
use std::os::unix::process::CommandExt;
use std::path::Path;
use std::time::Duration;
use tokio::sync::watch;
use tokio::time::Instant;

pub const VERSION: u8 = 1;
const NEXT: &str = "edge-update-next";
const TRIAL: &str = "edge-update-trial";
const ACTIVE: &str = "edge-update-active";
const CHECK_INTERVAL: Duration = Duration::from_secs(6 * 60 * 60);
const MIN_RETRY: Duration = Duration::from_secs(60);
const MAX_RETRY: Duration = Duration::from_secs(3 * 60 * 60);
const ROLLBACK_DELAY: Duration = Duration::from_secs(15 * 60);

#[derive(Debug, Deserialize)]
pub(super) struct Settings {
    repo: String,
    auto_update: bool,
    tag: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub(super) struct UpdatePlan {
    repo: String,
    tag: Option<String>,
}

impl Settings {
    pub(super) fn plan(self) -> Option<UpdatePlan> {
        (self.tag.is_some() || self.auto_update).then_some(UpdatePlan {
            repo: self.repo,
            tag: self.tag,
        })
    }
}

pub fn supported() -> bool {
    cfg!(target_os = "linux")
        && matches!(std::env::consts::ARCH, "x86_64" | "aarch64")
        && unsafe { libc::geteuid() } != 0
        && std::env::var_os("NOTIFY_SOCKET").is_some()
        && std::env::var_os("INVOCATION_ID").is_some()
}

fn running(path: &Path) -> bool {
    std::env::current_exe()
        .is_ok_and(|current| same_file::is_same_file(current, path).unwrap_or(false))
}

// The root-owned binary remains systemd's entry point. It only executes a
// replacement as the unprivileged service account and tries a candidate once.
pub fn launch(state_dir: &Path) {
    if !supported() {
        return;
    }
    let trial = state_dir.join(TRIAL);
    let active = state_dir.join(ACTIVE);
    if running(&trial) || running(&active) {
        return;
    }
    match fs::rename(state_dir.join(NEXT), &trial) {
        Ok(()) => {
            tracing::info!("starting staged Edge update");
            exec(&trial);
        }
        Err(error) if error.kind() == ErrorKind::NotFound => {}
        Err(error) => tracing::error!(%error, "could not start trial of staged Edge update"),
    }
    if active.is_file() {
        exec(&active);
    }
}

fn exec(path: &Path) {
    let error = std::process::Command::new(path)
        .args(std::env::args_os().skip(1))
        .exec();
    tracing::error!(%error, path = %path.display(), "could not start updated Edge binary");
}

pub fn trial_running(state_dir: &Path) -> bool {
    supported() && running(&state_dir.join(TRIAL))
}

pub fn restart_pending(state_dir: &Path) -> bool {
    supported() && state_dir.join(NEXT).exists()
}

pub fn confirm(state_dir: &Path) -> Result<()> {
    if trial_running(state_dir) {
        fs::rename(state_dir.join(TRIAL), state_dir.join(ACTIVE))
            .context("commit ready Edge update")?;
        tracing::info!(version = crate::EDGE_VERSION, "Edge update is ready");
    }
    Ok(())
}

fn rollback_delay(state_dir: &Path) -> Option<Duration> {
    let trial = state_dir.join(TRIAL);
    let modified = fs::metadata(trial).ok()?.modified().ok()?;
    Some(ROLLBACK_DELAY.saturating_sub(modified.elapsed().unwrap_or_default()))
}

pub(super) async fn run(
    state_dir: &Path,
    mut plans: watch::Receiver<Option<UpdatePlan>>,
    errors: watch::Sender<Option<String>>,
) {
    if !supported() {
        return;
    }
    let mut plan = None;
    let mut next_check = Instant::now();
    let mut retry = MIN_RETRY;
    if let Some(delay) = rollback_delay(state_dir) {
        next_check += delay;
        errors.send_replace(Some(
            "Edge update did not become ready. The previous version was restored.".into(),
        ));
    }
    loop {
        let incoming = plans.borrow_and_update().clone();
        if plan != incoming {
            retry = MIN_RETRY;
            if plan.is_some() {
                next_check = Instant::now();
                errors.send_replace(None);
            }
            plan = incoming;
        }
        let Some(current) = plan.as_ref() else {
            if plans.changed().await.is_err() {
                return;
            }
            continue;
        };
        let check = async {
            tokio::time::sleep_until(next_check).await;
            let client = release::client()?;
            release::download(&client, state_dir, current).await
        };
        let result = tokio::select! {
            biased;
            changed = plans.changed() => {
                if changed.is_err() {
                    return;
                }
                continue;
            }
            result = check => result,
        };
        let result = result.and_then(|candidate| {
            if let Some(candidate) = candidate {
                candidate
                    .persist(state_dir.join(NEXT))
                    .context("stage verified Edge update")?;
                return Ok(true);
            }
            Ok(false)
        });
        match result {
            Ok(true) => {
                tracing::info!(repo = %current.repo, tag = ?current.tag, "verified Edge update staged");
                crate::runtime::request_restart();
                return;
            }
            Ok(false) => {
                errors.send_replace(None);
                next_check = Instant::now() + CHECK_INTERVAL;
                retry = MIN_RETRY;
            }
            Err(error) => {
                tracing::warn!(%error, retry_seconds = retry.as_secs(), "Edge update failed");
                errors.send_replace(Some(format!("Edge update failed: {error}")));
                next_check = Instant::now() + retry;
                retry = (retry * 2).min(MAX_RETRY);
            }
        }
    }
}
