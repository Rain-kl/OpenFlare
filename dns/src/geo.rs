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

        if let Some(rec) = lookup_record::<geoip2::City>(&self.city_db, ip) {
            if let Some(country) = rec.country.iso_code {
                profile.country_iso = country.to_ascii_uppercase();
            }
            if let Some(sub_code) = rec.subdivisions.first().and_then(|first| first.iso_code)
                && !profile.country_iso.is_empty()
            {
                profile.region_code =
                    format!("{}-{}", profile.country_iso, sub_code.to_ascii_uppercase());
            }
            profile.latitude = rec.location.latitude;
            profile.longitude = rec.location.longitude;
        }

        if profile.country_iso.is_empty()
            && let Some(rec) = lookup_record::<geoip2::Country>(&self.country_db, ip)
            && let Some(iso) = rec.country.iso_code
        {
            profile.country_iso = iso.to_ascii_uppercase();
        }

        if let Some(rec) = lookup_record::<geoip2::Asn>(&self.asn_db, ip)
            && let Some(num) = rec.autonomous_system_number
        {
            profile.asn = num;
        }

        profile
    }
}

fn lookup_record<'a, T: serde::Deserialize<'a>>(
    db: &'a Option<Reader<Vec<u8>>>,
    ip: IpAddr,
) -> Option<T> {
    db.as_ref()?.lookup(ip).ok()?.decode::<T>().ok()?
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

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::Ipv4Addr;

    fn push_header(buf: &mut Vec<u8>, ty: u8, size: usize) {
        assert!(size < 29);
        if ty <= 7 {
            buf.push((ty << 5) | (size as u8));
        } else {
            buf.push(size as u8);
            buf.push(ty - 7);
        }
    }

    fn push_str(buf: &mut Vec<u8>, s: &str) {
        push_header(buf, 2, s.len());
        buf.extend_from_slice(s.as_bytes());
    }

    fn push_uint(buf: &mut Vec<u8>, ty: u8, val: u64) {
        let bytes = val.to_be_bytes();
        let trimmed = match bytes.iter().position(|&b| b != 0) {
            Some(idx) => &bytes[idx..],
            None => &[],
        };
        push_header(buf, ty, trimmed.len());
        buf.extend_from_slice(trimmed);
    }

    fn push_f64(buf: &mut Vec<u8>, val: f64) {
        push_header(buf, 3, 8);
        buf.extend_from_slice(&val.to_be_bytes());
    }

    fn push_map(buf: &mut Vec<u8>, len: usize) {
        push_header(buf, 7, len);
    }

    fn push_array(buf: &mut Vec<u8>, len: usize) {
        push_header(buf, 11, len);
    }

    fn build_single_node_v4_mmdb(db_type: &str, data: &[u8]) -> Reader<Vec<u8>> {
        let mut buf = Vec::new();
        // 1 node, 24-bit records:
        // left (0.0.0.0/1) -> node_count (1) + 16 + 0 = 17 (data offset 0)
        // right (128.0.0.0/1) -> node_count (1) (not found)
        buf.extend_from_slice(&[0x00, 0x00, 0x11, 0x00, 0x00, 0x01]);
        buf.extend_from_slice(&[0u8; 16]);
        buf.extend_from_slice(data);
        buf.extend_from_slice(b"\xab\xcd\xefMaxMind.com");

        push_map(&mut buf, 9);
        push_str(&mut buf, "binary_format_major_version");
        push_uint(&mut buf, 5, 2);
        push_str(&mut buf, "binary_format_minor_version");
        push_uint(&mut buf, 5, 0);
        push_str(&mut buf, "build_epoch");
        push_uint(&mut buf, 9, 1_700_000_000);
        push_str(&mut buf, "database_type");
        push_str(&mut buf, db_type);
        push_str(&mut buf, "description");
        push_map(&mut buf, 1);
        push_str(&mut buf, "en");
        push_str(&mut buf, "test");
        push_str(&mut buf, "ip_version");
        push_uint(&mut buf, 5, 4);
        push_str(&mut buf, "languages");
        push_array(&mut buf, 1);
        push_str(&mut buf, "en");
        push_str(&mut buf, "node_count");
        push_uint(&mut buf, 6, 1);
        push_str(&mut buf, "record_size");
        push_uint(&mut buf, 5, 24);

        let reader = Reader::from_source(buf).expect("valid synthetic mmdb");
        reader.verify().expect("mmdb verifies");
        reader
    }

    #[test]
    fn lookup_client_extracts_city_and_asn_fields() {
        let mut city_data = Vec::new();
        push_map(&mut city_data, 3);
        push_str(&mut city_data, "country");
        push_map(&mut city_data, 1);
        push_str(&mut city_data, "iso_code");
        push_str(&mut city_data, "hk");
        push_str(&mut city_data, "subdivisions");
        push_array(&mut city_data, 1);
        push_map(&mut city_data, 1);
        push_str(&mut city_data, "iso_code");
        push_str(&mut city_data, "hk");
        push_str(&mut city_data, "location");
        push_map(&mut city_data, 2);
        push_str(&mut city_data, "latitude");
        push_f64(&mut city_data, 22.3193);
        push_str(&mut city_data, "longitude");
        push_f64(&mut city_data, 114.1694);

        let mut asn_data = Vec::new();
        push_map(&mut asn_data, 1);
        push_str(&mut asn_data, "autonomous_system_number");
        push_uint(&mut asn_data, 6, 13335);

        let geo = GeoLookup {
            city_db: Some(build_single_node_v4_mmdb("GeoLite2-City", &city_data)),
            country_db: None,
            asn_db: Some(build_single_node_v4_mmdb("GeoLite2-ASN", &asn_data)),
        };

        let found = geo.lookup_client(IpAddr::V4(Ipv4Addr::new(1, 1, 1, 1)), "v4");
        assert_eq!(found.country_iso, "HK");
        assert_eq!(found.region_code, "HK-HK");
        assert_eq!(found.latitude, Some(22.3193));
        assert_eq!(found.longitude, Some(114.1694));
        assert_eq!(found.asn, 13335);
        assert_eq!(found.client_prefix, "1.1.1.0/24");

        let missing = geo.lookup_client(IpAddr::V4(Ipv4Addr::new(128, 0, 0, 1)), "v4");
        assert_eq!(missing.country_iso, "");
        assert_eq!(missing.region_code, "");
        assert_eq!(missing.latitude, None);
        assert_eq!(missing.longitude, None);
        assert_eq!(missing.asn, 0);
    }

    #[test]
    fn lookup_client_falls_back_to_country_db() {
        let mut country_data = Vec::new();
        push_map(&mut country_data, 1);
        push_str(&mut country_data, "country");
        push_map(&mut country_data, 1);
        push_str(&mut country_data, "iso_code");
        push_str(&mut country_data, "sg");

        let geo = GeoLookup {
            city_db: None,
            country_db: Some(build_single_node_v4_mmdb("GeoLite2-Country", &country_data)),
            asn_db: None,
        };

        let profile = geo.lookup_client(IpAddr::V4(Ipv4Addr::new(8, 8, 8, 8)), "v4");
        assert_eq!(profile.country_iso, "SG");
        assert_eq!(profile.region_code, "");
        assert_eq!(profile.asn, 0);
    }
}
