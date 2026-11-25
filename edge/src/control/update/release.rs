use crate::{edge_user_agent, EDGE_VERSION};
use anyhow::{bail, ensure, Context, Result};
use flate2::read::GzDecoder;
use reqwest::Client;
use semver::Version;
use serde::Deserialize;
use sha2::{Digest, Sha256};
use std::io::{self, Read, Seek, Write};
use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use std::time::Duration;
use tempfile::NamedTempFile;
use url::Url;

use super::UpdatePlan;

const MAX_RELEASE_LIST: usize = 16 * 1024 * 1024;
const RELEASE_PAGE_SIZE: usize = 100;
const MAX_PACKAGE: usize = 128 * 1024 * 1024;
const MAX_EXPANDED_PACKAGE: u64 = 256 * 1024 * 1024;
const MAX_MANIFEST: u64 = 4096;

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

pub(super) fn client() -> Result<Client> {
    Ok(Client::builder()
        .user_agent(edge_user_agent())
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
                attempt.error("unexpected GitHub release redirect")
            }
        }))
        .build()?)
}

pub(super) async fn download(
    client: &Client,
    state_dir: &Path,
    plan: &UpdatePlan,
) -> Result<Option<NamedTempFile>> {
    let repo = validate_repo(&plan.repo)?;
    let mut url = Url::parse(&format!("https://api.github.com/repos/{repo}/releases"))?;
    let selection = if let Some(tag) = &plan.tag {
        let version = parse_tag(tag).context("invalid requested Edge tag")?;
        if !is_newer(&version, EDGE_VERSION) {
            return Ok(None);
        }
        url.path_segments_mut().unwrap().extend(["tags", tag]);
        let release: Release = metadata(client, url).await?;
        ensure!(
            !release.draft && release.tag_name == *tag,
            "unexpected Edge release tag"
        );
        let asset = asset_for(&release)
            .context("Edge release package is unavailable for this architecture")?;
        Some((version, asset.clone()))
    } else {
        find_stable_update(client, url).await?
    };
    let Some((
        version,
        Asset {
            name: asset_name,
            digest,
        },
    )) = selection
    else {
        return Ok(None);
    };
    let tag = format!("edge-v{version}");
    let digest = digest
        .as_deref()
        .and_then(|digest| digest.strip_prefix("sha256:"))
        .filter(|digest| is_sha256(digest))
        .context("Edge release package has no SHA-256 digest")?;
    let url = format!("https://github.com/{repo}/releases/download/{tag}/{asset_name}");
    let mut response = client
        .get(url)
        .send()
        .await
        .and_then(reqwest::Response::error_for_status)
        .map_err(reqwest::Error::without_url)
        .context("could not download Edge release package")?;
    let mut package = tempfile::tempfile_in(state_dir)?;
    let mut hasher = Sha256::new();
    let mut size = 0;
    while let Some(chunk) = response
        .chunk()
        .await
        .map_err(reqwest::Error::without_url)?
    {
        size += chunk.len();
        ensure!(
            size <= MAX_PACKAGE,
            "Edge release package exceeds the download limit"
        );
        hasher.update(&chunk);
        package.write_all(&chunk)?;
    }
    ensure!(
        digest.eq_ignore_ascii_case(&format!("{:x}", hasher.finalize())),
        "Edge release package checksum mismatch"
    );

    let state_dir = state_dir.to_owned();
    let candidate = tokio::task::spawn_blocking(move || {
        package.rewind()?;
        extract(package, &state_dir, &asset_name)
    })
    .await??;
    validate_executable(candidate.path(), &version.to_string()).await?;
    Ok(Some(candidate))
}

async fn find_stable_update(client: &Client, mut base: Url) -> Result<Option<(Version, Asset)>> {
    base.query_pairs_mut()
        .append_pair("per_page", &RELEASE_PAGE_SIZE.to_string());
    for page in 1_u64.. {
        let mut url = base.clone();
        if page > 1 {
            url.query_pairs_mut().append_pair("page", &page.to_string());
        }
        let releases: Vec<Release> = metadata(client, url).await?;
        if let Some((version, asset)) = latest_stable(&releases) {
            return Ok(is_newer(&version, EDGE_VERSION).then(|| (version, asset.clone())));
        }
        if releases.len() < RELEASE_PAGE_SIZE {
            break;
        }
    }
    Ok(None)
}

async fn metadata<T: serde::de::DeserializeOwned>(client: &Client, url: Url) -> Result<T> {
    let mut response = client
        .get(url)
        .header(reqwest::header::ACCEPT, "application/vnd.github+json")
        .timeout(Duration::from_secs(15))
        .send()
        .await?
        .error_for_status()?;
    let mut body = Vec::new();
    while let Some(chunk) = response.chunk().await? {
        ensure!(
            body.len() + chunk.len() <= MAX_RELEASE_LIST,
            "Edge release metadata exceeds the size limit"
        );
        body.extend_from_slice(&chunk);
    }
    Ok(serde_json::from_slice(&body)?)
}

fn validate_repo(repo: &str) -> Result<&str> {
    let (owner, name) = repo
        .split_once('/')
        .context("invalid Edge release repository")?;
    let component = |value: &str, allow_dot| {
        !value.is_empty()
            && !matches!(value, "." | "..")
            && value.bytes().all(|c| {
                c.is_ascii_alphanumeric() || matches!(c, b'-' | b'_') || allow_dot && c == b'.'
            })
    };
    ensure!(
        component(owner, false) && component(name, true),
        "invalid Edge release repository"
    );
    Ok(repo)
}

fn parse_tag(tag: &str) -> Option<Version> {
    if tag.len() > 64 || tag.contains('+') {
        return None;
    }
    Version::parse(tag.strip_prefix("edge-v")?).ok()
}

fn is_newer(target: &Version, current: &str) -> bool {
    let current = current
        .strip_prefix("edge-v")
        .or_else(|| current.strip_prefix('v'))
        .unwrap_or(current);
    Version::parse(current).is_ok_and(|current| target.cmp_precedence(&current).is_gt())
        || current == "dev"
}

fn asset_for(release: &Release) -> Option<&Asset> {
    let arch = match std::env::consts::ARCH {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        _ => return None,
    };
    let expected = format!("openflare-edge_{}_linux_{arch}.tar.gz", release.tag_name);
    release.assets.iter().find(|asset| asset.name == expected)
}

fn latest_stable(releases: &[Release]) -> Option<(Version, &Asset)> {
    releases
        .iter()
        .filter(|release| !release.draft && !release.prerelease)
        .filter_map(|release| {
            let version = parse_tag(&release.tag_name)?;
            if !version.pre.is_empty() {
                return None;
            }
            Some((version, asset_for(release)?))
        })
        .max_by(|(left, _), (right, _)| left.cmp(right))
}

fn is_sha256(value: &str) -> bool {
    value.len() == 64 && value.bytes().all(|b| b.is_ascii_hexdigit())
}

fn extract(package: impl Read, state_dir: &Path, asset_name: &str) -> Result<NamedTempFile> {
    let prefix = Path::new(asset_name.trim_end_matches(".tar.gz"));
    let executable = prefix.join("openflare-edge");
    let manifest = prefix.join("SHA256SUMS");
    let mut archive = tar::Archive::new(GzDecoder::new(package).take(MAX_EXPANDED_PACKAGE));
    let mut candidate = NamedTempFile::new_in(state_dir)?;
    let mut seen_executable = false;
    let mut checksum = None;
    let mut hasher = Sha256::new();
    for entry in archive.entries()? {
        let mut entry = entry?;
        let path = entry.path()?;
        if path == executable {
            ensure!(
                !seen_executable && entry.header().entry_type().is_file(),
                "invalid Edge executable entry"
            );
            io::copy(&mut entry, &mut candidate)?;
            candidate.as_file_mut().rewind()?;
            io::copy(candidate.as_file_mut(), &mut hasher)?;
            seen_executable = true;
        } else if path == manifest {
            ensure!(
                checksum.is_none()
                    && entry.header().entry_type().is_file()
                    && entry.size() <= MAX_MANIFEST,
                "invalid Edge checksum manifest"
            );
            let mut contents = String::new();
            entry.read_to_string(&mut contents)?;
            checksum = contents.lines().find_map(|line| {
                let (sum, name) = line.split_once(' ')?;
                (name.trim_start().trim_start_matches('*') == "openflare-edge" && is_sha256(sum))
                    .then(|| sum.to_ascii_lowercase())
            });
            ensure!(checksum.is_some(), "missing Edge executable checksum");
        }
    }
    ensure!(
        seen_executable && checksum.as_deref() == Some(&format!("{:x}", hasher.finalize())),
        "Edge executable checksum mismatch"
    );
    candidate
        .as_file()
        .set_permissions(std::fs::Permissions::from_mode(0o700))?;
    candidate.as_file().sync_all()?;
    // Reopen read-only before execution to avoid ETXTBSY on Linux.
    let (file, path) = candidate.into_parts();
    drop(file);
    let file = std::fs::File::open(&path)?;
    Ok(NamedTempFile::from_parts(file, path))
}

async fn validate_executable(path: &Path, version: &str) -> Result<()> {
    for (arg, expected) in [
        ("--version", format!("openflare-edge {version}")),
        (
            "config-sync-version",
            crate::control::config_sync::VERSION.to_string(),
        ),
        ("self-update-version", super::VERSION.to_string()),
    ] {
        let output = tokio::time::timeout(
            Duration::from_secs(10),
            tokio::process::Command::new(path)
                .arg(arg)
                .env_remove("NOTIFY_SOCKET")
                .kill_on_drop(true)
                .output(),
        )
        .await
        .context("staged Edge binary compatibility check timed out")?
        .context("could not execute staged Edge binary")?;
        if !output.status.success() || String::from_utf8_lossy(&output.stdout).trim() != expected {
            bail!("Edge executable failed the {arg} compatibility check");
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use flate2::{write::GzEncoder, Compression};

    fn asset_name(tag: &str) -> String {
        let arch = if std::env::consts::ARCH == "aarch64" {
            "arm64"
        } else {
            "amd64"
        };
        format!("openflare-edge_{tag}_linux_{arch}.tar.gz")
    }

    fn release(tag: &str, prerelease: bool) -> Release {
        Release {
            tag_name: tag.to_owned(),
            draft: false,
            prerelease,
            assets: vec![Asset {
                name: asset_name(tag),
                digest: None,
            }],
        }
    }

    #[test]
    fn selects_a_stable_edge_release_for_this_architecture() {
        let mut draft = release("edge-v3.1.0", false);
        draft.draft = true;
        let mut missing_asset = release("edge-v3.0.0", false);
        missing_asset.assets.clear();
        let releases = [
            release("edge-v1.8.9", false),
            release("v99.0.0", false),
            release("edge-v2.0.0-rc.1", true),
            release("edge-v2.0.0-rc.2", false),
            missing_asset,
            draft,
            release("edge-v1.10.0", false),
        ];
        let (version, _) = latest_stable(&releases).unwrap();
        assert_eq!(version.to_string(), "1.10.0");
        assert!(is_newer(&version, "1.10.0-rc.1"));
        assert!(!is_newer(&version, "1.11.0"));
        assert!(!is_newer(&version, "1.10.0"));
    }

    #[test]
    fn stages_only_a_binary_matching_the_package_manifest() {
        let tag = "edge-v1.0.0";
        let asset = asset_name(tag);
        let dir = tempfile::tempdir().unwrap();
        let binary = b"#!/bin/sh\necho fixture\n";
        let valid_sum = format!("{:x}", Sha256::digest(binary));
        for (sum, valid) in [(valid_sum, true), ("0".repeat(64), false)] {
            let manifest = format!("{sum}  openflare-edge\n");
            let encoder = GzEncoder::new(Vec::new(), Compression::default());
            let mut archive = tar::Builder::new(encoder);
            let prefix = asset.trim_end_matches(".tar.gz");
            for (name, bytes) in [
                ("openflare-edge", binary.as_slice()),
                ("SHA256SUMS", manifest.as_bytes()),
            ] {
                let mut header = tar::Header::new_gnu();
                header.set_size(bytes.len() as u64);
                header.set_mode(0o600);
                archive
                    .append_data(&mut header, format!("{prefix}/{name}"), bytes)
                    .unwrap();
            }
            let archive = archive.into_inner().unwrap().finish().unwrap();
            let result = extract(archive.as_slice(), dir.path(), &asset);
            if valid {
                let candidate = result.unwrap();
                assert_eq!(std::fs::read(candidate.path()).unwrap(), binary);
                assert_eq!(
                    candidate.as_file().metadata().unwrap().permissions().mode() & 0o777,
                    0o700
                );
            } else {
                assert!(result
                    .unwrap_err()
                    .to_string()
                    .contains("checksum mismatch"));
            }
        }
        assert_eq!(std::fs::read_dir(dir.path()).unwrap().count(), 0);
    }
}
