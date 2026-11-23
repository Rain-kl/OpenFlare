use crate::config::SnapshotStore;
pub(crate) mod config_sync;
mod node_ip;
mod site_sync;
pub(crate) mod update;
mod websocket;

use config_sync::{Manifest, PERFORMANCE_KEY};
use site_sync::{ActiveConfigMeta, SiteSync};

use crate::config::{self, AppliedConfig, CertificateKeyRing};
use crate::pages::PagesSyncer;
use crate::runtime::{
    performance::{LivePolicy, NodePolicy},
    Listeners,
};
use crate::telemetry::{AccessLogSpool, HostCollector, HostTelemetry, NodeAccessLog};
use crate::{edge_user_agent, EDGE_VERSION};
use anyhow::{Context, Result};
use async_trait::async_trait;
use pingora::server::ShutdownWatch;
use pingora::services::background::BackgroundService;
use rand::RngCore;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::path::{Path, PathBuf};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::time::Duration;
use tokio::sync::{watch, Notify};

#[derive(Clone)]
pub struct Runtime {
    base_url: String,
    token_type: TokenType,
    token: String,
    state_dir: PathBuf,
    state: SnapshotStore,
    keys: CertificateKeyRing,
    client: Client,
    access_logs: AccessLogSpool,
    listeners: Listeners,
    geo_ip: crate::geoip::GeoIpStore,
    performance: LivePolicy,
    node_ip: node_ip::NodeIpResolver,
    ready: Arc<AtomicBool>,
    heartbeat_received: Arc<Notify>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum TokenType {
    Discovery,
    Node,
}

#[derive(Serialize)]
struct NodePayload<'a> {
    schema_version: u8,
    node_id: &'a str,
    name: &'a str,
    ip: &'a str,
    version: &'a str,
    ext_version: &'a str,
    current_version: &'a str,
    latest_config_version: &'a str,
    openresty_status: &'a str,
    openresty_message: &'a str,
    engine: &'a str,
    contract_version: u8,
    config_sync_version: u8,
    edge_update_supported: bool,
    edge_update_ready: bool,
    #[serde(skip_serializing_if = "Vec::is_empty", default)]
    access_logs: Vec<NodeAccessLog>,
    #[serde(skip_serializing_if = "str::is_empty")]
    last_error: &'a str,
    #[serde(flatten, skip_serializing_if = "Option::is_none")]
    telemetry: Option<HostTelemetry>,
}

#[derive(Debug, Deserialize)]
struct ApiResponse<T> {
    #[serde(default)]
    error_msg: String,
    data: Option<T>,
}

#[derive(Debug, Deserialize)]
struct Registration {
    node_id: String,
    agent_token: String,
}

#[derive(Debug, Default, Clone, Serialize, Deserialize)]
struct PersistedIdentity {
    node_id: String,
    agent_token: String,
    registration_key: Option<String>,
}

#[derive(Debug, Deserialize)]
struct Heartbeat {
    config: Manifest,
    #[serde(default)]
    agent_settings: Option<AgentSettings>,
}

#[derive(Debug, Deserialize)]
struct AgentSettings {
    #[serde(default = "default_heartbeat_interval")]
    heartbeat_interval: u32,
    #[serde(default)]
    restart_edge_now: bool,
    edge_update: Option<update::Settings>,
}

fn default_heartbeat_interval() -> u32 {
    3000
}

impl Runtime {
    pub fn new(
        server_url: String,
        token_type: String,
        token_file: PathBuf,
        state_dir: PathBuf,
        state: SnapshotStore,
        access_logs: AccessLogSpool,
        listeners: Listeners,
    ) -> Result<Self> {
        let token_type = match token_type.to_lowercase().as_str() {
            "discovery" => TokenType::Discovery,
            "node" => TokenType::Node,
            _ => anyhow::bail!("invalid token type; use discovery or node"),
        };
        let token = if matches!(token_type, TokenType::Discovery)
            && load_identity(&state_dir)?
                .is_some_and(|id| !id.node_id.is_empty() && !id.agent_token.is_empty())
        {
            String::new()
        } else {
            let token = std::fs::read_to_string(&token_file)
                .with_context(|| format!("failed to read token file {}", token_file.display()))?;
            if token.trim().is_empty() {
                anyhow::bail!("token file is empty");
            }
            token.trim().to_owned()
        };
        let url = url::Url::parse(&server_url).context("invalid control plane URL")?;
        let loopback = matches!(url.host_str(), Some("localhost" | "127.0.0.1" | "[::1]"));
        if url.scheme() != "https" && !(loopback && url.scheme() == "http") {
            anyhow::bail!("control plane URL requires HTTPS");
        }
        if !url.username().is_empty()
            || url.password().is_some()
            || url.query().is_some()
            || url.fragment().is_some()
            || url.host().is_none()
        {
            anyhow::bail!("invalid control plane URL components");
        }
        let base_url = url.as_str().trim_end_matches('/').to_owned();
        let client = Client::builder()
            .timeout(Duration::from_secs(15))
            .redirect(reqwest::redirect::Policy::none())
            .user_agent(edge_user_agent())
            .build()?;
        let performance = LivePolicy::new(NodePolicy::load(&state_dir));
        let node_ip = node_ip::NodeIpResolver::new(&base_url)?;
        Ok(Self {
            base_url,
            token_type,
            token,
            geo_ip: crate::geoip::GeoIpStore::open(state_dir.clone()),
            state_dir,
            state,
            keys: CertificateKeyRing::new(),
            client,
            access_logs,
            listeners,
            performance,
            node_ip,
            ready: Arc::default(),
            heartbeat_received: Arc::default(),
        })
    }

    pub fn with_key_ring(mut self, keys: CertificateKeyRing) -> Self {
        self.keys = keys;
        self
    }

    pub fn performance(&self) -> LivePolicy {
        self.performance.clone()
    }

    pub fn geo_ip(&self) -> crate::geoip::GeoIpStore {
        self.geo_ip.clone()
    }

    async fn run(&self) {
        let _lease = match crate::runtime::claim_control(&self.state_dir).await {
            Ok(lease) => lease,
            Err(error) => {
                tracing::error!(%error, "could not claim the Edge control loop");
                std::process::exit(1);
            }
        };
        self.state.recover_latest(&self.state_dir);
        tokio::join!(self.node_ip.initialize(), self.hydrate_persisted_keys());
        let node_ip_refresher = self.node_ip.clone();
        let control_loop = async {
            let mut retry = RetryDelay::default();
            let identity = loop {
                match self.ensure_identity().await {
                    Ok(identity) => break identity,
                    Err(error) => {
                        tracing::warn!(%error, "node enrollment failed");
                        retry.wait().await;
                    }
                }
            };
            let authenticated = Self {
                token_type: TokenType::Node,
                token: identity.agent_token,
                ..self.clone()
            };
            if let Some(current) = self.state.current() {
                if let Err(error) = authenticated.fetch_missing_keys(&current).await {
                    tracing::warn!(%error, "initial TLS private key hydration failed");
                }
            }
            let wake = websocket::Wake::default();
            let (plans, updates) = watch::channel(None);
            let (update_errors, errors) = watch::channel(None);
            tokio::join!(
                authenticated.run_heartbeats(&identity.node_id, &wake, plans, errors),
                websocket::run(
                    &authenticated.base_url,
                    &authenticated.token,
                    &wake,
                    &authenticated.keys
                ),
                update::run(&self.state_dir, updates, update_errors),
            );
        };
        tokio::join!(control_loop, node_ip_refresher.refresh_loop_if_needed());
    }

    async fn run_heartbeats(
        &self,
        node_id: &str,
        wake: &websocket::Wake,
        updates: watch::Sender<Option<update::UpdatePlan>>,
        errors: watch::Receiver<Option<String>>,
    ) {
        let mut interval = Duration::from_secs(3);
        let mut retry = RetryDelay::default();
        let mut site = SiteSync::open(&self.state_dir, self.state.current().as_deref());
        let mut collector = HostCollector::new(self.state_dir.clone());
        loop {
            let current = self.state.current();
            let update_error = errors.borrow().clone();
            let last_error = match (site.last_error(), update_error) {
                (config_error, Some(update_error)) if !config_error.is_empty() => {
                    format!("{config_error}\n{update_error}")
                }
                ("", Some(update_error)) => update_error,
                (config_error, _) => config_error.to_owned(),
            };
            let response = self
                .heartbeat(
                    node_id,
                    current.as_deref(),
                    site.latest_version(),
                    &last_error,
                    &mut collector,
                )
                .await;
            let response = match response {
                Ok(response) => response,
                Err(error) => {
                    tracing::warn!(%error, "node heartbeat failed");
                    retry.wait().await;
                    continue;
                }
            };
            retry = RetryDelay::default();
            if response.config.version == config_sync::VERSION {
                self.heartbeat_received.notify_one();
            }
            let plan = if let Some(settings) = response.agent_settings {
                interval = Duration::from_millis(settings.heartbeat_interval.max(100).into());
                if settings.restart_edge_now {
                    tracing::info!("controller requested a local Edge restart");
                    crate::runtime::request_restart();
                    return;
                }
                settings.edge_update.and_then(update::Settings::plan)
            } else {
                None
            };
            let plan = plan.filter(|_| self.ready.load(Ordering::Acquire));
            updates.send_if_modified(|previous| {
                if *previous == plan {
                    return false;
                }
                *previous = plan;
                true
            });
            let manifest = response.config;
            if manifest.version != config_sync::VERSION {
                tracing::error!(
                    version = manifest.version,
                    "unsupported Edge configuration sync protocol"
                );
                retry.wait().await;
                continue;
            }
            let performance_policy = match manifest.resources.get(PERFORMANCE_KEY) {
                Some(resource) => match resource.read(self, PERFORMANCE_KEY).await {
                    Ok(policy) => Some(policy),
                    Err(error) => {
                        tracing::error!(%error, "could not load Pingora performance policy");
                        None
                    }
                },
                None => None,
            };
            if current.is_some() && self.reconcile_performance(performance_policy.clone()) {
                return;
            }
            site.reconcile(self, node_id, &manifest, wake).await;
            if current.is_none() && self.reconcile_performance(performance_policy) {
                return;
            }
            tokio::select! {
                _ = tokio::time::sleep(interval) => {}
                _ = wake.changed() => {}
            }
        }
    }

    fn reconcile_performance(&self, desired: Option<NodePolicy>) -> bool {
        let Some(desired) = desired else {
            return false;
        };
        self.performance
            .reconcile(&self.state_dir, &self.listeners, desired)
            .unwrap_or_else(|error| {
                tracing::error!(%error, "could not apply Pingora performance policy");
                false
            })
    }

    async fn refresh_pages(&self) -> Result<()> {
        if let Some(current) = self.state.current() {
            if current.routes().any(|route| route.pages.is_some()) {
                let mut updated = (*current).clone();
                if self.prepare_pages(&mut updated).await? {
                    self.state.commit(&self.state_dir, updated)?;
                }
            }
        }
        Ok(())
    }

    async fn prepare_pages(&self, candidate: &mut AppliedConfig) -> Result<bool> {
        let syncer = PagesSyncer::new(
            self.base_url.clone(),
            self.token.clone(),
            self.client.clone(),
            self.state_dir.join("pages"),
        );
        let mut releases = std::collections::HashMap::new();
        for route in candidate.routes().filter(|route| route.is_pages()) {
            if let Some(deployment) = route.pages.as_ref() {
                if let std::collections::hash_map::Entry::Vacant(entry) =
                    releases.entry(deployment.project_id)
                {
                    entry.insert(syncer.ensure_project(deployment).await?);
                }
            }
        }
        let changed = candidate.pages_releases != releases;
        candidate.pages_releases = releases;
        Ok(changed)
    }

    async fn report_result(
        &self,
        node_id: &str,
        meta: &ActiveConfigMeta,
        result: &str,
        message: &str,
    ) {
        if let Err(error) = self.report_apply(node_id, meta, result, message).await {
            tracing::warn!(version = %meta.version, %error, "could not report application result");
        }
    }

    async fn ensure_identity(&self) -> Result<Registration> {
        let saved = load_identity(&self.state_dir)?;
        let node_token = match self.token_type {
            TokenType::Node => Some(self.token.as_str()),
            TokenType::Discovery => saved
                .as_ref()
                .filter(|identity| !identity.node_id.is_empty() && !identity.agent_token.is_empty())
                .map(|identity| identity.agent_token.as_str()),
        };
        let (registration, registration_key) = if let Some(token) = node_token {
            (
                self.register(token, None).await?,
                saved.and_then(|identity| identity.registration_key),
            )
        } else {
            let mut key = saved
                .and_then(|identity| identity.registration_key)
                .unwrap_or_default();
            if key.is_empty() {
                let mut bytes = [0u8; 32];
                rand::thread_rng().fill_bytes(&mut bytes);
                key = bytes.iter().map(|byte| format!("{byte:02x}")).collect();
                save_identity(
                    &self.state_dir,
                    &PersistedIdentity {
                        node_id: String::new(),
                        agent_token: String::new(),
                        registration_key: Some(key.clone()),
                    },
                )?;
            }
            (self.register(&self.token, Some(&key)).await?, Some(key))
        };
        let identity = PersistedIdentity {
            node_id: registration.node_id.clone(),
            agent_token: registration.agent_token.clone(),
            registration_key,
        };
        save_identity(&self.state_dir, &identity)?;
        Ok(Registration {
            node_id: identity.node_id,
            agent_token: identity.agent_token,
        })
    }

    async fn register(
        &self,
        bearer_token: &str,
        registration_key: Option<&str>,
    ) -> Result<Registration> {
        let node_ip = self.node_ip.current();
        let mut payload = json!(NodePayload {
            schema_version: 2,
            node_id: "",
            name: "openflare-edge",
            ip: &node_ip,
            version: EDGE_VERSION,
            ext_version: "pingora",
            current_version: self
                .state
                .current()
                .as_ref()
                .map_or("", |config| config.version.as_str()),
            latest_config_version: "",
            openresty_status: "unknown",
            openresty_message: "Pingora enrollment",
            engine: "pingora",
            contract_version: config::CONTRACT_VERSION as u8,
            config_sync_version: config_sync::VERSION,
            edge_update_supported: update::supported(),
            edge_update_ready: false,
            access_logs: Vec::new(),
            last_error: "",
            telemetry: None,
        });
        if let Some(registration_key) = registration_key {
            payload["registration_key"] = json!(registration_key);
        }
        let response = self
            .client
            .post(format!("{}/api/v1/agent/nodes/register", self.base_url))
            .header("X-Agent-Token", bearer_token)
            .json(&payload)
            .send()
            .await?;
        let registration: Registration = decode_response(response).await?;
        if registration.node_id.trim().is_empty() || registration.agent_token.trim().is_empty() {
            anyhow::bail!("registration response is missing node identity");
        }
        Ok(registration)
    }

    async fn heartbeat(
        &self,
        node_id: &str,
        current: Option<&AppliedConfig>,
        latest_version: &str,
        last_error: &str,
        collector: &mut HostCollector,
    ) -> Result<Heartbeat> {
        const MAX_HEARTBEAT_ACCESS_LOGS: usize = 1024;
        let spool = self.access_logs.clone();
        let logs = match tokio::task::spawn_blocking(move || spool.peek(MAX_HEARTBEAT_ACCESS_LOGS))
            .await
            .context("access log reader stopped")
            .and_then(|result| result)
        {
            Ok(logs) => logs,
            Err(error) => {
                tracing::warn!(%error, "access log spool unavailable");
                vec![]
            }
        };
        let (acknowledged, access_logs): (Vec<_>, Vec<_>) =
            logs.into_iter().map(|entry| (entry.id, entry.log)).unzip();
        let (status, message) = if !self.listeners.ready().await {
            ("unhealthy", "Pingora listener unavailable")
        } else if current.is_some() {
            ("healthy", "Pingora serving active configuration")
        } else {
            ("unknown", "Waiting for configuration")
        };
        let node_ip = self.node_ip.current();
        let payload = NodePayload {
            schema_version: 2,
            node_id,
            name: "openflare-edge",
            ip: &node_ip,
            version: EDGE_VERSION,
            ext_version: "pingora",
            current_version: current.map_or("", |config| config.version.as_str()),
            latest_config_version: latest_version,
            openresty_status: status,
            openresty_message: message,
            engine: "pingora",
            contract_version: config::CONTRACT_VERSION as u8,
            config_sync_version: config_sync::VERSION,
            edge_update_supported: update::supported(),
            edge_update_ready: self.ready.load(Ordering::Acquire),
            access_logs,
            last_error,
            telemetry: Some(collector.sample(status, message, last_error)),
        };
        let response = self
            .client
            .post(format!("{}/api/v1/agent/nodes/heartbeat", self.base_url))
            .header("X-Agent-Token", &self.token)
            .json(&payload)
            .send()
            .await?;
        self.clear_keys_if_revoked(&response);
        let decoded = decode_response(response).await?;
        if !acknowledged.is_empty() {
            let spool = self.access_logs.clone();
            if let Err(error) = tokio::task::spawn_blocking(move || spool.commit(acknowledged))
                .await
                .context("access log acknowledgement task stopped")
                .and_then(|result| result)
            {
                tracing::warn!(%error, "could not acknowledge access log batch");
            }
        }
        Ok(decoded)
    }

    async fn report_apply(
        &self,
        node_id: &str,
        config: &ActiveConfigMeta,
        result: &str,
        message: &str,
    ) -> Result<()> {
        let response = self
            .client
            .post(format!("{}/api/v1/agent/apply-logs", self.base_url))
            .header("X-Agent-Token", &self.token)
            .json(&json!({
                "node_id": node_id,
                "version": config.version,
                "result": result,
                "message": message,
                "checksum": config.checksum,
                "engine": "pingora",
                "release_sequence": config.release_sequence,
                "payload_checksum": config.payload_checksum,
            }))
            .send()
            .await?;
        decode_response::<serde_json::Value>(response).await?;
        Ok(())
    }

    fn clear_keys_if_revoked(&self, response: &reqwest::Response) -> bool {
        let quarantined = response
            .headers()
            .get("X-OpenFlare-Quarantined")
            .and_then(|value| value.to_str().ok())
            .is_some_and(|value| value.trim().eq_ignore_ascii_case("true"));
        let invalid_token = response.status() == reqwest::StatusCode::UNAUTHORIZED
            && response
                .headers()
                .get("X-OpenFlare-Auth-Error")
                .and_then(|value| value.to_str().ok())
                .is_some_and(|value| value.trim().eq_ignore_ascii_case("invalid_token"));
        if quarantined || invalid_token {
            tracing::warn!(
                status = %response.status(),
                quarantined,
                invalid_token,
                "control plane revoked node credentials; wiping in-memory TLS keys"
            );
            self.keys.clear();
            return true;
        }
        false
    }

    pub(super) async fn hydrate_persisted_keys(&self) {
        let Some(current) = self.state.current() else {
            return;
        };
        let token = match self.token_type {
            TokenType::Node => {
                let trimmed = self.token.trim();
                if trimmed.is_empty() {
                    return;
                }
                trimmed.to_owned()
            }
            TokenType::Discovery => match load_identity(&self.state_dir) {
                Ok(Some(identity)) if !identity.agent_token.trim().is_empty() => {
                    identity.agent_token
                }
                _ => return,
            },
        };
        let preauth = Self {
            token_type: TokenType::Node,
            token,
            ..self.clone()
        };
        match tokio::time::timeout(Duration::from_secs(3), preauth.fetch_missing_keys(&current))
            .await
        {
            Ok(Ok(())) => {}
            Ok(Err(error)) => {
                tracing::warn!(%error, "startup TLS private key hydration failed");
            }
            Err(_) => {
                tracing::warn!("startup TLS private key hydration timed out");
            }
        }
    }

    pub(super) async fn fetch_missing_keys(&self, config: &AppliedConfig) -> Result<()> {
        const MAX_TLS_KEY_BATCH: usize = 1024;
        const MAX_KEY_RESPONSE_BYTES: usize = 16 * 1024 * 1024;

        #[derive(Serialize)]
        struct TLSKeyRequestItem {
            certificate_id: u64,
            cert_sha256: String,
        }

        #[derive(Deserialize)]
        struct FetchTLSKeysData {
            #[serde(default)]
            keys: std::collections::HashMap<String, String>,
        }

        let missing = self.keys.missing_for(config);
        if missing.is_empty() {
            return Ok(());
        }

        for batch in missing.chunks(MAX_TLS_KEY_BATCH) {
            let certificates: Vec<TLSKeyRequestItem> = batch
                .iter()
                .map(|(certificate_id, cert_sha256)| TLSKeyRequestItem {
                    certificate_id: *certificate_id,
                    cert_sha256: cert_sha256.clone(),
                })
                .collect();
            let response = self
                .client
                .post(format!("{}/api/v1/agent/tls/keys", self.base_url))
                .header("X-Agent-Token", &self.token)
                .json(&json!({
                    "release_sequence": config.release_sequence,
                    "certificates": certificates,
                }))
                .send()
                .await?;
            self.clear_keys_if_revoked(&response);
            let mut response = response.error_for_status()?;
            if response
                .content_length()
                .is_some_and(|length| length > MAX_KEY_RESPONSE_BYTES as u64)
            {
                anyhow::bail!("TLS key response exceeds the size limit");
            }
            let mut body = zeroize::Zeroizing::new(Vec::new());
            while let Some(chunk) = response.chunk().await? {
                anyhow::ensure!(
                    body.len().saturating_add(chunk.len()) <= MAX_KEY_RESPONSE_BYTES,
                    "TLS key response exceeds the size limit"
                );
                body.extend_from_slice(&chunk);
            }
            let mut decoded: ApiResponse<FetchTLSKeysData> = serde_json::from_slice(&body)?;
            drop(body);
            if !decoded.error_msg.is_empty() {
                anyhow::bail!("server returned error: {}", decoded.error_msg);
            }
            let mut data = decoded
                .data
                .take()
                .ok_or_else(|| anyhow::anyhow!("server response has no TLS key data"))?;
            for (cert_id, _) in batch {
                let cert_key = cert_id.to_string();
                let key_pem = data
                    .keys
                    .remove(&cert_key)
                    .map(zeroize::Zeroizing::new)
                    .ok_or_else(|| {
                        anyhow::anyhow!("controller response omitted key for certificate {cert_id}")
                    })?;
                let loaded_cert = config.certificate_by_id(*cert_id).ok_or_else(|| {
                    anyhow::anyhow!("snapshot certificate {cert_id} disappeared during key install")
                })?;
                self.keys.install(*cert_id, loaded_cert, key_pem)?;
            }
            for (_, mut leftover) in data.keys.drain() {
                zeroize::Zeroize::zeroize(&mut leftover);
            }
        }
        Ok(())
    }
}

async fn decode_response<T: for<'de> Deserialize<'de>>(response: reqwest::Response) -> Result<T> {
    const MAX_CONTROL_RESPONSE_BYTES: usize = 128 * 1024 * 1024;
    let mut response = response.error_for_status()?;
    if response
        .content_length()
        .is_some_and(|length| length > MAX_CONTROL_RESPONSE_BYTES as u64)
    {
        anyhow::bail!("controller response exceeds the size limit");
    }
    let mut body = Vec::new();
    while let Some(chunk) = response.chunk().await? {
        anyhow::ensure!(
            body.len().saturating_add(chunk.len()) <= MAX_CONTROL_RESPONSE_BYTES,
            "controller response exceeds the size limit"
        );
        body.extend_from_slice(&chunk);
    }
    let decoded: ApiResponse<T> = serde_json::from_slice(&body)?;
    if !decoded.error_msg.is_empty() {
        anyhow::bail!("server returned error: {}", decoded.error_msg);
    }
    decoded
        .data
        .ok_or_else(|| anyhow::anyhow!("server response has no data"))
}

#[async_trait]
impl BackgroundService for Runtime {
    async fn start(&self, mut shutdown: ShutdownWatch) {
        tokio::select! {
            _ = async {
                let readiness = async {
                    if update::trial_running(&self.state_dir) {
                        self.heartbeat_received.notified().await;
                    }
                    if let Err(error) = crate::runtime::announce_ready(&self.listeners, &self.state_dir, &self.state).await {
                        tracing::error!(%error, "Edge data plane failed to become ready");
                        std::process::exit(1);
                    }
                    self.ready.store(true, Ordering::Release);
                };
                tokio::join!(self.run(), self.geo_ip.refresh_periodically(), readiness);
            } => {}
            _ = shutdown.changed() => {}
        }
    }
}

#[derive(Default)]
struct RetryDelay {
    attempts: u32,
}

impl RetryDelay {
    async fn wait(&mut self) {
        let seconds = 1_u64 << self.attempts.min(6);
        let millis = rand::random::<u64>() % (seconds * 500);
        self.attempts = self.attempts.saturating_add(1);
        tokio::time::sleep(Duration::from_millis(seconds * 500 + millis)).await;
    }
}

fn identity_path(state_dir: &Path) -> PathBuf {
    state_dir.join("identity.json")
}

fn load_identity(state_dir: &Path) -> Result<Option<PersistedIdentity>> {
    let path = identity_path(state_dir);
    if !path.exists() {
        return Ok(None);
    }
    let identity = serde_json::from_slice(&std::fs::read(&path)?)
        .with_context(|| format!("failed to decode identity file {}", path.display()))?;
    Ok(Some(identity))
}

fn save_identity(state_dir: &Path, identity: &PersistedIdentity) -> Result<()> {
    std::fs::create_dir_all(state_dir)?;
    config::persist_private_json(&identity_path(state_dir), identity)
}

#[cfg(test)]
mod tests {
    use super::*;
    use sha2::{Digest, Sha256};

    #[test]
    fn registration_decodes_the_controller_response() {
        let payload = r#"{"node_id":"node-1","agent_token":"issued","access_token":"issued","name":"Fixture"}"#;
        let registration: Registration = serde_json::from_str(payload).expect("registration");
        assert_eq!(registration.agent_token, "issued");
    }

    #[test]
    fn decodes_null_pow_lists_as_empty() {
        let payload = r#"{
          "schema_version":1,
          "contract_version":4,
          "required_capabilities":["tls-sni-h2-v1","policy-runtime-v1","shared-snapshot-v1","tls-memory-keys-v1"],
          "snapshot":{
          "routes":[{
            "id":1,
            "site_name":"example",
            "domains":["example.com"],
            "domain_cert_ids":[0],
            "origin_url":"http://127.0.0.1:9080",
            "upstreams":[],
            "enabled":true,
            "enable_https":true,
            "enable_http2":true,
            "redirect_http":true,
            "pow_config":{
              "difficulty":4,
              "algorithm":"fast",
              "session_ttl":600,
              "challenge_ttl":300,
              "whitelist":{"ips":null,"ip_cidrs":null,"paths":null,"path_regexes":null,"user_agents":null},
              "blacklist":{"ips":null,"ip_cidrs":null,"paths":null,"path_regexes":null,"user_agents":null}
            }
          }],
          "openresty_config":{}, "waf":{"rule_groups":[],"bindings":[]}},
          "assets":{"certificates":{}}
        }"#;
        let digest = format!("{:x}", Sha256::digest(payload.as_bytes()));
        let config = crate::config::decode("v1", "checksum", 1, &digest, payload)
            .expect("payload with null PoW lists decodes");
        let route = config
            .routes()
            .next()
            .expect("payload contains an enabled route");
        let pow = route.pow.as_ref().expect("PoW config");
        assert!(pow.whitelist.ips.is_empty());
        assert!(pow.blacklist.user_agents.is_empty());
    }
}
