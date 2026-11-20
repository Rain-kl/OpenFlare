mod crs;

use super::RequestContext;
use anyhow::{ensure, Result};
use serde::{Deserialize, Serialize};
use std::{collections::HashMap, sync::Arc};
use zentinel_modsec::{variables::MutableCollection, CompiledRuleset, ModSecurity};

const CATEGORIES: [&str; 17] = [
    "sql_injection",
    "xss",
    "malicious_upload",
    "path_traversal",
    "file_inclusion",
    "command_injection",
    "ssrf",
    "xxe",
    "crlf_injection",
    "java_injection",
    "java_deserialization",
    "php_injection",
    "php_deserialization",
    "asp_injection",
    "template_injection",
    "csrf",
    "malformed_http",
];

#[derive(Debug, Clone, Copy, Default, Deserialize, Eq, Hash, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum Level {
    #[default]
    Off,
    Observe,
    Balanced,
    Strict,
}

type Levels = [Level; CATEGORIES.len()];

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AttackGroup {
    pub id: u64,
    pub name: String,
    pub levels: HashMap<String, Level>,
}

#[derive(Deserialize, Default)]
pub struct LegacyConfig {
    #[serde(default)]
    sql_injection: bool,
    #[serde(default)]
    xss: bool,
    #[serde(default)]
    malicious_upload: bool,
    #[serde(default)]
    path_traversal: bool,
    #[serde(default)]
    file_inclusion: bool,
    #[serde(default)]
    command_injection: bool,
    #[serde(default)]
    ssrf: bool,
    #[serde(default)]
    xxe: bool,
    #[serde(default)]
    crlf_injection: bool,
}

impl LegacyConfig {
    pub fn levels(&self) -> HashMap<String, Level> {
        [
            ("sql_injection", self.sql_injection),
            ("xss", self.xss),
            ("malicious_upload", self.malicious_upload),
            ("path_traversal", self.path_traversal),
            ("file_inclusion", self.file_inclusion),
            ("command_injection", self.command_injection),
            ("ssrf", self.ssrf),
            ("xxe", self.xxe),
            ("crlf_injection", self.crlf_injection),
        ]
        .into_iter()
        .filter(|(_, enabled)| *enabled)
        .map(|(category, _)| (category.to_owned(), Level::Strict))
        .collect()
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RuleMatch {
    pub id: u32,
    pub category: String,
    pub message: String,
    pub level: String,
}

struct Engine {
    modsec: ModSecurity,
    rules: HashMap<String, RuleMatch>,
    checks_csrf: bool,
}

pub(crate) fn attack_rule_catalog() -> Result<serde_json::Value> {
    Ok(serde_json::to_value(crs::Rules::load()?.catalog()?)?)
}

#[derive(Clone)]
pub struct Detector {
    pub group: Option<(u64, String)>,
    compiled: Arc<Engine>,
}

#[derive(Default)]
pub struct EnginePool {
    source: Option<crs::Rules>,
    engines: HashMap<Levels, Arc<Engine>>,
}

impl EnginePool {
    pub fn compile(
        &mut self,
        group: Option<(u64, String)>,
        levels: HashMap<String, Level>,
    ) -> Result<Detector> {
        ensure!(
            levels.keys().all(|key| CATEGORIES.contains(&key.as_str())),
            "unknown attack protection category"
        );
        let key = CATEGORIES.map(|category| levels.get(category).copied().unwrap_or_default());
        let engine = match self.engines.get(&key) {
            Some(engine) => Arc::clone(engine),
            None => {
                let (ruleset, rules) = if key.iter().all(|level| *level == Level::Off) {
                    (CompiledRuleset::new(), HashMap::new())
                } else {
                    let source = match &mut self.source {
                        Some(source) => source,
                        None => self.source.insert(crs::Rules::load()?),
                    };
                    source.compile(key)?
                };
                let engine = Arc::new(Engine {
                    modsec: ModSecurity::new(ruleset),
                    checks_csrf: rules.contains_key("11151"),
                    rules,
                });
                self.engines.insert(key, Arc::clone(&engine));
                engine
            }
        };
        Ok(Detector {
            group,
            compiled: engine,
        })
    }
}

impl Detector {
    pub fn active(&self) -> bool {
        !self.compiled.rules.is_empty()
    }

    pub fn inspect(
        &self,
        context: &RequestContext,
        headers: &http::HeaderMap,
        version: http::Version,
        https: bool,
    ) -> Result<Vec<RuleMatch>> {
        if !self.active() {
            return Ok(Vec::new());
        }
        let mut tx = self.compiled.modsec.new_transaction();
        if let Some(source) = self
            .compiled
            .checks_csrf
            .then(|| csrf_context(context, headers, https))
            .flatten()
        {
            tx.tx_mut()
                .set("openflare_csrf_context".into(), source.into());
        }
        tx.set_client_addr(&context.remote_addr, 0);
        tx.set_server_name(&context.host);
        let uri = format!("{}{}", context.path, super::query_suffix(&context.query));
        tx.process_uri(
            &uri,
            &context.method,
            match version {
                http::Version::HTTP_2 => "HTTP/2.0",
                http::Version::HTTP_10 => "HTTP/1.0",
                _ => "HTTP/1.1",
            },
        )?;
        for (name, value) in headers {
            if let Ok(value) = value.to_str() {
                tx.add_request_header(name.as_str(), value)?;
            }
        }
        if !headers.contains_key(http::header::HOST) && !context.authority.is_empty() {
            tx.add_request_header("host", &context.authority)?;
        }
        tx.process_request_headers()?;
        tx.append_request_body(&context.body)?;
        tx.process_request_body()?;
        let mut matches: Vec<_> = tx
            .matched_rules()
            .iter()
            .filter_map(|id| self.compiled.rules.get(id))
            .collect();
        matches.sort_unstable_by_key(|matched| (matched.level == "observe", matched.id));
        matches.dedup_by_key(|matched| matched.id);
        matches.truncate(16);
        Ok(matches.into_iter().cloned().collect())
    }
}

fn csrf_context(
    context: &RequestContext,
    headers: &http::HeaderMap,
    https: bool,
) -> Option<&'static str> {
    if matches!(
        context.method.as_str(),
        "GET" | "HEAD" | "OPTIONS" | "TRACE"
    ) || !headers
        .get_all(http::header::COOKIE)
        .iter()
        .any(|value| !value.as_bytes().is_empty())
    {
        return None;
    }
    let scheme = if https { "https" } else { "http" };
    let authority = if context.authority.is_empty() {
        &context.host
    } else {
        &context.authority
    };
    let target = url::Url::parse(&format!("{scheme}://{authority}")).ok()?;
    if let Some(value) = headers.get(http::header::ORIGIN) {
        let origin = value
            .to_str()
            .ok()
            .and_then(|value| url::Url::parse(value).ok());
        return (!origin.is_some_and(|origin| origin.origin() == target.origin()))
            .then_some("cross_origin");
    }
    if let Some(referer) = headers
        .get(http::header::REFERER)
        .and_then(|value| value.to_str().ok())
        .and_then(|value| url::Url::parse(value).ok())
    {
        return (referer.origin() != target.origin()).then_some("cross_origin");
    }
    match headers
        .get("sec-fetch-site")
        .and_then(|value| value.to_str().ok())
    {
        Some("same-origin") => None,
        Some("cross-site" | "same-site") => Some("cross_origin"),
        _ => Some("missing_origin"),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use http::{header::CONTENT_TYPE, HeaderMap, HeaderValue, Version};

    #[test]
    fn compiles_every_category_and_accepts_ordinary_requests() {
        let detector = EnginePool::default()
            .compile(
                None,
                CATEGORIES
                    .into_iter()
                    .map(|category| (category.into(), Level::Strict))
                    .collect(),
            )
            .unwrap();
        assert!(CATEGORIES.iter().all(|category| detector
            .compiled
            .rules
            .values()
            .any(|rule| rule.category == *category)));
        assert!(detector.compiled.rules.len() > 100);
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        let request = RequestContext {
            method: "POST".into(),
            host: "example.test".into(),
            authority: "example.test".into(),
            path: "/v1/notes".into(),
            query: "page=2&order=recent".into(),
            body: br#"{"title":"Quarterly plan","items":["design","review"]}"#.to_vec(),
            ..RequestContext::default()
        };
        assert!(detector
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap()
            .is_empty());
    }

    #[test]
    fn applies_levels_to_parsed_json_fields() {
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        let request = RequestContext {
            method: "POST".into(),
            path: "/preview".into(),
            body: br#"{"template":"{% sample %}"}"#.to_vec(),
            ..RequestContext::default()
        };
        let mut pool = EnginePool::default();
        for (level, expected) in [
            (Level::Off, None),
            (Level::Balanced, None),
            (Level::Observe, Some("observe")),
            (Level::Strict, Some("strict")),
        ] {
            let detector = pool
                .compile(None, HashMap::from([("template_injection".into(), level)]))
                .unwrap();
            let matches = detector
                .inspect(&request, &headers, Version::HTTP_11, true)
                .unwrap();
            assert_eq!(
                matches.first().map(|matched| (
                    matched.id,
                    matched.message.as_str(),
                    matched.level.as_str()
                )),
                expected.map(|name| (934180, "SSTI Attack", name))
            );
        }
    }

    #[test]
    fn reports_complete_crs_chains_and_prioritizes_blocking_matches() {
        let detector = EnginePool::default()
            .compile(
                None,
                HashMap::from([
                    ("php_injection".into(), Level::Observe),
                    ("template_injection".into(), Level::Strict),
                ]),
            )
            .unwrap();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        let mut request = RequestContext {
            method: "POST".into(),
            path: "/preview".into(),
            body: br#"{"reference":"array_push"}"#.to_vec(),
            ..RequestContext::default()
        };
        let matches = detector
            .inspect(&request, &headers, Version::HTTP_11, true)
            .unwrap();
        assert!(!matches.iter().any(|rule| rule.id == 933150));

        request.body = br#"{"reference":"array_push()","template":"{% sample %}"}"#.to_vec();
        let matches = detector
            .inspect(&request, &headers, Version::HTTP_11, true)
            .unwrap();
        assert_eq!(
            matches.first().map(|rule| (rule.id, rule.level.as_str())),
            Some((934180, "strict"))
        );
        assert!(matches.iter().any(|rule| rule.id == 933150
            && rule.level == "observe"
            && rule.message == "PHP Injection Attack: High-Risk PHP Function Name Found"));
    }

    #[test]
    fn checked_in_rule_catalog_matches_the_engine() {
        let checked_in: serde_json::Value = serde_json::from_str(include_str!(
            "../../../../frontend/lib/openflare/attack-rule-catalog.generated.json"
        ))
        .unwrap();
        assert_eq!(checked_in, attack_rule_catalog().unwrap());
    }

    #[test]
    fn protocol_rules_report_the_crs_id() {
        let detector = EnginePool::default()
            .compile(
                None,
                HashMap::from([("malformed_http".into(), Level::Balanced)]),
            )
            .unwrap();
        let request = RequestContext {
            method: "GET".into(),
            path: "/fixture".into(),
            host: "example.test".into(),
            authority: "example.test".into(),
            ..RequestContext::default()
        };
        let mut headers = HeaderMap::new();
        headers.insert("range", HeaderValue::from_static("bytes=0-9"));
        assert!(detector
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap()
            .is_empty());
        headers.insert("range", HeaderValue::from_static("bytes=9-0"));
        let matched = detector
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap();
        assert!(matched
            .iter()
            .any(|rule| rule.id == 920190 && rule.category == "malformed_http"));
    }

    #[test]
    fn csrf_levels_check_credentialed_write_request_origins() {
        let mut pool = EnginePool::default();
        let balanced = pool
            .compile(None, HashMap::from([("csrf".into(), Level::Balanced)]))
            .unwrap();
        let strict = pool
            .compile(None, HashMap::from([("csrf".into(), Level::Strict)]))
            .unwrap();
        let mut request = RequestContext {
            method: "POST".into(),
            path: "/fixture".into(),
            host: "example.test".into(),
            authority: "example.test:443".into(),
            ..RequestContext::default()
        };
        let mut headers = HeaderMap::new();
        headers.insert("origin", HeaderValue::from_static("https://other.test"));
        assert!(balanced
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap()
            .is_empty());
        headers.insert("cookie", HeaderValue::from_static("fixture=guest"));
        let matched = balanced
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap();
        assert_eq!(
            matched.iter().map(|rule| rule.id).collect::<Vec<_>>(),
            [11151]
        );
        headers.insert("origin", HeaderValue::from_static("https://EXAMPLE.test"));
        assert!(strict
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap()
            .is_empty());
        headers.remove("origin");
        assert!(balanced
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap()
            .is_empty());
        let matched = strict
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap();
        assert_eq!(
            matched.iter().map(|rule| rule.id).collect::<Vec<_>>(),
            [11152]
        );
        headers.insert(
            "referer",
            HeaderValue::from_static("https://example.test/form"),
        );
        assert!(strict
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap()
            .is_empty());
        headers.remove("referer");
        request.method = "GET".into();
        assert!(strict
            .inspect(&request, &headers, Version::HTTP_2, true)
            .unwrap()
            .is_empty());
    }
}
