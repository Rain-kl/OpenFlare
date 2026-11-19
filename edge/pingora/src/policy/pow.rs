use super::{
    constant_time_equal, cookie_value, hex, ip_in_cidr, query_suffix, safe_redirect, RequestContext,
};
use crate::protocol::config::{PoWConfig, PoWPageConfig, PoWPathMatch, Route};
use rand::RngCore;
use regex::Regex;
use serde::Deserialize;
use sha2::{Digest, Sha256};
use std::{collections::HashMap, sync::Mutex};
pub const POW_COOKIE: &str = "__openflare_pow";
pub const POW_API_PREFIX: &str = "/.within.website/x/cmd/anubis/api/";
pub const POW_STATIC_PREFIX: &str = "/.within.website/x/cmd/anubis/static/";

static POW_STATIC_DIR: include_dir::Dir =
    include_dir::include_dir!("$CARGO_MANIFEST_DIR/../../internal/edge/agent/nginx/pow_static");
const POW_SHIELD_HTML: &str =
    include_str!("../../../../internal/openflare/openresty/pow_templates/shield.html");
const POW_CLASSIC_HTML: &str =
    include_str!("../../../../internal/openflare/openresty/pow_templates/classic.html");
const POW_CHALLENGE_EN: &str =
    include_str!("../../../../internal/edge/agent/nginx/pow_static/locales/challenge.en.json");
const POW_CHALLENGE_ZH_CN: &str =
    include_str!("../../../../internal/edge/agent/nginx/pow_static/locales/challenge.zh-CN.json");

fn resolved_pow_page(page: &PoWPageConfig) -> PoWPageConfig {
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

fn pow_translation(lang: &str) -> PowTranslation {
    if lang == "zh-CN" {
        serde_json::from_str(POW_CHALLENGE_ZH_CN).unwrap_or_default()
    } else {
        serde_json::from_str(POW_CHALLENGE_EN).unwrap_or_default()
    }
}

fn replace_pow_placeholders(html: &str, values: &HashMap<&str, String>, widget: &str) -> String {
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
    sessions: HashMap<(String, String), SessionExpiry>,
}

struct SessionExpiry {
    expiry: u64,
    ttl: u64,
}

pub fn session_cookie(token: &str, ttl: u64, https: bool) -> String {
    let secure = if https { "; Secure" } else { "" };
    format!("{POW_COOKIE}={token}; Path=/; HttpOnly; SameSite=Lax; Max-Age={ttl}{secure}")
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
    pow_policy_required(config, context)
}

pub(super) fn pow_policy_required(config: &PoWConfig, context: &RequestContext) -> bool {
    if let Some(rules) = config.path_rules.as_ref() {
        if pow_path_match(&rules.whitelist, &context.path)
            || (rules.mode == "blacklist" && !pow_path_match(&rules.blacklist, &context.path))
        {
            return false;
        }
    }
    if pow_list_match(&config.whitelist, context) {
        return false;
    }
    let list = &config.blacklist;
    let empty = list.ips.is_empty()
        && list.ip_cidrs.is_empty()
        && list.paths.is_empty()
        && list.path_regexes.is_empty()
        && list.user_agents.is_empty();
    empty || pow_list_match(list, context)
}

pub(crate) fn validate(config: &PoWConfig) -> anyhow::Result<()> {
    anyhow::ensure!(
        matches!(config.algorithm.as_str(), "fast" | "slow"),
        "unsupported PoW algorithm"
    );
    anyhow::ensure!(
        (1..=16).contains(&config.difficulty),
        "invalid PoW difficulty"
    );
    anyhow::ensure!(
        (60..=604800).contains(&config.session_ttl) && (30..=3600).contains(&config.challenge_ttl),
        "invalid PoW session or challenge lifetime"
    );
    for list in [&config.whitelist, &config.blacklist] {
        for pattern in &list.path_regexes {
            Regex::new(pattern)?;
        }
    }
    if let Some(rules) = &config.path_rules {
        anyhow::ensure!(
            matches!(rules.mode.as_str(), "all" | "blacklist"),
            "unsupported PoW path mode"
        );
        for rule in rules.whitelist.iter().chain(&rules.blacklist) {
            match rule.match_type.as_str() {
                "exact" | "prefix" | "wildcard" => {}
                "regex" => {
                    Regex::new(&rule.value)?;
                }
                kind => anyhow::bail!("unsupported PoW path match {kind}"),
            }
        }
    }
    Ok(())
}

fn pow_list_match(list: &crate::protocol::config::PoWListConfig, context: &RequestContext) -> bool {
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
        let kind = item.match_type.as_str();
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
        let expiry = inner.sessions.get(&key)?.expiry;
        if expiry <= now {
            inner.sessions.remove(&key);
            return None;
        }
        Some(token)
    }
    pub fn refresh_session(
        &self,
        host: &str,
        token: &str,
        ttl: Option<u64>,
        now: u64,
    ) -> Option<u64> {
        let mut inner = self.inner.lock().ok()?;
        let expiry = inner
            .sessions
            .get_mut(&(host.to_owned(), token.to_owned()))?;
        expiry.ttl = ttl.unwrap_or(expiry.ttl);
        expiry.expiry = now + expiry.ttl;
        Some(expiry.ttl)
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
        let mut inner = self.inner.lock().ok()?;
        if inner.challenges.len() >= 16_384 {
            inner
                .challenges
                .retain(|_, challenge| challenge.expires_at > now);
            if inner.challenges.len() >= 16_384 {
                return None;
            }
        }
        inner.challenges.insert(id.clone(), challenge.clone());
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
    ) -> Result<(String, String, u64), u16> {
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
        if inner.sessions.len() >= 65_536 {
            inner.sessions.retain(|_, session| session.expiry > now);
            if inner.sessions.len() >= 65_536 {
                return Err(503);
            }
        }
        inner.challenges.remove(id);
        let mut token = [0u8; 20];
        rand::thread_rng().fill_bytes(&mut token);
        let token = hex(token);
        inner.sessions.insert(
            (context.host.clone(), token.clone()),
            SessionExpiry {
                expiry: now + challenge.session_ttl,
                ttl: challenge.session_ttl,
            },
        );
        Ok((
            token,
            safe_redirect(&challenge.redir),
            challenge.session_ttl,
        ))
    }
}
