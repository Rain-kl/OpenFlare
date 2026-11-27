use crate::geo::{ClientProfile, haversine_km};
use crate::protocol::{CompactNodePrior, CompactQualityEntry, CompactQualitySnapshot};
use std::collections::HashMap;

#[derive(Debug, Clone)]
pub struct NodeLinkQuality {
    pub effective_rtt_ms: f64,
    pub loss_rate: f64,
    pub quality_score: f64,
    #[allow(dead_code)]
    pub source_tier: &'static str,
}

impl Default for NodeLinkQuality {
    fn default() -> Self {
        Self {
            effective_rtt_ms: 120.0,
            loss_rate: 0.0,
            quality_score: 40.0,
            source_tier: "geo_prior",
        }
    }
}

/// Anti-corruption oracle abstraction decoupling SmartDNS scoring from raw quality storage/models.
pub trait QualityOracle: Send + Sync {
    fn evaluate_node(&self, node_id: &str, client: &ClientProfile) -> NodeLinkQuality;
}

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
struct RegionKey {
    node_id: String,
    asn: u32,
    country: String,
    region: String,
    ip_family: String,
}

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
struct CountryKey {
    node_id: String,
    asn: u32,
    country: String,
    ip_family: String,
}

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
struct CountryAnyAsnKey {
    node_id: String,
    country: String,
    ip_family: String,
}

/// O(1) lookup oracle indexed directly from CompactQualitySnapshot during snapshot reload.
#[derive(Debug, Default)]
pub struct SnapshotQualityOracle {
    regions: HashMap<RegionKey, CompactQualityEntry>,
    countries: HashMap<CountryKey, CompactQualityEntry>,
    country_fallback: HashMap<CountryAnyAsnKey, CompactQualityEntry>,
    node_priors: HashMap<String, CompactNodePrior>,
}

impl SnapshotQualityOracle {
    pub fn from_snapshot(snap: Option<&CompactQualitySnapshot>) -> Self {
        let Some(s) = snap else {
            return Self::default();
        };
        let mut oracle = Self {
            regions: HashMap::with_capacity(s.entries.len()),
            countries: HashMap::with_capacity(s.entries.len()),
            country_fallback: HashMap::with_capacity(s.entries.len() / 2 + 1),
            node_priors: s.node_defaults.clone(),
        };
        for entry in &s.entries {
            let country = entry.client_country.trim().to_ascii_uppercase();
            let region = entry.client_region.trim().to_ascii_uppercase();
            let ip_family = entry.ip_family.trim().to_ascii_lowercase();

            if entry.level == "region" && !region.is_empty() {
                let key = RegionKey {
                    node_id: entry.node_id.clone(),
                    asn: entry.client_asn,
                    country: country.clone(),
                    region,
                    ip_family: ip_family.clone(),
                };
                oracle.regions.insert(key, entry.clone());
            } else {
                let key = CountryKey {
                    node_id: entry.node_id.clone(),
                    asn: entry.client_asn,
                    country: country.clone(),
                    ip_family: ip_family.clone(),
                };
                oracle.countries.insert(key, entry.clone());
            }

            let fb_key = CountryAnyAsnKey {
                node_id: entry.node_id.clone(),
                country,
                ip_family,
            };
            let replace = match oracle.country_fallback.get(&fb_key) {
                None => true,
                Some(prev) => entry.confidence > prev.confidence,
            };
            if replace {
                oracle.country_fallback.insert(fb_key, entry.clone());
            }
        }
        oracle
    }
}

impl QualityOracle for SnapshotQualityOracle {
    fn evaluate_node(&self, node_id: &str, client: &ClientProfile) -> NodeLinkQuality {
        let country = client.country_iso.trim().to_ascii_uppercase();
        let region = client.region_code.trim().to_ascii_uppercase();
        let family = client.ip_family.trim().to_ascii_lowercase();

        if client.asn > 0 && !country.is_empty() && !region.is_empty() {
            let r_key = RegionKey {
                node_id: node_id.to_string(),
                asn: client.asn,
                country: country.clone(),
                region,
                ip_family: family.clone(),
            };
            if let Some(entry) = self.regions.get(&r_key) {
                return entry_to_quality(entry, "measured_region");
            }
        }

        if client.asn > 0 && !country.is_empty() {
            let c_key = CountryKey {
                node_id: node_id.to_string(),
                asn: client.asn,
                country: country.clone(),
                ip_family: family.clone(),
            };
            if let Some(entry) = self.countries.get(&c_key) {
                return entry_to_quality(entry, "measured_country");
            }
        }

        if !country.is_empty() {
            let fb_key = CountryAnyAsnKey {
                node_id: node_id.to_string(),
                country,
                ip_family: family,
            };
            if let Some(entry) = self.country_fallback.get(&fb_key) {
                return entry_to_quality(entry, "inferred");
            }
        }

        if let Some(prior) = self.node_priors.get(node_id) {
            if let (Some(c_lat), Some(c_lon), Some(n_lat), Some(n_lon)) =
                (client.latitude, client.longitude, prior.lat, prior.lon)
            {
                let dist = haversine_km(c_lat, c_lon, n_lat, n_lon);
                let geo_rtt = 5.0 + dist / 100.0;
                let extra = prior.median_extra.max(0.0);
                let eff_rtt = (geo_rtt + extra).clamp(2.0, 600.0);
                let score = (100.0 / (1.0 + eff_rtt / 80.0)).clamp(5.0, 99.0);
                return NodeLinkQuality {
                    effective_rtt_ms: eff_rtt,
                    loss_rate: 0.0,
                    quality_score: score,
                    source_tier: "geo_prior",
                };
            }
        }

        NodeLinkQuality::default()
    }
}

fn entry_to_quality(entry: &CompactQualityEntry, tier: &'static str) -> NodeLinkQuality {
    let eff = if entry.effective_rtt_ms > 0.0 {
        entry.effective_rtt_ms
    } else {
        120.0
    };
    let loss = entry.loss_rate.clamp(0.0, 1.0);
    let score = (100.0 / (1.0 + eff / 80.0) * (1.0 - loss)).clamp(1.0, 99.9);
    NodeLinkQuality {
        effective_rtt_ms: eff,
        loss_rate: loss,
        quality_score: score,
        source_tier: tier,
    }
}
