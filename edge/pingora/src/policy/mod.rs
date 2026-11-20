mod attack;
mod auth;
mod limits;
mod pow;
mod service_worker;
mod waf;

pub(crate) use attack::attack_rule_catalog;
pub use auth::{basic_auth, site_auth, site_auth_login, strip_site_auth_cookie, SITE_AUTH_PATH};
pub use limits::{
    request_rate_limit, request_rate_limit_with_burst, ConcurrencyLimiter, RateLimiter,
    RequestPermit,
};
pub(crate) use pow::validate as validate_pow;
pub use pow::{
    pow_required, pow_static, render_pow_challenge, session_cookie, PowState, POW_API_PREFIX,
    POW_STATIC_PREFIX,
};
pub(crate) use service_worker::select as service_worker;
pub(crate) use waf::compile as compile_waf;
pub use waf::{has_waf_pow, CompiledWaf, WafAudit, WafDecision, WafRequest, WafRequestPreview};

use crate::geoip::GeoResult;
use std::{
    collections::HashMap,
    net::IpAddr,
    time::{SystemTime, UNIX_EPOCH},
};

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
    pub authority: String,
    pub geo: Option<GeoResult>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PolicyDecision {
    Continue,
    Respond {
        status: u16,
        headers: Vec<(String, String)>,
        body: String,
    },
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
            prefix == 0
                || (prefix <= 32
                    && (u32::from(ip) >> (32 - prefix)) == (u32::from(net) >> (32 - prefix)))
        }
        (IpAddr::V6(ip), IpAddr::V6(net)) => {
            prefix == 0
                || (prefix <= 128
                    && (u128::from(ip) >> (128 - prefix)) == (u128::from(net) >> (128 - prefix)))
        }
        _ => false,
    }
}

pub fn cookie_value(cookie: &str, name: &str) -> Option<String> {
    cookie.split(';').find_map(|part| {
        let part = part.trim();
        let (key, value) = part.split_once('=')?;
        (key == name).then(|| value.to_owned())
    })
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
    use super::auth::{site_auth_signature, SITE_AUTH_COOKIE};
    use super::limits::{calculate_burst, parse_rate};
    use super::pow::{PowChallenge, POW_COOKIE};
    use super::*;
    use crate::geoip::geo_region;
    use crate::pages::{pages_api_target, pages_resolve};
    use crate::protocol::config::{LimitConfig, WafBinding, WafGraph, WafNode};
    use crate::protocol::config::{PagesDeployment, PoWConfig, PoWPageConfig, Route, WafDocument};
    use base64::{engine::general_purpose::STANDARD as Base64, Engine as _};
    use sha2::{Digest, Sha256};
    use std::path::Path;
    use std::time::Instant;

    fn route() -> Route {
        Route {
            id: 1,
            site_name: "site".into(),
            domains: vec!["example.com".into()],
            domain_cert_ids: vec![0],
            origin_url: String::new(),
            origin_host: String::new(),
            custom_headers: vec![],
            cache: None,
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
            authority: "example.com".into(),
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
                body: String::new()
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
        for _ in 0..21 {
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
            rule_groups: vec![crate::protocol::config::WafRuleGroup {
                id: 7,
                name: "global".into(),
                enabled: true,
                is_global: true,
                graph,
            }],
            ip_groups: vec![crate::protocol::config::WafIPGroup {
                id: 7,
                name: "group".into(),
                enabled: true,
                checksum: String::new(),
                ip_list: vec!["192.0.2.0/24".into()],
            }],
            bindings: vec![WafBinding {
                route_id: route.id,
                site_name: route.site_name.clone(),
                rule_group_ids: vec![],
            }],
        };
        let waf = compile_waf(&document).unwrap();
        let evaluate = |context: &RequestContext, route: &Route| {
            waf.execute(
                &document,
                route,
                WafRequest {
                    context,
                    headers: &http::HeaderMap::new(),
                    version: http::Version::HTTP_11,
                    https: true,
                },
                false,
            )
            .unwrap()
        };
        let (decision, audit) = evaluate(&context, &route);
        let audit = audit.unwrap();
        assert_eq!(audit.flow_id, 7);
        assert_eq!(audit.flow_node_id, "ip");
        assert_eq!(audit.reason, "ip_match");
        assert_eq!(
            decision,
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
        assert_eq!(evaluate(&other, &route).0, WafDecision::Allow);
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
        let (token, _, _) = state
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
            "https://api.example.com/v1/users?x=1"
        );
        assert!(pages_resolve(&deployment, Path::new("/tmp/pages"), "/../etc/passwd").is_none());
    }
}
