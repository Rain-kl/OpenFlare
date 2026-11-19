use crate::{
    config::AppliedConfig,
    protocol::config::{PagesDeployment, Route},
};
use bytes::Bytes;
use http::{header, Method};
use pingora::{proxy::Session, Error, Result};
use pingora_error::ErrorType::InternalError;
use pingora_http::ResponseHeader;
use std::{
    path::{Path, PathBuf},
    time::SystemTime,
};
use tokio::io::{AsyncReadExt, AsyncSeekExt};
use url::Url;

pub(super) fn entry_file(deployment: &PagesDeployment) -> &str {
    if deployment.entry_file.trim().is_empty() {
        "index.html"
    } else {
        deployment.entry_file.trim_start_matches('/')
    }
}

pub fn pages_resolve(deployment: &PagesDeployment, root: &Path, path: &str) -> Option<PathBuf> {
    let path = decode_path(path)?;
    let relative = path.trim_start_matches('/');
    let mut candidate = root.join(relative);
    if relative.is_empty() && deployment.spa_fallback_enabled {
        return Some(root.join(pages_fallback(deployment)));
    }
    if relative.is_empty() || relative.ends_with('/') {
        candidate = candidate.join(entry_file(deployment));
    }
    Some(candidate)
}

fn decode_path(path: &str) -> Option<String> {
    // URI paths preserve '+' and use percent escapes for encoded bytes.
    let path = crate::policy::percent_decode(&path.replace('+', "%2B"));
    (!path.split('/').any(|segment| {
        matches!(segment, ".." | ".openflare-pages.json")
            || segment.contains('\\')
            || segment.contains('\0')
    }))
    .then_some(path)
}

fn pages_fallback(deployment: &PagesDeployment) -> &str {
    let path = deployment.spa_fallback_path.trim().trim_start_matches('/');
    if path.is_empty() {
        "index.html"
    } else {
        path
    }
}

pub fn pages_api_target(deployment: &PagesDeployment, path: &str) -> Option<String> {
    if !deployment.api_proxy_enabled {
        return None;
    }
    let raw_prefix = deployment.api_proxy_path.trim();
    if raw_prefix.is_empty() || deployment.api_proxy_pass.trim().is_empty() {
        return None;
    }
    let prefix = format!("/{}", raw_prefix.trim_matches('/'));
    let (path, query) = path
        .split_once('?')
        .map_or((path, None), |(path, query)| (path, Some(query)));
    let tail = path.strip_prefix(&prefix)?;
    let mut target = Url::parse(&deployment.api_proxy_pass).ok()?;
    let rewrite = deployment.api_proxy_rewrite.trim();
    let uri = if !rewrite.is_empty() && (tail.is_empty() || tail.starts_with('/')) {
        format!(
            "/{}{}",
            rewrite.trim_matches('/'),
            if rewrite == "/" {
                tail.trim_start_matches('/')
            } else {
                tail
            }
        )
    } else if let Some(raw) = deployment
        .api_proxy_pass
        .split_once("://")
        .map(|(_, raw)| raw)
    {
        if raw.contains('/') {
            format!("{}{tail}", target.path())
        } else {
            path.to_owned()
        }
    } else {
        path.to_owned()
    };
    target.set_path(&uri);
    if query.is_some() {
        target.set_query(query);
    }
    Some(target.into())
}

fn release_root(
    config: &AppliedConfig,
    deployment: &PagesDeployment,
    state_dir: &Path,
) -> Option<PathBuf> {
    let hash = config.pages_releases.get(&deployment.project_id)?;
    Some(
        state_dir
            .join("pages")
            .join("projects")
            .join(deployment.project_id.to_string())
            .join("releases")
            .join(hash),
    )
}

pub fn resources_ready(config: &AppliedConfig, state_dir: &Path) -> bool {
    config
        .routes()
        .filter_map(|route| route.pages.as_ref())
        .all(|deployment| {
            release_root(config, deployment, state_dir)
                .is_some_and(|root| root.join(entry_file(deployment)).is_file())
        })
}

pub async fn serve_pages(
    session: &mut Session,
    route: &Route,
    config: &AppliedConfig,
    state_dir: &Path,
    pow_cookie: Option<&str>,
) -> Result<bool> {
    let Some(deployment) = route.pages.as_ref() else {
        return Ok(false);
    };
    let req = session.req_header();
    let method = req.method.clone();
    let path = req.uri.path();
    let Some(root) = release_root(config, deployment, state_dir) else {
        return Ok(false);
    };
    let Some(mut file_path) = pages_resolve(deployment, &root, path) else {
        return Ok(false);
    };
    if tokio::fs::metadata(&file_path)
        .await
        .is_ok_and(|metadata| metadata.is_dir())
    {
        let uri = req
            .uri
            .path_and_query()
            .map_or(path, |value| value.as_str());
        let mut location = format!("{path}/");
        location.push_str(&uri[path.len()..]);
        let mut response = ResponseHeader::build(301, None)?;
        response.insert_header(header::LOCATION, location)?;
        response.insert_header(header::CONTENT_LENGTH, 0)?;
        session
            .write_response_header(Box::new(response), true)
            .await?;
        return Ok(true);
    }
    let mut file = match tokio::fs::File::open(&file_path).await {
        Ok(file) => file,
        Err(_) if deployment.spa_fallback_enabled => {
            file_path = root.join(pages_fallback(deployment));
            let Ok(file) = tokio::fs::File::open(&file_path).await else {
                return Ok(false);
            };
            file
        }
        Err(_) => return Ok(false),
    };
    let metadata = file
        .metadata()
        .await
        .map_err(|error| Error::because(InternalError, "stat Pages file", error))?;
    if !metadata.is_file() {
        return Ok(false);
    }
    if method != Method::GET && method != Method::HEAD {
        session.respond_error(405).await?;
        return Ok(true);
    }

    let size = metadata.len();
    let modified = metadata.modified().unwrap_or(SystemTime::UNIX_EPOCH);
    let modified_seconds = modified
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    let etag = format!("\"{modified_seconds:x}-{size:x}\"");
    let http_date = httpdate::fmt_http_date(modified);
    let headers = &req.headers;
    let header_value = |name| headers.get(name).and_then(|value| value.to_str().ok());
    let not_modified = header_value(header::IF_NONE_MATCH).map_or_else(
        || {
            header_value(header::IF_MODIFIED_SINCE)
                .and_then(|value| httpdate::parse_http_date(value).ok())
                .is_some_and(|client_date| {
                    client_date
                        >= SystemTime::UNIX_EPOCH + std::time::Duration::from_secs(modified_seconds)
                })
        },
        |value| {
            value.split(',').any(|candidate| {
                candidate.trim() == "*" || candidate.trim().trim_start_matches("W/") == etag
            })
        },
    );
    let mut start = 0;
    let mut length = size;
    let mut status = if not_modified { 304 } else { 200 };
    if !not_modified
        && header_value(header::IF_RANGE).is_none_or(|value| value == etag || value == http_date)
    {
        if let Some(range) = header_value(header::RANGE).and_then(parse_range) {
            if let Some((first, last)) = range.apply(size) {
                start = first;
                length = last - first + 1;
                status = 206;
            } else {
                status = 416;
                length = 0;
            }
        }
    }
    let mut response = ResponseHeader::build(status, None)?;
    response.insert_header(header::ACCEPT_RANGES, "bytes")?;
    response.insert_header(header::LAST_MODIFIED, http_date)?;
    response.insert_header(header::ETAG, etag)?;
    if let Some(cookie) = pow_cookie {
        response.append_header(header::SET_COOKIE, cookie)?;
    }
    if route.site_auth_ttl_hours != 0 {
        response.insert_header(header::CACHE_CONTROL, "private, no-store")?;
    }
    if status == 206 {
        response.insert_header(
            header::CONTENT_RANGE,
            format!("bytes {start}-{}/{size}", start + length - 1),
        )?;
    } else if status == 416 {
        response.insert_header(header::CONTENT_RANGE, format!("bytes */{size}"))?;
    }
    if status != 304 {
        response.insert_header(header::CONTENT_LENGTH, length)?;
        response.insert_header(header::CONTENT_TYPE, content_type(file_path.file_name()))?;
    }
    let has_body = method != Method::HEAD && status != 304 && length != 0;
    session
        .write_response_header(Box::new(response), !has_body)
        .await?;
    if has_body {
        file.seek(std::io::SeekFrom::Start(start))
            .await
            .map_err(|error| Error::because(InternalError, "seek Pages file", error))?;
        let mut buffer = vec![0; 64 * 1024];
        while length > 0 {
            let count = length.min(buffer.len() as u64) as usize;
            file.read_exact(&mut buffer[..count])
                .await
                .map_err(|error| Error::because(InternalError, "read Pages file", error))?;
            length -= count as u64;
            session
                .write_response_body(Some(Bytes::copy_from_slice(&buffer[..count])), length == 0)
                .await?;
        }
    }
    Ok(true)
}

enum ByteRange {
    Absolute(u64, Option<u64>),
    Suffix(u64),
}

impl ByteRange {
    fn apply(self, size: u64) -> Option<(u64, u64)> {
        let max = size.checked_sub(1)?;
        match self {
            Self::Absolute(start, end) if start <= max => {
                let end = end.unwrap_or(max).min(max);
                (start <= end).then_some((start, end))
            }
            Self::Suffix(length) if length > 0 => Some((size.saturating_sub(length), max)),
            _ => None,
        }
    }
}

fn parse_range(value: &str) -> Option<ByteRange> {
    let (start, end) = value.strip_prefix("bytes=")?.split_once('-')?;
    if start.is_empty() {
        Some(ByteRange::Suffix(end.parse().ok()?))
    } else {
        let end = if end.is_empty() {
            None
        } else {
            Some(end.parse().ok()?)
        };
        Some(ByteRange::Absolute(start.parse().ok()?, end))
    }
}

pub fn validate(deployment: &PagesDeployment) -> anyhow::Result<()> {
    for path in [entry_file(deployment), pages_fallback(deployment)] {
        if decode_path(path).is_none() || path.starts_with('/') {
            anyhow::bail!("Pages deployment has an invalid entry or fallback path");
        }
    }
    if deployment.api_proxy_enabled {
        let url = Url::parse(&deployment.api_proxy_pass)?;
        if !matches!(url.scheme(), "http" | "https") || url.host().is_none() {
            return Err(anyhow::anyhow!("Pages API target must be an HTTP(S) URL"));
        }
    }
    Ok(())
}

fn content_type(file_name: Option<&std::ffi::OsStr>) -> &'static str {
    let name = file_name
        .and_then(|value| value.to_str())
        .unwrap_or_default();
    match name
        .rsplit_once('.')
        .map(|(_, extension)| extension.to_ascii_lowercase())
    {
        Some(extension) => match extension.as_str() {
            "html" | "htm" => "text/html; charset=utf-8",
            "css" => "text/css; charset=utf-8",
            "js" | "mjs" => "text/javascript; charset=utf-8",
            "json" => "application/json; charset=utf-8",
            "svg" => "image/svg+xml",
            "png" => "image/png",
            "jpg" | "jpeg" => "image/jpeg",
            "webp" => "image/webp",
            "avif" => "image/avif",
            "ico" => "image/x-icon",
            "txt" => "text/plain; charset=utf-8",
            "xml" => "application/xml; charset=utf-8",
            "wasm" => "application/wasm",
            "woff" => "font/woff",
            "woff2" => "font/woff2",
            _ => "application/octet-stream",
        },
        None => "application/octet-stream",
    }
}
