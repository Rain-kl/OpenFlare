use anyhow::{Context, Result};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::Write;
use std::path::Path;
use tempfile::NamedTempFile;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelayIdentity {
    pub server_url: String,
    pub relay_id: String,
    pub pool_id: u64,
    pub access_token: String,
    #[serde(default)]
    pub noise_private_key: String,
    #[serde(default)]
    pub noise_public_key: String,
    #[serde(default)]
    pub gateway_secret: String,
    #[serde(default)]
    pub source_token_sha256: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConnectorIdentity {
    pub server_url: String,
    pub connector_id: String,
    pub tunnel_token: String,
    #[serde(default)]
    pub source_token_sha256: String,
}

pub fn token_sha256(token: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(token.trim().as_bytes());
    format!("{:x}", hasher.finalize())
}

pub fn load_json<T: DeserializeOwned>(path: &Path) -> Result<Option<T>> {
    match fs::read(path) {
        Ok(bytes) => {
            let value = serde_json::from_slice(&bytes)
                .with_context(|| format!("decode persisted state {}", path.display()))?;
            Ok(Some(value))
        }
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(err) => Err(err).with_context(|| format!("read {}", path.display())),
    }
}

pub fn save_json_secret<T: Serialize>(path: &Path, value: &T) -> Result<()> {
    let parent = path
        .parent()
        .context("identity state file path must have parent directory")?;
    fs::create_dir_all(parent)
        .with_context(|| format!("create state dir {}", parent.display()))?;
    set_dir_permissions_0700(parent)?;

    let encoded = serde_json::to_vec_pretty(value).context("serialize persisted identity")?;
    let mut temp = NamedTempFile::new_in(parent).context("create temp identity file")?;
    temp.write_all(&encoded)?;
    temp.as_file().sync_all()?;
    set_file_permissions_0600(temp.as_file())?;
    temp.persist(path)
        .map_err(|err| err.error)
        .with_context(|| format!("persist {}", path.display()))?;
    Ok(())
}

pub fn ensure_connector_id(state_dir: &Path) -> Result<String> {
    let path = state_dir.join("connector-id");
    match fs::read_to_string(&path) {
        Ok(existing) => {
            let trimmed = existing.trim().to_owned();
            if !trimmed.is_empty() {
                return Ok(trimmed);
            }
        }
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => {}
        Err(err) => return Err(err).with_context(|| format!("read {}", path.display())),
    }

    fs::create_dir_all(state_dir)?;
    let generated = uuid::Uuid::new_v4().to_string();
    let mut temp = NamedTempFile::new_in(state_dir)?;
    temp.write_all(generated.as_bytes())?;
    temp.as_file().sync_all()?;
    set_file_permissions_0600(temp.as_file())?;
    temp.persist(&path).map_err(|err| err.error)?;
    Ok(generated)
}

#[cfg(unix)]
fn set_file_permissions_0600(file: &fs::File) -> Result<()> {
    use std::os::unix::fs::PermissionsExt;
    file.set_permissions(fs::Permissions::from_mode(0o600))?;
    Ok(())
}

#[cfg(not(unix))]
fn set_file_permissions_0600(_file: &fs::File) -> Result<()> {
    Ok(())
}

#[cfg(unix)]
fn set_dir_permissions_0700(dir: &Path) -> Result<()> {
    use std::os::unix::fs::PermissionsExt;
    fs::set_permissions(dir, fs::Permissions::from_mode(0o700))?;
    Ok(())
}

#[cfg(not(unix))]
fn set_dir_permissions_0700(_dir: &Path) -> Result<()> {
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn persists_connector_id_and_secret_identity_with_0600() {
        let dir = tempfile::tempdir().expect("tempdir");
        let id1 = ensure_connector_id(dir.path()).expect("generate id");
        let id2 = ensure_connector_id(dir.path()).expect("reuse id");
        assert_eq!(id1, id2);
        assert!(!id1.is_empty());

        let state_path = dir.path().join("relay_identity.json");
        let fp = token_sha256("secret-access");
        assert_eq!(fp, token_sha256("  secret-access  "));
        assert_eq!(fp.len(), 64);

        let identity = RelayIdentity {
            server_url: "https://panel.example.com".to_owned(),
            relay_id: "relay-1".to_owned(),
            pool_id: 7,
            access_token: "secret-access".to_owned(),
            noise_private_key: "priv".to_owned(),
            noise_public_key: "pub".to_owned(),
            gateway_secret: "gwsec".to_owned(),
            source_token_sha256: fp.clone(),
        };
        save_json_secret(&state_path, &identity).expect("save secret");
        let loaded: RelayIdentity = load_json(&state_path)
            .expect("load")
            .expect("some");
        assert_eq!(loaded.access_token, "secret-access");
        assert_eq!(loaded.source_token_sha256, fp);

        let legacy_json = r#"{"server_url":"https://panel.example.com","relay_id":"r1","pool_id":1,"access_token":"tok"}"#;
        let legacy: RelayIdentity = serde_json::from_str(legacy_json).expect("legacy json");
        assert!(legacy.source_token_sha256.is_empty());

        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mode = fs::metadata(&state_path).expect("meta").permissions().mode() & 0o777;
            assert_eq!(mode, 0o600);
        }
    }
}
