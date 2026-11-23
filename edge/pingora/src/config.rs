use crate::protocol::config::{Config, PoWPageConfig, Route, WafIPGroup};
use anyhow::{bail, Context, Result};
use pingora::tls::pkey::{PKey, Private};
use pingora::tls::x509::X509;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::io::Write;
use std::path::Path;
use std::sync::{Arc, RwLock};

pub const CONTRACT_VERSION: i64 = 4;
const PROXY_CAPABILITY: &str = "shared-snapshot-v1";
const BASE_CAPABILITY: &str = "tls-sni-h2-v1";
const POLICY_CAPABILITY: &str = "policy-runtime-v1";
const MEMORY_KEYS_CAPABILITY: &str = "tls-memory-keys-v1";
const WAF_WHITELIST_CAPABILITY: &str = "waf-request-whitelist-v1";

#[derive(Debug, Clone)]
pub struct LoadedCertificate {
    pub cert: X509,
    pub chain: Vec<X509>,
    pub cert_sha256: String,
}

/// Computes the lowercase hex SHA-256 digest of a normalized certificate PEM.
pub fn certificate_pem_digest(cert_pem: &str) -> String {
    let trimmed = cert_pem.trim();
    if trimmed.is_empty() {
        return String::new();
    }
    let normalized = format!("{trimmed}\n");
    format!("{:x}", Sha256::digest(normalized.as_bytes()))
}

#[derive(Clone)]
struct LoadedPrivateKey {
    cert_sha256: String,
    key: PKey<Private>,
}

/// Thread-safe in-memory store holding TLS private keys hydrated from the control plane.
#[derive(Clone, Default)]
pub struct CertificateKeyRing {
    inner: std::sync::Arc<std::sync::RwLock<HashMap<u64, LoadedPrivateKey>>>,
}

impl CertificateKeyRing {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn get(&self, cert_id: u64, expected_sha256: &str) -> Option<PKey<Private>> {
        let guard = self.inner.read().ok()?;
        let entry = guard.get(&cert_id)?;
        if !entry.cert_sha256.eq_ignore_ascii_case(expected_sha256) {
            return None;
        }
        Some(entry.key.clone())
    }

    pub fn missing_for(&self, config: &AppliedConfig) -> Vec<(u64, String)> {
        let Ok(guard) = self.inner.read() else {
            return config.certificate_digests();
        };
        let mut missing = Vec::new();
        for (cert_id, expected_sha256) in config.certificate_digests() {
            let matched = guard
                .get(&cert_id)
                .is_some_and(|entry| entry.cert_sha256.eq_ignore_ascii_case(&expected_sha256));
            if !matched {
                missing.push((cert_id, expected_sha256));
            }
        }
        missing
    }

    pub fn install(
        &self,
        cert_id: u64,
        cert: &LoadedCertificate,
        key_pem: zeroize::Zeroizing<String>,
    ) -> Result<()> {
        let key = PKey::private_key_from_pem(key_pem.as_bytes())
            .with_context(|| format!("failed to decode private key {cert_id}"))?;
        if !cert.cert.public_key()?.public_eq(&key) {
            bail!("certificate {cert_id} does not match its private key");
        }
        let mut guard = self
            .inner
            .write()
            .map_err(|_| anyhow::anyhow!("certificate key ring lock poisoned"))?;
        guard.insert(
            cert_id,
            LoadedPrivateKey {
                cert_sha256: cert.cert_sha256.clone(),
                key,
            },
        );
        Ok(())
    }

    pub fn retain_for(&self, config: &AppliedConfig) {
        let expected: HashMap<u64, String> = config.certificate_digests().into_iter().collect();
        if let Ok(mut guard) = self.inner.write() {
            guard.retain(|cert_id, entry| {
                expected
                    .get(cert_id)
                    .is_some_and(|sha| entry.cert_sha256.eq_ignore_ascii_case(sha))
            });
        }
    }

    pub fn clear(&self) {
        if let Ok(mut guard) = self.inner.write() {
            guard.clear();
        }
    }

    #[cfg(test)]
    pub fn is_empty(&self) -> bool {
        self.inner
            .read()
            .map(|guard| guard.is_empty())
            .unwrap_or(true)
    }
}

#[derive(Debug, Clone)]
pub struct AppliedConfig {
    pub version: String,
    pub checksum: String,
    pub release_sequence: i64,
    pub payload_checksum: String,
    pub payload_json: String,
    pub config: Config,
    pub waf: crate::policy::CompiledWaf,
    pub certificates: HashMap<u64, LoadedCertificate>,
    pub pages_releases: HashMap<u64, String>,
    upstream_cursors: Arc<HashMap<u64, std::sync::atomic::AtomicUsize>>,
}

impl AppliedConfig {
    pub fn merge_ip_groups(&mut self, groups: &[WafIPGroup]) -> bool {
        let mut changed = false;
        for group in groups {
            if let Some(existing) = self
                .config
                .waf
                .ip_groups
                .iter_mut()
                .find(|existing| group.id == existing.id)
            {
                if existing.checksum != group.checksum {
                    *existing = group.clone();
                    changed = true;
                }
            }
        }
        changed
    }

    pub fn upstream_start(&self, route_id: u64) -> usize {
        self.upstream_cursors.get(&route_id).map_or(0, |cursor| {
            cursor.fetch_add(1, std::sync::atomic::Ordering::Relaxed)
        })
    }

    pub fn routes(&self) -> impl Iterator<Item = &Route> {
        self.config.routes.iter().filter(|route| route.enabled)
    }

    pub fn pow_page(&self) -> &PoWPageConfig {
        &self.config.pow_page
    }

    pub fn allows_http2(&self, host: &str) -> bool {
        self.route_for_host(host)
            .is_some_and(|route| route.enable_https && route.enable_http2)
    }

    pub fn certificate_for_host(&self, host: &str) -> Option<(u64, &LoadedCertificate)> {
        let route = self.route_for_host(host)?;
        if !route.enable_https {
            return None;
        }
        let cert_id = route.certificate_id_for_host(host)?;
        let cert = self.certificates.get(&cert_id)?;
        Some((cert_id, cert))
    }

    pub fn certificate_by_id(&self, cert_id: u64) -> Option<&LoadedCertificate> {
        self.certificates.get(&cert_id)
    }

    pub fn certificate_digests(&self) -> Vec<(u64, String)> {
        let mut digests: Vec<(u64, String)> = self
            .certificates
            .iter()
            .map(|(id, cert)| (*id, cert.cert_sha256.clone()))
            .collect();
        digests.sort_by_key(|(id, _)| *id);
        digests
    }

    pub fn route_for_host(&self, host: &str) -> Option<&Route> {
        find_route(self.routes(), host)
    }
}

pub fn decode(
    version: &str,
    checksum: &str,
    release_sequence: i64,
    payload_checksum: &str,
    payload_json: &str,
) -> Result<AppliedConfig> {
    if version.trim().is_empty()
        || checksum.trim().is_empty()
        || payload_checksum.trim().is_empty()
        || payload_json.trim().is_empty()
    {
        bail!("active configuration is incomplete");
    }
    if release_sequence <= 0 {
        bail!("active configuration has an invalid release sequence");
    }
    let digest = format!("{:x}", Sha256::digest(payload_json.as_bytes()));
    if !constant_time_equal(digest.as_bytes(), payload_checksum.trim().as_bytes()) {
        bail!("active configuration payload checksum mismatch");
    }
    let envelope: crate::protocol::snapshot::SharedSnapshotEnvelope =
        serde_json::from_str(payload_json).context("decode shared edge snapshot")?;
    let config = envelope.compile()?;
    validate(&config)?;
    let waf = crate::policy::compile_waf(&config.waf)?;
    let certificates = load_certificates(&config)?;
    let upstream_cursors = Arc::new(
        config
            .routes
            .iter()
            .filter(|route| route.upstreams.len() > 1)
            .map(|route| (route.id, std::sync::atomic::AtomicUsize::new(0)))
            .collect(),
    );
    Ok(AppliedConfig {
        version: version.to_owned(),
        checksum: checksum.to_owned(),
        release_sequence,
        payload_checksum: digest,
        payload_json: payload_json.to_owned(),
        config,
        waf,
        certificates,
        pages_releases: HashMap::new(),
        upstream_cursors,
    })
}

fn validate(config: &Config) -> Result<()> {
    if config.schema_version != 1 || config.contract_version != CONTRACT_VERSION {
        bail!(
            "unsupported Pingora contract: schema {} / contract {}",
            config.schema_version,
            config.contract_version
        );
    }
    for capability in &config.required_capabilities {
        if !matches!(
            capability.as_str(),
            BASE_CAPABILITY
                | POLICY_CAPABILITY
                | PROXY_CAPABILITY
                | MEMORY_KEYS_CAPABILITY
                | WAF_WHITELIST_CAPABILITY
        ) {
            bail!("unsupported required capability {capability}");
        }
    }
    for required in [
        BASE_CAPABILITY,
        POLICY_CAPABILITY,
        PROXY_CAPABILITY,
        MEMORY_KEYS_CAPABILITY,
    ] {
        if !config
            .required_capabilities
            .iter()
            .any(|capability| capability == required)
        {
            bail!("configuration does not declare required capability {required}");
        }
    }
    for route in &config.routes {
        if !route.enabled {
            continue;
        }
        if let Some(pow) = &route.pow {
            crate::policy::validate_pow(pow)?;
        }
        if route.site_auth_ttl_hours != 0 {
            let key = &route.site_auth_key;
            if !route.enable_https
                || route.site_auth_ttl_hours < 1
                || route.site_auth_ttl_hours > 720
                || key.len() != 64
                || !key.bytes().all(|byte| byte.is_ascii_hexdigit())
                || route.basic_auth_enabled
                || route.basic_auth_username.is_empty()
                || route.basic_auth_password.is_empty()
            {
                bail!(
                    "site {} has invalid site authentication configuration",
                    route.display_name()
                );
            }
        }
        for header in &route.custom_headers {
            http::header::HeaderName::from_bytes(header.key.as_bytes())?;
            http::header::HeaderValue::from_str(&header.value)?;
        }
        if let Some(deployment) = &route.pages {
            crate::pages::validate(deployment)?;
        } else if !route.is_pages() {
            for upstream in route.upstreams() {
                let url = url::Url::parse(&upstream).context("invalid upstream URL")?;
                if !matches!(url.scheme(), "http" | "https") || url.host().is_none() {
                    bail!("unsupported upstream transport on {}", route.display_name());
                }
            }
        }
        if route.domains.is_empty() {
            bail!("enabled route {} has no domains", route.display_name());
        }
        if route.upstreams.is_empty() && route.origin_url.is_empty() {
            bail!("enabled route {} has no upstream", route.display_name());
        }
        if route.domain_cert_ids.len() != route.domains.len() {
            bail!(
                "enabled route {} has inconsistent domain certificate references",
                route.display_name()
            );
        }
        for cert_id in route.domain_cert_ids.iter().filter(|id| **id != 0) {
            if !config
                .assets
                .certificates
                .contains_key(&cert_id.to_string())
            {
                bail!(
                    "certificate {cert_id} required by route {} is missing",
                    route.display_name()
                );
            }
        }
    }
    Ok(())
}

fn load_certificates(config: &Config) -> Result<HashMap<u64, LoadedCertificate>> {
    let mut certificates = HashMap::with_capacity(config.assets.certificates.len());
    for (id, asset) in &config.assets.certificates {
        let parsed_id = id
            .parse::<u64>()
            .with_context(|| format!("certificate asset has invalid identifier {id}"))?;
        let mut chain = X509::stack_from_pem(asset.cert_pem.as_bytes())
            .with_context(|| format!("failed to decode certificate {id}"))?
            .into_iter();
        let cert = chain
            .next()
            .with_context(|| format!("certificate {id} has no leaf"))?;
        let expected_sha256 = certificate_pem_digest(&asset.cert_pem);
        let provided_sha256 = asset.cert_sha256.trim().to_ascii_lowercase();
        if expected_sha256.is_empty() || provided_sha256 != expected_sha256 {
            bail!(
                "certificate {id} digest mismatch: expected {expected_sha256}, got {provided_sha256}"
            );
        }
        certificates.insert(
            parsed_id,
            LoadedCertificate {
                cert,
                chain: chain.collect(),
                cert_sha256: expected_sha256,
            },
        );
    }
    Ok(certificates)
}

fn find_route<'a>(routes: impl Iterator<Item = &'a Route>, host: &str) -> Option<&'a Route> {
    let host = normalize_host(host);
    routes
        .flat_map(|route| route.domains.iter().map(move |domain| (route, domain)))
        .filter(|(_, domain)| domain_matches(domain, &host))
        .max_by_key(|(_, domain)| !domain.starts_with("*."))
        .map(|(route, _)| route)
}

fn domain_matches(domain: &str, host: &str) -> bool {
    let domain = normalize_host(domain);
    if domain == host {
        return true;
    }
    let Some(prefix) = domain.strip_prefix("*.") else {
        return false;
    };
    let Some((first, rest)) = host.split_once('.') else {
        return false;
    };
    !first.is_empty() && rest == prefix
}

fn normalize_host(value: &str) -> String {
    value.trim().trim_end_matches('.').to_ascii_lowercase()
}

fn constant_time_equal(left: &[u8], right: &[u8]) -> bool {
    if left.len() != right.len() {
        return false;
    }
    left.iter()
        .zip(right)
        .fold(0u8, |difference, (left, right)| difference | (left ^ right))
        == 0
}

impl Route {
    pub fn display_name(&self) -> &str {
        if self.site_name.is_empty() {
            "unnamed"
        } else {
            &self.site_name
        }
    }

    pub fn upstreams(&self) -> Vec<String> {
        if self.upstreams.is_empty() {
            vec![self.origin_url.clone()]
        } else {
            self.upstreams.clone()
        }
    }

    pub fn certificate_id_for_host(&self, host: &str) -> Option<u64> {
        let normalized_host = normalize_host(host);
        self.domains
            .iter()
            .enumerate()
            .filter(|(_, domain)| domain_matches(domain, &normalized_host))
            .max_by_key(|(_, domain)| !domain.starts_with("*."))
            .map(|(index, _)| index)
            .and_then(|index| self.domain_cert_ids.get(index).copied())
            .filter(|id| *id != 0)
    }

    pub fn is_pages(&self) -> bool {
        self.upstream_type == "pages" && self.pages.is_some()
    }
}

#[derive(Clone, Default)]
pub struct SnapshotStore(Arc<RwLock<Option<Arc<AppliedConfig>>>>);

impl SnapshotStore {
    pub fn open(state_dir: &Path) -> Self {
        let restored = ["snapshot.json", "snapshot.previous.json"]
            .into_iter()
            .find_map(|name| {
                let path = state_dir.join(name);
                match std::fs::read_to_string(&path) {
                    Ok(data) => match restore_snapshot_from_str(&data) {
                        Ok(snapshot) => snapshot.filter(|snapshot| {
                            if crate::pages::resources_ready(snapshot, state_dir) {
                                true
                            } else {
                                tracing::warn!(path = %path.display(), "saved snapshot references an unavailable Pages release");
                                false
                            }
                        }),
                        Err(error) => {
                            tracing::warn!(path = %path.display(), %error, "snapshot rejected");
                            None
                        }
                    },
                    Err(error) if error.kind() == std::io::ErrorKind::NotFound => None,
                    Err(error) => {
                        tracing::warn!(path = %path.display(), %error, "snapshot unavailable");
                        None
                    }
                }
            });
        Self(Arc::new(RwLock::new(restored.map(Arc::new))))
    }

    pub fn current(&self) -> Option<Arc<AppliedConfig>> {
        self.0.read().expect("snapshot lock poisoned").clone()
    }

    pub fn recover_latest(&self, state_dir: &Path) {
        let Some(recovered) = Self::open(state_dir).current() else {
            return;
        };
        let mut current = self.0.write().expect("snapshot lock poisoned");
        if current
            .as_ref()
            .is_none_or(|snapshot| recovered.release_sequence >= snapshot.release_sequence)
        {
            *current = Some(recovered);
        }
    }

    pub fn commit(&self, state_dir: &Path, candidate: AppliedConfig) -> Result<()> {
        if let Some(current) = self.current() {
            if current.release_sequence != candidate.release_sequence {
                persist_snapshot(&state_dir.join("snapshot.previous.json"), &current)?;
            }
        }
        persist_snapshot(&state_dir.join("snapshot.json"), &candidate)?;
        *self.0.write().expect("snapshot lock poisoned") = Some(Arc::new(candidate));
        Ok(())
    }
    pub fn apply_ip_groups(&self, state_dir: &Path, groups: &[WafIPGroup]) -> Result<()> {
        if groups.is_empty() {
            return Ok(());
        }
        if let Some(snapshot) = self.current() {
            let mut candidate = (*snapshot).clone();
            if candidate.merge_ip_groups(groups) {
                self.commit(state_dir, candidate)?;
            }
        }
        Ok(())
    }
}

pub fn persist_private_json<T: Serialize>(path: &Path, value: &T) -> Result<()> {
    let parent = path
        .parent()
        .context("state file requires a parent directory")?;
    std::fs::create_dir_all(parent)?;
    let mut file = tempfile::NamedTempFile::new_in(parent)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt as _;
        file.as_file()
            .set_permissions(std::fs::Permissions::from_mode(0o600))?;
    }
    serde_json::to_writer(file.as_file_mut(), value)?;
    file.flush()?;
    file.as_file().sync_all()?;
    file.persist(path)?;
    std::fs::File::open(parent)?.sync_all()?;
    Ok(())
}

#[derive(Serialize, Deserialize)]
struct SnapshotDocument {
    version: String,
    checksum: String,
    release_sequence: i64,
    payload_checksum: String,
    payload_json: String,
    pages_releases: HashMap<u64, String>,
    ip_groups: Vec<WafIPGroup>,
}

fn persist_snapshot(path: &Path, config: &AppliedConfig) -> Result<()> {
    persist_private_json(
        path,
        &SnapshotDocument {
            version: config.version.clone(),
            checksum: config.checksum.clone(),
            release_sequence: config.release_sequence,
            payload_checksum: config.payload_checksum.clone(),
            payload_json: config.payload_json.clone(),
            pages_releases: config.pages_releases.clone(),
            ip_groups: config.config.waf.ip_groups.clone(),
        },
    )
}

fn restore_snapshot_from_str(document: &str) -> Result<Option<AppliedConfig>> {
    let document: SnapshotDocument = serde_json::from_str(document)?;
    let mut snapshot = decode(
        &document.version,
        &document.checksum,
        document.release_sequence,
        &document.payload_checksum,
        &document.payload_json,
    )?;
    for hash in document.pages_releases.values() {
        if hash.len() != 64 || !hash.bytes().all(|byte| byte.is_ascii_hexdigit()) {
            bail!("invalid Pages release hash in saved snapshot");
        }
    }
    snapshot.pages_releases = document.pages_releases;
    snapshot.config.waf.ip_groups = document.ip_groups;
    Ok(Some(snapshot))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn payload(cert: u64) -> String {
        serde_json::json!({
            "schema_version": 1, "contract_version": CONTRACT_VERSION,
            "required_capabilities": [
                BASE_CAPABILITY,
                POLICY_CAPABILITY,
                PROXY_CAPABILITY,
                MEMORY_KEYS_CAPABILITY
            ],
            "snapshot": {
                "routes": [{
                    "id": 1, "site_name": "example", "domains": ["example.com"], "domain_cert_ids": [cert],
                    "origin_url": "http://127.0.0.1:9080", "enabled": true,
                    "enable_https": cert != 0, "enable_http2": cert != 0, "redirect_http": false
                }],
                "openresty_config": {}, "waf": {"rule_groups": [], "bindings": []}
            },
            "assets": { "certificates": {} }
        }).to_string()
    }

    #[test]
    fn checks_payload_digest_before_decoding() {
        let payload = payload(0);
        let digest = format!("{:x}", Sha256::digest(payload.as_bytes()));
        assert!(decode("v1", "checksum", 1, &digest, &payload).is_ok());
        assert!(decode("v1", "checksum", 1, "bad", &payload).is_err());
    }

    #[test]
    fn snapshot_round_trips_and_restores_the_previous_snapshot() {
        let payload = payload(0);
        let digest = format!("{:x}", Sha256::digest(payload.as_bytes()));
        let first = decode("v1", "checksum", 7, &digest, &payload).unwrap();
        let second = decode("v2", "checksum", 8, &digest, &payload).unwrap();
        let state_dir = tempfile::tempdir().unwrap();
        let store = SnapshotStore::default();
        store.commit(state_dir.path(), first).unwrap();
        store.commit(state_dir.path(), second).unwrap();
        let restored = SnapshotStore::open(state_dir.path()).current().unwrap();
        assert_eq!(restored.release_sequence, 8);
        assert_eq!(restored.payload_json, payload);
        std::fs::write(state_dir.path().join("snapshot.json"), "broken").unwrap();
        let restored = SnapshotStore::open(state_dir.path()).current().unwrap();
        assert_eq!(restored.release_sequence, 7);
    }

    #[test]
    fn matches_exact_and_single_level_wildcard_domains() {
        let routes = [
            Route {
                id: 1,
                site_name: "wildcard".into(),
                domains: vec!["*.example.com".into()],
                domain_cert_ids: vec![10],
                origin_url: "http://127.0.0.1:9080".into(),
                origin_host: String::new(),
                upstreams: vec![],
                enabled: true,
                enable_https: true,
                enable_http2: true,
                redirect_http: true,
                ..Route::default()
            },
            Route {
                id: 2,
                site_name: "exact".into(),
                domains: vec!["api.example.com".into()],
                domain_cert_ids: vec![20],
                origin_url: "http://127.0.0.1:9081".into(),
                origin_host: String::new(),
                upstreams: vec![],
                enabled: true,
                enable_https: true,
                enable_http2: true,
                redirect_http: true,
                ..Route::default()
            },
        ];
        let exact_first = [routes[1].clone(), routes[0].clone()];
        let wildcard_first = [routes[0].clone(), routes[1].clone()];
        assert_eq!(
            find_route(exact_first.iter(), "api.example.com").map(|r| r.id),
            Some(2)
        );
        assert_eq!(
            find_route(wildcard_first.iter(), "a.example.com").map(|r| r.id),
            Some(1)
        );
        assert!(find_route(wildcard_first.iter(), "a.b.example.com").is_none());
        assert_eq!(routes[0].certificate_id_for_host("a.example.com"), Some(10));
        assert_eq!(
            routes[1].certificate_id_for_host("api.example.com"),
            Some(20)
        );
    }

    #[test]
    fn rejects_missing_certificate_asset() {
        let payload = payload(10);
        let digest = format!("{:x}", Sha256::digest(payload.as_bytes()));
        assert!(decode("v1", "checksum", 1, &digest, &payload).is_err());
    }

    fn test_cert_and_key() -> (String, String) {
        use openssl::asn1::Asn1Time;
        use openssl::bn::{BigNum, MsbOption};
        use openssl::ec::{EcGroup, EcKey};
        use openssl::hash::MessageDigest;
        use openssl::nid::Nid;
        use openssl::x509::{X509NameBuilder, X509};

        let group = EcGroup::from_curve_name(Nid::X9_62_PRIME256V1).unwrap();
        let ec_key = EcKey::generate(&group).unwrap();
        let pkey = PKey::from_ec_key(ec_key).unwrap();
        let mut name = X509NameBuilder::new().unwrap();
        name.append_entry_by_text("CN", "example.com").unwrap();
        let name = name.build();
        let mut serial = BigNum::new().unwrap();
        serial.rand(64, MsbOption::MAYBE_ZERO, false).unwrap();
        let serial = serial.to_asn1_integer().unwrap();
        let mut builder = X509::builder().unwrap();
        builder.set_version(2).unwrap();
        builder.set_serial_number(&serial).unwrap();
        builder.set_subject_name(&name).unwrap();
        builder.set_issuer_name(&name).unwrap();
        builder.set_pubkey(&pkey).unwrap();
        let not_before = Asn1Time::days_from_now(0).unwrap();
        let not_after = Asn1Time::days_from_now(30).unwrap();
        builder.set_not_before(&not_before).unwrap();
        builder.set_not_after(&not_after).unwrap();
        builder.sign(&pkey, MessageDigest::sha256()).unwrap();
        let cert_pem = String::from_utf8(builder.build().to_pem().unwrap()).unwrap();
        let key_pem = String::from_utf8(pkey.private_key_to_pem_pkcs8().unwrap()).unwrap();
        (cert_pem, key_pem)
    }

    fn payload_with_cert(cert_id: u64, cert_pem: &str, cert_sha256: &str) -> String {
        serde_json::json!({
            "schema_version": 1,
            "contract_version": CONTRACT_VERSION,
            "required_capabilities": [
                BASE_CAPABILITY,
                POLICY_CAPABILITY,
                PROXY_CAPABILITY,
                MEMORY_KEYS_CAPABILITY
            ],
            "snapshot": {
                "routes": [{
                    "id": 1,
                    "site_name": "example",
                    "domains": ["example.com"],
                    "domain_cert_ids": [cert_id],
                    "origin_url": "http://127.0.0.1:9080",
                    "enabled": true,
                    "enable_https": true,
                    "enable_http2": true,
                    "redirect_http": false
                }],
                "openresty_config": {},
                "waf": {"rule_groups": [], "bindings": []}
            },
            "assets": {
                "certificates": {
                    cert_id.to_string(): {
                        "cert_pem": cert_pem,
                        "cert_sha256": cert_sha256
                    }
                }
            }
        })
        .to_string()
    }

    #[test]
    fn snapshot_persists_zero_private_keys_and_key_ring_enforces_digest() {
        let (cert1, key1) = test_cert_and_key();
        let sha1 = certificate_pem_digest(&cert1);
        let payload1 = payload_with_cert(10, &cert1, &sha1);
        let digest1 = format!("{:x}", Sha256::digest(payload1.as_bytes()));
        let first = decode("v1", "c1", 1, &digest1, &payload1).unwrap();

        let state_dir = tempfile::tempdir().unwrap();
        let store = SnapshotStore::default();
        store.commit(state_dir.path(), first.clone()).unwrap();
        let persisted = std::fs::read_to_string(state_dir.path().join("snapshot.json")).unwrap();
        assert!(!persisted.contains("PRIVATE KEY"));
        assert!(!persisted.contains("key_pem"));
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt as _;
            let mode = std::fs::metadata(state_dir.path().join("snapshot.json"))
                .unwrap()
                .permissions()
                .mode();
            assert_eq!(mode & 0o077, 0);
        }

        let (cert2, key2) = test_cert_and_key();
        let sha2 = certificate_pem_digest(&cert2);
        let payload2 = payload_with_cert(10, &cert2, &sha2);
        let digest2 = format!("{:x}", Sha256::digest(payload2.as_bytes()));
        let second = decode("v2", "c2", 2, &digest2, &payload2).unwrap();

        let ring = CertificateKeyRing::new();
        assert_eq!(ring.missing_for(&first), vec![(10, sha1.clone())]);

        let loaded1 = first.certificate_by_id(10).unwrap();
        assert!(ring
            .install(10, loaded1, zeroize::Zeroizing::new(key2.clone()))
            .is_err());
        ring.install(10, loaded1, zeroize::Zeroizing::new(key1))
            .unwrap();
        assert!(ring.get(10, &sha1).is_some());
        assert!(ring.missing_for(&first).is_empty());

        // Digest rotation invalidates stale lookups and retains nothing old after retain_for.
        assert!(ring.get(10, &sha2).is_none());
        assert_eq!(ring.missing_for(&second), vec![(10, sha2.clone())]);
        ring.retain_for(&second);
        assert!(ring.is_empty());

        let loaded2 = second.certificate_by_id(10).unwrap();
        ring.install(10, loaded2, zeroize::Zeroizing::new(key2))
            .unwrap();
        assert!(ring.get(10, &sha2).is_some());
        ring.clear();
        assert!(ring.is_empty());
        assert!(ring.get(10, &sha2).is_none());
    }
}
