use crate::types::{ExecRequest, ExecResult};
use std::path::Path;
use std::process::Stdio;
use std::time::Duration;
use time::format_description::well_known::Rfc3339;
use time::OffsetDateTime;
use tokio::io::{AsyncRead, AsyncReadExt};
use tokio::process::Command;

/// Execution guardrails for remote shell commands.
#[derive(Debug, Clone)]
pub struct ExecOptions {
    pub default_timeout: Duration,
    pub max_timeout: Duration,
    pub max_output_bytes: usize,
}

impl Default for ExecOptions {
    fn default() -> Self {
        Self {
            default_timeout: Duration::from_secs(60),
            max_timeout: Duration::from_secs(600),
            max_output_bytes: 256 * 1024,
        }
    }
}

/// Executes a remote shell command with bounded output and hard timeout.
/// Automatically falls back from `/bin/bash` to `/bin/sh` on minimal Alpine/musl systems.
pub async fn execute_command(req: &ExecRequest, options: &ExecOptions) -> ExecResult {
    let timeout = req
        .timeout_seconds
        .filter(|&secs| secs > 0)
        .map(Duration::from_secs)
        .unwrap_or(options.default_timeout)
        .min(options.max_timeout);

    let (shell, flag) = resolve_shell();
    let mut cmd = Command::new(shell);
    cmd.arg(flag)
        .arg(&req.command)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .kill_on_drop(true);

    let mut child = match cmd.spawn() {
        Ok(child) => child,
        Err(error) => {
            return ExecResult {
                task_id: req.task_id.clone(),
                result: format!("failed to spawn command shell ({shell}): {error}"),
                exit_code: -1,
                finished_at: format_rfc3339_now(),
            };
        }
    };

    let stdout_pipe = child.stdout.take();
    let stderr_pipe = child.stderr.take();
    let limit = options.max_output_bytes.max(1024);

    let stdout_task = tokio::spawn(async move { read_bounded(stdout_pipe, limit).await });
    let stderr_task = tokio::spawn(async move { read_bounded(stderr_pipe, limit).await });

    match tokio::time::timeout(timeout, child.wait()).await {
        Ok(Ok(status)) => {
            let (stdout_buf, stdout_trunc) = stdout_task.await.unwrap_or_default();
            let (stderr_buf, stderr_trunc) = stderr_task.await.unwrap_or_default();
            let mut combined = combine_output(&stdout_buf, &stderr_buf, limit);
            if stdout_trunc || stderr_trunc {
                combined.push_str("\n[output truncated]");
            }
            ExecResult {
                task_id: req.task_id.clone(),
                result: combined,
                exit_code: status.code().unwrap_or(1),
                finished_at: format_rfc3339_now(),
            }
        }
        Ok(Err(error)) => {
            stdout_task.abort();
            stderr_task.abort();
            ExecResult {
                task_id: req.task_id.clone(),
                result: format!("command wait error: {error}"),
                exit_code: -1,
                finished_at: format_rfc3339_now(),
            }
        }
        Err(_) => {
            let _ = child.kill().await;
            let _ = child.wait().await;
            stdout_task.abort();
            stderr_task.abort();
            ExecResult {
                task_id: req.task_id.clone(),
                result: format!("command timed out after {}s", timeout.as_secs()),
                exit_code: -1,
                finished_at: format_rfc3339_now(),
            }
        }
    }
}

fn resolve_shell() -> (&'static str, &'static str) {
    #[cfg(windows)]
    {
        ("cmd", "/C")
    }
    #[cfg(not(windows))]
    {
        if Path::new("/bin/bash").exists() {
            ("/bin/bash", "-c")
        } else {
            ("/bin/sh", "-c")
        }
    }
}

async fn read_bounded<R: AsyncRead + Unpin>(
    reader: Option<R>,
    max_bytes: usize,
) -> (Vec<u8>, bool) {
    let Some(mut reader) = reader else {
        return (Vec::new(), false);
    };
    let mut collected = Vec::new();
    let mut chunk = [0u8; 4096];
    let mut truncated = false;

    loop {
        match reader.read(&mut chunk).await {
            Ok(0) => break,
            Ok(n) => {
                if collected.len() < max_bytes {
                    let remaining = max_bytes - collected.len();
                    let take = remaining.min(n);
                    collected.extend_from_slice(&chunk[..take]);
                    if take < n {
                        truncated = true;
                    }
                } else {
                    truncated = true;
                }
            }
            Err(_) => break,
        }
    }

    (collected, truncated)
}

fn combine_output(stdout: &[u8], stderr: &[u8], limit: usize) -> String {
    let mut out = String::from_utf8_lossy(stdout).into_owned();
    let err = String::from_utf8_lossy(stderr);
    if !err.is_empty() {
        out.push_str(&err);
    }
    if out.len() > limit {
        let mut cut = limit;
        while cut > 0 && !out.is_char_boundary(cut) {
            cut -= 1;
        }
        out.truncate(cut);
        out.push_str("\n[output truncated]");
    }
    out
}

fn format_rfc3339_now() -> String {
    OffsetDateTime::now_utc()
        .format(&Rfc3339)
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn execute_command_captures_output_and_exit_code() {
        let res = execute_command(
            &ExecRequest {
                task_id: "t-1".into(),
                command: "echo hello-openflare".into(),
                timeout_seconds: Some(5),
            },
            &ExecOptions::default(),
        )
        .await;
        assert_eq!(res.task_id, "t-1");
        assert_eq!(res.exit_code, 0);
        assert!(res.result.contains("hello-openflare"));
    }

    #[tokio::test]
    async fn execute_command_enforces_timeout() {
        let res = execute_command(
            &ExecRequest {
                task_id: "t-timeout".into(),
                command: "sleep 10".into(),
                timeout_seconds: Some(1),
            },
            &ExecOptions::default(),
        )
        .await;
        assert_eq!(res.exit_code, -1);
        assert!(res.result.contains("timed out"));
    }
}
