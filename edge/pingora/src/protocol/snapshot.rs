use super::config::{
    deserialize_null_default_vec, Assets, CachePolicy, Config, LimitConfig, OriginErrorPage,
    PoWPageConfig, Route, RouteCache, ServiceWorker, WafDocument,
};
use anyhow::{bail, Context, Result};
use serde::Deserialize;

#[derive(Deserialize)]
pub(crate) struct SharedSnapshotEnvelope {
    schema_version: i64,
    contract_version: i64,
    required_capabilities: Vec<String>,
    snapshot: SharedSnapshot,
    #[serde(flatten)]
    assets: Resources,
}

#[derive(Deserialize)]
struct SharedSnapshot {
    routes: Vec<SharedRoute>,
    waf: WafDocument,
    #[serde(rename = "openresty_config")]
    settings: Settings,
}

#[derive(Deserialize)]
struct SharedRoute {
    #[serde(flatten)]
    route: Route,
    #[serde(flatten)]
    limits: RouteLimits,
    #[serde(default)]
    cache_enabled: bool,
    #[serde(default)]
    cache_policy: String,
    #[serde(default)]
    cache_rules: Vec<String>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct RouteLimits {
    limit_conn_per_server: i64,
    limit_conn_per_ip: i64,
    limit_rate: String,
    limit_req_per_ip: String,
}

#[derive(Deserialize)]
struct Resources {
    assets: AssetBundle,
}

#[derive(Deserialize)]
struct AssetBundle {
    #[serde(flatten)]
    tls: Assets,
    #[serde(default)]
    static_cache_extensions: Vec<String>,
    #[serde(default)]
    origin_error_html: String,
    #[serde(default)]
    service_worker_script: String,
    #[serde(default)]
    service_worker_offline_html: String,
}

#[derive(Deserialize)]
#[serde(default)]
struct Settings {
    #[serde(deserialize_with = "deserialize_null_default_vec")]
    trusted_proxy_cidrs: Vec<String>,
    default_server_return_status: u16,
    client_max_body_size: String,
    websocket_enabled: bool,
    gzip_enabled: bool,
    gzip_min_length: u64,
    cache_enabled: bool,
    cache_key_template: String,
    cache_lock_enabled: bool,
    cache_lock_timeout: String,
    cache_use_stale: String,
    default_limit_conn_per_server: i64,
    default_limit_conn_per_ip: i64,
    default_limit_rate: String,
    default_limit_req_per_ip: String,
    origin_error_page_enabled: bool,
    origin_error_page_status_codes: Vec<String>,
    origin_error_page_get_only: bool,
    sw_offline_enabled: bool,
    sw_offline_domains: Vec<String>,
    pow_page: PoWPageConfig,
    site_auth_html: String,
}

impl Default for Settings {
    fn default() -> Self {
        Self {
            trusted_proxy_cidrs: vec![],
            default_server_return_status: 421,
            client_max_body_size: "64m".to_owned(),
            websocket_enabled: true,
            gzip_enabled: true,
            gzip_min_length: 1024,
            cache_enabled: false,
            cache_key_template: String::new(),
            cache_lock_enabled: true,
            cache_lock_timeout: "5s".to_owned(),
            cache_use_stale: String::new(),
            default_limit_conn_per_server: 0,
            default_limit_conn_per_ip: 0,
            default_limit_rate: String::new(),
            default_limit_req_per_ip: String::new(),
            origin_error_page_enabled: false,
            origin_error_page_status_codes: vec![],
            origin_error_page_get_only: false,
            sw_offline_enabled: false,
            sw_offline_domains: vec![],
            pow_page: PoWPageConfig::default(),
            site_auth_html: String::new(),
        }
    }
}

impl SharedSnapshotEnvelope {
    pub(crate) fn compile(self) -> Result<Config> {
        let settings = self.snapshot.settings;
        let assets = self.assets.assets;
        let routes = self
            .snapshot
            .routes
            .into_iter()
            .map(|entry| entry.compile(&settings))
            .collect();
        let cache = if settings.cache_enabled {
            let timeout = duration_millis(&settings.cache_lock_timeout)?;
            Some(CachePolicy {
                key_template: settings.cache_key_template,
                lock_enabled: settings.cache_lock_enabled,
                lock_timeout_ms: timeout,
                stale_on_error: settings
                    .cache_use_stale
                    .split_whitespace()
                    .any(|value| matches!(value, "error" | "timeout")),
                stale_updating: settings
                    .cache_use_stale
                    .split_whitespace()
                    .any(|value| value == "updating"),
                static_extensions: assets.static_cache_extensions,
            })
        } else {
            None
        };
        let origin_error_page = if settings.origin_error_page_enabled {
            if assets.origin_error_html.is_empty() {
                bail!("shared snapshot is missing the origin error page asset");
            }
            Some(OriginErrorPage {
                status_codes: expand_status_codes(&settings.origin_error_page_status_codes)?,
                html: assets.origin_error_html,
                get_only: settings.origin_error_page_get_only,
            })
        } else {
            None
        };
        let service_worker =
            if settings.sw_offline_enabled && !settings.sw_offline_domains.is_empty() {
                if assets.service_worker_script.is_empty()
                    || assets.service_worker_offline_html.is_empty()
                {
                    bail!("shared snapshot is missing Service Worker assets");
                }
                Some(ServiceWorker {
                    domains: settings.sw_offline_domains,
                    script: assets.service_worker_script,
                    offline_html: assets.service_worker_offline_html,
                })
            } else {
                None
            };
        Ok(Config {
            schema_version: self.schema_version,
            contract_version: self.contract_version,
            required_capabilities: self.required_capabilities,
            routes,
            waf: self.snapshot.waf,
            assets: assets.tls,
            pow_page: settings.pow_page,
            site_auth_html: settings.site_auth_html,
            trusted_proxy_cidrs: settings.trusted_proxy_cidrs,
            default_server_return_status: settings.default_server_return_status,
            max_request_body: byte_size(&settings.client_max_body_size)?,
            origin_error_page,
            service_worker,
            gzip: settings.gzip_enabled,
            gzip_min_length: settings.gzip_min_length,
            websocket: settings.websocket_enabled,
            cache,
        })
    }
}

impl SharedRoute {
    fn compile(self, settings: &Settings) -> Route {
        let conn = |value: i64, default: i64| if value == 0 { default } else { value.max(0) };
        let mut route = self.route;
        route.limit = LimitConfig {
            conn_per_server: conn(
                self.limits.limit_conn_per_server,
                settings.default_limit_conn_per_server,
            ),
            conn_per_ip: conn(
                self.limits.limit_conn_per_ip,
                settings.default_limit_conn_per_ip,
            ),
            rate: limit_value(self.limits.limit_rate, &settings.default_limit_rate),
            req_per_ip: limit_value(
                self.limits.limit_req_per_ip,
                &settings.default_limit_req_per_ip,
            ),
        };
        route.cache = (self.cache_enabled && settings.cache_enabled).then_some(RouteCache {
            policy: self.cache_policy,
            rules: self.cache_rules,
        });
        route
    }
}

fn limit_value(value: String, default: &str) -> String {
    match value.trim() {
        "" | "0" => default.to_owned(),
        "-1" => String::new(),
        _ => value,
    }
}

pub(crate) fn byte_size(input: &str) -> Result<u64> {
    let input = input.trim();
    let (number, multiplier) = match input.as_bytes().last() {
        Some(b'k' | b'K') => (&input[..input.len() - 1], 1024),
        Some(b'm' | b'M') => (&input[..input.len() - 1], 1024 * 1024),
        Some(b'g' | b'G') => (&input[..input.len() - 1], 1024 * 1024 * 1024),
        _ => (input, 1),
    };
    number
        .parse::<u64>()
        .ok()
        .and_then(|value| value.checked_mul(multiplier))
        .with_context(|| format!("invalid request size {input}"))
}

fn duration_millis(input: &str) -> Result<u64> {
    let input = input.trim();
    let (number, multiplier) = if let Some(number) = input.strip_suffix("ms") {
        (number, 1)
    } else if let Some(number) = input.strip_suffix('s') {
        (number, 1_000)
    } else if let Some(number) = input.strip_suffix('m') {
        (number, 60_000)
    } else if let Some(number) = input.strip_suffix('h') {
        (number, 3_600_000)
    } else {
        bail!("unsupported cache lock timeout {input}");
    };
    number
        .parse::<u64>()
        .ok()
        .and_then(|value| value.checked_mul(multiplier))
        .filter(|value| *value > 0)
        .with_context(|| format!("invalid cache lock timeout {input}"))
}

fn expand_status_codes(tags: &[String]) -> Result<Vec<u16>> {
    if tags.is_empty() {
        return Ok((500..=599).collect());
    }
    let mut codes = Vec::new();
    for tag in tags {
        let parse = |value: &str| {
            value
                .trim()
                .parse::<u16>()
                .ok()
                .filter(|code| (400..=599).contains(code))
        };
        let (first, last) = tag.split_once('-').unwrap_or((tag, tag));
        let first = parse(first).context("invalid origin error status")?;
        let last = parse(last)
            .filter(|last| *last >= first)
            .context("invalid origin error status range")?;
        codes.extend(first..=last);
    }
    codes.sort_unstable();
    codes.dedup();
    Ok(codes)
}
