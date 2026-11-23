use anyhow::{bail, ensure, Context, Result};
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine;
use serde::{Deserialize, Serialize};
use std::path::Path;

pub const BOOTSTRAP_TOKEN_PREFIX: &str = "oftok1_";

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum BootstrapKind {
    Relay,
    Tunnel,
}

impl BootstrapKind {
    pub fn as_str(self) -> &'static str {
        match self {
            BootstrapKind::Relay => "relay",
            BootstrapKind::Tunnel => "tunnel",
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct BootstrapTokenPayload {
    pub v: u8,
    pub s: String,
    pub k: BootstrapKind,
    pub t: String,
    #[serde(default)]
    pub p: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ResolvedLaunchAuth {
    pub server_url: String,
    pub raw_token: String,
    pub pool_key: Option<String>,
    pub used_bootstrap: bool,
}

pub fn encode_bootstrap_token(
    server_url: &str,
    kind: BootstrapKind,
    token: &str,
    pool_key: Option<&str>,
) -> Result<String> {
    let trimmed_url = normalize_server_url(server_url)?;
    let trimmed_token = token.trim();
    ensure!(!trimmed_token.is_empty(), "token must not be empty");
    let payload = BootstrapTokenPayload {
        v: 1,
        s: trimmed_url,
        k: kind,
        t: trimmed_token.to_owned(),
        p: pool_key.unwrap_or_default().trim().to_owned(),
    };
    let json = serde_json::to_vec(&payload).context("serialize bootstrap token payload")?;
    Ok(format!(
        "{BOOTSTRAP_TOKEN_PREFIX}{}",
        URL_SAFE_NO_PAD.encode(json)
    ))
}

pub fn decode_bootstrap_token(raw: &str) -> Result<BootstrapTokenPayload> {
    let trimmed = raw.trim();
    let encoded = trimmed
        .strip_prefix(BOOTSTRAP_TOKEN_PREFIX)
        .context("bootstrap token missing oftok1_ prefix")?;
    let bytes = URL_SAFE_NO_PAD
        .decode(encoded)
        .context("invalid base64url encoding in bootstrap token")?;
    let payload: BootstrapTokenPayload =
        serde_json::from_slice(&bytes).context("invalid JSON inside bootstrap token")?;
    ensure!(payload.v == 1, "unsupported bootstrap token version");
    ensure!(
        !payload.s.trim().is_empty() && !payload.t.trim().is_empty(),
        "bootstrap token missing server_url or token"
    );
    Ok(payload)
}

pub fn resolve_launch_auth(
    cli_server_url: Option<String>,
    cli_token: Option<String>,
    cli_token_file: Option<&Path>,
    cli_bootstrap_token: Option<String>,
    expected_kind: BootstrapKind,
) -> Result<Option<ResolvedLaunchAuth>> {
    let file_token = if let Some(path) = cli_token_file {
        let content = std::fs::read_to_string(path)
            .with_context(|| format!("read token file {}", path.display()))?;
        let trimmed = content.trim().to_owned();
        if trimmed.is_empty() {
            bail!("token file {} is empty", path.display());
        }
        Some(trimmed)
    } else {
        None
    };

    let candidate_token = cli_bootstrap_token.or(cli_token).or(file_token);
    let Some(raw) = candidate_token.map(|v| v.trim().to_owned()) else {
        return Ok(None);
    };
    if raw.is_empty() {
        return Ok(None);
    }

    if raw.starts_with(BOOTSTRAP_TOKEN_PREFIX) {
        let payload = decode_bootstrap_token(&raw)?;
        ensure!(
            payload.k == expected_kind,
            "bootstrap token kind {:?} does not match expected {:?}",
            payload.k,
            expected_kind
        );
        let server_url = match cli_server_url {
            Some(explicit) if !explicit.trim().is_empty() => normalize_server_url(&explicit)?,
            _ => normalize_server_url(&payload.s)?,
        };
        let pool_key = if payload.p.is_empty() {
            None
        } else {
            Some(payload.p)
        };
        return Ok(Some(ResolvedLaunchAuth {
            server_url,
            raw_token: payload.t,
            pool_key,
            used_bootstrap: true,
        }));
    }

    let Some(server_url) = cli_server_url
        .map(|v| v.trim().to_owned())
        .filter(|v| !v.is_empty())
    else {
        bail!("--server-url is required when using a raw token without a bootstrap token");
    };
    Ok(Some(ResolvedLaunchAuth {
        server_url: normalize_server_url(&server_url)?,
        raw_token: raw,
        pool_key: None,
        used_bootstrap: false,
    }))
}

pub fn normalize_server_url(raw: &str) -> Result<String> {
    let trimmed = raw.trim().trim_end_matches('/');
    let parsed = url::Url::parse(trimmed).context("invalid server URL")?;
    ensure!(
        matches!(parsed.scheme(), "http" | "https"),
        "server URL scheme must be http or https"
    );
    Ok(trimmed.to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn round_trips_bootstrap_token_and_validates_kind() {
        let tok = encode_bootstrap_token(
            "https://panel.example.com/",
            BootstrapKind::Relay,
            "secret-boot-123",
            Some("hk-pool"),
        )
        .expect("encode bootstrap token");
        assert!(tok.starts_with(BOOTSTRAP_TOKEN_PREFIX));

        let decoded = decode_bootstrap_token(&tok).expect("decode bootstrap token");
        assert_eq!(decoded.s, "https://panel.example.com");
        assert_eq!(decoded.k, BootstrapKind::Relay);
        assert_eq!(decoded.t, "secret-boot-123");
        assert_eq!(decoded.p, "hk-pool");

        let resolved = resolve_launch_auth(None, None, None, Some(tok.clone()), BootstrapKind::Relay)
            .expect("resolve bootstrap token")
            .expect("present");
        assert!(resolved.used_bootstrap);
        assert_eq!(resolved.server_url, "https://panel.example.com");
        assert_eq!(resolved.raw_token, "secret-boot-123");
        assert_eq!(resolved.pool_key.as_deref(), Some("hk-pool"));

        let wrong_kind =
            resolve_launch_auth(None, Some(tok), None, None, BootstrapKind::Tunnel);
        assert!(wrong_kind.is_err());
    }

    #[test]
    fn resolves_token_file_with_server_url() {
        let dir = tempfile::tempdir().expect("temp dir");
        let file = dir.path().join("token");
        std::fs::write(&file, "  raw-tunnel-token \n").expect("write token file");
        let resolved = resolve_launch_auth(
            Some("https://ctrl.example.com/".to_owned()),
            None,
            Some(&file),
            None,
            BootstrapKind::Tunnel,
        )
        .expect("resolve")
        .expect("some");
        assert!(!resolved.used_bootstrap);
        assert_eq!(resolved.server_url, "https://ctrl.example.com");
        assert_eq!(resolved.raw_token, "raw-tunnel-token");
    }
}
