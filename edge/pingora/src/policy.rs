use crate::config::{PagesDeployment, PoWPathMatch, Route};
use base64::{engine::general_purpose::STANDARD as Base64, Engine as _};
use hmac::{Hmac, Mac};
use regex::Regex;
use sha2::{Digest, Sha256};
use std::net::IpAddr;
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

pub const SITE_AUTH_COOKIE: &str = "__Host-openflare_site_auth";
pub const SITE_AUTH_PATH: &str = "/.openflare/site-auth";
pub const POW_API_PREFIX: &str = "/.within.website/x/cmd/anubis/api/";
pub const POW_STATIC_PREFIX: &str = "/.within.website/x/cmd/anubis/static/";

pub struct RequestContext<'a> {
    pub method: &'a str,
    pub host: &'a str,
    pub path: &'a str,
    pub query: &'a str,
    pub remote_addr: &'a str,
    pub user_agent: &'a str,
    pub cookie: &'a str,
    pub authorization: &'a str,
    pub content_type: &'a str,
    pub body: &'a [u8],
    pub origin_header: Option<&'a str>,
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
                "WWW-Authenticate".to_owned(),
                "Basic realm=\"Restricted\"".to_owned(),
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
    if context.path == SITE_AUTH_PATH {
        return PolicyDecision::Continue;
    }
    if valid_site_auth_cookie(route, context, now) {
        return PolicyDecision::Continue;
    }
    let headers = vec![("Cache-Control".to_owned(), "private, no-store".to_owned())];
    if context.method.eq_ignore_ascii_case("GET") || context.method.eq_ignore_ascii_case("HEAD") {
        let next = format!(
            "{}?next={}",
            SITE_AUTH_PATH,
            encode_uri(&format!("{}{}", context.path, query_suffix(context.query)))
        );
        PolicyDecision::Respond {
            status: 303,
            headers: vec![
                ("Location".to_owned(), next),
                ("Cache-Control".to_owned(), "private, no-store".to_owned()),
            ],
            body: "",
        }
    } else {
        PolicyDecision::Respond {
            status: 403,
            headers,
            body: "",
        }
    }
}

pub fn site_auth_login(route: &Route, context: &RequestContext, now: u64) -> PolicyDecision {
    let no_store = ("Cache-Control".to_owned(), "private, no-store".to_owned());
    if context.method.eq_ignore_ascii_case("GET") || context.method.eq_ignore_ascii_case("HEAD") {
        return PolicyDecision::Respond {
            status: 200,
            headers: vec![
                no_store,
                (
                    "Content-Type".to_owned(),
                    "text/html; charset=utf-8".to_owned(),
                ),
                ("Referrer-Policy".to_owned(), "strict-origin".to_owned()),
                (
                    "Content-Security-Policy".to_owned(),
                    "form-action 'self'; frame-ancestors 'none'; base-uri 'none'".to_owned(),
                ),
            ],
            body: "",
        };
    }
    if !context.method.eq_ignore_ascii_case("POST") {
        return PolicyDecision::Respond {
            status: 405,
            headers: vec![no_store, ("Allow".to_owned(), "GET, HEAD, POST".to_owned())],
            body: "",
        };
    }
    let expected_origin = format!("https://{}", context.host);
    if let Some(origin) = context.origin_header {
        if origin != expected_origin {
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
    let form = parse_form(context.body);
    let username = form.get("username").map(String::as_str).unwrap_or("");
    let password = form.get("password").map(String::as_str).unwrap_or("");
    if username.len() > 255
        || password.len() > 255
        || username.contains('\0')
        || password.contains('\0')
    {
        return invalid_site_auth_login(no_store);
    }
    let digest = site_auth_credential_digest(&route.site_auth_key, username, password);
    let expected_digest = site_auth_credential_digest(
        &route.site_auth_key,
        &route.basic_auth_username,
        &route.basic_auth_password,
    );
    if !constant_time_equal(digest.as_bytes(), expected_digest.as_bytes()) {
        return invalid_site_auth_login(no_store);
    }
    let ttl = (route.site_auth_ttl_hours as u64) * 3600;
    let expiry = now + ttl;
    let cookie = format!(
        "{}={}.{}; Max-Age={}; Path=/; Secure; HttpOnly; SameSite=Lax",
        SITE_AUTH_COOKIE,
        expiry,
        site_auth_signature(&route.site_auth_key, context.host, expiry),
        ttl
    );
    PolicyDecision::Respond {
        status: 303,
        headers: vec![
            ("Location".to_owned(), safe_next_path(context.query)),
            ("Set-Cookie".to_owned(), cookie),
            no_store,
        ],
        body: "",
    }
}

fn invalid_site_auth_login(no_store: (String, String)) -> PolicyDecision {
    PolicyDecision::Respond {
        status: 403,
        headers: vec![
            no_store,
            (
                "Content-Type".to_owned(),
                "text/html; charset=utf-8".to_owned(),
            ),
            ("Referrer-Policy".to_owned(), "strict-origin".to_owned()),
            (
                "Content-Security-Policy".to_owned(),
                "form-action 'self'; frame-ancestors 'none'; base-uri 'none'".to_owned(),
            ),
        ],
        body: "",
    }
}

pub fn valid_site_auth_cookie(route: &Route, context: &RequestContext, now: u64) -> bool {
    let Some(value) = cookie_value(context.cookie, SITE_AUTH_COOKIE) else {
        return false;
    };
    let Some((expiry, signature)) = value.split_once('.') else {
        return false;
    };
    if expiry.len() > 11 || !expiry.bytes().all(|byte| byte.is_ascii_digit()) {
        return false;
    }
    let Ok(expiry) = expiry.parse::<u64>() else {
        return false;
    };
    expiry > now
        && constant_time_equal(
            signature.as_bytes(),
            site_auth_signature(&route.site_auth_key, context.host, expiry).as_bytes(),
        )
}

pub fn site_auth_signature(key: &str, host: &str, expiry: u64) -> String {
    let mut mac =
        Hmac::<Sha1>::new_from_slice(key.as_bytes()).expect("HMAC accepts any key length");
    mac.update(format!("{}:{}", host, expiry).as_bytes());
    hex(mac.finalize().into_bytes())
}

pub fn site_auth_credential_digest(key: &str, username: &str, password: &str) -> String {
    let digest = Sha256::digest(format!("{key}\0{username}\0{password}").as_bytes());
    hex(digest)
}

type Sha1 = sha1::Sha1;

#[derive(Default)]
pub struct RateLimiter {
    inner: Mutex<RateLimiterInner>,
}

#[derive(Default)]
struct RateLimiterInner {
    req_tokens: std::collections::HashMap<(String, String), (u64, Instant)>,
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
    let key = (site.to_owned(), ip.to_owned());
    let capacity = calculate_burst(rate).max(1);
    let state = inner.req_tokens.entry(key).or_insert((capacity, now));
    let elapsed = now.duration_since(state.1).as_millis() as u64;
    let refill = elapsed * rate.tokens / rate.period_ms;
    if refill > 0 {
        state.0 = (state.0 + refill).min(capacity);
        state.1 += Duration::from_millis(refill * rate.period_ms / rate.tokens.max(1));
    }
    if state.0 == 0 {
        return false;
    }
    state.0 -= 1;
    true
}

#[derive(Debug, Clone, Copy)]
pub struct ParsedRate {
    pub tokens: u64,
    pub period_ms: u64,
}

pub fn parse_rate(value: &str) -> Option<ParsedRate> {
    let value = value.trim().to_ascii_lowercase();
    let (tokens, unit) = value.split_once("r")?;
    let unit = unit.trim_start_matches('/');
    let tokens = tokens.parse::<u64>().ok()?;
    if tokens == 0 {
        return None;
    }
    let period_ms = match unit.trim() {
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
        if pow_path_match(&rules.whitelist, context.path) {
            return false;
        }
        if rules.mode != "blacklist" && !pow_path_match(&rules.blacklist, context.path) {
            return false;
        }
    }
    if config.path_rules.is_none() && !pow_list_match(&config.blacklist, context) {
        return false;
    }
    true
}

fn pow_list_match(list: &crate::config::PoWListConfig, context: &RequestContext) -> bool {
    list.ips.iter().any(|ip| ip == context.remote_addr)
        || list
            .ip_cidrs
            .iter()
            .any(|cidr| ip_in_cidr(context.remote_addr, cidr))
        || list.paths.iter().any(|path| *path == context.path)
        || list
            .path_regexes
            .iter()
            .any(|pattern| Regex::new(pattern).is_ok_and(|regex| regex.is_match(context.path)))
        || list
            .user_agents
            .iter()
            .any(|ua| context.user_agent.contains(ua.as_str()))
}

fn pow_path_match(matches: &[PoWPathMatch], path: &str) -> bool {
    matches.iter().any(|item| {
        let match_type = if item.match_type.is_empty() {
            item.match_type_alias.as_str()
        } else {
            item.match_type.as_str()
        };
        match match_type {
            "exact" => item.value == path,
            "prefix" => path.starts_with(&item.value),
            "wildcard" => {
                Regex::new(&wildcard_to_regex(&item.value)).is_ok_and(|regex| regex.is_match(path))
            }
            "regex" => Regex::new(&item.value).is_ok_and(|regex| regex.is_match(path)),
            _ => false,
        }
    })
}

fn wildcard_to_regex(value: &str) -> String {
    let mut regex = String::from('^');
    for byte in value.chars() {
        if byte == '*' {
            regex.push_str(".*");
        } else {
            regex.push_str(&regex::escape(&byte.to_string()));
        }
    }
    regex.push('$');
    regex
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
        (IpAddr::V4(ip), IpAddr::V4(network)) => {
            let bits = 32 - prefix;
            if prefix > 32 {
                return false;
            }
            (u32::from(ip) >> bits) == (u32::from(network) >> bits)
        }
        (IpAddr::V6(ip), IpAddr::V6(network)) => {
            let bits = 128 - prefix;
            if prefix > 128 {
                return false;
            }
            (u128::from(ip) >> bits) == (u128::from(network) >> bits)
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

#[cfg(test)]
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

#[cfg(test)]
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
                return None;
            }
            Some(part.to_owned())
        })
        .collect::<Vec<_>>()
        .join("; ")
}

pub fn unix_now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or_default()
}

pub fn parse_form(body: &[u8]) -> std::collections::HashMap<String, String> {
    let text = String::from_utf8_lossy(body);
    text.split('&')
        .filter_map(|pair| {
            let (key, value) = pair.split_once('=')?;
            Some((percent_decode(key), percent_decode(value)))
        })
        .collect()
}

pub fn percent_decode(value: &str) -> String {
    let bytes = value.as_bytes();
    let mut output = Vec::with_capacity(bytes.len());
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'+' {
            output.push(b' ');
            index += 1;
        } else if index + 2 < bytes.len() && bytes[index] == b'%' {
            if let Ok(byte) = u8::from_str_radix(&value[index + 1..index + 3], 16) {
                output.push(byte);
                index += 3;
            } else {
                output.push(bytes[index]);
                index += 1;
            }
        } else {
            output.push(bytes[index]);
            index += 1;
        }
    }
    String::from_utf8_lossy(&output).into_owned()
}

pub fn encode_uri(value: &str) -> String {
    let mut output = String::new();
    for byte in value.bytes() {
        match byte {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'.' | b'_' | b'~' => {
                output.push(byte as char)
            }
            _ => output.push_str(&format!("%{byte:02X}")),
        }
    }
    output
}

fn safe_next_path(query: &str) -> String {
    let next = query
        .split('&')
        .find_map(|part| part.strip_prefix("next="))
        .unwrap_or("");
    let decoded = percent_decode(next);
    if decoded.starts_with('/')
        && !decoded.starts_with("//")
        && !decoded
            .bytes()
            .any(|byte| byte.is_ascii_control() || byte == b' ' || byte == b'\\')
    {
        decoded
    } else {
        "/".to_owned()
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
    if left.len() != right.len() {
        return false;
    }
    left.iter()
        .zip(right)
        .fold(0, |difference, (left, right)| difference | (left ^ right))
        == 0
}

pub fn hex(bytes: impl AsRef<[u8]>) -> String {
    bytes
        .as_ref()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::{LimitConfig, PoWConfig, PoWListConfig};

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

    fn context<'a>() -> RequestContext<'a> {
        RequestContext {
            method: "GET",
            host: "example.com",
            path: "/private",
            query: "",
            remote_addr: "192.0.2.1",
            user_agent: "curl/8",
            cookie: "",
            authorization: "",
            content_type: "",
            body: b"",
            origin_header: None,
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
                        "Location".to_owned(),
                        "/.openflare/site-auth?next=%2Fprivate".to_owned()
                    ),
                    ("Cache-Control".to_owned(), "private, no-store".to_owned())
                ],
                body: ""
            }
        );
        let signature = site_auth_signature(&route.site_auth_key, context.host, 200);
        let signed = RequestContext {
            cookie: &format!("{SITE_AUTH_COOKIE}=200.{signature}"),
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
            authorization: &format!("Basic {}", Base64.encode("user:pass")),
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
    fn pow_whitelist_blacklist_and_path_rules() {
        let mut route = route();
        route.pow_enabled = true;
        route.pow = Some(PoWConfig {
            blacklist: PoWListConfig::default(),
            path_rules: Some(crate::config::PoWPathRules {
                mode: "all".into(),
                blacklist: vec![PoWPathMatch {
                    match_type: "prefix".into(),
                    match_type_alias: String::new(),
                    value: "/admin".into(),
                }],
                whitelist: vec![],
            }),
            ..Default::default()
        });
        assert!(!pow_required(&route, &context(), false));
        assert!(pow_required(
            &route,
            &RequestContext {
                path: "/admin/users",
                ..context()
            },
            false
        ));
        assert!(!pow_required(
            &route,
            &RequestContext {
                path: "/public",
                ..context()
            },
            false
        ));
    }

    #[test]
    fn pages_paths_are_contained_and_api_target_is_rewritten() {
        let mut deployment = PagesDeployment {
            entry_file: "index.html".into(),
            api_proxy_enabled: true,
            api_proxy_path: "/api".into(),
            api_proxy_pass: "https://api.example.com/base".into(),
            api_proxy_rewrite: "/v1".into(),
            ..Default::default()
        };
        let root = Path::new("/tmp/pages");
        assert_eq!(
            pages_api_target(&deployment, "/api/users?x=1").unwrap(),
            "https://api.example.com/base/v1/users?x=1"
        );
        assert!(pages_resolve(&deployment, root, "/../etc/passwd").is_none());
        deployment.spa_fallback_enabled = true;
        deployment.spa_fallback_path = "app.html".into();
        assert_eq!(pages_fallback(&deployment), PathBuf::from("app.html"));
    }
}
