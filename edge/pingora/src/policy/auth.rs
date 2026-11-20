use super::{
    constant_time_equal, cookie_value, encode_uri, hex, parse_form, percent_decode, query_suffix,
    safe_redirect, PolicyDecision, RequestContext,
};
use crate::protocol::config::Route;
use base64::{engine::general_purpose::STANDARD as Base64, Engine as _};
use hmac::{Hmac, Mac};
use sha1::Sha1 as Sha1Digest;
use sha2::{Digest, Sha256};
pub const SITE_AUTH_COOKIE: &str = "__Host-openflare_site_auth";
pub const SITE_AUTH_PATH: &str = "/.openflare/site-auth";
pub fn basic_auth(route: &Route, context: &RequestContext) -> PolicyDecision {
    if !route.basic_auth_enabled {
        return PolicyDecision::Continue;
    }
    let expected = format!(
        "{}:{}",
        route.basic_auth_username, route.basic_auth_password
    );
    let presented = context
        .authorization
        .split_once(' ')
        .filter(|(scheme, _)| scheme.eq_ignore_ascii_case("basic"))
        .and_then(|(_, encoded)| Base64.decode(encoded.trim()).ok());
    if presented.is_some_and(|credentials| constant_time_equal(&credentials, expected.as_bytes())) {
        PolicyDecision::Continue
    } else {
        PolicyDecision::Respond {
            status: 401,
            headers: vec![(
                "WWW-Authenticate".into(),
                "Basic realm=\"Restricted\"".into(),
            )],
            body: String::new(),
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
            body: String::new(),
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
            body: String::new(),
        }
    } else {
        PolicyDecision::Respond {
            status: 403,
            headers: vec![("Cache-Control".into(), "private, no-store".into())],
            body: String::new(),
        }
    }
}

pub fn site_auth_login(
    route: &Route,
    context: &RequestContext,
    template: &str,
    now: u64,
) -> PolicyDecision {
    let no_store = ("Cache-Control".to_owned(), "private, no-store".to_owned());
    let page_headers = vec![
        ("Content-Type".into(), "text/html; charset=utf-8".into()),
        ("Referrer-Policy".into(), "strict-origin".into()),
        (
            "Content-Security-Policy".into(),
            "form-action 'self'; frame-ancestors 'none'; base-uri 'none'".into(),
        ),
    ];
    let html = if template.trim().is_empty() {
        include_str!("../../../../internal/openflare/edgeconfig/site_auth.html")
    } else {
        template
    };
    if context.method.eq_ignore_ascii_case("GET") || context.method.eq_ignore_ascii_case("HEAD") {
        return PolicyDecision::Respond {
            status: 200,
            headers: vec![no_store].into_iter().chain(page_headers).collect(),
            body: html.replace("{{login_error}}", ""),
        };
    }
    if !context.method.eq_ignore_ascii_case("POST") {
        return PolicyDecision::Respond {
            status: 405,
            headers: vec![no_store, ("Allow".into(), "GET, HEAD, POST".into())],
            body: String::new(),
        };
    }
    if let Some(origin) = context.origin_header.as_ref() {
        if *origin != format!("https://{}", context.authority) {
            return PolicyDecision::Respond {
                status: 403,
                headers: vec![no_store],
                body: String::new(),
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
            body: String::new(),
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
            body: html.replace(
                "{{login_error}}",
                "账号或密码错误 / Invalid username or password",
            ),
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
        body: String::new(),
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
fn safe_next_path(query: &str) -> String {
    safe_redirect(&percent_decode(
        query
            .split('&')
            .find_map(|p| p.strip_prefix("next="))
            .unwrap_or(""),
    ))
}
