use super::pow::pow_policy_required;
use super::{ip_in_cidr, percent_decode, RequestContext};
use crate::protocol::config::{PoWConfig, Route, WafDocument};
use serde::Deserialize;
#[derive(Debug, PartialEq, Eq)]
pub enum WafDecision {
    Allow,
    Block { status: u16, body: String },
    Takeover(Box<PoWConfig>),
}

pub(crate) fn validate(document: &WafDocument) -> anyhow::Result<()> {
    let group_ids: std::collections::HashSet<_> =
        document.rule_groups.iter().map(|group| group.id).collect();
    for binding in &document.bindings {
        anyhow::ensure!(
            binding
                .rule_group_ids
                .iter()
                .all(|id| group_ids.contains(id)),
            "WAF binding references an unknown rule group"
        );
    }
    for group in document.rule_groups.iter().filter(|group| group.enabled) {
        let graph = &group.graph;
        anyhow::ensure!(
            graph.nodes.contains_key(&graph.entry),
            "WAF rule {} has no entry node",
            group.id
        );
        for (id, node) in &graph.nodes {
            let decode = |error: serde_json::Error| {
                anyhow::anyhow!("WAF rule {} node {}: {error}", group.id, id)
            };
            let edges: &[&str] = match node.node_type.as_str() {
                "start" => &["next"],
                "allow" => &[],
                "block" => {
                    let block: BlockConfig =
                        serde_json::from_value(node.config.clone()).map_err(decode)?;
                    anyhow::ensure!(
                        (400..=599).contains(&block.status_code),
                        "invalid WAF block status"
                    );
                    &[]
                }
                "ip_match" => {
                    serde_json::from_value::<IpMatchConfig>(node.config.clone()).map_err(decode)?;
                    &["true", "false"]
                }
                "geo_match" => {
                    serde_json::from_value::<GeoMatchConfig>(node.config.clone())
                        .map_err(decode)?;
                    &["true", "false"]
                }
                "ua_check" => {
                    serde_json::from_value::<UaCheckConfig>(node.config.clone()).map_err(decode)?;
                    &["true", "false"]
                }
                "security_check" => {
                    serde_json::from_value::<SecurityCheckConfig>(node.config.clone())
                        .map_err(decode)?;
                    &["true", "false"]
                }
                "pow" => {
                    let policy: PoWConfig =
                        serde_json::from_value(node.config.clone()).map_err(decode)?;
                    super::pow::validate(&policy)?;
                    &["next"]
                }
                kind => anyhow::bail!("unsupported WAF node type {kind}"),
            };
            for edge in edges {
                anyhow::ensure!(
                    node.next
                        .get(*edge)
                        .is_some_and(|next| graph.nodes.contains_key(next)),
                    "WAF rule {} node {} has an invalid {edge} edge",
                    group.id,
                    id
                );
            }
        }
    }
    Ok(())
}

fn waf_groups<'a>(
    document: &'a WafDocument,
    route: &Route,
) -> impl Iterator<Item = &'a crate::protocol::config::WafRuleGroup> {
    let global = document
        .rule_groups
        .iter()
        .filter(|group| group.enabled && group.is_global);
    let bound = document
        .bindings
        .iter()
        .find(|binding| binding.route_id == route.id)
        .into_iter()
        .flat_map(|binding| &binding.rule_group_ids)
        .filter_map(|id| {
            document
                .rule_groups
                .iter()
                .find(|group| group.id == *id && group.enabled && !group.is_global)
        });
    global.chain(bound)
}

pub fn has_waf_pow(document: &WafDocument, route: &Route) -> bool {
    route.pow.is_none()
        && waf_groups(document, route).any(|group| {
            group
                .graph
                .nodes
                .values()
                .any(|node| node.node_type == "pow")
        })
}

pub fn waf_inspects_body(document: &WafDocument, route: &Route) -> bool {
    waf_groups(document, route).any(|group| {
        group
            .graph
            .nodes
            .values()
            .any(|node| node.node_type == "security_check")
    })
}

pub const SECURITY_HEADERS: &[&str] = &["cookie", "referer"];

pub fn execute_waf(
    document: &WafDocument,
    route: &Route,
    context: &RequestContext,
    pow_valid: bool,
) -> Result<WafDecision, String> {
    for group in waf_groups(document, route) {
        match execute_graph(
            &group.graph,
            document,
            context,
            pow_valid,
            route.pow.is_some(),
        )? {
            WafDecision::Allow => continue,
            decision => return Ok(decision),
        }
    }
    Ok(WafDecision::Allow)
}

fn execute_graph(
    graph: &crate::protocol::config::WafGraph,
    document: &WafDocument,
    context: &RequestContext,
    pow_valid: bool,
    independent_pow: bool,
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
                if security_matches(&config, context) {
                    "true"
                } else {
                    "false"
                }
            }
            "pow" if independent_pow || pow_valid => "next",
            "pow" => {
                let policy: PoWConfig = serde_json::from_value(node.config.clone())
                    .map_err(|error| format!("invalid WAF PoW node: {error}"))?;
                if pow_policy_required(&policy, context) {
                    return Ok(WafDecision::Takeover(Box::new(policy)));
                }
                "next"
            }
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
fn security_matches(config: &SecurityCheckConfig, context: &RequestContext) -> bool {
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
        percent_decode(&context.path).to_ascii_lowercase(),
        percent_decode(&context.query).to_ascii_lowercase(),
        percent_decode(&String::from_utf8_lossy(&context.body)).to_ascii_lowercase(),
    ];
    inputs.extend(
        context
            .sensitive_headers
            .iter()
            .map(|value| percent_decode(value).to_ascii_lowercase()),
    );
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
