use crate::config::{self, AppliedConfig};
use crate::pages::PagesSyncer;
use crate::policy::{AccessLogBuffer, NodeAccessLog};
use crate::proxy::DataPlaneState;
use anyhow::{Context, Result};
use rand::RngCore;
use reqwest::{Client, StatusCode};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::{Arc, RwLock};
use std::time::Duration;

#[derive(Debug)]
pub struct Runtime {
    base_url: String,
    token_type: TokenType,
    token: String,
    state_dir: PathBuf,
    http_listen: String,
    tls_listen: Option<String>,
    client: Client,
    access_logs: AccessLogBuffer,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum TokenType {
    Discovery,
    Node,
}

#[derive(Debug, Clone, Serialize)]
struct NodePayload<'a> {
    schema_version: u8,
    node_id: &'a str,
    name: &'a str,
    version: &'a str,
    ext_version: &'a str,
    current_version: &'a str,
    openresty_status: &'a str,
    openresty_message: &'a str,
    engine: &'a str,
    contract_version: u8,
    #[serde(skip_serializing_if = "Vec::is_empty", default)]
    access_logs: Vec<NodeAccessLog>,
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
    #[allow(dead_code)]
    #[serde(alias = "agent_token", alias = "access_token")]
    agent_token: String,
}

#[derive(Debug, Default, Clone, Serialize, Deserialize)]
struct PersistedIdentity {
    node_id: String,
    agent_token: String,
    registration_key: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
struct Heartbeat {
    #[serde(default)]
    agent_settings: Option<AgentSettings>,
    #[serde(default)]
    active_config: Option<ActiveConfigMeta>,
}

#[derive(Debug, Clone, Deserialize)]
struct AgentSettings {
    #[serde(default = "default_heartbeat_interval")]
    heartbeat_interval: u32,
}

#[derive(Debug, Clone, Deserialize)]
struct ActiveConfigMeta {
    version: String,
    checksum: String,
    #[serde(default)]
    engine: String,
    #[serde(default)]
    release_sequence: i64,
    #[serde(default)]
    payload_checksum: String,
}

#[derive(Debug, Clone, Deserialize)]
struct ActiveConfig {
    version: String,
    checksum: String,
    #[serde(default)]
    #[allow(dead_code)]
    engine: String,
    #[serde(default)]
    release_sequence: i64,
    payload_json: String,
    payload_checksum: String,
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
        http_listen: String,
        tls_listen: Option<String>,
    ) -> Result<Self> {
        let token = std::fs::read_to_string(&token_file)
            .with_context(|| format!("failed to read token file {}", token_file.display()))?;
        let token = token.trim().to_owned();
        if token.is_empty() {
            anyhow::bail!("token file is empty");
        }
        let token_type = match token_type.to_lowercase().as_str() {
            "discovery" => TokenType::Discovery,
            "node" => TokenType::Node,
            _ => anyhow::bail!("invalid token type; use discovery or node"),
        };
        let base_url = server_url.trim_end_matches('/').to_owned();
        let client = Client::builder()
            .timeout(Duration::from_secs(15))
            .user_agent(concat!("openflare-edge/", env!("CARGO_PKG_VERSION")))
            .build()?;
        Ok(Self {
            base_url,
            token_type,
            token,
            state_dir: state_dir.clone(),
            http_listen,
            tls_listen,
            client,
            access_logs: AccessLogBuffer::persistent(state_dir),
        })
    }

    pub async fn run(self) -> Result<()> {
        let identity = self.ensure_identity().await?;
        let state = SharedState::new();
        if let Some(restored) = config::restore_snapshot(&self.state_dir)? {
            state.set_config(restored)?;
        }
        start_and_supervise_data_plane(
            self.state_dir.clone(),
            self.http_listen.clone(),
            self.tls_listen.clone(),
        )
        .await?;

        let mut interval = Duration::from_millis(3000);
        loop {
            let authenticated = self.with_identity(&identity).await?;
            let heartbeat = authenticated
                .heartbeat(
                    &identity.node_id,
                    state.current().map(|config| config.version),
                )
                .await;
            if let Ok(response) = heartbeat {
                if let Some(settings) = response.agent_settings {
                    interval = Duration::from_millis(settings.heartbeat_interval.max(100).into());
                }
                if let Some(meta) = response
                    .active_config
                    .filter(|meta| meta.engine == "pingora")
                {
                    let current = state.current();
                    let unchanged = current.is_some_and(|config| {
                        config.version == meta.version
                            && config.payload_checksum == meta.payload_checksum
                    });
                    if !unchanged {
                        match authenticated.fetch_active_config().await {
                            Ok(remote) => {
                                match config::decode(
                                    &remote.version,
                                    &remote.checksum,
                                    remote.release_sequence,
                                    &remote.payload_checksum,
                                    &remote.payload_json,
                                ) {
                                    Ok(applied) => {
                                        if let Err(error) = state.set_config(applied.clone()) {
                                            tracing::error!(version = %remote.version, error = %error, "failed to apply configuration");
                                            let _ = authenticated
                                                .report_apply(
                                                    &identity.node_id,
                                                    &applied,
                                                    "failed",
                                                    &error.to_string(),
                                                )
                                                .await;
                                        } else {
                                            for route in
                                                applied.routes().filter(|route| route.is_pages())
                                            {
                                                if let Some(deployment) = route.pages.as_ref() {
                                                    let syncer = PagesSyncer::new(
                                                        authenticated.base_url.clone(),
                                                        authenticated.token.clone(),
                                                        authenticated.client.clone(),
                                                        self.state_dir.join("pages"),
                                                    );
                                                    if let Err(error) =
                                                        syncer.ensure_project(deployment).await
                                                    {
                                                        tracing::error!(project_id = deployment.project_id, error = %error, "failed to sync Pages project");
                                                    }
                                                }
                                            }
                                            config::persist_snapshot(&self.state_dir, &applied)?;
                                            let _ = authenticated
                                                .report_apply(
                                                    &identity.node_id,
                                                    &applied,
                                                    "success",
                                                    "configuration applied",
                                                )
                                                .await;
                                        }
                                    }
                                    Err(error) => {
                                        let _ = authenticated
                                            .report_apply(
                                                &identity.node_id,
                                                &fallback_applied_config(&meta),
                                                "failed",
                                                &error.to_string(),
                                            )
                                            .await;
                                    }
                                }
                            }
                            Err(error) => {
                                tracing::warn!(error = %error, "failed to fetch active configuration")
                            }
                        }
                    }
                }
            }
            tokio::time::sleep(interval).await;
        }
    }

    async fn ensure_identity(&self) -> Result<Registration> {
        if let Some(mut identity) = load_identity(&self.state_dir)? {
            if self.token_type == TokenType::Node || !identity.agent_token.is_empty() {
                let registration = self.register(&identity.agent_token, None).await?;
                identity.node_id = registration.node_id;
                save_identity(&self.state_dir, &identity)?;
                return Ok(Registration {
                    node_id: identity.node_id,
                    agent_token: identity.agent_token,
                });
            }
        }
        if self.token_type == TokenType::Node {
            let registration = self.register(&self.token, None).await?;
            save_identity(
                &self.state_dir,
                &PersistedIdentity {
                    node_id: registration.node_id.clone(),
                    agent_token: registration.agent_token.clone(),
                    registration_key: None,
                },
            )?;
            return Ok(Registration {
                node_id: registration.node_id,
                agent_token: registration.agent_token,
            });
        }

        let mut registration_key = load_identity(&self.state_dir)?
            .and_then(|identity| identity.registration_key)
            .unwrap_or_else(String::new);
        if registration_key.is_empty() {
            let mut bytes = [0u8; 32];
            rand::thread_rng().fill_bytes(&mut bytes);
            registration_key = bytes.iter().map(|byte| format!("{byte:02x}")).collect();
            save_identity(
                &self.state_dir,
                &PersistedIdentity {
                    node_id: String::new(),
                    agent_token: String::new(),
                    registration_key: Some(registration_key.clone()),
                },
            )?;
        }
        let registration = self.register(&self.token, Some(&registration_key)).await?;
        let identity = PersistedIdentity {
            node_id: registration.node_id.clone(),
            agent_token: registration.agent_token.clone(),
            registration_key: Some(registration_key),
        };
        save_identity(&self.state_dir, &identity)?;
        Ok(Registration {
            node_id: identity.node_id,
            agent_token: identity.agent_token,
        })
    }

    async fn with_identity(&self, identity: &Registration) -> Result<Self> {
        Ok(Self {
            base_url: self.base_url.clone(),
            token_type: TokenType::Node,
            token: identity.agent_token.clone(),
            state_dir: self.state_dir.clone(),
            http_listen: self.http_listen.clone(),
            tls_listen: self.tls_listen.clone(),
            client: self.client.clone(),
            access_logs: self.access_logs.clone(),
        })
    }

    async fn register(
        &self,
        bearer_token: &str,
        registration_key: Option<&str>,
    ) -> Result<Registration> {
        let mut payload = json!(NodePayload {
            schema_version: 2,
            node_id: "",
            name: "openflare-edge",
            version: env!("CARGO_PKG_VERSION"),
            ext_version: "pingora",
            current_version: "",
            openresty_status: "healthy",
            openresty_message: "Pingora default runtime",
            engine: "pingora",
            contract_version: 2,
            access_logs: Vec::new(),
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
        let registration = decode_response(response).await?;
        Ok(registration)
    }

    async fn heartbeat(&self, node_id: &str, current_version: Option<String>) -> Result<Heartbeat> {
        const MAX_HEARTBEAT_ACCESS_LOGS: usize = 1024;
        let logs = self.access_logs.peek(MAX_HEARTBEAT_ACCESS_LOGS);
        let payload = NodePayload {
            schema_version: 2,
            node_id,
            name: "openflare-edge",
            version: env!("CARGO_PKG_VERSION"),
            ext_version: "pingora",
            current_version: current_version.as_deref().unwrap_or_default(),
            openresty_status: "healthy",
            openresty_message: "Pingora default runtime",
            engine: "pingora",
            contract_version: 2,
            access_logs: logs.clone(),
        };
        let response = self
            .client
            .post(format!("{}/api/v1/agent/nodes/heartbeat", self.base_url))
            .header("X-Agent-Token", &self.token)
            .json(&payload)
            .send()
            .await?;
        let decoded = decode_response(response).await?;
        self.access_logs.commit(logs.len());
        Ok(decoded)
    }

    async fn fetch_active_config(&self) -> Result<ActiveConfig> {
        let response = self
            .client
            .get(format!(
                "{}/api/v1/agent/pingora/config-versions/active",
                self.base_url
            ))
            .header("X-Agent-Token", &self.token)
            .send()
            .await?;
        decode_response(response).await
    }

    async fn report_apply(
        &self,
        node_id: &str,
        config: &AppliedConfig,
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
}

async fn decode_response<T: for<'de> Deserialize<'de>>(response: reqwest::Response) -> Result<T> {
    let status = response.status();
    let body = response.text().await?;
    if status != StatusCode::OK {
        anyhow::bail!("server returned {} {}", status.as_u16(), body);
    }
    let decoded: ApiResponse<T> = serde_json::from_str(&body)?;
    if !decoded.error_msg.is_empty() {
        anyhow::bail!("server returned error: {}", decoded.error_msg);
    }
    decoded
        .data
        .ok_or_else(|| anyhow::anyhow!("server response has no data"))
}

#[derive(Clone, Debug)]
pub struct SharedState {
    inner: Arc<RwLock<Option<AppliedConfig>>>,
}

impl SharedState {
    fn new() -> Self {
        Self {
            inner: Arc::new(RwLock::new(None)),
        }
    }

    pub fn current(&self) -> Option<AppliedConfig> {
        self.inner.read().ok().and_then(|guard| guard.clone())
    }

    pub fn set_config(&self, config: AppliedConfig) -> Result<()> {
        let mut guard = self
            .inner
            .write()
            .map_err(|error| anyhow::anyhow!(error.to_string()))?;
        *guard = Some(config);
        Ok(())
    }
}

fn spawn_data_plane(
    state_dir: PathBuf,
    http_listen: String,
    tls_listen: Option<String>,
) -> Result<std::process::Child> {
    let mut command = Command::new(std::env::current_exe()?);
    command
        .arg("run")
        .arg("--data-plane")
        .arg("--server-url")
        .arg("local://data-plane")
        .arg("--token-type")
        .arg("node")
        .arg("--token-file")
        .arg(identity_path(&state_dir))
        .arg("--state-dir")
        .arg(&state_dir)
        .arg("--http-listen")
        .arg(&http_listen);
    if let Some(listen) = tls_listen.as_ref() {
        command.arg("--tls-listen").arg(listen);
    }
    command
        .spawn()
        .with_context(|| "failed to start OpenFlare Edge data plane")
}

#[cfg(not(target_os = "windows"))]
#[cfg(not(target_os = "windows"))]
fn data_plane_pid(state_dir: &Path) -> Option<u32> {
    std::fs::read_to_string(state_dir.join("openflare-edge.pid"))
        .ok()
        .and_then(|content| content.trim().parse::<u32>().ok())
}

#[cfg(not(target_os = "windows"))]
fn process_running(pid: u32) -> bool {
    pid != 0 && unsafe { libc::kill(pid as libc::pid_t, 0) == 0 }
}

#[cfg(not(target_os = "windows"))]
fn signal_process(pid: u32, signal: i32) -> bool {
    pid != 0 && unsafe { libc::kill(pid as libc::pid_t, signal) == 0 }
}

#[cfg(not(target_os = "windows"))]
async fn wait_for_process_exit(pid: u32) {
    while process_running(pid) {
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}

#[cfg(not(target_os = "windows"))]
async fn monitor_data_plane(
    state_dir: PathBuf,
    launcher_pid: u32,
    previous_pid: Option<u32>,
) -> Result<u32> {
    let pid_path = state_dir.join("openflare-edge.pid");
    let timeout = tokio::time::Instant::now() + Duration::from_secs(60);
    let mut ready_pid: Option<u32> = None;
    loop {
        if tokio::time::Instant::now() >= timeout {
            anyhow::bail!("data plane did not report a ready pid within 60 seconds");
        }
        if ready_pid.is_none() {
            if let Ok(content) = std::fs::read_to_string(&pid_path) {
                ready_pid = content.trim().parse::<u32>().ok();
            }
        }
        if let Some(pid) = ready_pid {
            if previous_pid.map(|old| pid != old).unwrap_or(true) {
                return Ok(pid);
            }
            ready_pid = None;
        }
        let launcher_running =
            launcher_pid != 0 && unsafe { libc::kill(launcher_pid as libc::pid_t, 0) == 0 };
        if !launcher_running {
            anyhow::bail!("data plane launcher exited before reporting readiness");
        }
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}

#[cfg(not(target_os = "windows"))]
async fn start_and_supervise_data_plane(
    state_dir: PathBuf,
    http_listen: String,
    tls_listen: Option<String>,
) -> Result<()> {
    let child = spawn_data_plane(state_dir.clone(), http_listen.clone(), tls_listen.clone())?;
    let initial_pid =
        monitor_data_plane(state_dir.clone(), child.id(), data_plane_pid(&state_dir)).await?;
    let supervisor_state_dir = state_dir.clone();
    tokio::spawn(async move {
        let mut quit_signal = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::quit())
            .expect("failed to listen for SIGQUIT");
        let mut terminate_signal =
            tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
                .expect("failed to listen for SIGTERM");
        let mut interrupt_signal =
            tokio::signal::unix::signal(tokio::signal::unix::SignalKind::interrupt())
                .expect("failed to listen for SIGINT");
        let mut current_pid = initial_pid;
        let shutting_down = false;
        loop {
            tokio::select! {
                _ = quit_signal.recv() => {
                    if shutting_down {
                        continue;
                    }
                    if let Ok(child) = spawn_data_plane(
                        supervisor_state_dir.clone(),
                        http_listen.clone(),
                        tls_listen.clone(),
                    ) {
                        match monitor_data_plane(
                            supervisor_state_dir.clone(),
                            child.id(),
                            Some(current_pid),
                        )
                        .await
                        {
                            Ok(new_pid) => {
                                if signal_process(current_pid, libc::SIGQUIT) {
                                    wait_for_process_exit(current_pid).await;
                                }
                                current_pid = new_pid;
                            }
                            Err(error) => {
                                tracing::error!(error = %error, "failed to start replacement data plane");
                            }
                        }
                    }
                }
                _ = terminate_signal.recv() => {
                    signal_process(current_pid, libc::SIGTERM);
                    wait_for_process_exit(current_pid).await;
                    std::process::exit(0);
                }
                _ = interrupt_signal.recv() => {
                    signal_process(current_pid, libc::SIGINT);
                    wait_for_process_exit(current_pid).await;
                    std::process::exit(0);
                }
                _ = tokio::time::sleep(Duration::from_secs(1)) => {
                    if shutting_down || process_running(current_pid) {
                        continue;
                    }
                    tracing::warn!(pid = current_pid, "data plane exited; restarting");
                    if let Ok(child) = spawn_data_plane(
                        supervisor_state_dir.clone(),
                        http_listen.clone(),
                        tls_listen.clone(),
                    ) {
                        match monitor_data_plane(
                            supervisor_state_dir.clone(),
                            child.id(),
                            Some(current_pid),
                        )
                        .await
                        {
                            Ok(new_pid) => current_pid = new_pid,
                            Err(error) => tracing::error!(error = %error, "failed to restart data plane"),
                        }
                    }
                }
            }
        }
    });
    Ok(())
}

#[cfg(target_os = "windows")]
async fn start_and_supervise_data_plane(
    _state_dir: PathBuf,
    _http_listen: String,
    _tls_listen: Option<String>,
) -> Result<()> {
    anyhow::bail!("OpenFlare Edge data plane requires Unix signal and FD transfer support")
}

pub fn load_restored_state(
    state_dir: PathBuf,
    http_listen: String,
    tls_listen: Option<String>,
) -> Result<DataPlaneState> {
    let access_logs = AccessLogBuffer::persistent(&state_dir);
    let shared = SharedState::new();
    if let Some(restored) = config::restore_snapshot(&state_dir)? {
        shared.set_config(restored)?;
    }
    Ok(DataPlaneState {
        shared,
        state_dir,
        http_listen,
        tls_listen,
        access_logs,
    })
}

fn identity_path(state_dir: &Path) -> PathBuf {
    state_dir.join("identity.json")
}

fn fallback_applied_config(meta: &ActiveConfigMeta) -> AppliedConfig {
    AppliedConfig {
        version: meta.version.clone(),
        checksum: meta.checksum.clone(),
        release_sequence: meta.release_sequence,
        payload_checksum: meta.payload_checksum.clone(),
        config: crate::config::Config {
            schema_version: 1,
            contract_version: 2,
            required_capabilities: vec![],
            routes: vec![],
            waf: Default::default(),
            assets: crate::config::Assets::default(),
            pow_page: Default::default(),
        },
        certificates: Default::default(),
    }
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
    let temporary = state_dir.join("identity.json.tmp");
    let final_path = identity_path(state_dir);
    std::fs::write(&temporary, serde_json::to_vec_pretty(identity)?)?;
    std::fs::rename(&temporary, &final_path)?;
    Ok(())
}
