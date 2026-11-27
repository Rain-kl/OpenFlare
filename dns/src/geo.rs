use maxminddb::{Reader, geoip2};
use std::net::IpAddr;
use std::path::Path;
use tracing::warn;

#[derive(Debug, Clone, Default)]
pub struct ClientProfile {
    #[allow(dead_code)]
    pub client_ip: String,
    pub client_prefix: String,
    pub asn: u32,
    pub country_iso: String,
    pub region_code: String,
    pub ip_family: String,
    pub latitude: Option<f64>,
    pub longitude: Option<f64>,
    pub from_ecs: bool,
}

pub struct GeoLookup {
    city_db: Option<Reader<Vec<u8>>>,
    country_db: Option<Reader<Vec<u8>>>,
    asn_db: Option<Reader<Vec<u8>>>,
}

impl GeoLookup {
    pub fn empty() -> Self {
        Self {
            city_db: None,
            country_db: None,
            asn_db: None,
        }
    }

    pub fn load_from_dir(dir: &str) -> Self {
        let base = Path::new(dir);
        if !base.exists() {
            return Self::empty();
        }
        let city_db = open_first_mmdb(
            base,
            &["GeoLite2-City.mmdb", "GeoIP2-City.mmdb", "dbip-city.mmdb"],
        );
        let country_db = open_first_mmdb(
            base,
            &["GeoLite2-Country.mmdb", "Country.mmdb", "dbip-country.mmdb"],
        );
        let asn_db = open_first_mmdb(
            base,
            &["GeoLite2-ASN.mmdb", "GeoIP2-ASN.mmdb", "dbip-asn.mmdb"],
        );
        Self {
            city_db,
            country_db,
            asn_db,
        }
    }

    pub fn lookup_client(&self, ip: IpAddr, answer_family: &str) -> ClientProfile {
        let mut profile = ClientProfile {
            client_ip: ip.to_string(),
            client_prefix: normalize_client_prefix(ip),
            asn: 0,
            country_iso: String::new(),
            region_code: String::new(),
            ip_family: answer_family.to_string(),
            latitude: None,
            longitude: None,
            from_ecs: false,
        };

        if let Some(ref city) = self.city_db {
            if let Ok(rec) = city.lookup::<geoip2::City>(ip) {
                if let Some(country) = rec.country.and_then(|c| c.iso_code) {
                    profile.country_iso = country.to_ascii_uppercase();
                }
                if let Some(subs) = rec.subdivisions {
                    if let Some(first) = subs.first() {
                        if let Some(sub_code) = first.iso_code {
                            if !profile.country_iso.is_empty() {
                                profile.region_code = format!(
                                    "{}-{}",
                                    profile.country_iso,
                                    sub_code.to_ascii_uppercase()
                                );
                            }
                        }
                    }
                }
                if let Some(loc) = rec.location {
                    profile.latitude = loc.latitude;
                    profile.longitude = loc.longitude;
                }
            }
        }

        if profile.country_iso.is_empty() {
            if let Some(ref country_db) = self.country_db {
                if let Ok(rec) = country_db.lookup::<geoip2::Country>(ip) {
                    if let Some(iso) = rec.country.and_then(|c| c.iso_code) {
                        profile.country_iso = iso.to_ascii_uppercase();
                    }
                }
            }
        }

        if let Some(ref asn_db) = self.asn_db {
            if let Ok(rec) = asn_db.lookup::<geoip2::Asn>(ip) {
                if let Some(num) = rec.autonomous_system_number {
                    profile.asn = num;
                }
            }
        }

        profile
    }
}

fn open_first_mmdb(dir: &Path, candidates: &[&str]) -> Option<Reader<Vec<u8>>> {
    for name in candidates {
        let path = dir.join(name);
        if path.is_file() {
            match Reader::open_readfile(&path) {
                Ok(reader) => return Some(reader),
                Err(err) => warn!("failed to open MMDB {:?}: {}", path, err),
            }
        }
    }
    None
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ClientPrefixKey {
    V4(u32), // top 24 bits (/24)
    V6(u64), // top 48 bits (/48)
}

pub fn client_prefix_key(ip: IpAddr) -> ClientPrefixKey {
    match ip {
        IpAddr::V4(v4) => ClientPrefixKey::V4(u32::from(v4) & 0xFFFF_FF00),
        IpAddr::V6(v6) => ClientPrefixKey::V6((u128::from(v6) >> 80) as u64),
    }
}

#[allow(dead_code)]
pub fn normalize_client_prefix(ip: IpAddr) -> String {
    match ip {
        IpAddr::V4(v4) => {
            let [a, b, c, _] = v4.octets();
            format!("{a}.{b}.{c}.0/24")
        }
        IpAddr::V6(v6) => {
            let seg = v6.segments();
            format!("{:x}:{:x}:{:x}::/48", seg[0], seg[1], seg[2])
        }
    }
}

pub fn haversine_km(lat1: f64, lon1: f64, lat2: f64, lon2: f64) -> f64 {
    const EARTH_RADIUS_KM: f64 = 6371.0;
    let rlat1 = lat1.to_radians();
    let rlat2 = lat2.to_radians();
    let dlat = (lat2 - lat1).to_radians();
    let dlon = (lon2 - lon1).to_radians();
    let a = (dlat / 2.0).sin().powi(2) + rlat1.cos() * rlat2.cos() * (dlon / 2.0).sin().powi(2);
    let c = 2.0 * a.sqrt().atan2((1.0 - a).sqrt());
    EARTH_RADIUS_KM * c
}
