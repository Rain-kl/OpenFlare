use anyhow::{bail, ensure, Context, Result};
use flate2::read::GzDecoder;
use reqwest::Client;
use semver::Version;
use serde::Deserialize;
use sha2::{Digest, Sha256};
use std::fs;
use std::io::{self, ErrorKind, Read, Seek, Write};
use std::os::unix::process::CommandExt;
use std::path::Path;
use std::time::Duration;
use tempfile::NamedTempFile;
use tokio::sync::watch;
use url::Url;

use crate::{user_agent, TUNNEL_VERSION};

const MAX_RELEASE_LIST: usize = 16 * 1024 * 1024;
const MAX_PACKAGE: usize = 128 * 1024 * 1024;
const MAX_EXPANDED_PACKAGE: u64 = 256 * 1024 * 1024;
const MAX_MANIFEST: u64 = 4096;
const CHECK_INTERVAL: Duration = Duration::from_secs(6 * 60 * 60);
const MIN_RETRY: Duration = Duration::from_secs(60);
const MAX_RETRY: Duration = Duration::from_secs(3 * 60 * 60);

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct TunnelUpdatePlan {
    pub repo: String,
    pub tag: Option<String>,
    pub binary_name: String,
}

#[derive(Deserialize)]
struct Release {
    tag_name: String,
    draft: bool,
    prerelease: bool,
    assets: Vec<Asset>,
}

#[derive(Clone, Deserialize)]
struct Asset {
    name: String,
    digest: Option<String>,
}

pub fn supported() -> bool {
    cfg!(target_os = "linux")
        && matches!(std::env::consts::ARCH, "x86_64" | "aarch64")
        && unsafe { libc::geteuid() } != 0
        && std::env::var_os("NOTIFY_SOCKET").is_some()
        && std::env::var_os("INVOCATION_ID").is_some()
}

fn slot_paths(state_dir: &Path, binary_name: &str) -> (std::path::PathBuf, std::path::PathBuf, std::path::PathBuf) {
    (
        state_dir.join(format!("{binary_name}-next")),
        state_dir.join(format!("{binary_name}-trial")),
        state_dir.join(format!("{binary_name}-active")),
    )
}

fn running(path: &Path) -> bool {
    std::env::current_exe()
        .is_ok_and(|current| same_file::is_same_file(current, path).unwrap_or(false))
}

pub fn launch_staged_if_supported(state_dir: &Path, binary_name: &str) {
    if !supported() {
        return;
    }
    let (next, trial, active) = slot_paths(state_dir, binary_name);
    if running(&trial) || running(&active) {
        return;
    }
    match fs::rename(&next, &trial) {
        Ok(()) => {
            tracing::info!(binary = binary_name, "starting staged tunnel binary trial");
            exec_binary(&trial);
        }
        Err(err) if err.kind() == ErrorKind::NotFound => {}
        Err(err) => tracing::warn!(%err, "failed to move next tunnel binary to trial slot"),
    }
    if active.is_file() {
        exec_binary(&active);
    }
}

pub fn confirm_trial_if_running(state_dir: &Path, binary_name: &str) -> Result<()> {
    if !supported() {
        return Ok(());
    }
    let (_, trial, active) = slot_paths(state_dir, binary_name);
    if running(&trial) {
        fs::rename(&trial, &active).context("promote staged trial tunnel binary to active")?;
        tracing::info!(version = TUNNEL_VERSION, binary = binary_name, "promoted updated tunnel binary");
    }
    Ok(())
}

#[cfg(target_os = "linux")]
pub fn notify_systemd_ready(status: &str) -> Result<()> {
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
    let payload = format!("READY=1\nSTATUS={status}");
    UnixDatagram::unbound()?.send_to_addr(payload.as_bytes(), &addr)?;
    Ok(())
}

#[cfg(not(target_os = "linux"))]
pub fn notify_systemd_ready(_status: &str) -> Result<()> {
    Ok(())
}

fn exec_binary(path: &Path) {
    let err = std::process::Command::new(path)
        .args(std::env::args_os().skip(1))
        .exec();
    tracing::error!(%err, path = %path.display(), "exec staged tunnel binary failed");
}

pub fn spawn_update_loop(
    state_dir: std::path::PathBuf,
    binary_name: String,
    mut plans: watch::Receiver<Option<TunnelUpdatePlan>>,
    restart_notify: tokio::sync::mpsc::Sender<()>,
) {
    tokio::spawn(async move {
        if !supported() {
            return;
        }
        let mut retry = MIN_RETRY;
        loop {
            let plan = plans.borrow_and_update().clone();
            let Some(current) = plan else {
                if plans.changed().await.is_err() {
                    return;
                }
                continue;
            };
            let check = async {
                let client = github_client(&binary_name)?;
                download_and_stage(&client, &state_dir, &current).await
            };
            tokio::select! {
                _ = plans.changed() => {
                    retry = MIN_RETRY;
                    continue;
                }
                res = check => match res {
                    Ok(true) => {
                        tracing::info!(repo = %current.repo, tag = ?current.tag, "staged verified tunnel update; requesting restart");
                        let _ = restart_notify.send(()).await;
                        return;
                    }
                    Ok(false) => {
                        retry = MIN_RETRY;
                        tokio::time::sleep(CHECK_INTERVAL).await;
                    }
                    Err(err) => {
                        tracing::warn!(%err, retry_seconds = retry.as_secs(), "tunnel self-update check failed");
                        tokio::time::sleep(retry).await;
                        retry = (retry * 2).min(MAX_RETRY);
                    }
                }
            }
        }
    });
}

fn github_client(binary_name: &str) -> Result<Client> {
    Ok(Client::builder()
        .user_agent(user_agent(binary_name))
        .https_only(true)
        .connect_timeout(Duration::from_secs(10))
        .timeout(Duration::from_secs(180))
        .redirect(reqwest::redirect::Policy::custom(|attempt| {
            let allowed = matches!(
                attempt.url().host_str(),
                Some(
                    "api.github.com"
                        | "github.com"
                        | "release-assets.githubusercontent.com"
                        | "objects.githubusercontent.com"
                )
            );
            if allowed && attempt.url().scheme() == "https" && attempt.previous().len() < 5 {
                attempt.follow()
            } else {
                attempt.error("unexpected GitHub redirect")
            }
        }))
        .build()?)
}

async fn download_and_stage(
    client: &Client,
    state_dir: &Path,
    plan: &TunnelUpdatePlan,
) -> Result<bool> {
    let repo = validate_repo(&plan.repo)?;
    let mut url = Url::parse(&format!("https://api.github.com/repos/{repo}/releases"))?;
    let selection = if let Some(tag) = &plan.tag {
        let ver = parse_tunnel_tag(tag).context("invalid tunnel release tag")?;
        if !is_newer(&ver, TUNNEL_VERSION) {
            return Ok(false);
        }
        url.path_segments_mut().unwrap().extend(["tags", tag]);
        let release: Release = fetch_json(client, url).await?;
        ensure!(!release.draft && release.tag_name == *tag, "unexpected tunnel release");
        let asset = select_asset(&release, &plan.binary_name)
            .cloned()
            .context("tunnel release asset missing for host architecture")?;
        Some((ver, release.tag_name, asset))
    } else {
        url.query_pairs_mut().append_pair("per_page", "30");
        let releases: Vec<Release> = fetch_json(client, url).await?;
        releases
            .iter()
            .filter(|r| !r.draft && !r.prerelease)
            .filter_map(|r| {
                let ver = parse_tunnel_tag(&r.tag_name)?;
                if !ver.pre.is_empty() {
                    return None;
                }
                let asset = select_asset(r, &plan.binary_name)?;
                Some((ver, r.tag_name.clone(), asset.clone()))
            })
            .max_by(|a, b| a.0.cmp(&b.0))
            .filter(|(ver, _, _)| is_newer(ver, TUNNEL_VERSION))
    };

    let Some((ver, tag, asset)) = selection else {
        return Ok(false);
    };
    let expected_sha = asset
        .digest
        .as_deref()
        .and_then(|d| d.strip_prefix("sha256:"))
        .filter(|d| is_sha256(d))
        .map(|s| s.to_ascii_lowercase());

    let download_url = format!("https://github.com/{repo}/releases/download/{tag}/{}", asset.name);
    let mut resp = client.get(download_url).send().await?.error_for_status()?;
    let mut package = tempfile::tempfile_in(state_dir)?;
    let mut hasher = Sha256::new();
    let mut total = 0usize;
    while let Some(chunk) = resp.chunk().await? {
        total += chunk.len();
        ensure!(total <= MAX_PACKAGE, "tunnel package exceeds download limit");
        hasher.update(&chunk);
        package.write_all(&chunk)?;
    }
    if let Some(expected) = expected_sha {
        let actual = format!("{:x}", hasher.finalize());
        ensure!(actual == expected, "tunnel release asset checksum mismatch");
    }

    let state_clone = state_dir.to_owned();
    let asset_name = asset.name.clone();
    let bin_name = plan.binary_name.clone();
    let candidate = tokio::task::spawn_blocking(move || {
        package.rewind()?;
        extract_binary(package, &state_clone, &asset_name, &bin_name)
    })
    .await??;

    validate_executable(candidate.path(), &plan.binary_name, &ver.to_string()).await?;
    let (next_path, _, _) = slot_paths(state_dir, &plan.binary_name);
    candidate
        .persist(&next_path)
        .map_err(|e| e.error)
        .context("persist verified staged binary")?;
    Ok(true)
}

fn validate_repo(repo: &str) -> Result<&str> {
    let (owner, name) = repo.split_once('/').context("invalid update repository")?;
    ensure!(!owner.is_empty() && !name.is_empty(), "invalid update repository");
    Ok(repo)
}

async fn fetch_json<T: serde::de::DeserializeOwned>(client: &Client, url: Url) -> Result<T> {
    let mut resp = client
        .get(url)
        .header(reqwest::header::ACCEPT, "application/vnd.github+json")
        .send()
        .await?
        .error_for_status()?;
    let mut body = Vec::new();
    while let Some(chunk) = resp.chunk().await? {
        ensure!(body.len() + chunk.len() <= MAX_RELEASE_LIST, "release list too large");
        body.extend_from_slice(&chunk);
    }
    Ok(serde_json::from_slice(&body)?)
}

pub fn parse_tunnel_tag(tag: &str) -> Option<Version> {
    if tag.len() > 64 || tag.contains('+') {
        return None;
    }
    Version::parse(tag.strip_prefix("tunnel-v")?).ok()
}

pub fn is_newer(target: &Version, current: &str) -> bool {
    let stripped = current
        .strip_prefix("tunnel-v")
        .or_else(|| current.strip_prefix('v'))
        .unwrap_or(current);
    stripped == "dev" || Version::parse(stripped).is_ok_and(|cur| target.cmp_precedence(&cur).is_gt())
}

fn select_asset<'a>(release: &'a Release, binary_name: &str) -> Option<&'a Asset> {
    let arch = match std::env::consts::ARCH {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        _ => return None,
    };
    let specific = format!("{binary_name}_{}_linux_{arch}.tar.gz", release.tag_name);
    let combined = format!("openflare-tunnel_{}_linux_{arch}.tar.gz", release.tag_name);
    release
        .assets
        .iter()
        .find(|a| a.name == specific || a.name == combined)
}

fn is_sha256(value: &str) -> bool {
    value.len() == 64 && value.bytes().all(|b| b.is_ascii_hexdigit())
}

fn extract_binary(package: impl Read, state_dir: &Path, asset_name: &str, binary_name: &str) -> Result<NamedTempFile> {
    let prefix = Path::new(asset_name.trim_end_matches(".tar.gz"));
    let target_exec = prefix.join(binary_name);
    let target_manifest = prefix.join("SHA256SUMS");
    let mut archive = tar::Archive::new(GzDecoder::new(package).take(MAX_EXPANDED_PACKAGE));
    let mut candidate = NamedTempFile::new_in(state_dir)?;
    let mut seen_exec = false;
    let mut checksum = None;
    let mut hasher = Sha256::new();

    for entry in archive.entries()? {
        let mut entry = entry?;
        let path = entry.path()?.into_owned();
        if path == target_exec || path == Path::new(binary_name) {
            ensure!(!seen_exec && entry.header().entry_type().is_file(), "invalid binary entry");
            io::copy(&mut entry, &mut candidate)?;
            candidate.as_file_mut().rewind()?;
            io::copy(candidate.as_file_mut(), &mut hasher)?;
            seen_exec = true;
        } else if path == target_manifest || path == Path::new("SHA256SUMS") {
            ensure!(checksum.is_none() && entry.size() <= MAX_MANIFEST, "invalid SHA256SUMS");
            let mut text = String::new();
            entry.read_to_string(&mut text)?;
            checksum = text.lines().find_map(|line| {
                let (sum, name) = line.split_once(' ')?;
                (name.trim_start().trim_start_matches('*') == binary_name && is_sha256(sum))
                    .then(|| sum.to_ascii_lowercase())
            });
        }
    }
    ensure!(seen_exec, "release tarball missing executable {binary_name}");
    if let Some(expected) = checksum {
        ensure!(expected == format!("{:x}", hasher.finalize()), "checksum mismatch against SHA256SUMS");
    } else {
        bail!("missing checksum for {binary_name} in SHA256SUMS");
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        candidate.as_file().set_permissions(fs::Permissions::from_mode(0o700))?;
    }
    candidate.as_file().sync_all()?;
    let (file, path) = candidate.into_parts();
    drop(file);
    let ro = fs::File::open(&path)?;
    Ok(NamedTempFile::from_parts(ro, path))
}

async fn validate_executable(path: &Path, binary_name: &str, expected_ver: &str) -> Result<()> {
    let out = tokio::time::timeout(
        Duration::from_secs(10),
        tokio::process::Command::new(path)
            .arg("--version")
            .env_remove("NOTIFY_SOCKET")
            .kill_on_drop(true)
            .output(),
    )
    .await
    .context("staged binary version check timed out")??;
    let stdout = String::from_utf8_lossy(&out.stdout);
    ensure!(
        out.status.success() && stdout.contains(binary_name) && stdout.contains(expected_ver),
        "staged binary failed --version validation"
    );
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_tunnel_tags_and_compares_versions() {
        let v = parse_tunnel_tag("tunnel-v1.2.0").expect("parse");
        assert!(is_newer(&v, "1.1.9"));
        assert!(is_newer(&v, "dev"));
        assert!(!is_newer(&v, "1.2.0"));
        assert!(parse_tunnel_tag("edge-v1.2.0").is_none());
    }
}
