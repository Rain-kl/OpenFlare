use crate::config::{PagesDeployment, PoWConfig, PoWPathMatch, Route, WafDocument};
use base64::{engine::general_purpose::STANDARD as Base64, Engine as _};
use hmac::{Hmac, Mac};
use maxminddb::Reader;
use rand::RngCore;
use regex::Regex;
use serde::{Deserialize, Serialize};
use sha1::Sha1 as Sha1Digest;
use sha2::{Digest, Sha256};
use std::collections::{HashMap, VecDeque};
use std::net::IpAddr;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::sync::Mutex;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

pub const SITE_AUTH_COOKIE: &str = "__Host-openflare_site_auth";
pub const SITE_AUTH_PATH: &str = "/.openflare/site-auth";
pub const POW_COOKIE: &str = "__openflare_pow";
pub const POW_API_PREFIX: &str = "/.within.website/x/cmd/anubis/api/";
pub const POW_STATIC_PREFIX: &str = "/.within.website/x/cmd/anubis/static/";

#[derive(Debug, Clone, Default)]
pub struct RequestContext {
    pub method: String,
    pub host: String,
    pub path: String,
    pub query: String,
    pub remote_addr: String,
    pub user_agent: String,
    pub cookie: String,
    pub authorization: String,
    pub content_type: String,
    pub body: Vec<u8>,
    pub origin_header: Option<String>,
    pub accept_language: String,
    pub geo: Option<GeoResult>,
}

#[derive(Debug, Clone, Default)]
pub struct GeoResult {
    pub country: Option<String>,
    pub region: Option<String>,
}

pub struct GeoIp {
    city: Option<Reader<Vec<u8>>>,
    country: Option<Reader<Vec<u8>>>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PoWPageConfig {
    #[serde(default)]
    pub template: String,
    #[serde(default)]
    pub widget: String,
    #[serde(default)]
    pub html: String,
}

static POW_STATIC_DIR: include_dir::Dir =
    include_dir::include_dir!("../../internal/edge/agent/nginx/pow_static");
const POW_SHIELD_HTML: &str =
    include_str!("../../../internal/openflare/openresty/pow_templates/shield.html");
const POW_CLASSIC_HTML: &str =
    include_str!("../../../internal/openflare/openresty/pow_templates/classic.html");
const POW_CHALLENGE_EN: &str =
    include_str!("../../../internal/edge/agent/nginx/pow_static/locales/challenge.en.json");
const POW_CHALLENGE_ZH_CN: &str =
    include_str!("../../../internal/edge/agent/nginx/pow_static/locales/challenge.zh-CN.json");

pub const MAX_ACCESS_LOG_BUFFER: usize = 2048;

#[derive(Debug, Clone, Serialize)]
pub struct NodeAccessLog {
    pub logged_at_unix: i64,
    pub remote_addr: String,
    pub host: String,
    pub path: String,
    pub user_agent: String,
    pub cache_status: String,
    pub status_code: i32,
    pub bytes_sent: i64,
    pub request_length: i64,
    pub request_time_ms: i64,
}

#[derive(Clone, Default)]
pub struct AccessLogBuffer {
    inner: Arc<Mutex<AccessLogBufferInner>>,
}

impl std::fmt::Debug for AccessLogBuffer {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let len = self
            .inner
            .lock()
            .map(|inner| inner.logs.len())
            .unwrap_or_default();
        formatter
            .debug_struct("AccessLogBuffer")
            .field("len", &len)
            .finish()
    }
}

#[derive(Default)]
struct AccessLogBufferInner {
    logs: VecDeque<NodeAccessLog>,
}

impl AccessLogBuffer {
    pub fn push(&self, log: NodeAccessLog) {
        let Ok(mut inner) = self.inner.lock() else {
            return;
        };
        inner.logs.push_back(log);
        while inner.logs.len() > MAX_ACCESS_LOG_BUFFER {
            inner.logs.pop_front();
        }
    }

    pub fn peek(&self, limit: usize) -> Vec<NodeAccessLog> {
        let Ok(inner) = self.inner.lock() else {
            return Vec::new();
        };
        inner.logs.iter().take(limit).cloned().collect()
    }

    pub fn commit(&self, count: usize) {
        let Ok(mut inner) = self.inner.lock() else {
            return;
        };
        let count = count.min(inner.logs.len());
        for _ in 0..count {
            inner.logs.pop_front();
        }
    }
}

impl GeoIp {
    pub fn open(country_path: Option<&Path>, city_path: Option<&Path>) -> Option<Self> {
        let country = country_path.and_then(|path| Reader::open_readfile(path).ok());
        let city = city_path.and_then(|path| Reader::open_readfile(path).ok());
        (country.is_some() || city.is_some()).then_some(Self { city, country })
    }

    pub fn lookup(&self, ip: &str) -> Option<GeoResult> {
        let ip = ip.parse().ok()?;
        if let Some(reader) = self.city.as_ref() {
            if let Some(Some(city)) = reader
                .lookup(ip)
                .ok()
                .and_then(|lookup| lookup.decode::<maxminddb::geoip2::City>().ok())
            {
                let country = city.country.iso_code.map(|code| code.to_uppercase());
                let subdivision = city.subdivisions.first().and_then(|item| item.iso_code);
                let region = geo_region(country.as_deref(), subdivision);
                return Some(GeoResult { country, region });
            }
        }
        if let Some(reader) = self.country.as_ref() {
            if let Some(Some(country_record)) = reader
                .lookup(ip)
                .ok()
                .and_then(|lookup| lookup.decode::<maxminddb::geoip2::Country>().ok())
            {
                return Some(GeoResult {
                    country: country_record
                        .country
                        .iso_code
                        .map(|code| code.to_uppercase()),
                    region: None,
                });
            }
        }
        None
    }
}

pub fn geo_region(country: Option<&str>, subdivision: Option<&str>) -> Option<String> {
    let country = country?;
    let subdivision = subdivision?;
    if subdivision.starts_with(&format!("{country}-")) {
        Some(subdivision.to_owned())
    } else {
        Some(format!("{country}-{subdivision}"))
    }
}

pub fn resolved_pow_page(page: &PoWPageConfig) -> PoWPageConfig {
    let mut resolved = page.clone();
    resolved.html = Default::default();
    if resolved.template.is_empty() {
        resolved.template = "shield".to_owned();
    }
    if resolved.widget.is_empty() {
        resolved.widget = "progress".to_owned();
    }
    if resolved.html.trim().is_empty() {
        resolved.html = if resolved.template == "classic" {
            POW_CLASSIC_HTML.to_owned()
        } else {
            POW_SHIELD_HTML.to_owned()
        };
    }
    resolved
}

pub fn render_pow_challenge(
    config: &PoWConfig,
    _route: &Route,
    page: &PoWPageConfig,
    context: &RequestContext,
    id: &str,
    challenge: &PowChallenge,
) -> String {
    let page = resolved_pow_page(page);
    let template = if page.template.is_empty() {
        "shield"
    } else {
        page.template.as_str()
    };
    let default_html = if template == "classic" {
        POW_CLASSIC_HTML
    } else {
        POW_SHIELD_HTML
    };
    let html = if page.html.trim().is_empty() {
        default_html
    } else {
        page.html.as_str()
    };
    let widget = match page.widget.as_str() {
        "turnstile" => "turnstile",
        _ => "progress",
    };
    let lang = if context.accept_language.to_ascii_lowercase().contains("zh") {
        "zh-CN"
    } else {
        "en"
    };
    let translation = pow_translation(lang);
    let values: HashMap<&str, String> = HashMap::from([
        ("lang", lang.to_owned()),
        ("host", context.host.clone()),
        ("title", translation.title.clone()),
        ("status", translation.status.clone()),
        ("why", translation.why.clone()),
        ("why_description", translation.why_description.clone()),
        ("noscript", translation.noscript.clone()),
        ("protected", translation.protected.clone()),
    ]);
    let rendered = replace_pow_placeholders(html, &values, widget);
    let boot = serde_json::json!({
        "id": id,
        "randomData": challenge.data,
        "difficulty": challenge.difficulty,
        "algorithm": challenge.algorithm,
        "challengeTTL": config.challenge_ttl,
        "lang": lang,
        "widget": widget,
    })
    .to_string()
    .replace('<', "\\u003c");
    let runtime = format!(
        "<script id=\"openflare-pow-config\" type=\"application/json\">{boot}</script>\n<script type=\"module\" src=\"/.within.website/x/cmd/anubis/static/js/main.mjs\"></script>\n"
    );
    insert_before_closing_body(&rendered, &runtime)
}

pub fn pow_translation(lang: &str) -> PowTranslation {
    if lang == "zh-CN" {
        serde_json::from_str(POW_CHALLENGE_ZH_CN).unwrap_or_default()
    } else {
        serde_json::from_str(POW_CHALLENGE_EN).unwrap_or_default()
    }
}

pub fn replace_pow_placeholders(
    html: &str,
    values: &HashMap<&str, String>,
    widget: &str,
) -> String {
    let mut output = String::with_capacity(html.len());
    let mut chars = html.char_indices().peekable();
    while let Some((_start, ch)) = chars.next() {
        if ch != '{' || chars.peek().is_none_or(|&(_, next)| next != '{') {
            output.push(ch);
            continue;
        }
        chars.next();
        let mut name = String::new();
        let mut closed = false;
        while let Some((_, ch)) = chars.next() {
            if ch == '}' {
                if chars.peek().is_some_and(|&(_, next)| next == '}') {
                    chars.next();
                }
                closed = true;
                break;
            } else if ch.is_ascii_alphanumeric() || ch == '_' {
                name.push(ch);
            } else {
                closed = false;
                break;
            }
        }
        if closed {
            if name == "widget" {
                output.push_str(&format!(
                    "<openflare-pow appearance=\"{widget}\"></openflare-pow>"
                ));
            } else if let Some(value) = values.get(name.as_str()) {
                output.push_str(&html_escape(value));
            } else {
                output.push_str("{{");
                output.push_str(&name);
                output.push_str("}}");
            }
        } else {
            output.push_str("{{");
            output.push_str(&name);
        }
    }
    output
}

fn insert_before_closing_body(html: &str, runtime: &str) -> String {
    let Some(index) = html.rfind("</body>") else {
        return format!("{html}{runtime}");
    };
    let mut output = html[..index].to_owned();
    output.push_str(runtime);
    output.push_str(&html[index..]);
    output
}

fn html_escape(value: &str) -> String {
    value
        .replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
        .replace('\'', "&#39;")
}

pub fn pow_static(relative: &str) -> Option<(&'static [u8], &'static str)> {
    if relative.contains('\\') || relative.contains('\0') {
        return None;
    }
    let file = POW_STATIC_DIR.get_file(relative)?;
    Some((
        file.contents(),
        static_content_type(file.path().as_os_str().to_string_lossy().as_ref()),
    ))
}

fn static_content_type(path: &str) -> &'static str {
    match path.rsplit('.').next().unwrap_or_default() {
        "css" => "text/css; charset=utf-8",
        "js" | "mjs" => "text/javascript; charset=utf-8",
        "json" => "application/json; charset=utf-8",
        "svg" => "image/svg+xml",
        "png" => "image/png",
        "jpg" | "jpeg" => "image/jpeg",
        "webp" => "image/webp",
        "avif" => "image/avif",
        "ico" => "image/x-icon",
        "txt" => "text/plain; charset=utf-8",
        "xml" => "application/xml; charset=utf-8",
        "wasm" => "application/wasm",
        "woff" => "font/woff",
        "woff2" => "font/woff2",
        _ => "application/octet-stream",
    }
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct PowTranslation {
    title: String,
    status: String,
    protected: String,
    why: String,
    why_description: String,
    noscript: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PolicyDecision {
    Continue,
    Respond {
        status: u16,
        headers: Vec<(String, String)>,
        body: &'static str,
    },
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum WafDecision {
    Allow,
    Block { status: u16, body: String },
    Takeover,
}

pub fn basic_auth(route: &Route, context: &RequestContext) -> PolicyDecision {
    if !route.basic_auth_enabled {
        return PolicyDecision::Continue;
    }
    let expected = format!(
        "Basic {}",
        Base64.encode(format!(
            "{}:{}",
            route.basic_auth_username, route.basic_auth_password
        ))
    );
    if constant_time_equal(context.authorization.as_bytes(), expected.as_bytes()) {
        PolicyDecision::Continue
    } else {
        PolicyDecision::Respond {
            status: 401,
            headers: vec![(
                "WWW-Authenticate".into(),
                "Basic realm=\"Restricted\"".into(),
            )],
            body: "",
        }
    }
}

pub fn site_auth(route: &Route, context: &RequestContext, https: bool, now: u64) -> PolicyDecision {
    if route.site_auth_ttl_hours == 0 {
        return PolicyDecision::Continue;
    }
    if !https {
        return PolicyDecision::Respond {
            status: 403,
            headers: vec![],
            body: "",
        };
    }
    if context.path == SITE_AUTH_PATH || valid_site_auth_cookie(route, context, now) {
        return PolicyDecision::Continue;
    }
    if context.method.eq_ignore_ascii_case("GET") || context.method.eq_ignore_ascii_case("HEAD") {
        PolicyDecision::Respond {
            status: 303,
            headers: vec![
                (
                    "Location".into(),
                    format!(
                        "{}?next={}",
                        SITE_AUTH_PATH,
                        encode_uri(&format!("{}{}", context.path, query_suffix(&context.query)))
                    ),
                ),
                ("Cache-Control".into(), "private, no-store".into()),
            ],
            body: "",
        }
    } else {
        PolicyDecision::Respond {
            status: 403,
            headers: vec![("Cache-Control".into(), "private, no-store".into())],
            body: "",
        }
    }
}

pub fn site_auth_login(route: &Route, context: &RequestContext, now: u64) -> PolicyDecision {
    let no_store = ("Cache-Control".to_owned(), "private, no-store".to_owned());
    let page_headers = vec![
        ("Content-Type".into(), "text/html; charset=utf-8".into()),
        ("Referrer-Policy".into(), "strict-origin".into()),
        (
            "Content-Security-Policy".into(),
            "form-action 'self'; frame-ancestors 'none'; base-uri 'none'".into(),
        ),
    ];
    if context.method.eq_ignore_ascii_case("GET") || context.method.eq_ignore_ascii_case("HEAD") {
        return PolicyDecision::Respond {
            status: 200,
            headers: vec![no_store].into_iter().chain(page_headers).collect(),
            body: "",
        };
    }
    if !context.method.eq_ignore_ascii_case("POST") {
        return PolicyDecision::Respond {
            status: 405,
            headers: vec![no_store, ("Allow".into(), "GET, HEAD, POST".into())],
            body: "",
        };
    }
    if let Some(origin) = context.origin_header.as_ref() {
        if *origin != format!("https://{}", context.host) {
            return PolicyDecision::Respond {
                status: 403,
                headers: vec![no_store],
                body: "",
            };
        }
    }
    if !context
        .content_type
        .to_ascii_lowercase()
        .starts_with("application/x-www-form-urlencoded")
    {
        return PolicyDecision::Respond {
            status: 415,
            headers: vec![no_store],
            body: "",
        };
    }
    let form = parse_form(&context.body);
    let username = form.get("username").map(String::as_str).unwrap_or("");
    let password = form.get("password").map(String::as_str).unwrap_or("");
    let digest = site_auth_credential_digest(&route.site_auth_key, username, password);
    let expected = site_auth_credential_digest(
        &route.site_auth_key,
        &route.basic_auth_username,
        &route.basic_auth_password,
    );
    if username.len() > 255
        || password.len() > 255
        || username.contains('\0')
        || password.contains('\0')
        || !constant_time_equal(digest.as_bytes(), expected.as_bytes())
    {
        return PolicyDecision::Respond {
            status: 403,
            headers: vec![no_store].into_iter().chain(page_headers).collect(),
            body: "",
        };
    }
    let ttl = (route.site_auth_ttl_hours as u64) * 3600;
    let expiry = now + ttl;
    let cookie = format!(
        "{}={}.{}; Max-Age={ttl}; Path=/; Secure; HttpOnly; SameSite=Lax",
        SITE_AUTH_COOKIE,
        expiry,
        site_auth_signature(&route.site_auth_key, &context.host, expiry)
    );
    PolicyDecision::Respond {
        status: 303,
        headers: vec![
            ("Location".into(), safe_next_path(&context.query)),
            ("Set-Cookie".into(), cookie),
            no_store,
        ],
        body: "",
    }
}

pub fn valid_site_auth_cookie(route: &Route, context: &RequestContext, now: u64) -> bool {
    let Some(value) = cookie_value(&context.cookie, SITE_AUTH_COOKIE) else {
        return false;
    };
    let Some((expiry, signature)) = value.split_once('.') else {
        return false;
    };
    if expiry.len() > 11 || !expiry.bytes().all(|b| b.is_ascii_digit()) {
        return false;
    }
    let Ok(expiry) = expiry.parse::<u64>() else {
        return false;
    };
    expiry > now
        && constant_time_equal(
            signature.as_bytes(),
            site_auth_signature(&route.site_auth_key, &context.host, expiry).as_bytes(),
        )
}

pub fn site_auth_signature(key: &str, host: &str, expiry: u64) -> String {
    let mut mac =
        Hmac::<Sha1Digest>::new_from_slice(key.as_bytes()).expect("HMAC accepts any key length");
    mac.update(format!("{host}:{expiry}").as_bytes());
    hex(mac.finalize().into_bytes())
}

pub fn site_auth_credential_digest(key: &str, username: &str, password: &str) -> String {
    hex(Sha256::digest(
        format!("{key}\0{username}\0{password}").as_bytes(),
    ))
}

pub fn execute_waf(
    document: &WafDocument,
    route: &Route,
    context: &RequestContext,
    pow_valid: bool,
) -> Result<WafDecision, String> {
    let global = document
        .rule_groups
        .iter()
        .filter(|g| g.enabled && g.is_global);
    let bound_ids = document
        .bindings
        .iter()
        .find(|b| b.route_id == route.id)
        .map(|b| b.rule_group_ids.as_slice())
        .unwrap_or_default();
    let bound = document
        .rule_groups
        .iter()
        .filter(|g| g.enabled && !g.is_global && bound_ids.contains(&g.id));
    for group in global.chain(bound) {
        match execute_graph(&group.graph, document, context, pow_valid)? {
            WafDecision::Allow => continue,
            decision => return Ok(decision),
        }
    }
    Ok(WafDecision::Allow)
}

fn execute_graph(
    graph: &crate::config::WafGraph,
    document: &WafDocument,
    context: &RequestContext,
    pow_valid: bool,
) -> Result<WafDecision, String> {
    if graph.entry.is_empty() || graph.nodes.is_empty() {
        return Err("WAF graph is empty".into());
    }
    let mut current = graph.entry.as_str();
    for _ in 0..graph.nodes.len() {
        let node = graph
            .nodes
            .get(current)
            .ok_or_else(|| format!("missing WAF node {current}"))?;
        let handle = match node.node_type.as_str() {
            "start" => "next",
            "allow" => return Ok(WafDecision::Allow),
            "block" => {
                let config: BlockConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid block node: {e}"))?;
                return Ok(WafDecision::Block {
                    status: config.status_code,
                    body: config.response_body,
                });
            }
            "ip_match" => {
                let config: IpMatchConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid ip_match node: {e}"))?;
                if ip_matches(document, &config, &context.remote_addr) {
                    "true"
                } else {
                    "false"
                }
            }
            "geo_match" => {
                let config: GeoMatchConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid geo_match node: {e}"))?;
                if geo_matches(&config, context) {
                    "true"
                } else {
                    "false"
                }
            }
            "ua_check" => {
                let config: UaCheckConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid ua_check node: {e}"))?;
                if ua_matches(&config, &context.user_agent) {
                    "true"
                } else {
                    "false"
                }
            }
            "security_check" => {
                let config: SecurityCheckConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid security_check node: {e}"))?;
                if security_matches(&config, &context.path, &context.query, &context.body) {
                    "true"
                } else {
                    "false"
                }
            }
            "pow" if pow_valid => "next",
            "pow" => return Ok(WafDecision::Takeover),
            other => return Err(format!("unknown WAF node type {other}")),
        };
        let next = node
            .next
            .get(handle)
            .ok_or_else(|| format!("missing {handle} edge from node {current}"))?;
        current = next.as_str();
    }
    Err("WAF graph exceeded maximum steps".into())
}

#[derive(Deserialize)]
struct BlockConfig {
    #[serde(default = "default_status")]
    status_code: u16,
    #[serde(default)]
    response_body: String,
}
fn default_status() -> u16 {
    403
}
#[derive(Deserialize, Default)]
struct IpMatchConfig {
    #[serde(default)]
    ips: Vec<String>,
    #[serde(default)]
    cidrs: Vec<String>,
    #[serde(default)]
    ip_group_ids: Vec<u64>,
}
#[derive(Deserialize, Default)]
struct GeoMatchConfig {
    #[serde(default)]
    countries: Vec<String>,
    #[serde(default)]
    regions: Vec<String>,
}
#[derive(Deserialize, Default)]
struct UaCheckConfig {
    #[serde(default)]
    require_ua: bool,
    #[serde(default)]
    browsers: Vec<String>,
    #[serde(default)]
    operating_systems: Vec<String>,
    #[serde(default)]
    match_mode: String,
    #[serde(default)]
    block_common_bots: bool,
    #[serde(default)]
    block_abnormal_ua: bool,
    #[serde(default)]
    block_custom_ua: bool,
    #[serde(default)]
    custom_ua_patterns: Vec<String>,
}
#[derive(Deserialize, Default)]
struct SecurityCheckConfig {
    #[serde(default)]
    sql_injection: bool,
    #[serde(default)]
    path_traversal: bool,
    #[serde(default)]
    command_injection: bool,
    #[serde(default)]
    xss: bool,
    #[serde(default)]
    ssrf: bool,
    #[serde(default)]
    file_inclusion: bool,
    #[serde(default)]
    malicious_upload: bool,
    #[serde(default)]
    xxe: bool,
    #[serde(default)]
    crlf_injection: bool,
}

fn ip_matches(document: &WafDocument, config: &IpMatchConfig, ip: &str) -> bool {
    config.ips.iter().any(|v| v == ip)
        || config.cidrs.iter().any(|cidr| ip_in_cidr(ip, cidr))
        || config.ip_group_ids.iter().any(|id| {
            document
                .ip_groups
                .iter()
                .find(|g| g.id == *id && g.enabled)
                .is_some_and(|g| g.ip_list.iter().any(|v| v == ip || ip_in_cidr(ip, v)))
        })
}
fn geo_matches(config: &GeoMatchConfig, context: &RequestContext) -> bool {
    let Some(geo) = context.geo.as_ref() else {
        return false;
    };
    config.countries.iter().any(|v| {
        geo.country
            .as_deref()
            .is_some_and(|c| c.eq_ignore_ascii_case(v))
    }) || config.regions.iter().any(|v| {
        geo.region
            .as_deref()
            .is_some_and(|r| r.eq_ignore_ascii_case(v))
    })
}
fn ua_matches(config: &UaCheckConfig, user_agent: &str) -> bool {
    let ua = user_agent.trim();
    if config.require_ua && ua.is_empty() {
        return false;
    }
    let browser = classify_browser(ua);
    let os = classify_os(ua);
    if config.block_common_bots && (browser == "Bot" || os == "Bot") {
        return false;
    }
    if config.block_abnormal_ua && (browser == "Other" || browser == "Unknown") {
        return false;
    }
    if config.block_custom_ua
        && config
            .custom_ua_patterns
            .iter()
            .any(|pattern| ua.contains(pattern.as_str()))
    {
        return false;
    }
    let browser_ok = config.browsers.iter().any(|v| v == browser);
    let os_ok = config.operating_systems.iter().any(|v| v == os);
    if config.browsers.is_empty() && config.operating_systems.is_empty() {
        return true;
    }
    if config.browsers.is_empty() {
        return os_ok;
    }
    if config.operating_systems.is_empty() {
        return browser_ok;
    }
    if config.match_mode == "and" {
        browser_ok && os_ok
    } else {
        browser_ok || os_ok
    }
}
fn classify_browser(ua: &str) -> &'static str {
    let ua = ua.to_ascii_lowercase();
    if ua.contains("micromessenger") {
        "WeChat"
    } else if ua.contains("postman") {
        "Postman"
    } else if ua.contains("curl/") || ua.contains("wget/") {
        "CLI"
    } else if ua.contains("edg/") || ua.contains("edgios/") || ua.contains("edga/") {
        "Edge"
    } else if ua.contains("opr/") || ua.contains("opera") {
        "Opera"
    } else if ua.contains("firefox") || ua.contains("fxios") {
        "Firefox"
    } else if (ua.contains("crios") || ua.contains("chrome")) && !ua.contains("chromium") {
        "Chrome"
    } else if ua.contains("chromium") {
        "Chromium"
    } else if ua.contains("safari") {
        "Safari"
    } else if ua.contains("bot")
        || ua.contains("spider")
        || ua.contains("crawler")
        || ua.contains("slurp")
    {
        "Bot"
    } else if ua.is_empty() {
        "Unknown"
    } else {
        "Other"
    }
}
fn classify_os(ua: &str) -> &'static str {
    let ua = ua.to_ascii_lowercase();
    if ua.contains("android") {
        "Android"
    } else if ua.contains("iphone")
        || ua.contains("ipad")
        || ua.contains("ipod")
        || ua.contains("ios")
    {
        "iOS"
    } else if ua.contains("windows") {
        "Windows"
    } else if ua.contains("mac os x") || ua.contains("macintosh") || ua.contains("macos") {
        "macOS"
    } else if ua.contains("cros") {
        "Chrome OS"
    } else if ua.contains("linux") {
        "Linux"
    } else if ua.contains("bot") || ua.contains("spider") || ua.contains("crawler") {
        "Bot"
    } else {
        "Other"
    }
}
fn security_matches(config: &SecurityCheckConfig, path: &str, query: &str, body: &[u8]) -> bool {
    if !config.path_traversal
        && !config.file_inclusion
        && !config.sql_injection
        && !config.command_injection
        && !config.xss
        && !config.ssrf
        && !config.crlf_injection
        && !config.malicious_upload
        && !config.xxe
    {
        return true;
    }
    let mut inputs = vec![
        percent_decode(path).to_ascii_lowercase(),
        percent_decode(query).to_ascii_lowercase(),
        String::from_utf8_lossy(body).to_ascii_lowercase(),
        percent_decode(query).to_ascii_lowercase(),
    ];
    inputs.retain(|value| !value.is_empty());
    let patterns: &[(&[&str], bool)] = &[
        (
            &[
                "../",
                "..\\",
                "..%2f",
                "..%5c",
                "%2e%2e/",
                "%2e%2e\\",
                "%252e%252e",
                "....//",
                "/etc/passwd",
            ],
            config.path_traversal,
        ),
        (
            &[
                "php://",
                "file://",
                "zip://",
                "data://text",
                "expect://",
                "/proc/self",
                "%00",
            ],
            config.file_inclusion,
        ),
        (
            &[
                "union select",
                " or 1=1",
                "' or '",
                "\" or \"",
                "information_schema",
                "xp_cmdshell",
                "load_file(",
                " into outfile",
                "/**/",
                "/*!",
                "*/--",
                "@@version",
            ],
            config.sql_injection,
        ),
        (
            &[
                ";wget",
                ";curl",
                ";bash",
                ";sh ",
                "|bash",
                "|sh ",
                "`id`",
                "$(id)",
                "&&wget",
                "&&curl",
                "&&bash",
                "&&sh ",
                "||wget",
                "||curl",
                "||bash",
                "/bin/sh ",
                "/bin/bash ",
                "cmd.exe /c",
                "powershell -",
                "powershell.exe",
            ],
            config.command_injection,
        ),
        (
            &[
                "<script",
                "<iframe",
                "onerror=",
                "onload=",
                "onmouseover=",
                "document.cookie",
            ],
            config.xss,
        ),
        (
            &[
                "http://127.0.0.1",
                "https://127.0.0.1",
                "http://localhost",
                "https://localhost",
                "http://0.0.0.0",
                "https://0.0.0.0",
                "://169.254.",
                "169.254.169.254",
                "metadata.google",
                "file://",
                "gopher://",
                "dict://",
            ],
            config.ssrf,
        ),
        (&["%0d%0a", "\r\n"], config.crlf_injection),
        (
            &[
                ".php.",
                ".jsp.",
                ".asp.",
                ".aspx.",
                ".phtml",
                ".phar",
                "application/x-php",
                "application/x-httpd-php",
            ],
            config.malicious_upload,
        ),
        (&["<!entity", " system \"", " system '"], config.xxe),
    ];
    !patterns
        .iter()
        .filter(|(_, enabled)| *enabled)
        .any(|(values, _)| {
            inputs
                .iter()
                .any(|input| values.iter().any(|value| input.contains(value)))
        })
}

#[derive(Default)]
pub struct RateLimiter {
    inner: Mutex<HashMap<(String, String), (u64, Instant)>>,
}
pub fn request_rate_limit(
    limiter: &RateLimiter,
    site: &str,
    ip: &str,
    rate: &str,
    now: Instant,
) -> bool {
    let Some(rate) = parse_rate(rate) else {
        return true;
    };
    let mut inner = limiter
        .inner
        .lock()
        .expect("rate limiter state is not poisoned");
    let capacity = calculate_burst(rate).max(1);
    let state = inner
        .entry((site.into(), ip.into()))
        .or_insert((capacity, now));
    let elapsed = now.duration_since(state.1).as_millis() as u64;
    let refill = elapsed * rate.tokens / rate.period_ms;
    if refill > 0 {
        state.0 = (state.0 + refill).min(capacity);
        state.1 += Duration::from_millis(refill * rate.period_ms / rate.tokens);
    }
    if state.0 == 0 {
        return false;
    }
    state.0 -= 1;
    true
}
#[derive(Debug, Clone, Copy)]
pub struct ParsedRate {
    tokens: u64,
    period_ms: u64,
}
pub fn parse_rate(value: &str) -> Option<ParsedRate> {
    let value = value.trim().to_ascii_lowercase();
    let (tokens, unit) = value.split_once('r')?;
    let tokens = tokens.parse().ok()?;
    if tokens == 0 {
        return None;
    }
    let period_ms = match unit.trim_start_matches('/') {
        "s" => 1000,
        "m" => 60_000,
        _ => return None,
    };
    Some(ParsedRate { tokens, period_ms })
}
pub fn calculate_burst(rate: ParsedRate) -> u64 {
    if rate.period_ms == 1000 {
        rate.tokens * 2
    } else {
        (rate.tokens / 5).max(5)
    }
}

#[derive(Debug, Clone)]
pub struct PowChallenge {
    pub data: String,
    pub difficulty: u8,
    pub host: String,
    pub site: String,
    pub redir: String,
    pub algorithm: String,
    pub session_ttl: u64,
    pub expires_at: u64,
}
#[derive(Default)]
pub struct PowState {
    inner: Mutex<PowStateInner>,
}
#[derive(Default)]
struct PowStateInner {
    challenges: HashMap<String, PowChallenge>,
    sessions: HashMap<(String, String), u64>,
}

pub fn pow_required(route: &Route, context: &RequestContext, session_valid: bool) -> bool {
    if !route.pow_enabled || session_valid {
        return false;
    }
    if context.path.starts_with(POW_API_PREFIX) || context.path.starts_with(POW_STATIC_PREFIX) {
        return false;
    }
    let Some(config) = route.pow.as_ref() else {
        return false;
    };
    if pow_list_match(&config.whitelist, context) {
        return false;
    }
    if let Some(rules) = config.path_rules.as_ref() {
        if pow_path_match(&rules.whitelist, &context.path) {
            return false;
        }
        if rules.mode != "blacklist" && !pow_path_match(&rules.blacklist, &context.path) {
            return false;
        }
    } else if !pow_list_match(&config.blacklist, context) {
        return false;
    }
    true
}
fn pow_list_match(list: &crate::config::PoWListConfig, context: &RequestContext) -> bool {
    list.ips.contains(&context.remote_addr)
        || list
            .ip_cidrs
            .iter()
            .any(|cidr| ip_in_cidr(&context.remote_addr, cidr))
        || list.paths.contains(&context.path)
        || list
            .path_regexes
            .iter()
            .any(|pattern| Regex::new(pattern).is_ok_and(|re| re.is_match(&context.path)))
        || list
            .user_agents
            .iter()
            .any(|ua| context.user_agent.contains(ua.as_str()))
}
fn pow_path_match(matches: &[PoWPathMatch], path: &str) -> bool {
    matches.iter().any(|item| {
        let kind = if item.match_type.is_empty() {
            item.match_type_alias.as_str()
        } else {
            item.match_type.as_str()
        };
        match kind {
            "exact" => item.value == path,
            "prefix" => path.starts_with(&item.value),
            "wildcard" => {
                Regex::new(&wildcard_to_regex(&item.value)).is_ok_and(|re| re.is_match(path))
            }
            "regex" => Regex::new(&item.value).is_ok_and(|re| re.is_match(path)),
            _ => false,
        }
    })
}
fn wildcard_to_regex(value: &str) -> String {
    let mut regex = String::from('^');
    for ch in value.chars() {
        if ch == '*' {
            regex.push_str(".*");
        } else {
            regex.push_str(&regex::escape(&ch.to_string()));
        }
    }
    regex.push('$');
    regex
}

impl PowState {
    pub fn valid_session(&self, host: &str, cookie: &str, now: u64) -> Option<String> {
        let token = cookie_value(cookie, POW_COOKIE)?;
        let mut inner = self.inner.lock().ok()?;
        let key = (host.to_owned(), token.clone());
        let expiry = *inner.sessions.get(&key)?;
        if expiry <= now {
            inner.sessions.remove(&key);
            return None;
        }
        Some(token)
    }
    pub fn refresh_session(&self, host: &str, token: &str, ttl: u64, now: u64) {
        if let Ok(mut inner) = self.inner.lock() {
            let key = (host.to_owned(), token.to_owned());
            if let Some(expiry) = inner.sessions.get_mut(&key) {
                *expiry = now + ttl;
            }
        }
    }
    pub fn issue(
        &self,
        config: &PoWConfig,
        route: &Route,
        context: &RequestContext,
        now: u64,
    ) -> Option<(String, PowChallenge)> {
        let mut id = [0u8; 16];
        let mut data = [0u8; 32];
        rand::thread_rng().fill_bytes(&mut id);
        rand::thread_rng().fill_bytes(&mut data);
        let id = hex(id);
        let data = hex(data);
        let challenge = PowChallenge {
            data: data.clone(),
            difficulty: config.difficulty,
            host: context.host.clone(),
            site: route.site_name.clone(),
            redir: format!("{}{}", context.path, query_suffix(&context.query)),
            algorithm: config.algorithm.clone(),
            session_ttl: config.session_ttl,
            expires_at: now + config.challenge_ttl,
        };
        self.inner
            .lock()
            .ok()?
            .challenges
            .insert(id.clone(), challenge.clone());
        Some((id, challenge))
    }
    pub fn verify(
        &self,
        route: &Route,
        context: &RequestContext,
        id: &str,
        response: &str,
        nonce: &str,
        now: u64,
    ) -> Result<(String, String), u16> {
        if id.is_empty() || response.is_empty() || nonce.is_empty() {
            return Err(400);
        }
        let Ok(nonce) = nonce.parse::<u64>() else {
            return Err(400);
        };
        let mut inner = self.inner.lock().map_err(|_| 500u16)?;
        let challenge = inner.challenges.get(id).cloned().ok_or(410u16)?;
        if challenge.expires_at <= now {
            inner.challenges.remove(id);
            return Err(410);
        }
        if challenge.host != context.host || challenge.site != route.site_name {
            return Err(403);
        }
        let hash = hex(Sha256::digest(
            format!("{}{}", challenge.data, nonce).as_bytes(),
        ));
        if !constant_time_equal(hash.as_bytes(), response.to_ascii_lowercase().as_bytes()) {
            return Err(403);
        }
        if !hash.starts_with(&"0".repeat(challenge.difficulty.into())) {
            return Err(403);
        }
        inner.challenges.remove(id);
        let mut token = [0u8; 20];
        rand::thread_rng().fill_bytes(&mut token);
        let token = hex(token);
        inner.sessions.insert(
            (context.host.clone(), token.clone()),
            now + challenge.session_ttl,
        );
        Ok((token, safe_redirect(&challenge.redir)))
    }
}

pub fn ip_in_cidr(ip: &str, cidr: &str) -> bool {
    let Ok(ip) = ip.parse::<IpAddr>() else {
        return false;
    };
    let Some((network, prefix)) = cidr.split_once('/') else {
        return false;
    };
    let Ok(network) = network.parse::<IpAddr>() else {
        return false;
    };
    let Ok(prefix) = prefix.parse::<u32>() else {
        return false;
    };
    match (ip, network) {
        (IpAddr::V4(ip), IpAddr::V4(net)) => {
            prefix <= 32 && (u32::from(ip) >> (32 - prefix)) == (u32::from(net) >> (32 - prefix))
        }
        (IpAddr::V6(ip), IpAddr::V6(net)) => {
            prefix <= 128
                && (u128::from(ip) >> (128 - prefix)) == (u128::from(net) >> (128 - prefix))
        }
        _ => false,
    }
}

pub fn pages_resolve(deployment: &PagesDeployment, root: &Path, path: &str) -> Option<PathBuf> {
    let decoded = percent_decode(path.split('?').next().unwrap_or(""));
    let relative = decoded.trim_start_matches('/');
    if relative
        .split('/')
        .any(|segment| segment == ".." || segment.contains('\\') || segment.contains('\0'))
    {
        return None;
    }
    let mut candidate = root.join(relative);
    if relative.is_empty() || relative.ends_with('/') {
        candidate = candidate.join(&deployment.entry_file);
    } else if candidate.is_dir() {
        candidate = candidate.join("index.html");
    }
    candidate.is_file().then_some(candidate)
}
pub fn pages_fallback(deployment: &PagesDeployment) -> PathBuf {
    if !deployment.spa_fallback_enabled {
        return PathBuf::new();
    }
    let value = deployment.spa_fallback_path.trim();
    if value.is_empty() {
        return PathBuf::from("index.html");
    }
    PathBuf::from(value.trim_start_matches('/'))
}
pub fn pages_api_target(deployment: &PagesDeployment, path: &str) -> Option<String> {
    if !deployment.api_proxy_enabled {
        return None;
    }
    let prefix = format!("/{}", deployment.api_proxy_path.trim_start_matches('/'));
    let prefix = prefix.trim_end_matches('/');
    if path != prefix && !path.starts_with(&format!("{prefix}/")) {
        return None;
    }
    let rewrite = deployment.api_proxy_rewrite.trim();
    let tail = path
        .strip_prefix(prefix)
        .unwrap_or("")
        .trim_start_matches('/');
    let rewritten = if rewrite.is_empty() {
        tail.to_owned()
    } else {
        format!("{}/{}", rewrite.trim_matches('/'), tail)
    };
    Some(format!(
        "{}/{}",
        deployment.api_proxy_pass.trim_end_matches('/'),
        rewritten
    ))
}
pub fn cookie_value(cookie: &str, name: &str) -> Option<String> {
    cookie.split(';').find_map(|part| {
        let part = part.trim();
        let (key, value) = part.split_once('=')?;
        (key == name).then(|| value.to_owned())
    })
}
pub fn strip_site_auth_cookie(cookie: &str) -> String {
    cookie
        .split(';')
        .filter_map(|part| {
            let part = part.trim();
            if part.starts_with(&format!("{SITE_AUTH_COOKIE}=")) {
                None
            } else {
                Some(part.to_owned())
            }
        })
        .collect::<Vec<_>>()
        .join("; ")
}
pub fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or_default()
}
pub fn parse_form(body: &[u8]) -> HashMap<String, String> {
    String::from_utf8_lossy(body)
        .split('&')
        .filter_map(|pair| {
            let (key, value) = pair.split_once('=')?;
            Some((percent_decode(key), percent_decode(value)))
        })
        .collect()
}
pub fn percent_decode(value: &str) -> String {
    let bytes = value.as_bytes();
    let mut output = Vec::with_capacity(bytes.len());
    let mut i = 0;
    while i < bytes.len() {
        if bytes[i] == b'+' {
            output.push(b' ');
            i += 1;
        } else if i + 2 < bytes.len() && bytes[i] == b'%' {
            if let Ok(byte) = u8::from_str_radix(&value[i + 1..i + 3], 16) {
                output.push(byte);
                i += 3;
            } else {
                output.push(bytes[i]);
                i += 1;
            }
        } else {
            output.push(bytes[i]);
            i += 1;
        }
    }
    String::from_utf8_lossy(&output).into_owned()
}
pub fn encode_uri(value: &str) -> String {
    value
        .bytes()
        .map(|b| match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'.' | b'_' | b'~' => {
                (b as char).to_string()
            }
            _ => format!("%{b:02X}"),
        })
        .collect()
}
fn safe_next_path(query: &str) -> String {
    safe_redirect(&percent_decode(
        query
            .split('&')
            .find_map(|p| p.strip_prefix("next="))
            .unwrap_or(""),
    ))
}
fn safe_redirect(value: &str) -> String {
    if value.starts_with('/')
        && !value.starts_with("//")
        && !value
            .bytes()
            .any(|b| b.is_ascii_control() || b == b' ' || b == b'\\')
    {
        value.into()
    } else {
        "/".into()
    }
}
fn query_suffix(query: &str) -> String {
    if query.is_empty() {
        String::new()
    } else {
        format!("?{query}")
    }
}
pub fn constant_time_equal(left: &[u8], right: &[u8]) -> bool {
    left.len() == right.len()
        && left
            .iter()
            .zip(right)
            .fold(0, |diff, (l, r)| diff | (l ^ r))
            == 0
}
pub fn hex(bytes: impl AsRef<[u8]>) -> String {
    bytes.as_ref().iter().map(|b| format!("{b:02x}")).collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::{LimitConfig, WafBinding, WafGraph, WafNode};

    fn route() -> Route {
        Route {
            id: 1,
            site_name: "site".into(),
            domains: vec!["example.com".into()],
            domain_cert_ids: vec![0],
            origin_url: String::new(),
            origin_host: String::new(),
            upstreams: vec![],
            enabled: true,
            enable_https: true,
            enable_http2: true,
            redirect_http: true,
            limit: LimitConfig::default(),
            pow_enabled: false,
            pow: None,
            basic_auth_enabled: false,
            basic_auth_username: "user".into(),
            basic_auth_password: "pass".into(),
            site_auth_ttl_hours: 2,
            site_auth_key: "32bytes".into(),
            upstream_type: String::new(),
            pages: None,
        }
    }
    fn context() -> RequestContext {
        RequestContext {
            method: "GET".into(),
            host: "example.com".into(),
            path: "/private".into(),
            query: String::new(),
            remote_addr: "192.0.2.1".into(),
            user_agent: "curl/8".into(),
            cookie: String::new(),
            authorization: String::new(),
            content_type: String::new(),
            body: Vec::new(),
            origin_header: None,
            accept_language: String::new(),
            geo: None,
        }
    }
    fn next_node(kind: &str, next: &str) -> WafNode {
        WafNode {
            node_type: kind.into(),
            config: serde_json::Value::Null,
            next: HashMap::from([("next".into(), next.into())]),
        }
    }

    #[test]
    fn site_auth_redirect_and_cookie_signature_match() {
        let route = route();
        let context = context();
        assert_eq!(
            site_auth(&route, &context, true, 100),
            PolicyDecision::Respond {
                status: 303,
                headers: vec![
                    (
                        "Location".into(),
                        "/.openflare/site-auth?next=%2Fprivate".into()
                    ),
                    ("Cache-Control".into(), "private, no-store".into())
                ],
                body: ""
            }
        );
        let signature = site_auth_signature(&route.site_auth_key, &context.host, 200);
        let signed = RequestContext {
            cookie: format!("{SITE_AUTH_COOKIE}=200.{signature}"),
            ..context
        };
        assert_eq!(
            site_auth(&route, &signed, true, 100),
            PolicyDecision::Continue
        );
    }
    #[test]
    fn basic_auth_uses_constant_time_expected_header() {
        let mut route = route();
        route.basic_auth_enabled = true;
        let context = RequestContext {
            authorization: format!("Basic {}", Base64.encode("user:pass")),
            ..context()
        };
        assert_eq!(basic_auth(&route, &context), PolicyDecision::Continue);
    }
    #[test]
    fn rate_limit_matches_openresty_burst_and_rejects_after_burst() {
        let limiter = RateLimiter::default();
        let start = Instant::now();
        assert_eq!(calculate_burst(parse_rate("10r/s").unwrap()), 20);
        assert_eq!(calculate_burst(parse_rate("10r/m").unwrap()), 5);
        for _ in 0..20 {
            assert!(request_rate_limit(
                &limiter,
                "site",
                "192.0.2.1",
                "10r/s",
                start
            ));
        }
        assert!(!request_rate_limit(
            &limiter,
            "site",
            "192.0.2.1",
            "10r/s",
            start
        ));
    }
    #[test]
    fn waf_graph_executes_ip_group_and_block() {
        let mut route = route();
        let context = context();
        let mut block = next_node("block", "");
        block.node_type = "block".into();
        block.config = serde_json::json!({"status_code":403,"response_body":"blocked"});
        let mut ip = WafNode {
            node_type: "ip_match".into(),
            config: serde_json::json!({"ip_group_ids":[7]}),
            next: HashMap::from([
                ("true".into(), "block".into()),
                ("false".into(), "allow".into()),
            ]),
        };
        ip.node_type = "ip_match".into();
        let mut allow = next_node("allow", "");
        allow.node_type = "allow".into();
        let mut start = next_node("start", "ip");
        start.node_type = "start".into();
        let graph = WafGraph {
            entry: "start".into(),
            nodes: HashMap::from([
                ("start".into(), start),
                ("ip".into(), ip),
                ("block".into(), block),
                ("allow".into(), allow),
            ]),
        };
        let document = WafDocument {
            rule_groups: vec![crate::config::WafRuleGroup {
                id: 7,
                name: "global".into(),
                enabled: true,
                is_global: true,
                graph,
            }],
            ip_groups: vec![crate::config::WafIPGroup {
                id: 7,
                name: "group".into(),
                enabled: true,
                ip_list: vec!["192.0.2.0/24".into()],
            }],
            bindings: vec![WafBinding {
                route_id: route.id,
                site_name: route.site_name.clone(),
                rule_group_ids: vec![],
            }],
        };
        assert_eq!(
            execute_waf(&document, &route, &context, false).unwrap(),
            WafDecision::Block {
                status: 403,
                body: "blocked".into()
            }
        );
        route.id = 2;
        let other = RequestContext {
            remote_addr: "198.51.100.1".into(),
            ..context
        };
        assert_eq!(
            execute_waf(&document, &route, &other, false).unwrap(),
            WafDecision::Allow
        );
    }
    #[test]
    fn geo_region_matches_openresty_country_subdivision_format() {
        assert_eq!(geo_region(Some("US"), Some("CA")).as_deref(), Some("US-CA"));
        assert_eq!(
            geo_region(Some("US"), Some("US-CA")).as_deref(),
            Some("US-CA")
        );
        assert_eq!(geo_region(Some("US"), None), None);
        assert_eq!(geo_region(None, Some("CA")), None);
    }

    #[test]
    fn access_log_buffer_is_bounded_and_commits_only_reported_entries() {
        let buffer = AccessLogBuffer::default();
        let log = NodeAccessLog {
            logged_at_unix: 100,
            remote_addr: "192.0.2.1".into(),
            host: "example.com".into(),
            path: "/".into(),
            user_agent: "curl/8".into(),
            cache_status: "-".into(),
            status_code: 200,
            bytes_sent: 123,
            request_length: 45,
            request_time_ms: 6,
        };
        buffer.push(log.clone());
        buffer.push(NodeAccessLog {
            path: "/second".into(),
            ..log.clone()
        });
        let reported = buffer.peek(1);
        assert_eq!(reported.len(), 1);
        assert_eq!(reported[0].path, "/");
        buffer.commit(reported.len());
        assert_eq!(buffer.peek(10).len(), 1);
        assert_eq!(buffer.peek(10)[0].path, "/second");
        for i in 0..(MAX_ACCESS_LOG_BUFFER + 10) {
            buffer.push(NodeAccessLog {
                path: format!("/{i}"),
                ..log.clone()
            });
        }
        assert_eq!(
            buffer.peek(MAX_ACCESS_LOG_BUFFER).len(),
            MAX_ACCESS_LOG_BUFFER
        );
        let oldest = buffer.peek(MAX_ACCESS_LOG_BUFFER)[0]
            .path
            .trim_start_matches('/')
            .parse::<usize>()
            .unwrap();
        assert!(oldest > 0 && oldest <= 10);
    }

    #[test]
    fn pow_html_challenge_matches_openresty_runtime_shape() {
        let mut route = route();
        route.pow_enabled = true;
        route.pow = Some(PoWConfig {
            difficulty: 4,
            algorithm: "fast".into(),
            challenge_ttl: 300,
            ..Default::default()
        });
        let mut context = context();
        context.accept_language = "zh-CN,zh;q=0.9".into();
        let challenge = PowChallenge {
            data: "randomdata".into(),
            difficulty: 4,
            host: context.host.clone(),
            site: route.site_name.clone(),
            redir: "/".into(),
            algorithm: "fast".into(),
            session_ttl: 600,
            expires_at: 400,
        };
        let html = render_pow_challenge(
            route.pow.as_ref().unwrap(),
            &route,
            &PoWPageConfig::default(),
            &context,
            "id-1",
            &challenge,
        );
        assert!(html.contains(r#"<script id="openflare-pow-config" type="application/json">"#));
        assert!(html.contains(r#"<script type="module" src="/.within.website/x/cmd/anubis/static/js/main.mjs"></script>"#));
        assert!(html.contains(r#""id":"id-1""#));
        assert!(html.contains(r#""randomData":"randomdata""#));
        assert!(html.contains(r#""challengeTTL":300"#));
        assert!(
            html.contains("正在确认你是不是机器人") || html.contains("正在确认你是不是机器人！")
        );
        assert!(html.contains(r#"<openflare-pow appearance="progress"></openflare-pow>"#));
        assert!(html.contains("</script>\n</body>"));
    }

    #[test]
    fn pow_static_assets_are_embedded_and_path_lookup_is_safe() {
        assert!(pow_static("js/main.mjs").is_some());
        assert_eq!(
            pow_static("js/main.mjs").unwrap().1,
            "text/javascript; charset=utf-8"
        );
        assert!(pow_static("css/xess.css").is_some());
        assert!(pow_static("locales/challenge.zh-CN.json").is_some());
        assert!(pow_static("../secret").is_none());
        assert!(pow_static("missing.js").is_none());
    }

    #[test]
    fn pow_issue_verify_and_replay_are_bound_to_site() {
        let mut route = route();
        route.pow_enabled = true;
        route.pow = Some(PoWConfig {
            difficulty: 1,
            session_ttl: 600,
            challenge_ttl: 300,
            ..Default::default()
        });
        let state = PowState::default();
        let context = context();
        let (challenge_id, challenge) = state
            .issue(route.pow.as_ref().unwrap(), &route, &context, 100)
            .unwrap();
        let mut nonce = 0;
        let hash = loop {
            let hash = hex(Sha256::digest(
                format!("{}{}", challenge.data, nonce).as_bytes(),
            ));
            if hash.starts_with('0') {
                break hash;
            }
            nonce += 1;
        };
        let (token, _) = state
            .verify(
                &route,
                &context,
                &challenge_id,
                &hash,
                &nonce.to_string(),
                100,
            )
            .unwrap();
        let cookie = format!("{POW_COOKIE}={token}");
        assert!(state.valid_session(&context.host, &cookie, 101).is_some());
        assert_eq!(
            state
                .verify(
                    &route,
                    &context,
                    &challenge_id,
                    &hash,
                    &nonce.to_string(),
                    101
                )
                .unwrap_err(),
            410
        );
    }
    #[test]
    fn pages_paths_are_contained_and_api_target_is_rewritten() {
        let deployment = PagesDeployment {
            entry_file: "index.html".into(),
            api_proxy_enabled: true,
            api_proxy_path: "/api".into(),
            api_proxy_pass: "https://api.example.com/base".into(),
            api_proxy_rewrite: "/v1".into(),
            ..Default::default()
        };
        assert_eq!(
            pages_api_target(&deployment, "/api/users?x=1").unwrap(),
            "https://api.example.com/base/v1/users?x=1"
        );
        assert!(pages_resolve(&deployment, Path::new("/tmp/pages"), "/../etc/passwd").is_none());
    }
}
