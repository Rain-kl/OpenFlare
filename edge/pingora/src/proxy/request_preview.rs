use crate::policy::WafRequestPreview;
use http::{header, HeaderMap};
use pingora_http::RequestHeader;
use serde_json::Value;
use std::borrow::Cow;
use std::fmt::Write as _;

const MAX_PREVIEW_BYTES: usize = 8 * 1024;
const REDACTED: &str = "[REDACTED]";

pub(super) fn capture(req: &RequestHeader, body: &[u8], has_body: bool) -> WafRequestPreview {
    let target = req.uri.path_and_query().map_or_else(
        || {
            req.uri
                .authority()
                .map_or("/", |authority| authority.as_str())
        },
        |target| target.as_str(),
    );
    let mut message = format!(
        "{} {} {:?}\r\n",
        req.method,
        redact_target(target),
        req.version
    );
    // HTTP/2 carries the host in :authority instead of a Host header.
    if let Some(authority) = req
        .uri
        .authority()
        .filter(|_| !req.headers.contains_key(header::HOST))
    {
        let _ = write!(message, "host: {authority}\r\n");
    }
    for (name, value) in &req.headers {
        let value = if sensitive_name(name.as_str().trim_start_matches("x-")) {
            Cow::Borrowed(REDACTED)
        } else if name == header::REFERER {
            redact_target(value.to_str().unwrap_or("[NON-TEXT]"))
        } else {
            Cow::Borrowed(value.to_str().unwrap_or("[NON-TEXT]"))
        };
        let _ = write!(message, "{name}: {value}\r\n");
    }
    message.push_str("\r\n");
    let body = text_body(&req.headers, body);
    let body_omitted = has_body && body.is_none();
    if let Some(body) = body {
        message.push_str(&body);
    }
    let truncated = message.len() > MAX_PREVIEW_BYTES;
    if truncated {
        message.truncate(message.floor_char_boundary(MAX_PREVIEW_BYTES));
    }
    WafRequestPreview {
        message,
        truncated,
        body_omitted,
    }
}

fn redact_target(target: &str) -> Cow<'_, str> {
    let Some((path, query)) = target.split_once('?') else {
        return Cow::Borrowed(target);
    };
    match redact_parameters(query) {
        Cow::Borrowed(_) => Cow::Borrowed(target),
        Cow::Owned(query) => Cow::Owned(format!("{path}?{query}")),
    }
}

fn redact_parameters(input: &str) -> Cow<'_, str> {
    let mut result = String::new();
    let mut copied = 0;
    let mut end = 0;
    for pair in input.split_inclusive(['&', ';']) {
        let start = end;
        end += pair.len();
        let parameter = pair.trim_end_matches(['&', ';']);
        let Some((key, _)) = parameter.split_once('=') else {
            continue;
        };
        if url::form_urlencoded::parse(parameter.as_bytes())
            .next()
            .is_some_and(|(name, _)| sensitive_name(&name))
        {
            result.push_str(&input[copied..start + key.len() + 1]);
            result.push_str(REDACTED);
            copied = start + parameter.len();
        }
    }
    if copied == 0 {
        return Cow::Borrowed(input);
    }
    result.push_str(&input[copied..]);
    Cow::Owned(result)
}

fn text_body<'a>(headers: &HeaderMap, body: &'a [u8]) -> Option<Cow<'a, str>> {
    if body.is_empty()
        || headers
            .get_all(header::CONTENT_ENCODING)
            .iter()
            .any(|value| {
                !value
                    .to_str()
                    .is_ok_and(|value| value.trim().eq_ignore_ascii_case("identity"))
            })
    {
        return None;
    }
    let text = std::str::from_utf8(body).ok()?;
    if text
        .chars()
        .any(|ch| ch.is_control() && !matches!(ch, '\t' | '\r' | '\n'))
    {
        return None;
    }
    let media_type = headers
        .get(header::CONTENT_TYPE)?
        .to_str()
        .ok()?
        .split(';')
        .next()?
        .trim()
        .to_ascii_lowercase();
    match media_type.as_str() {
        "application/json" => redacted_json(text),
        "application/x-www-form-urlencoded" | "text/plain" => Some(redact_parameters(text)),
        media_type if media_type.ends_with("+json") => redacted_json(text),
        _ => None,
    }
}

fn redacted_json(text: &str) -> Option<Cow<'_, str>> {
    let mut value = serde_json::from_str(text).ok()?;
    redact_json_value(&mut value);
    serde_json::to_string(&value).ok().map(Cow::Owned)
}

fn redact_json_value(value: &mut Value) {
    match value {
        Value::Object(fields) => {
            for (key, value) in fields {
                if sensitive_name(key) {
                    *value = Value::String(REDACTED.into());
                } else {
                    redact_json_value(value);
                }
            }
        }
        Value::Array(items) => items.iter_mut().for_each(redact_json_value),
        _ => {}
    }
}

fn sensitive_name(name: &str) -> bool {
    name.split(['[', ']']).any(|part| {
        let normalized = part.trim().to_ascii_lowercase().replace(['-', '_'], "");
        matches!(
            normalized.as_str(),
            "authorization"
                | "proxyauthorization"
                | "cookie"
                | "setcookie"
                | "password"
                | "passwd"
                | "pwd"
                | "secret"
                | "auth"
                | "session"
                | "sessionid"
                | "csrf"
        ) || ["token", "apikey", "secret", "signature"]
            .iter()
            .any(|suffix| normalized.ends_with(suffix))
    })
}
