use super::RequestContext;
use anyhow::{ensure, Result};
use serde::{Deserialize, Serialize};
use std::{collections::HashMap, fmt::Write, sync::Arc};
use zentinel_modsec::ModSecurity;

const CATEGORIES: [&str; 15] = [
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

struct Signature {
    id: u32,
    category: &'static str,
    strict: bool,
    targets: &'static str,
    operator: &'static str,
    transforms: &'static str,
    message: &'static str,
}

macro_rules! rule {
    ($id:literal, $cat:literal, $strict:literal, $targets:expr, $operator:literal, $transforms:expr, $message:literal) => {
        Signature {
            id: $id,
            category: $cat,
            strict: $strict,
            targets: $targets,
            operator: $operator,
            transforms: $transforms,
            message: $message,
        }
    };
}

const INPUT: &str = "ARGS|ARGS_NAMES|REQUEST_COOKIES|REQUEST_URI|REQUEST_HEADERS:Referer";
const RAW_INPUT: &str =
    "ARGS|ARGS_NAMES|REQUEST_COOKIES|REQUEST_URI|REQUEST_BODY|REQUEST_HEADERS:Referer";
const DECODE: &str = "t:urlDecodeUni,t:htmlEntityDecode,t:lowercase";
const SIGNATURES: &[Signature] = &[
    rule!(
        11001,
        "sql_injection",
        false,
        INPUT,
        "@detectSQLi",
        "t:urlDecodeUni",
        "SQL injection detected by libinjection"
    ),
    rule!(
        11002,
        "sql_injection",
        true,
        RAW_INPUT,
        r"@rx \b(?:union\s+(?:all\s+)?select|information_schema|xp_cmdshell)\b",
        DECODE,
        "SQL injection keywords"
    ),
    rule!(
        11011,
        "xss",
        false,
        INPUT,
        "@detectXSS",
        "t:urlDecodeUni,t:htmlEntityDecode",
        "Cross-site scripting detected by libinjection"
    ),
    rule!(
        11012,
        "xss",
        true,
        RAW_INPUT,
        r"@rx (?:<\s*(?:script|iframe)\b|\bon(?:error|load|mouseover)\s*=|\bdocument\.cookie\b)",
        DECODE,
        "Active HTML or JavaScript content"
    ),
    rule!(
        11021,
        "malicious_upload",
        false,
        "FILES",
        r"@rx \.(?:php[0-9]?|phtml|phar|jsp|jspx|asp|aspx)(?:[.;]|$)",
        "t:lowercase",
        "Executable upload filename"
    ),
    rule!(
        11022,
        "malicious_upload",
        true,
        "MULTIPART_PART_HEADERS|REQUEST_HEADERS:Content-Type",
        "@pm application/x-php application/x-httpd-php",
        "t:lowercase",
        "Executable upload media type"
    ),
    rule!(
        11031,
        "path_traversal",
        false,
        RAW_INPUT,
        r"@rx (?:^|[=/:\\])\.\.(?:[/\\]|$)",
        "t:urlDecodeUni",
        "Parent-directory traversal"
    ),
    rule!(
        11032,
        "path_traversal",
        true,
        RAW_INPUT,
        r"@rx (?:\.{2,}[/\\]+|/etc/passwd\b|/proc/self/)",
        "t:urlDecodeUni,t:urlDecodeUni,t:lowercase",
        "Sensitive path or nested path encoding"
    ),
    rule!(
        11041,
        "file_inclusion",
        false,
        RAW_INPUT,
        r"@rx \b(?:php|file|zip|expect|phar)://",
        DECODE,
        "File inclusion stream scheme"
    ),
    rule!(
        11042,
        "file_inclusion",
        true,
        RAW_INPUT,
        r"@rx \bdata://text|/proc/self/|\x00",
        DECODE,
        "Local file inclusion signature"
    ),
    rule!(
        11051,
        "command_injection",
        false,
        RAW_INPUT,
        r"@rx (?:[;|&`]\s*(?:ba?sh|curl|wget|powershell|cmd\.exe)\b|\$\(\s*id\s*\))",
        DECODE,
        "Shell command separator and executable"
    ),
    rule!(
        11052,
        "command_injection",
        true,
        RAW_INPUT,
        r"@rx (?:/(?:usr/)?bin/(?:ba?sh|dash|zsh)\b|\bcmd\.exe\s+/c\b|\bpowershell(?:\.exe)?\s+-)",
        DECODE,
        "Shell executable or interpreter arguments"
    ),
    rule!(
        11061,
        "ssrf",
        false,
        RAW_INPUT,
        r"@rx \bhttps?://(?:localhost\b|127\.[0-9.]+|169\.254\.[0-9.]+|\[::1\])",
        DECODE,
        "Local or link-local URL"
    ),
    rule!(
        11062,
        "ssrf",
        true,
        RAW_INPUT,
        r"@rx \b(?:(?:gopher|dict|file)://|https?://(?:0\.0\.0\.0|10\.[0-9.]+|192\.168\.[0-9.]+|metadata\.google\.internal)\b)",
        DECODE,
        "Internal service URL or server-side protocol"
    ),
    rule!(
        11071,
        "xxe",
        false,
        "REQUEST_BODY",
        r"@rx <!entity\s+[^>]*\b(?:system|public)\s+",
        "t:lowercase",
        "XML external entity declaration"
    ),
    rule!(
        11072,
        "xxe",
        true,
        "REQUEST_BODY",
        r"@rx <!doctype\s+[^>]*\b(?:system|public)\s+|<!entity\s",
        "t:lowercase",
        "XML DTD or entity declaration"
    ),
    rule!(
        11081,
        "crlf_injection",
        false,
        "REQUEST_URI",
        r"@rx \x0d\x0a",
        "t:urlDecodeUni",
        "Encoded CRLF in request URI"
    ),
    rule!(
        11082,
        "crlf_injection",
        true,
        "ARGS|REQUEST_URI",
        r"@rx \x0d\x0a[ \t]*[a-z0-9-]+[ \t]*:",
        "t:urlDecodeUni,t:urlDecodeUni,t:lowercase",
        "Header-like content after CRLF"
    ),
    rule!(
        11091,
        "java_injection",
        false,
        RAW_INPUT,
        r"@rx \b(?:java\.lang\.runtime\s*\.\s*getruntime|java\.lang\.processbuilder)\b",
        DECODE,
        "Java runtime command invocation"
    ),
    rule!(
        11092,
        "java_injection",
        true,
        RAW_INPUT,
        r"@rx \$\{\s*jndi\s*:|\bclass\.forname\s*\(",
        DECODE,
        "Java naming lookup or reflective class loading"
    ),
    rule!(
        11101,
        "java_deserialization",
        false,
        "REQUEST_BODY",
        r"@rx ^(?:\x{fffd}\x{fffd}\x00\x05|rO0AB)",
        "",
        "Java serialized object stream"
    ),
    rule!(
        11102,
        "java_deserialization",
        true,
        "ARGS|REQUEST_BODY",
        r"@rx \bro0ab[a-z0-9+/]{8,}",
        "t:lowercase",
        "Encoded Java serialized object"
    ),
    rule!(
        11111,
        "php_injection",
        false,
        RAW_INPUT,
        r"@rx <\?php\b|<\?=",
        DECODE,
        "PHP opening tag"
    ),
    rule!(
        11112,
        "php_injection",
        true,
        RAW_INPUT,
        r"@rx \b(?:eval|assert|passthru|shell_exec)\s*\(",
        DECODE,
        "Dynamic PHP evaluation or execution function"
    ),
    rule!(
        11121,
        "php_deserialization",
        false,
        "ARGS|REQUEST_BODY",
        r##"@rx (?:^|[^a-z0-9_])(?:O|C):[0-9]+:\x22[^\x22]+\x22:[0-9]+:\{"##,
        "t:urlDecodeUni",
        "PHP serialized object"
    ),
    rule!(
        11122,
        "php_deserialization",
        true,
        "ARGS|REQUEST_BODY",
        r"@rx \b(?:unserialize\s*\(|phar://)",
        DECODE,
        "PHP deserialization entry point"
    ),
    rule!(
        11131,
        "asp_injection",
        false,
        RAW_INPUT,
        r"@rx <%\s*(?:@\s*page\b|response\s*\.|server\s*\.)",
        DECODE,
        "ASP server-side code block"
    ),
    rule!(
        11132,
        "asp_injection",
        true,
        RAW_INPUT,
        r"@rx <%[=!]|\bserver\.createobject\s*\(",
        DECODE,
        "ASP expression or object creation"
    ),
    rule!(
        11141,
        "template_injection",
        false,
        INPUT,
        r"@rx (?:\{\{|\$\{|#\{)[^}\r\n]{0,128}(?:__class__|__globals__|__mro__|t\(java\.)",
        DECODE,
        "Template expression accessing runtime internals"
    ),
    rule!(
        11142,
        "template_injection",
        true,
        INPUT,
        r"@rx \{\{\s*[0-9]+\s*[*+]\s*[0-9]+\s*\}\}|\{%\s*(?:include|import)\s",
        DECODE,
        "Template expression or directive"
    ),
];

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
}

#[derive(Clone)]
pub struct Detector {
    pub group: Option<(u64, String)>,
    compiled: Arc<Engine>,
}

#[derive(Default)]
pub struct EnginePool(HashMap<Levels, Arc<Engine>>);

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
        let engine = match self.0.get(&key) {
            Some(engine) => Arc::clone(engine),
            None => {
                let engine = Arc::new(Engine::compile(key)?);
                self.0.insert(key, Arc::clone(&engine));
                engine
            }
        };
        Ok(Detector {
            group,
            compiled: engine,
        })
    }
}

impl Engine {
    fn compile(levels: Levels) -> Result<Self> {
        let levels: HashMap<_, _> = CATEGORIES.into_iter().zip(levels).collect();
        let mut source = String::from("SecRuleEngine On\nSecRequestBodyAccess On\n");
        let mut rules = HashMap::new();
        for signature in SIGNATURES {
            let level = match levels[signature.category] {
                Level::Off => continue,
                Level::Balanced if signature.strict => continue,
                Level::Balanced => "balanced",
                Level::Strict => "strict",
                Level::Observe => "observe",
            };
            let id = signature.id;
            let transforms = if signature.transforms.is_empty() {
                String::new()
            } else {
                format!(",{}", signature.transforms)
            };
            writeln!(
                source,
                "SecRule {} \"{}\" \"id:{id},phase:2,pass,nolog,t:none{}\"",
                signature.targets, signature.operator, transforms
            )?;
            rules.insert(
                id.to_string(),
                RuleMatch {
                    id,
                    category: signature.category.into(),
                    message: signature.message.into(),
                    level: level.into(),
                },
            );
        }
        let engine = ModSecurity::from_string(&source)?;
        ensure!(
            engine.rule_count() == rules.len(),
            "incomplete ModSecurity ruleset"
        );
        Ok(Self {
            modsec: engine,
            rules,
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
    ) -> Result<Vec<RuleMatch>> {
        if !self.active() {
            return Ok(Vec::new());
        }
        let mut tx = self.compiled.modsec.new_transaction();
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
        tx.process_request_headers()?;
        tx.append_request_body(&context.body)?;
        tx.process_request_body()?;
        let mut matches: Vec<_> = tx
            .matched_rules()
            .iter()
            .filter_map(|id| self.compiled.rules.get(id).cloned())
            .collect();
        matches.sort_unstable_by_key(|matched| (matched.level == "observe", matched.id));
        matches.dedup_by_key(|matched| matched.id);
        matches.truncate(16);
        Ok(matches)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use http::{header::CONTENT_TYPE, HeaderMap, HeaderValue, Version};

    #[test]
    fn compiles_all_signatures_and_accepts_ordinary_requests() {
        let detector = EnginePool::default()
            .compile(
                None,
                CATEGORIES
                    .map(|category| (category.into(), Level::Strict))
                    .into_iter()
                    .collect(),
            )
            .unwrap();
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        let request = RequestContext {
            method: "POST".into(),
            path: "/v1/notes".into(),
            query: "page=2&order=recent".into(),
            body: br#"{"title":"Quarterly plan","items":["design","review"]}"#.to_vec(),
            ..RequestContext::default()
        };
        assert_eq!(detector.compiled.modsec.rule_count(), SIGNATURES.len());
        assert_eq!(
            detector
                .inspect(&request, &headers, Version::HTTP_2)
                .unwrap(),
            []
        );
    }

    #[test]
    fn applies_levels_to_parsed_json_fields() {
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        let request = RequestContext {
            method: "POST".into(),
            path: "/preview".into(),
            body: br#"{"expression":"{{ 2 + 3 }}"}"#.to_vec(),
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
                .inspect(&request, &headers, Version::HTTP_11)
                .unwrap();
            assert_eq!(
                matches
                    .first()
                    .map(|matched| (matched.id, matched.level.as_str())),
                expected.map(|name| (11142, name))
            );
        }
    }
}
