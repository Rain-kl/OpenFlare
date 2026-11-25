use super::{cookie_value, query_suffix, PolicyDecision, RequestContext};
use crate::protocol::config::{Route, ServiceWorker};

pub(crate) fn select(
    worker: Option<&ServiceWorker>,
    route: &Route,
    request: &RequestContext,
    https: bool,
) -> Option<PolicyDecision> {
    let worker = worker.filter(|worker| {
        https
            && route
                .domains
                .iter()
                .any(|domain| worker.domains.contains(domain))
    })?;
    let (mime, body, mut headers) = match request.path.as_str() {
        "/sw.js" => (
            "application/javascript",
            worker.script.clone(),
            vec![("Service-Worker-Allowed".to_owned(), "/".to_owned())],
        ),
        "/offline.html" => ("text/html", worker.offline_html.clone(), vec![]),
        "/" if request.method == "GET"
            && is_browser(&request.user_agent)
            && cookie_value(&request.cookie, "__openflare_sw")
                .is_none_or(|value| value.is_empty()) =>
        {
            let redirect = serde_json::to_string(&format!("/{}", query_suffix(&request.query)))
                .ok()?
                .replace('<', "\\u003c");
            let body = format!(
                r#"<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="robots" content="noindex,nofollow"><script>
const redirect = {redirect};
function complete() {{
  document.cookie = "__openflare_sw=1; Path=/; Max-Age=31536000; Secure; SameSite=Lax";
  location.replace(redirect);
}}
if ("serviceWorker" in navigator) {{
  navigator.serviceWorker.register("/sw.js").then(complete, () => location.replace(redirect));
}} else {{
  complete();
}}
</script></head><body></body></html>"#
            );
            ("text/html; charset=utf-8", body, vec![])
        }
        _ => return None,
    };
    if request.method != "GET" && request.method != "HEAD" {
        return None;
    }
    headers.push(("Content-Type".into(), mime.into()));
    headers.push((
        "Cache-Control".into(),
        if route.site_auth_ttl_hours == 0 {
            "no-cache"
        } else {
            "private, no-store"
        }
        .into(),
    ));
    Some(PolicyDecision::Respond {
        status: 200,
        headers,
        body,
    })
}

fn is_browser(ua: &str) -> bool {
    let version = |name| {
        ua.split_once(name)
            .and_then(|(_, suffix)| suffix.as_bytes().first())
            .is_some_and(u8::is_ascii_digit)
    };
    version("Chrome/") || version("Firefox/") || (!ua.contains("Chrome") && ua.contains("Safari"))
}
