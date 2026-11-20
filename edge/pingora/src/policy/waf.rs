use super::pow::pow_policy_required;
use super::{
    attack::{AttackGroup, Detector, EnginePool, LegacyConfig, RuleMatch},
    ip_in_cidr, RequestContext,
};
use crate::protocol::config::{PoWConfig, Route, WafDocument, WafRuleGroup};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, PartialEq, Eq)]
pub enum WafDecision {
    Allow,
    Block { status: u16, body: String },
    Takeover(Box<PoWConfig>),
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WafAudit {
    pub action: String,
    pub reason: String,
    pub method: String,
    pub https: bool,
    pub flow_id: u64,
    pub flow_name: String,
    pub flow_node_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attack_group_id: Option<u64>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub attack_group_name: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub rules: Vec<RuleMatch>,
}

impl WafAudit {
    fn mark_observed(&mut self) {
        self.action = "observed".into();
    }
}

#[derive(Clone)]
pub struct CompiledWaf {
    detectors: HashMap<u64, HashMap<String, Detector>>,
}

impl std::fmt::Debug for CompiledWaf {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("CompiledWaf")
            .field("graphs", &self.detectors.len())
            .finish()
    }
}

pub(crate) fn compile(document: &WafDocument) -> anyhow::Result<CompiledWaf> {
    let mut detectors = HashMap::<u64, HashMap<String, Detector>>::new();
    let mut engines = EnginePool::default();
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
                    let config: LegacyConfig =
                        serde_json::from_value(node.config.clone()).map_err(decode)?;
                    let detector = engines.compile(None, config.levels())?;
                    detectors
                        .entry(group.id)
                        .or_default()
                        .insert(id.clone(), detector);
                    &["true", "false"]
                }
                "attack_protection" => {
                    let config: AttackGroup =
                        serde_json::from_value(node.config.clone()).map_err(decode)?;
                    anyhow::ensure!(
                        config.id != 0 && !config.name.trim().is_empty(),
                        "invalid attack protection group"
                    );
                    let detector =
                        engines.compile(Some((config.id, config.name)), config.levels)?;
                    detectors
                        .entry(group.id)
                        .or_default()
                        .insert(id.clone(), detector);
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
    Ok(CompiledWaf { detectors })
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

impl CompiledWaf {
    pub fn inspects(&self, document: &WafDocument, route: &Route) -> bool {
        waf_groups(document, route).any(|group| {
            self.detectors
                .get(&group.id)
                .is_some_and(|nodes| nodes.values().any(Detector::active))
        })
    }

    pub fn execute(
        &self,
        document: &WafDocument,
        route: &Route,
        request: WafRequest<'_>,
        pow_valid: bool,
    ) -> Result<(WafDecision, Option<WafAudit>), String> {
        let mut observation = None;
        for group in waf_groups(document, route) {
            let (decision, audit) = execute_graph(
                group,
                document,
                self.detectors.get(&group.id),
                &request,
                pow_valid,
                route.pow.is_some(),
            )?;
            match decision {
                WafDecision::Block { .. } => return Ok((decision, audit)),
                WafDecision::Takeover(_) => return Ok((decision, audit.or(observation))),
                WafDecision::Allow => observation = observation.or(audit),
            }
        }
        Ok((WafDecision::Allow, observation))
    }
}

pub struct WafRequest<'a> {
    pub context: &'a RequestContext,
    pub headers: &'a http::HeaderMap,
    pub version: http::Version,
    pub https: bool,
}

impl WafRequest<'_> {
    fn audit(&self, group: &WafRuleGroup, node: &str, reason: &str) -> WafAudit {
        WafAudit {
            action: "blocked".into(),
            reason: reason.into(),
            method: self.context.method.clone(),
            https: self.https,
            flow_id: group.id,
            flow_name: group.name.clone(),
            flow_node_id: node.into(),
            attack_group_id: None,
            attack_group_name: String::new(),
            rules: Vec::new(),
        }
    }
}

fn execute_graph(
    group: &WafRuleGroup,
    document: &WafDocument,
    detectors: Option<&HashMap<String, Detector>>,
    request: &WafRequest<'_>,
    pow_valid: bool,
    independent_pow: bool,
) -> Result<(WafDecision, Option<WafAudit>), String> {
    let graph = &group.graph;
    let context = request.context;
    let mut previous_condition = None;
    let mut detection: Option<WafAudit> = None;
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
            "allow" => {
                if let Some(audit) = &mut detection {
                    audit.mark_observed();
                }
                return Ok((WafDecision::Allow, detection));
            }
            "block" => {
                let config: BlockConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid block node: {e}"))?;
                let audit = detection
                    .filter(|audit| audit.action == "blocked")
                    .unwrap_or_else(|| {
                        let (id, reason) = previous_condition.unwrap_or((current, "block"));
                        request.audit(group, id, reason)
                    });
                return Ok((
                    WafDecision::Block {
                        status: config.status_code,
                        body: config.response_body,
                    },
                    Some(audit),
                ));
            }
            "ip_match" => {
                previous_condition = Some((current, "ip_match"));
                let config: IpMatchConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid ip_match node: {e}"))?;
                if ip_matches(document, &config, &context.remote_addr) {
                    "true"
                } else {
                    "false"
                }
            }
            "geo_match" => {
                previous_condition = Some((current, "geo_match"));
                let config: GeoMatchConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid geo_match node: {e}"))?;
                if geo_matches(&config, context) {
                    "true"
                } else {
                    "false"
                }
            }
            "ua_check" => {
                previous_condition = Some((current, "ua_check"));
                let config: UaCheckConfig = serde_json::from_value(node.config.clone())
                    .map_err(|e| format!("invalid ua_check node: {e}"))?;
                if ua_matches(&config, &context.user_agent) {
                    "true"
                } else {
                    "false"
                }
            }
            "security_check" | "attack_protection" => {
                let detector = detectors
                    .and_then(|nodes| nodes.get(current))
                    .ok_or_else(|| format!("missing attack detector for {current}"))?;
                let matches = detector
                    .inspect(context, request.headers, request.version, request.https)
                    .map_err(|error| format!("attack detection failed: {error}"))?;
                let blocked = matches.first().is_some_and(|rule| rule.level != "observe");
                if !matches.is_empty() && (blocked || detection.is_none()) {
                    let mut audit = request.audit(group, current, "attack_protection");
                    if let Some((id, name)) = &detector.group {
                        audit.attack_group_id = Some(*id);
                        audit.attack_group_name.clone_from(name);
                    }
                    if !blocked {
                        audit.mark_observed();
                    }
                    audit.rules = matches;
                    detection = Some(audit);
                }
                if blocked {
                    "false"
                } else {
                    "true"
                }
            }
            "pow" if independent_pow || pow_valid => "next",
            "pow" => {
                let policy: PoWConfig = serde_json::from_value(node.config.clone())
                    .map_err(|error| format!("invalid WAF PoW node: {error}"))?;
                if pow_policy_required(&policy, context) {
                    if let Some(audit) = &mut detection {
                        audit.mark_observed();
                    }
                    return Ok((WafDecision::Takeover(Box::new(policy)), detection));
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
