use maxminddb::Reader;
use std::path::Path;
#[derive(Debug, Clone, Default)]
pub struct GeoResult {
    pub country: Option<String>,
    pub region: Option<String>,
}

pub struct GeoIp {
    city: Option<Reader<Vec<u8>>>,
    country: Option<Reader<Vec<u8>>>,
}

impl GeoIp {
    pub fn open(country_path: Option<&Path>, city_path: Option<&Path>) -> Option<Self> {
        let country = country_path.and_then(|path| Reader::open_readfile(path).ok());
        let city = city_path.and_then(|path| Reader::open_readfile(path).ok());
        (country.is_some() || city.is_some()).then_some(Self { city, country })
    }

    pub fn lookup(&self, ip: &str) -> Option<GeoResult> {
        let ip = ip.parse().ok()?;
        if let Some(reader) = self.city.as_ref() {
            if let Some(Some(city)) = reader
                .lookup(ip)
                .ok()
                .and_then(|lookup| lookup.decode::<maxminddb::geoip2::City>().ok())
            {
                let country = city.country.iso_code.map(|code| code.to_uppercase());
                let subdivision = city.subdivisions.first().and_then(|item| item.iso_code);
                let region = geo_region(country.as_deref(), subdivision);
                if country.is_some() {
                    return Some(GeoResult { country, region });
                }
            }
        }
        if let Some(reader) = self.country.as_ref() {
            if let Some(Some(country_record)) = reader
                .lookup(ip)
                .ok()
                .and_then(|lookup| lookup.decode::<maxminddb::geoip2::Country>().ok())
            {
                return Some(GeoResult {
                    country: country_record
                        .country
                        .iso_code
                        .map(|code| code.to_uppercase()),
                    region: None,
                });
            }
        }
        None
    }
}

pub fn geo_region(country: Option<&str>, subdivision: Option<&str>) -> Option<String> {
    let country = country?;
    let subdivision = subdivision?;
    if subdivision.starts_with(&format!("{country}-")) {
        Some(subdivision.to_owned())
    } else {
        Some(format!("{country}-{subdivision}"))
    }
}

#[derive(Clone)]
pub struct GeoIpStore {
    current: std::sync::Arc<std::sync::RwLock<Option<GeoIp>>>,
    directory: std::path::PathBuf,
}

impl GeoIpStore {
    pub fn open(directory: std::path::PathBuf) -> Self {
        let current = GeoIp::open(
            Some(&directory.join(COUNTRY_FILE)),
            Some(&directory.join(CITY_FILE)),
        );
        Self {
            current: std::sync::Arc::new(std::sync::RwLock::new(current)),
            directory,
        }
    }

    pub fn lookup(&self, ip: &str) -> Option<GeoResult> {
        self.current.read().ok()?.as_ref()?.lookup(ip)
    }

    pub async fn refresh_periodically(&self) {
        let client = match reqwest::Client::builder()
            .https_only(true)
            .connect_timeout(std::time::Duration::from_secs(10))
            .timeout(std::time::Duration::from_secs(120))
            .build()
        {
            Ok(client) => client,
            Err(error) => {
                tracing::error!(%error, "cannot initialize GeoIP updater");
                return;
            }
        };
        // Give already seeded nodes time to start serving before fetching an update.
        tokio::time::sleep(std::time::Duration::from_secs(60)).await;
        loop {
            let mut failed = false;
            for file in [COUNTRY_FILE, CITY_FILE] {
                if !self.needs_refresh(file) {
                    continue;
                }
                match self.download(&client, file).await {
                    Ok(()) => {
                        let fresh = Self::open(self.directory.clone());
                        if let Ok(mut current) = self.current.write() {
                            if let Ok(mut replacement) = fresh.current.write() {
                                *current = replacement.take();
                            }
                        }
                    }
                    Err(error) => {
                        tracing::warn!(file, %error, "GeoIP database refresh failed");
                        failed = true;
                    }
                }
            }
            let seconds = if failed { 900 } else { 86_400 };
            tokio::time::sleep(std::time::Duration::from_secs(seconds)).await;
        }
    }

    fn needs_refresh(&self, file: &str) -> bool {
        let age = std::fs::metadata(self.directory.join(file))
            .and_then(|metadata| metadata.modified())
            .ok()
            .and_then(|modified| modified.elapsed().ok());
        age.is_none_or(|age| age >= std::time::Duration::from_secs(86_400))
    }

    async fn download(&self, client: &reqwest::Client, name: &str) -> anyhow::Result<()> {
        use std::io::Write as _;

        const MAX_SIZE: usize = 128 * 1024 * 1024;
        let url = format!("https://github.com/FyraLabs/geolite2/releases/latest/download/{name}");
        let mut response = client.get(url).send().await?.error_for_status()?;
        let mut output = tempfile::NamedTempFile::new_in(&self.directory)?;
        let mut written = 0usize;
        while let Some(chunk) = response.chunk().await? {
            written = written.saturating_add(chunk.len());
            anyhow::ensure!(
                written <= MAX_SIZE,
                "GeoIP database exceeds the download limit"
            );
            output.write_all(&chunk)?;
        }
        let candidate = Reader::open_readfile(output.path())?;
        let kind = &candidate.metadata().database_type;
        let correct_type = if name == CITY_FILE {
            kind.ends_with("-City")
        } else {
            kind.ends_with("-Country")
        };
        anyhow::ensure!(correct_type, "unexpected GeoIP database type {kind}");
        output.as_file().sync_all()?;
        output.persist(self.directory.join(name))?;
        Ok(())
    }
}

const COUNTRY_FILE: &str = "GeoLite2-Country.mmdb";
const CITY_FILE: &str = "GeoLite2-City.mmdb";
