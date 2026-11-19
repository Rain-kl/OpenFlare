use crate::config::PagesDeployment;
use anyhow::{bail, Context, Result};
use reqwest::{Client, StatusCode};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};
use zip::ZipArchive;

const MAX_PACKAGE_BYTES: u64 = 2 * 1024 * 1024 * 1024;
const MAX_FILES: usize = 1000;
const MAX_FILE_BYTES: u64 = 8 * 1024 * 1024 * 1024;
const MAX_TOTAL_BYTES: u64 = 8 * 1024 * 1024 * 1024;

#[derive(Debug, Clone, Deserialize)]
struct ApiResponse<T> {
    #[serde(default)]
    error_msg: String,
    data: Option<T>,
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq, Eq)]
pub struct PagesLatest {
    pub project_id: u64,
    pub deployment_id: u64,
    pub hash: String,
    pub package_size: i64,
    pub file_count: i64,
    pub total_size: i64,
}

pub struct PagesSyncer {
    base_url: String,
    token: String,
    client: Client,
    pages_dir: PathBuf,
}

impl PagesSyncer {
    pub fn new(base_url: String, token: String, client: Client, pages_dir: PathBuf) -> Self {
        Self {
            base_url,
            token,
            client,
            pages_dir,
        }
    }

    pub async fn ensure_project(&self, deployment: &PagesDeployment) -> Result<()> {
        let latest = self.fetch_latest(deployment.project_id).await?;
        validate_latest(deployment.project_id, &latest)?;
        let hash = latest.hash.trim().to_owned();
        let release = self.release_dir(deployment.project_id, &hash);
        if release_ready(&release, &latest) {
            self.switch_current(deployment.project_id, &release)?;
            return cleanup_stale_releases(&release, &hash);
        }

        let package = self
            .download_package(deployment.project_id, &latest)
            .await?;
        let verify = self.fetch_latest(deployment.project_id).await?;
        validate_latest(deployment.project_id, &verify)?;
        if latest != verify {
            let _ = fs::remove_file(&package);
            bail!("Pages latest metadata changed during download");
        }
        let result = self.prepare_release(package.as_path(), &release, &latest, deployment);
        let _ = fs::remove_file(&package);
        result?;
        self.switch_current(deployment.project_id, &release)?;
        cleanup_stale_releases(&release, &hash)
    }

    fn prepare_release(
        &self,
        package: &Path,
        release: &Path,
        latest: &PagesLatest,
        deployment: &PagesDeployment,
    ) -> Result<()> {
        let staging = self.staging_dir(release)?;
        let extraction = extract_package(package, &staging, latest);
        if let Err(error) = extraction {
            let _ = fs::remove_dir_all(&staging);
            return Err(error);
        }
        if !staging.join(&deployment.entry_file).is_file() {
            let _ = fs::remove_dir_all(&staging);
            bail!("Pages package has no entry file {}", deployment.entry_file);
        }
        write_marker(&staging, latest)?;
        if release.exists() {
            let _ = fs::remove_dir_all(release);
        }
        fs::rename(&staging, release).with_context(|| {
            format!(
                "promote Pages release {} to {}",
                staging.display(),
                release.display()
            )
        })
    }

    async fn fetch_latest(&self, project_id: u64) -> Result<PagesLatest> {
        let response = self
            .client
            .get(format!(
                "{}/api/v1/agent/pages/projects/{project_id}/latest/hash",
                self.base_url
            ))
            .header("X-Agent-Token", &self.token)
            .send()
            .await?;
        decode_json(response).await
    }

    async fn download_package(&self, project_id: u64, latest: &PagesLatest) -> Result<PathBuf> {
        let response = self
            .client
            .get(format!(
                "{}/api/v1/agent/pages/projects/{project_id}/latest/package",
                self.base_url
            ))
            .header("X-Agent-Token", &self.token)
            .send()
            .await?;
        if response.status() != StatusCode::OK {
            bail!("Pages package download returned HTTP {}", response.status());
        }
        let releases = self
            .pages_dir
            .join("projects")
            .join(project_id.to_string())
            .join("releases");
        fs::create_dir_all(&releases)?;
        let package = releases.join(format!(".package-{}.tmp", unix_now()));
        let result = stream_package(response, &package, latest).await;
        if result.is_err() {
            let _ = fs::remove_file(&package);
        }
        result?;
        Ok(package)
    }

    fn release_dir(&self, project_id: u64, hash: &str) -> PathBuf {
        self.pages_dir
            .join("projects")
            .join(project_id.to_string())
            .join("releases")
            .join(hash)
    }

    fn staging_dir(&self, release: &Path) -> Result<PathBuf> {
        let parent = release.parent().context("release has a parent")?;
        fs::create_dir_all(parent)?;
        let prefix = format!(
            ".{}-",
            release
                .file_name()
                .and_then(|value| value.to_str())
                .unwrap_or("release")
        );
        let staging = parent.join(format!("{prefix}{}.tmp", unix_now()));
        if staging.exists() {
            fs::remove_dir_all(&staging)?;
        }
        fs::create_dir_all(&staging)?;
        Ok(staging)
    }

    fn switch_current(&self, project_id: u64, release: &Path) -> Result<()> {
        let project = self.pages_dir.join("projects").join(project_id.to_string());
        let current = project.join("current");
        fs::create_dir_all(&project)?;
        let temporary = project.join(".current.tmp");
        let _ = fs::remove_file(&temporary);
        let target = path_relative(&project, release);
        #[cfg(unix)]
        std::os::unix::fs::symlink(&target, &temporary)?;
        #[cfg(not(unix))]
        bail!("atomic Pages current switching requires Unix symlink support");
        let verification = verify_current(&temporary, release);
        if verification.is_err() {
            let _ = fs::remove_file(&temporary);
            return verification;
        }
        let previous = fs::read_link(&current).ok();
        if let Err(error) = fs::rename(&temporary, &current) {
            let _ = fs::remove_file(&temporary);
            return Err(error)
                .with_context(|| format!("switch Pages current to {}", release.display()));
        }
        if let Err(error) = verify_current(&current, release) {
            if let Err(rollback_error) = rollback_current(&current, previous.as_deref(), &project) {
                return Err(rollback_error).context(error);
            }
            return Err(error);
        }
        Ok(())
    }
}

async fn decode_json<T: for<'de> Deserialize<'de>>(response: reqwest::Response) -> Result<T> {
    let status = response.status();
    let body = response.text().await?;
    if status != StatusCode::OK {
        bail!("server returned HTTP {status}: {body}");
    }
    let decoded: ApiResponse<T> = serde_json::from_str(&body)?;
    if !decoded.error_msg.is_empty() {
        bail!("server returned error: {}", decoded.error_msg);
    }
    decoded
        .data
        .ok_or_else(|| anyhow::anyhow!("server response has no data"))
}

fn validate_latest(project_id: u64, latest: &PagesLatest) -> Result<()> {
    if latest.project_id != project_id
        || latest.deployment_id == 0
        || latest.hash.trim().is_empty()
        || latest.package_size <= 0
        || latest.file_count <= 0
        || latest.total_size < 0
        || latest.package_size as u64 > MAX_PACKAGE_BYTES
        || latest.file_count as usize > MAX_FILES
        || latest.total_size as u64 > MAX_TOTAL_BYTES
    {
        bail!("invalid Pages latest metadata");
    }
    Ok(())
}

async fn stream_package(
    response: reqwest::Response,
    package: &Path,
    latest: &PagesLatest,
) -> Result<()> {
    let mut file = File::create(package)?;
    let mut hasher = Sha256::new();
    let mut written = 0u64;
    let mut stream = response;
    while let Some(chunk) = stream.chunk().await? {
        if chunk.len() as u64 > MAX_PACKAGE_BYTES.saturating_sub(written) {
            bail!("Pages package exceeds the 2 GiB limit");
        }
        file.write_all(&chunk)?;
        hasher.update(&chunk);
        written += chunk.len() as u64;
    }
    file.flush()?;
    if written != latest.package_size.unsigned_abs() {
        bail!("Pages package size does not match metadata");
    }
    let digest = format!("{:x}", hasher.finalize());
    if digest != latest.hash.trim() {
        bail!("Pages package SHA-256 mismatch");
    }
    Ok(())
}

fn release_ready(release: &Path, latest: &PagesLatest) -> bool {
    let Ok(marker) = fs::read_to_string(release.join(".openflare-pages.json")) else {
        return false;
    };
    let Ok(marker) = serde_json::from_str::<PagesLatest>(&marker) else {
        return false;
    };
    marker == *latest
        && fs::read_dir(release).is_ok_and(|mut entries| {
            entries
                .any(|entry| entry.is_ok_and(|entry| entry.file_name() != ".openflare-pages.json"))
        })
}

fn write_marker(release: &Path, latest: &PagesLatest) -> Result<()> {
    fs::write(
        release.join(".openflare-pages.json"),
        serde_json::to_vec(latest)?,
    )?;
    Ok(())
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ArchiveFormat {
    Zip,
    Tar,
    TarGz,
    TarXz,
    TarBz2,
    SevenZip,
}

fn detect_format(package: &Path) -> Result<ArchiveFormat> {
    let mut file = File::open(package)?;
    let mut head = vec![0u8; 512];
    let read = file.read(&mut head)?;
    head.truncate(read);
    if head.starts_with(b"PK\x03\x04") {
        return Ok(ArchiveFormat::Zip);
    }
    if head.starts_with(b"7z\xbc\xaf\x27\x1c") {
        return Ok(ArchiveFormat::SevenZip);
    }
    if head.starts_with(b"\xfd7zXZ\x00") {
        return Ok(ArchiveFormat::TarXz);
    }
    if head.starts_with(b"\x1f\x8b") {
        return Ok(ArchiveFormat::TarGz);
    }
    if head.starts_with(b"BZh") {
        return Ok(ArchiveFormat::TarBz2);
    }
    if head.len() >= 262 && &head[257..262] == b"ustar" {
        return Ok(ArchiveFormat::Tar);
    }
    bail!("unsupported Pages package format")
}

fn extract_package(package: &Path, destination: &Path, latest: &PagesLatest) -> Result<()> {
    match detect_format(package)? {
        ArchiveFormat::Zip => extract_zip(package, destination, latest),
        ArchiveFormat::Tar => extract_tar(File::open(package)?, destination, latest),
        ArchiveFormat::TarGz => extract_tar(
            flate2::read::GzDecoder::new(File::open(package)?),
            destination,
            latest,
        ),
        ArchiveFormat::TarXz => extract_tar(
            xz2::read::XzDecoder::new(File::open(package)?),
            destination,
            latest,
        ),
        ArchiveFormat::TarBz2 => extract_tar(
            bzip2::read::BzDecoder::new(File::open(package)?),
            destination,
            latest,
        ),
        ArchiveFormat::SevenZip => extract_7z(package, destination, latest),
    }
}

fn extract_zip(package: &Path, destination: &Path, latest: &PagesLatest) -> Result<()> {
    let file = File::open(package)?;
    let mut archive = ZipArchive::new(file)?;
    let mut names = Vec::new();
    for index in 0..archive.len() {
        let entry = archive.by_index(index)?;
        if !entry.is_dir() {
            names.push(entry.name().to_owned());
        }
    }
    let metadata_file_count = latest.file_count.unsigned_abs() as usize;
    if names.len() != metadata_file_count {
        bail!(
            "Pages archive file count {} does not match metadata {}",
            names.len(),
            metadata_file_count
        );
    }
    let common_root = common_root(&names)?;
    let mut measured = MeasuredFiles::default();
    for index in 0..archive.len() {
        let mut entry = archive.by_index(index)?;
        if entry.is_dir() {
            continue;
        }
        let name = entry.name().to_owned();
        let relative = strip_common_root(&name, &common_root)?;
        if let Some(parent) = Path::new(&relative).parent() {
            fs::create_dir_all(destination.join(parent))?;
        }
        let mut output = File::create(destination.join(&relative))?;
        let written = copy_limited(&mut entry, &mut output, latest)?;
        measured.record(&relative, written)?;
    }
    measured.finish(latest)
}

fn extract_tar<R: Read>(reader: R, destination: &Path, latest: &PagesLatest) -> Result<()> {
    let mut archive = tar::Archive::new(reader);
    let mut measured = MeasuredFiles::default();
    for entry in archive.entries()? {
        let mut entry = entry?;
        if entry.header().entry_type().is_dir() {
            continue;
        }
        if !entry.header().entry_type().is_file() {
            bail!("Pages tar archive contains a non-regular file");
        }
        let name = entry
            .path()?
            .to_string_lossy()
            .trim_start_matches('/')
            .to_owned();
        let common_root = common_root(std::slice::from_ref(&name))?;
        let relative = strip_common_root(&name, &common_root)?;
        if let Some(parent) = Path::new(&relative).parent() {
            fs::create_dir_all(destination.join(parent))?;
        }
        let mut output = File::create(destination.join(&relative))?;
        let written = copy_limited(&mut entry, &mut output, latest)?;
        measured.record(&relative, written)?;
    }
    measured.finish(latest)
}

fn extract_7z(package: &Path, destination: &Path, latest: &PagesLatest) -> Result<()> {
    let mut reader = sevenz_rust::SevenZReader::open(package, sevenz_rust::Password::empty())
        .map_err(|error| anyhow::anyhow!("open 7z Pages package: {error}"))?;
    let mut measured = MeasuredFiles::default();
    let mut names = Vec::new();
    let archive = reader.archive().clone();
    for entry in archive.files {
        names.push(entry.name);
    }
    let common_root = common_root(&names)?;
    reader
        .for_each_entries(|entry, input| {
            let result: anyhow::Result<bool> = (|| {
                if entry.is_directory() || entry.is_anti_item {
                    return Ok(true);
                }
                let relative = strip_common_root(&entry.name, &common_root)?;
                if let Some(parent) = Path::new(&relative).parent() {
                    if let Err(error) = fs::create_dir_all(destination.join(parent)) {
                        return Err(error.into());
                    }
                }
                let mut output = File::create(destination.join(&relative))?;
                let written = copy_limited(input, &mut output, latest)?;
                measured.record(&relative, written)?;
                Ok(true)
            })();
            result.map_err(|error| sevenz_rust::Error::other(error.to_string()))
        })
        .map_err(|error| anyhow::anyhow!("extract 7z Pages package: {error}"))?;
    measured.finish(latest)
}

#[derive(Default)]
struct MeasuredFiles {
    count: usize,
    total: u64,
}

impl MeasuredFiles {
    fn record(&mut self, relative: &str, written: u64) -> Result<()> {
        if written > MAX_FILE_BYTES || self.total.saturating_add(written) > MAX_TOTAL_BYTES {
            bail!("Pages archive exceeds byte limits");
        }
        self.count += 1;
        self.total += written;
        if self.count > MAX_FILES {
            bail!("Pages archive exceeds file count limit");
        }
        if relative == ".openflare-pages.json" {
            bail!("Pages archive contains the reserved marker file");
        }
        Ok(())
    }

    fn finish(&self, latest: &PagesLatest) -> Result<()> {
        if self.count as i64 != latest.file_count || self.total as i64 != latest.total_size {
            bail!("Pages archive metadata does not match extracted files");
        }
        Ok(())
    }
}

fn copy_limited<R: Read + ?Sized, W: Write>(
    source: &mut R,
    output: &mut W,
    latest: &PagesLatest,
) -> Result<u64> {
    let mut buffer = vec![0u8; 64 * 1024];
    let mut written = 0u64;
    loop {
        let read = source.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        written += read as u64;
        if written > MAX_FILE_BYTES
            || written > latest.total_size.unsigned_abs()
            || written > latest.file_count.unsigned_abs() * MAX_FILE_BYTES
        {
            bail!("Pages archive file exceeds size limits");
        }
        output.write_all(&buffer[..read])?;
    }
    if written > latest.total_size.unsigned_abs() {
        bail!("Pages archive file exceeds total size");
    }
    Ok(written)
}

fn common_root(names: &[String]) -> Result<String> {
    let Some(first) = names.first().and_then(|name| name.split('/').next()) else {
        return Ok(String::new());
    };
    if first.is_empty()
        || !names
            .iter()
            .all(|name| name.split('/').next() == Some(first))
    {
        return Ok(String::new());
    }
    Ok(format!("{first}/"))
}

fn strip_common_root(name: &str, common_root: &str) -> Result<String> {
    let value = if common_root.is_empty() {
        name.trim_start_matches('/')
    } else {
        name.strip_prefix(common_root)
            .unwrap_or("")
            .trim_start_matches('/')
    };
    let relative = safe_relative(value)?;
    if relative.is_empty() {
        bail!("Pages archive contains an empty file path");
    }
    Ok(relative.to_owned())
}

fn safe_relative(value: &str) -> Result<&str> {
    if value.contains('\\') || value.contains('\0') || value.starts_with('/') {
        bail!("unsafe Pages archive path");
    }
    for segment in value.split('/') {
        if segment == ".." || segment.is_empty() && value.len() > 1 {
            bail!("unsafe Pages archive path");
        }
    }
    Ok(value)
}

fn path_relative(base: &Path, target: &Path) -> PathBuf {
    let base_components: Vec<_> = base.components().collect();
    let target_components: Vec<_> = target.components().collect();
    let mut index = 0;
    while index < base_components.len()
        && index < target_components.len()
        && base_components[index] == target_components[index]
    {
        index += 1;
    }
    let mut result = PathBuf::new();
    for _ in index..base_components.len() {
        result.push("..");
    }
    for component in &target_components[index..] {
        result.push(component.as_os_str());
    }
    if result.as_os_str().is_empty() {
        result.push(target);
    }
    result
}

fn verify_current(current: &Path, release: &Path) -> Result<()> {
    if !same_file::is_same_file(current, release)
        .with_context(|| format!("verify Pages current {}", current.display()))?
    {
        bail!(
            "Pages current {} does not resolve to release {}",
            current.display(),
            release.display()
        );
    }
    Ok(())
}

fn rollback_current(current: &Path, previous: Option<&Path>, base: &Path) -> Result<()> {
    let _ = fs::remove_file(current);
    if let Some(previous) = previous {
        let rollback = base.join(".current.rollback.tmp");
        let _ = fs::remove_file(&rollback);
        std::os::unix::fs::symlink(previous, &rollback)?;
        fs::rename(&rollback, current)?;
    }
    Ok(())
}

fn cleanup_stale_releases(release: &Path, keep: &str) -> Result<()> {
    let parent = release.parent().context("release has a parent")?;
    for entry in fs::read_dir(parent)? {
        let entry = entry?;
        if entry.file_name().to_string_lossy() != keep {
            let _ = fs::remove_dir_all(entry.path());
        }
    }
    Ok(())
}

fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|value| value.as_nanos() as u64)
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn safe_relative_rejects_traversal_and_absolute_paths() {
        assert_eq!(safe_relative("assets/app.js").unwrap(), "assets/app.js");
        assert!(safe_relative("../escape").is_err());
        assert!(safe_relative("/etc/passwd").is_err());
        assert!(safe_relative("a\\b").is_err());
    }

    #[test]
    fn switch_current_uses_relative_symlink_and_cleans_temporary() {
        let root = tempfile::tempdir().unwrap();
        let project = root.path().join("projects").join("7");
        let release = project.join("releases").join("hash");
        fs::create_dir_all(&release).unwrap();
        fs::write(release.join("index.html"), b"new").unwrap();
        let syncer = PagesSyncer::new(
            String::new(),
            String::new(),
            Client::new(),
            root.path().to_path_buf(),
        );
        syncer.switch_current(7, &release).unwrap();
        let current = project.join("current");
        assert_eq!(fs::read_link(&current).unwrap(), Path::new("releases/hash"));
        assert_eq!(fs::read(current.join("index.html")).unwrap(), b"new");
        assert!(!project.join(".current.tmp").exists());
    }

    #[test]
    fn validates_latest_metadata_bounds() {
        let latest = PagesLatest {
            project_id: 7,
            deployment_id: 1,
            hash: "a".repeat(64),
            package_size: 10,
            file_count: 1,
            total_size: 5,
        };
        assert!(validate_latest(7, &latest).is_ok());
        assert!(validate_latest(8, &latest).is_err());
    }

    #[test]
    fn extracts_zip_directories_without_counting_them_as_files() {
        let root = tempfile::tempdir().unwrap();
        let package = root.path().join("site.zip");
        let destination = root.path().join("site");
        fs::create_dir_all(&destination).unwrap();

        let mut archive = zip::ZipWriter::new(File::create(&package).unwrap());
        archive
            .add_directory::<_, ()>("site/assets/", Default::default())
            .unwrap();
        archive
            .start_file::<_, ()>("site/index.html", Default::default())
            .unwrap();
        archive.write_all(b"site").unwrap();
        archive.finish().unwrap();

        let latest = PagesLatest {
            project_id: 7,
            deployment_id: 1,
            hash: "a".repeat(64),
            package_size: 0,
            file_count: 1,
            total_size: 4,
        };
        extract_zip(&package, &destination, &latest).unwrap();
        assert_eq!(fs::read(destination.join("index.html")).unwrap(), b"site");
    }
}
