use crate::geo::{ClientProfile, haversine_km};
use crate::protocol::{SnapshotEnvelope, SnapshotNode, SnapshotPolicy};
use crate::quality::QualityOracle;
use anyhow::Result;
use moka::sync::Cache;
use serde_json::{Value, json};
use sha2::{Digest, Sha256};
use std::collections::HashSet;
use std::net::IpAddr;
use std::sync::Arc;
use std::time::{Duration, Instant};
use zen_engine::Decision;
use zen_engine::DecisionEngine;
use zen_engine::Variable;
use zen_engine::model::DecisionContent;

const OVERLOAD_THRESHOLD: f64 = 0.92;
const HYSTERESIS_MARGIN: f64 = 1.05;
const HYSTERESIS_TTL: Duration = Duration::from_secs(600);

#[derive(Clone)]
pub struct CompiledPolicy {
    pub raw: SnapshotPolicy,
    pub decision: Option<Arc<Decision>>,
}

impl CompiledPolicy {
    pub fn compile(policy: SnapshotPolicy) -> Self {
        let engine = DecisionEngine::default();
        let trimmed = policy.jdm_json.trim();
        let decision = if !trimmed.is_empty() && trimmed != "{}" {
            serde_json::from_str::<DecisionContent>(trimmed)
                .ok()
                .and_then(|content| engine.create_decision(Arc::new(content)).ok())
                .map(Arc::new)
        } else {
            None
        };
        Self {
            raw: policy,
            decision,
        }
    }
}

#[derive(Debug, Clone)]
pub struct PolicyParams {
    pub mode: String,
    pub w_lat: f64,
    pub w_load: f64,
    pub w_geo: f64,
    pub use_tag_weights: bool,
    pub filter_node_ids: Vec<String>,
    pub filter_countries: Vec<String>,
    pub filter_tags: Vec<String>,
    pub filter_network_groups: Vec<String>,
    pub max_answers: usize,
    pub ttl: u32,
}

impl PolicyParams {
    fn from_policy(policy: &SnapshotPolicy) -> Self {
        let is_lb = policy.strategy_type.eq_ignore_ascii_case("load_balance");
        Self {
            mode: if is_lb {
                "load_balance".to_string()
            } else {
                "smart".to_string()
            },
            w_lat: if is_lb { 0.0 } else { 0.55 },
            w_load: if is_lb { 1.0 } else { 0.30 },
            w_geo: if is_lb { 0.0 } else { 0.15 },
            use_tag_weights: true,
            filter_node_ids: Vec::new(),
            filter_countries: Vec::new(),
            filter_tags: Vec::new(),
            filter_network_groups: Vec::new(),
            max_answers: policy.max_answers.max(1) as usize,
            ttl: policy.ttl.clamp(10, 86400) as u32,
        }
    }

    fn apply_jdm_result(&mut self, val: &Value) {
        if let Some(m) = val.get("mode").and_then(|v| v.as_str()) {
            if m == "smart" || m == "load_balance" {
                self.mode = m.to_string();
            }
        }
        if let Some(weights) = val.get("weights") {
            if let Some(w) = weights.get("latency").and_then(|v| v.as_f64()) {
                self.w_lat = w.max(0.0);
            }
            if let Some(w) = weights.get("load").and_then(|v| v.as_f64()) {
                self.w_load = w.max(0.0);
            }
            if let Some(w) = weights.get("geo").and_then(|v| v.as_f64()) {
                self.w_geo = w.max(0.0);
            }
        }
        if let Some(u) = val.get("use_tag_weights").and_then(|v| v.as_bool()) {
            self.use_tag_weights = u;
        }
        if let Some(filters) = val.get("filters") {
            if let Some(arr) = parse_str_array(filters.get("node_ids")) {
                self.filter_node_ids = arr;
            }
            if let Some(arr) = parse_str_array(filters.get("countries")) {
                self.filter_countries = arr;
            }
            if let Some(arr) = parse_str_array(filters.get("tags")) {
                self.filter_tags = arr;
            }
            if let Some(arr) = parse_str_array(filters.get("network_groups")) {
                self.filter_network_groups = arr;
            }
        }
        if let Some(ma) = val.get("max_answers").and_then(|v| v.as_i64()) {
            if (1..=16).contains(&ma) {
                self.max_answers = ma as usize;
            }
        }
        if let Some(ttl) = val.get("ttl").and_then(|v| v.as_i64()) {
            if (10..=86400).contains(&ttl) {
                self.ttl = ttl as u32;
            }
        }
    }
}

fn parse_str_array(v: Option<&Value>) -> Option<Vec<String>> {
    let arr = v?.as_array()?;
    Some(
        arr.iter()
            .filter_map(|item| item.as_str().map(|s| s.trim().to_string()))
            .filter(|s| !s.is_empty())
            .collect(),
    )
}

#[derive(Debug, Clone)]
struct StickyNode {
    node_id: String,
    updated_at: Instant,
}

pub struct HysteresisTracker {
    cache: Cache<String, StickyNode>,
}

impl HysteresisTracker {
    pub fn new(capacity: u64) -> Self {
        Self {
            cache: Cache::builder()
                .max_capacity(capacity)
                .time_to_live(HYSTERESIS_TTL)
                .build(),
        }
    }

    pub fn get(&self, key: &str, now: Instant) -> Option<String> {
        let entry = self.cache.get(key)?;
        if now.saturating_duration_since(entry.updated_at) <= HYSTERESIS_TTL {
            Some(entry.node_id)
        } else {
            None
        }
    }

    pub fn set(&self, key: String, node_id: String, now: Instant) {
        self.cache.insert(
            key,
            StickyNode {
                node_id,
                updated_at: now,
            },
        );
    }
}

#[derive(Debug, Clone)]
pub struct SelectedIp {
    #[allow(dead_code)]
    pub node_id: String,
    pub ip: IpAddr,
    #[allow(dead_code)]
    pub score: f64,
}

#[derive(Debug, Clone)]
pub struct SelectionOutput {
    pub answers: Vec<SelectedIp>,
    pub ttl: u32,
}

pub fn match_policy<'a>(
    env: &SnapshotEnvelope,
    policies: &'a [CompiledPolicy],
    qname: &str,
) -> Option<&'a CompiledPolicy> {
    if policies.is_empty() {
        return None;
    }
    let clean_q = normalize_domain(qname);

    for route in &env.domain_routes {
        if normalize_domain(&route.target_host) == clean_q
            || route
                .member_domains
                .iter()
                .any(|pat| match_domain_pattern(pat, &clean_q))
        {
            if let Some(p) = policies.iter().find(|p| p.raw.id == route.policy_id) {
                return Some(p);
            }
        }
    }

    for p in policies {
        if p.raw.scope_type.eq_ignore_ascii_case("domains")
            && p.raw
                .scope_domains
                .iter()
                .any(|pat| match_domain_pattern(pat, &clean_q))
        {
            return Some(p);
        }
    }

    if let Some(def) = policies.iter().find(|p| p.raw.is_default) {
        return Some(def);
    }
    if let Some(all_p) = policies
        .iter()
        .find(|p| p.raw.scope_type.eq_ignore_ascii_case("all") || p.raw.scope_type.is_empty())
    {
        return Some(all_p);
    }
    policies.first()
}

pub fn normalize_domain(raw: &str) -> String {
    raw.trim()
        .trim_start_matches('.')
        .trim_end_matches('.')
        .to_ascii_lowercase()
}

pub fn match_domain_pattern(pattern: &str, domain: &str) -> bool {
    let pat = normalize_domain(pattern);
    let dom = normalize_domain(domain);
    if pat.is_empty() || dom.is_empty() {
        return false;
    }
    if pat == dom {
        return true;
    }
    if let Some(suffix) = pat.strip_prefix("*.") {
        return dom == suffix
            || (dom.len() > suffix.len()
                && dom.ends_with(suffix)
                && dom.as_bytes()[dom.len() - suffix.len() - 1] == b'.');
    }
    false
}

#[derive(Clone)]
struct ScoredCandidate<'a> {
    node: &'a SnapshotNode,
    ip: IpAddr,
    load_index: f64,
    final_score: f64,
    sort_rank: f64,
}

pub async fn evaluate_and_select(
    oracle: &dyn QualityOracle,
    nodes: &[SnapshotNode],
    policy: &CompiledPolicy,
    client: &ClientProfile,
    domain: &str,
    qtype: &str,
    hysteresis: Option<&HysteresisTracker>,
    now: Instant,
) -> Result<SelectionOutput> {
    let mut params = PolicyParams::from_policy(&policy.raw);
    let clean_domain = normalize_domain(domain);
    let upper_qtype = qtype.to_ascii_uppercase();

    if let Some(ref decision) = policy.decision {
        let decision = decision.clone();
        let ctx_val = json!({
            "client": {
                "country": client.country_iso.to_ascii_uppercase(),
                "region": client.region_code.to_ascii_uppercase(),
                "asn": client.asn,
                "ip_family": client.ip_family,
            },
            "domain": clean_domain,
            "qtype": upper_qtype,
        });
        if let Ok(Some(val)) = tokio::task::spawn_blocking(move || {
            thread_local! {
                static LOCAL_RT: std::cell::RefCell<Option<tokio::runtime::Runtime>> =
                    const { std::cell::RefCell::new(None) };
            }
            LOCAL_RT.with(|cell| {
                let mut borrow = cell.borrow_mut();
                let rt = borrow.get_or_insert_with(|| {
                    tokio::runtime::Builder::new_current_thread()
                        .build()
                        .expect("build zen-engine runtime")
                });
                let input = Variable::from(ctx_val);
                rt.block_on(async move {
                    decision
                        .evaluate(input)
                        .await
                        .ok()
                        .map(|resp| Value::from(resp.result))
                })
            })
        })
        .await
        {
            params.apply_jdm_result(&val);
        }
    }

    let is_v6 = upper_qtype == "AAAA";
    let mut primary: Vec<ScoredCandidate<'_>> = Vec::new();
    let mut overloaded: Vec<ScoredCandidate<'_>> = Vec::new();
    let mut unfiltered_online: Vec<ScoredCandidate<'_>> = Vec::new();
    let mut any_non_quarantined: Vec<ScoredCandidate<'_>> = Vec::new();

    for node in nodes {
        if node.quarantined {
            continue;
        }
        let Some(ip) = pick_node_ip(node, is_v6) else {
            continue;
        };
        let scored = score_candidate(oracle, node, ip, &params, client, &clean_domain);
        any_non_quarantined.push(scored.clone());

        if !node.status.eq_ignore_ascii_case("online") {
            continue;
        }
        unfiltered_online.push(scored.clone());

        if !passes_filters(node, &params) {
            continue;
        }
        if scored.load_index > OVERLOAD_THRESHOLD {
            overloaded.push(scored);
        } else {
            primary.push(scored);
        }
    }

    sort_candidates(&mut primary);

    let chosen: Vec<ScoredCandidate<'_>> = if !primary.is_empty() {
        if params.mode == "smart" {
            if let Some(hyst) = hysteresis {
                let key = format!("{}:{}:{}", policy.raw.id, client.client_prefix, upper_qtype);
                if let Some(prev_id) = hyst.get(&key, now) {
                    if primary[0].node.node_id != prev_id {
                        if let Some(idx) = primary.iter().position(|c| c.node.node_id == prev_id) {
                            if primary[0].final_score <= primary[idx].final_score * HYSTERESIS_MARGIN
                            {
                                let promoted = primary.remove(idx);
                                primary.insert(0, promoted);
                            }
                        }
                    }
                }
                hyst.set(key, primary[0].node.node_id.clone(), now);
            }
        }
        primary.into_iter().take(params.max_answers).collect()
    } else {
        select_fallback(
            &policy.raw,
            &params,
            overloaded,
            unfiltered_online,
            any_non_quarantined,
        )
    };

    let answers = chosen
        .into_iter()
        .map(|c| SelectedIp {
            node_id: c.node.node_id.clone(),
            ip: c.ip,
            score: c.final_score,
        })
        .collect();

    Ok(SelectionOutput {
        answers,
        ttl: params.ttl,
    })
}

fn select_fallback<'a>(
    policy: &SnapshotPolicy,
    params: &PolicyParams,
    mut overloaded: Vec<ScoredCandidate<'a>>,
    mut unfiltered_online: Vec<ScoredCandidate<'a>>,
    mut any_non_quarantined: Vec<ScoredCandidate<'a>>,
) -> Vec<ScoredCandidate<'a>> {
    if policy
        .fallback_strategy
        .eq_ignore_ascii_case("static_nodes")
        && !policy.fallback_node_ids.is_empty()
    {
        let allowed: HashSet<&str> = policy
            .fallback_node_ids
            .iter()
            .map(String::as_str)
            .collect();
        let mut static_list: Vec<ScoredCandidate<'a>> = unfiltered_online
            .iter()
            .filter(|c| allowed.contains(c.node.node_id.as_str()))
            .cloned()
            .collect();
        sort_candidates(&mut static_list);
        if !static_list.is_empty() {
            return static_list.into_iter().take(params.max_answers).collect();
        }
    }
    if !overloaded.is_empty() {
        sort_candidates(&mut overloaded);
        return overloaded.into_iter().take(params.max_answers).collect();
    }
    if !unfiltered_online.is_empty() {
        sort_candidates(&mut unfiltered_online);
        return unfiltered_online
            .into_iter()
            .take(params.max_answers)
            .collect();
    }
    sort_candidates(&mut any_non_quarantined);
    any_non_quarantined
        .into_iter()
        .take(params.max_answers)
        .collect()
}

fn score_candidate<'a>(
    oracle: &dyn QualityOracle,
    node: &'a SnapshotNode,
    ip: IpAddr,
    params: &PolicyParams,
    client: &ClientProfile,
    domain: &str,
) -> ScoredCandidate<'a> {
    let link = oracle.evaluate_node(&node.node_id, client);

    let bw_util = if node.capacity_bandwidth_mbps > 0 && node.bandwidth_mbps > 0.0 {
        (node.bandwidth_mbps / node.capacity_bandwidth_mbps as f64).clamp(0.0, 1.5)
    } else {
        0.0
    };
    let conn_util = if node.capacity_connections > 0 && node.active_connections > 0 {
        (node.active_connections as f64 / node.capacity_connections as f64).clamp(0.0, 1.5)
    } else {
        0.0
    };
    let cap_util = bw_util.max(conn_util);
    let cpu_ratio = node.cpu_usage_percent.clamp(0.0, 100.0) / 100.0;
    let mem_ratio = node.memory_usage_percent.clamp(0.0, 100.0) / 100.0;
    let load_index = 0.45 * cpu_ratio + 0.25 * mem_ratio + 0.30 * cap_util;
    let s_load = (1.0 - load_index.clamp(0.0, 1.5).powf(1.8)).max(0.0);

    let s_geo = match (
        client.latitude,
        client.longitude,
        node.geo_latitude,
        node.geo_longitude,
    ) {
        (Some(clat), Some(clon), Some(nlat), Some(nlon)) => {
            let dist = haversine_km(clat, clon, nlat, nlon);
            1.0 / (1.0 + dist / 2000.0)
        }
        _ => 0.5,
    };

    let s_lat = if link.effective_rtt_ms > 0.0 {
        ((1.0 / (1.0 + link.effective_rtt_ms / 80.0)) * (1.0 - link.loss_rate.clamp(0.0, 1.0)))
            .clamp(0.0, 1.0)
    } else {
        (link.quality_score / 100.0).clamp(0.0, 1.0)
    };

    let tag_mult = if params.use_tag_weights && !node.tags.is_empty() {
        let mut prod = 1.0;
        for t in &node.tags {
            if t.effective_weight > 0.0 {
                prod *= t.effective_weight;
            }
        }
        prod.clamp(0.25, 4.0)
    } else {
        1.0
    };

    let cap_wt = (node.capacity_weight.max(1)) as f64;
    let (final_score, sort_rank) = if params.mode == "load_balance" {
        let g = (cap_wt / 100.0) * s_load.max(0.05) * tag_mult;
        let u = deterministic_unit_float(&client.client_prefix, domain, &node.node_id);
        let rank = g / (-u.ln()).max(1e-6);
        (g, rank)
    } else {
        let sum = params.w_lat + params.w_load + params.w_geo;
        let (wl, wlo, wg) = if sum <= 0.0 {
            (0.55, 0.30, 0.15)
        } else {
            (params.w_lat / sum, params.w_load / sum, params.w_geo / sum)
        };
        let score = (wl * s_lat + wlo * s_load + wg * s_geo) * tag_mult * (cap_wt / 100.0).powf(0.15);
        (score, score)
    };

    ScoredCandidate {
        node,
        ip,
        load_index,
        final_score,
        sort_rank,
    }
}

fn sort_candidates(items: &mut [ScoredCandidate<'_>]) {
    items.sort_by(|a, b| {
        b.sort_rank
            .partial_cmp(&a.sort_rank)
            .unwrap_or(std::cmp::Ordering::Equal)
            .then_with(|| a.node.node_id.cmp(&b.node.node_id))
    });
}

fn passes_filters(node: &SnapshotNode, params: &PolicyParams) -> bool {
    if !params.filter_node_ids.is_empty()
        && !params
            .filter_node_ids
            .iter()
            .any(|id| id.eq_ignore_ascii_case(&node.node_id))
    {
        return false;
    }
    if !params.filter_countries.is_empty()
        && !params
            .filter_countries
            .iter()
            .any(|c| c.eq_ignore_ascii_case(&node.country))
    {
        return false;
    }
    if !params.filter_network_groups.is_empty()
        && !params
            .filter_network_groups
            .iter()
            .any(|g| g.eq_ignore_ascii_case(&node.network_group))
    {
        return false;
    }
    if !params.filter_tags.is_empty() {
        let matched = node.tags.iter().any(|nt| {
            params
                .filter_tags
                .iter()
                .any(|req| req.eq_ignore_ascii_case(&nt.slug))
        });
        if !matched {
            return false;
        }
    }
    true
}

fn pick_node_ip(node: &SnapshotNode, is_v6: bool) -> Option<IpAddr> {
    if is_v6 {
        let ip = node.ipv6.trim().parse::<IpAddr>().ok()?;
        if ip.is_ipv6() { Some(ip) } else { None }
    } else {
        let ip = node.ipv4.trim().parse::<IpAddr>().ok()?;
        if ip.is_ipv4() { Some(ip) } else { None }
    }
}

fn deterministic_unit_float(client_key: &str, domain: &str, node_id: &str) -> f64 {
    let mut hasher = Sha256::new();
    hasher.update(client_key.as_bytes());
    hasher.update(b"|");
    hasher.update(domain.as_bytes());
    hasher.update(b"|");
    hasher.update(node_id.as_bytes());
    let digest = hasher.finalize();
    let raw = u64::from_be_bytes(digest[..8].try_into().unwrap_or([0; 8]));
    ((raw % 1_000_000) as f64 + 1.0) / 1_000_002.0
}
