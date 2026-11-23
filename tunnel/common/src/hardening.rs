use anyhow::Result;
use serde::{Deserialize, Serialize};
use std::fs::OpenOptions;
use std::io::Write;
use std::path::Path;
use subtle::ConstantTimeEq;

pub fn constant_time_eq(left: &str, right: &str) -> bool {
    left.as_bytes().ct_eq(right.as_bytes()).into()
}

/// Write a TOML/config file modifying the existing inode in place when present.
/// Upstream rathole's file watcher reacts to `EventKind::Modify(_)`, so in-place
/// truncation + write ensures inotify triggers a reload reliably.
pub fn write_file_in_place_0600(path: &Path, content: &str) -> Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }
    let mut file = OpenOptions::new()
        .create(true)
        .write(true)
        .truncate(true)
        .open(path)?;
    file.write_all(content.as_bytes())?;
    file.flush()?;
    file.sync_all()?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        file.set_permissions(std::fs::Permissions::from_mode(0o600))?;
    }
    Ok(())
}

pub fn http_to_ws_url(base_url: &str, path: &str) -> Result<String> {
    let mut parsed = url::Url::parse(base_url.trim().trim_end_matches('/'))?;
    match parsed.scheme() {
        "http" => {
            parsed.set_scheme("ws").map_err(|()| anyhow::anyhow!("set ws scheme failed"))?;
        }
        "https" => {
            parsed.set_scheme("wss").map_err(|()| anyhow::anyhow!("set wss scheme failed"))?;
        }
        "ws" | "wss" => {}
        other => anyhow::bail!("unsupported scheme {other} for websocket conversion"),
    }
    let normalized_path = if path.starts_with('/') {
        path.to_owned()
    } else {
        format!("/{path}")
    };
    parsed.set_path(&normalized_path);
    Ok(parsed.to_string())
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WakeMessage {
    #[serde(rename = "type", default)]
    pub kind: String,
    #[serde(default)]
    pub reason: String,
    #[serde(default)]
    pub config_revision: i64,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn constant_time_compare_and_ws_url() {
        assert!(constant_time_eq("abc-123", "abc-123"));
        assert!(!constant_time_eq("abc-123", "abc-124"));
        assert_eq!(
            http_to_ws_url("https://panel.example.com", "/api/v1/relay/ws").unwrap(),
            "wss://panel.example.com/api/v1/relay/ws"
        );
        assert_eq!(
            http_to_ws_url("http://127.0.0.1:8080/", "api/v1/tunnel/ws").unwrap(),
            "ws://127.0.0.1:8080/api/v1/tunnel/ws"
        );
    }

    #[test]
    fn write_file_in_place_updates_existing_inode() {
        let dir = tempfile::tempdir().expect("tempdir");
        let path = dir.path().join("rathole.toml");
        write_file_in_place_0600(&path, "first = 1\n").expect("write 1");
        #[cfg(unix)]
        let inode_1 = {
            use std::os::unix::fs::MetadataExt;
            std::fs::metadata(&path).unwrap().ino()
        };
        write_file_in_place_0600(&path, "second = 2\n").expect("write 2");
        assert_eq!(std::fs::read_to_string(&path).unwrap(), "second = 2\n");
        #[cfg(unix)]
        {
            use std::os::unix::fs::MetadataExt;
            let inode_2 = std::fs::metadata(&path).unwrap().ino();
            assert_eq!(inode_1, inode_2);
        }
    }
}
