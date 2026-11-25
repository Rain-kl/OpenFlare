use std::{
    collections::HashMap,
    sync::Mutex,
    time::{Duration, Instant},
};
#[derive(Default)]
pub struct RateLimiter {
    inner: Mutex<HashMap<(String, String), (u64, Instant)>>,
}
pub fn request_rate_limit(
    limiter: &RateLimiter,
    site: &str,
    ip: &str,
    rate: &str,
    now: Instant,
) -> bool {
    request_rate_limit_with_burst(limiter, site, ip, rate, None, now)
}

pub fn request_rate_limit_with_burst(
    limiter: &RateLimiter,
    site: &str,
    ip: &str,
    rate: &str,
    capacity: Option<u64>,
    now: Instant,
) -> bool {
    let Some(rate) = parse_rate(rate) else {
        return true;
    };
    let mut inner = limiter
        .inner
        .lock()
        .expect("rate limiter state is not poisoned");
    let capacity = capacity.unwrap_or_else(|| calculate_burst(rate).saturating_add(1));
    if inner.len() >= 65_536 {
        inner
            .retain(|_, (_, last)| now.saturating_duration_since(*last) < Duration::from_secs(600));
        if inner.len() >= 65_536 && !inner.contains_key(&(site.to_owned(), ip.to_owned())) {
            return false;
        }
    }
    let state = inner
        .entry((site.into(), ip.into()))
        .or_insert((capacity, now));
    let elapsed = now.duration_since(state.1).as_millis() as u64;
    let refill = elapsed * rate.tokens / rate.period_ms;
    if refill > 0 {
        state.0 = (state.0 + refill).min(capacity);
        state.1 += Duration::from_millis(refill * rate.period_ms / rate.tokens);
    }
    if state.0 == 0 {
        return false;
    }
    state.0 -= 1;
    true
}
#[derive(Debug, Clone, Copy)]
pub struct ParsedRate {
    tokens: u64,
    period_ms: u64,
}
pub fn parse_rate(value: &str) -> Option<ParsedRate> {
    let value = value.trim().to_ascii_lowercase();
    let (tokens, unit) = value.split_once('r')?;
    let tokens = tokens.parse().ok()?;
    if tokens == 0 {
        return None;
    }
    let period_ms = match unit.trim_start_matches('/') {
        "s" => 1000,
        "m" => 60_000,
        _ => return None,
    };
    Some(ParsedRate { tokens, period_ms })
}
pub fn calculate_burst(rate: ParsedRate) -> u64 {
    if rate.period_ms == 1000 {
        rate.tokens * 2
    } else {
        (rate.tokens / 5).max(5)
    }
}

#[derive(Clone, Default)]
pub struct ConcurrencyLimiter {
    counts: std::sync::Arc<Mutex<ConnectionCounts>>,
}

#[derive(Default)]
struct ConnectionCounts {
    sites: HashMap<u64, u64>,
    ips: HashMap<String, u64>,
}

pub struct RequestPermit {
    counts: std::sync::Arc<Mutex<ConnectionCounts>>,
    site: Option<u64>,
    ip: Option<String>,
}

impl ConcurrencyLimiter {
    pub fn enter(&self, route: &crate::protocol::config::Route, ip: &str) -> Option<RequestPermit> {
        let site_limit = route.limit.conn_per_server.max(0) as u64;
        let ip_limit = route.limit.conn_per_ip.max(0) as u64;
        let mut counts = self.counts.lock().ok()?;
        if site_limit != 0 && counts.sites.get(&route.id).copied().unwrap_or(0) >= site_limit
            || ip_limit != 0 && counts.ips.get(ip).copied().unwrap_or(0) >= ip_limit
        {
            return None;
        }
        let site = (site_limit != 0).then_some(route.id);
        let ip = (ip_limit != 0).then(|| ip.to_owned());
        if let Some(site) = site {
            *counts.sites.entry(site).or_default() += 1;
        }
        if let Some(ip) = ip.as_ref() {
            *counts.ips.entry(ip.clone()).or_default() += 1;
        }
        Some(RequestPermit {
            counts: self.counts.clone(),
            site,
            ip,
        })
    }
}

impl Drop for RequestPermit {
    fn drop(&mut self) {
        if let Ok(mut counts) = self.counts.lock() {
            fn release<K: std::hash::Hash + Eq>(entries: &mut HashMap<K, u64>, key: K) {
                if let std::collections::hash_map::Entry::Occupied(mut entry) = entries.entry(key) {
                    *entry.get_mut() -= 1;
                    if *entry.get() == 0 {
                        entry.remove();
                    }
                }
            }
            if let Some(site) = self.site.take() {
                release(&mut counts.sites, site);
            }
            if let Some(ip) = self.ip.take() {
                release(&mut counts.ips, ip);
            }
        }
    }
}

impl std::fmt::Debug for RequestPermit {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("RequestPermit")
            .finish_non_exhaustive()
    }
}
