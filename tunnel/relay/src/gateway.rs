use bytes::Bytes;
use http::{HeaderMap, HeaderValue, Method, Request, Response, StatusCode, Version};
use http_body_util::{combinators::BoxBody, BodyExt, Empty, Full};
use hyper::body::Incoming;
use hyper::service::service_fn;
use hyper_util::rt::{TokioExecutor, TokioIo};
use hyper_util::server::conn::auto::Builder as ServerBuilder;
use openflare_tunnel_common::hardening::constant_time_eq;
use openflare_tunnel_common::protocol::{
    HealthCheckResponse, RelayVerifiedAttachment, HEADER_OPENFLARE_RELAY_AUTH,
    HEADER_OPENFLARE_TUNNEL_ID, PATH_RELAY_HEALTH,
};
use openflare_tunnel_common::TUNNEL_VERSION;
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::atomic::{AtomicBool, AtomicI64, AtomicUsize, Ordering};
use std::sync::Arc;
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::{broadcast, RwLock};

pub type RespBody = BoxBody<Bytes, hyper::Error>;

pub struct ConnectorEntry {
    pub connector_id: String,
    pub service_name: String,
    pub weight: i32,
    pub candidate_addrs: Vec<SocketAddr>,
    pub active_addr: RwLock<Option<SocketAddr>>,
    pub healthy: AtomicBool,
    pub rtt_ms: AtomicI64,
    pub active_streams: AtomicI64,
}

impl ConnectorEntry {
    pub fn new(
        connector_id: String,
        service_name: String,
        weight: i32,
        candidate_addrs: Vec<SocketAddr>,
    ) -> Self {
        Self {
            connector_id,
            service_name,
            weight,
            candidate_addrs,
            active_addr: RwLock::new(None),
            healthy: AtomicBool::new(false),
            rtt_ms: AtomicI64::new(-1),
            active_streams: AtomicI64::new(0),
        }
    }

    pub async fn preferred_addrs(&self) -> Vec<SocketAddr> {
        let mut addrs = Vec::with_capacity(self.candidate_addrs.len());
        if let Some(active) = *self.active_addr.read().await {
            addrs.push(active);
        }
        for addr in &self.candidate_addrs {
            if !addrs.contains(addr) {
                addrs.push(*addr);
            }
        }
        addrs
    }
}

pub struct RelayGatewayState {
    pub relay_id: String,
    gateway_secret: RwLock<String>,
    draining: AtomicBool,
    rr_cursor: AtomicUsize,
    connectors: RwLock<HashMap<u64, Vec<Arc<ConnectorEntry>>>>,
}

impl RelayGatewayState {
    pub fn new(relay_id: String, gateway_secret: String) -> Self {
        Self {
            relay_id,
            gateway_secret: RwLock::new(gateway_secret),
            draining: AtomicBool::new(false),
            rr_cursor: AtomicUsize::new(0),
            connectors: RwLock::new(HashMap::new()),
        }
    }

    pub async fn set_gateway_secret(&self, secret: String) {
        *self.gateway_secret.write().await = secret;
    }

    pub fn set_draining(&self, draining: bool) {
        self.draining.store(draining, Ordering::Release);
    }

    pub fn is_draining(&self) -> bool {
        self.draining.load(Ordering::Acquire)
    }

    pub async fn replace_connectors(&self, next: HashMap<u64, Vec<Arc<ConnectorEntry>>>) {
        let mut guard = self.connectors.write().await;
        // Preserve probe status across service reloads where service_name is unchanged.
        let mut prev_by_name: HashMap<(u64, String), Arc<ConnectorEntry>> = HashMap::new();
        for (tid, entries) in guard.iter() {
            for e in entries {
                prev_by_name.insert((*tid, e.service_name.clone()), e.clone());
            }
        }
        let mut merged = HashMap::with_capacity(next.len());
        for (tid, entries) in next {
            let mut updated = Vec::with_capacity(entries.len());
            for entry in entries {
                if let Some(old) = prev_by_name.get(&(tid, entry.service_name.clone())) {
                    if old.candidate_addrs == entry.candidate_addrs {
                        updated.push(old.clone());
                        continue;
                    }
                }
                updated.push(entry);
            }
            merged.insert(tid, updated);
        }
        *guard = merged;
    }

    pub async fn all_connectors(&self) -> Vec<Arc<ConnectorEntry>> {
        let guard = self.connectors.read().await;
        guard.values().flatten().cloned().collect()
    }

    pub async fn verified_attachments(&self) -> Vec<RelayVerifiedAttachment> {
        let guard = self.connectors.read().await;
        let mut out = Vec::new();
        for entry in guard.values().flatten() {
            if entry.healthy.load(Ordering::Acquire) {
                let rtt = entry.rtt_ms.load(Ordering::Acquire);
                out.push(RelayVerifiedAttachment {
                    connector_id: entry.connector_id.clone(),
                    service_name: entry.service_name.clone(),
                    rtt_ms: rtt.max(0),
                    active_streams: entry.active_streams.load(Ordering::Acquire),
                });
            }
        }
        out
    }

    pub fn connector_stats(&self, verified: &[RelayVerifiedAttachment]) -> (i64, i64) {
        let active_connectors = verified.len() as i64;
        let active_streams: i64 = verified.iter().map(|v| v.active_streams).sum();
        (active_connectors, active_streams)
    }

    pub async fn select_connectors(&self, tunnel_id: u64) -> Vec<Arc<ConnectorEntry>> {
        let guard = self.connectors.read().await;
        let Some(list) = guard.get(&tunnel_id) else {
            return Vec::new();
        };
        let mut healthy: Vec<Arc<ConnectorEntry>> = list
            .iter()
            .filter(|c| c.healthy.load(Ordering::Acquire))
            .cloned()
            .collect();
        let mut fallback: Vec<Arc<ConnectorEntry>> = list
            .iter()
            .filter(|c| !c.healthy.load(Ordering::Acquire))
            .cloned()
            .collect();

        let cursor = self.rr_cursor.fetch_add(1, Ordering::Relaxed);
        if !healthy.is_empty() {
            let rot = cursor % healthy.len();
            healthy.rotate_left(rot);
        }
        if !fallback.is_empty() {
            let rot = cursor % fallback.len();
            fallback.rotate_left(rot);
        }
        healthy.extend(fallback);
        healthy
    }
}

pub fn spawn_gateway_server(
    bind_addr: String,
    state: Arc<RelayGatewayState>,
    mut shutdown: broadcast::Receiver<bool>,
) {
    tokio::spawn(async move {
        let listener = match TcpListener::bind(&bind_addr).await {
            Ok(l) => l,
            Err(err) => {
                tracing::error!(%err, addr = %bind_addr, "failed to bind relay gateway listener");
                return;
            }
        };
        tracing::info!(addr = %bind_addr, "relay HTTP/WS ingress gateway listening");

        loop {
            tokio::select! {
                _ = shutdown.recv() => return,
                accepted = listener.accept() => {
                    match accepted {
                        Ok((stream, _)) => {
                            let _ = stream.set_nodelay(true);
                            let state = state.clone();
                            tokio::spawn(async move {
                                let io = TokioIo::new(stream);
                                let svc = service_fn(move |req| {
                                    let state = state.clone();
                                    async move { Ok::<_, hyper::Error>(handle_request(state, req).await) }
                                });
                                if let Err(err) = ServerBuilder::new(TokioExecutor::new())
                                    .serve_connection_with_upgrades(io, svc)
                                    .await
                                {
                                    tracing::debug!(%err, "relay gateway connection closed");
                                }
                            });
                        }
                        Err(err) => {
                            tracing::warn!(%err, "relay gateway accept failed");
                        }
                    }
                }
            }
        }
    });
}

pub async fn handle_request(
    state: Arc<RelayGatewayState>,
    mut req: Request<Incoming>,
) -> Response<RespBody> {
    if req.uri().path() == PATH_RELAY_HEALTH && req.method() == Method::GET {
        let payload = HealthCheckResponse {
            status: if state.is_draining() {
                "draining".to_owned()
            } else {
                "ok".to_owned()
            },
            version: TUNNEL_VERSION.to_owned(),
            role: "relay".to_owned(),
            relay_id: (!state.relay_id.is_empty()).then(|| state.relay_id.clone()),
            draining: Some(state.is_draining()),
        };
        let bytes = serde_json::to_vec(&payload).unwrap_or_else(|_| b"{}".to_vec());
        return Response::builder()
            .status(StatusCode::OK)
            .header(http::header::CONTENT_TYPE, "application/json")
            .body(full_body(bytes))
            .unwrap();
    }

    let expected_secret = state.gateway_secret.read().await.clone();
    let provided_secret = req
        .headers()
        .get(HEADER_OPENFLARE_RELAY_AUTH)
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    if expected_secret.is_empty() || !constant_time_eq(provided_secret, &expected_secret) {
        return text_response(StatusCode::FORBIDDEN, "invalid relay auth");
    }

    let Some(tunnel_id) = req
        .headers()
        .get(HEADER_OPENFLARE_TUNNEL_ID)
        .and_then(|v| v.to_str().ok())
        .and_then(|s| s.parse::<u64>().ok())
    else {
        return text_response(StatusCode::BAD_REQUEST, "missing or invalid tunnel id");
    };

    let candidates = state.select_connectors(tunnel_id).await;
    if candidates.is_empty() {
        return text_response(StatusCode::SERVICE_UNAVAILABLE, "no active tunnel connectors");
    }

    // Strip internal gateway auth secret before forwarding toward connector.
    req.headers_mut().remove(HEADER_OPENFLARE_RELAY_AUTH);

    let is_upgrade = req
        .headers()
        .get(http::header::CONNECTION)
        .and_then(|v| v.to_str().ok())
        .is_some_and(|v| v.to_ascii_lowercase().contains("upgrade"))
        && req.headers().contains_key(http::header::UPGRADE);

    if is_upgrade {
        return forward_with_upgrade(candidates, req).await;
    }

    let (parts, body) = req.into_parts();
    let collected = match body.collect().await {
        Ok(b) => b.to_bytes(),
        Err(err) => {
            tracing::debug!(%err, "failed reading gateway request body");
            return text_response(StatusCode::BAD_REQUEST, "invalid request body");
        }
    };

    for entry in candidates {
        let addrs = entry.preferred_addrs().await;
        for addr in addrs {
            let _guard = StreamCounterGuard::new(&entry);
            match forward_single(addr, &parts, collected.clone()).await {
                Ok(resp) => {
                    entry.healthy.store(true, Ordering::Release);
                    return resp;
                }
                Err(err) => {
                    tracing::debug!(%err, connector = %entry.connector_id, %addr, "relay connector forward failed, trying next");
                }
            }
        }
    }

    text_response(StatusCode::BAD_GATEWAY, "all tunnel connectors unreachable")
}

async fn forward_with_upgrade(
    candidates: Vec<Arc<ConnectorEntry>>,
    mut req: Request<Incoming>,
) -> Response<RespBody> {
    let mut selected_stream = None;
    for entry in candidates {
        for addr in entry.preferred_addrs().await {
            match TcpStream::connect(addr).await {
                Ok(s) => {
                    let _ = s.set_nodelay(true);
                    selected_stream = Some((entry, s));
                    break;
                }
                Err(_) => continue,
            }
        }
        if selected_stream.is_some() {
            break;
        }
    }

    let Some((entry, stream)) = selected_stream else {
        return text_response(StatusCode::BAD_GATEWAY, "no tunnel connector available for upgrade");
    };

    entry.active_streams.fetch_add(1, Ordering::Relaxed);
    let entry_clone = entry.clone();
    let io = TokioIo::new(stream);
    let (mut sender, conn) = match hyper::client::conn::http1::handshake(io).await {
        Ok(pair) => pair,
        Err(err) => {
            entry.active_streams.fetch_sub(1, Ordering::Relaxed);
            tracing::debug!(%err, "upstream ws handshake failed");
            return text_response(StatusCode::BAD_GATEWAY, "tunnel handshake failed");
        }
    };
    tokio::spawn(async move {
        let _ = conn.with_upgrades().await;
    });

    let client_upgrade = hyper::upgrade::on(&mut req);
    let mut upstream_req = Request::builder()
        .method(req.method().clone())
        .uri(req.uri().clone())
        .version(Version::HTTP_11);
    if let Some(headers) = upstream_req.headers_mut() {
        *headers = req.headers().clone();
    }
    let upstream_req = match upstream_req.body(Empty::<Bytes>::new()) {
        Ok(r) => r,
        Err(_) => {
            entry_clone.active_streams.fetch_sub(1, Ordering::Relaxed);
            return text_response(StatusCode::INTERNAL_SERVER_ERROR, "build request failed");
        }
    };

    let mut upstream_resp = match sender.send_request(upstream_req).await {
        Ok(r) => r,
        Err(err) => {
            entry_clone.active_streams.fetch_sub(1, Ordering::Relaxed);
            tracing::debug!(%err, "upstream ws request error");
            return text_response(StatusCode::BAD_GATEWAY, "tunnel request failed");
        }
    };

    if upstream_resp.status() == StatusCode::SWITCHING_PROTOCOLS {
        let status = upstream_resp.status();
        let headers = upstream_resp.headers().clone();
        let server_upgrade = hyper::upgrade::on(&mut upstream_resp);
        tokio::spawn(async move {
            if let (Ok(client), Ok(server)) = (client_upgrade.await, server_upgrade.await) {
                let mut client_io = TokioIo::new(client);
                let mut server_io = TokioIo::new(server);
                let _ = tokio::io::copy_bidirectional(&mut client_io, &mut server_io).await;
            }
            entry_clone.active_streams.fetch_sub(1, Ordering::Relaxed);
        });
        let mut resp = Response::builder().status(status).body(empty_body()).unwrap();
        *resp.headers_mut() = headers;
        return resp;
    }

    entry_clone.active_streams.fetch_sub(1, Ordering::Relaxed);
    let (parts, body) = upstream_resp.into_parts();
    Response::from_parts(parts, body.boxed())
}

async fn forward_single(
    addr: SocketAddr,
    parts: &http::request::Parts,
    body_bytes: Bytes,
) -> anyhow::Result<Response<RespBody>> {
    let stream = tokio::time::timeout(std::time::Duration::from_secs(3), TcpStream::connect(addr))
        .await??;
    let _ = stream.set_nodelay(true);
    let io = TokioIo::new(stream);
    let (mut sender, conn) = hyper::client::conn::http1::handshake(io).await?;
    tokio::spawn(async move {
        let _ = conn.await;
    });

    let mut builder = Request::builder()
        .method(parts.method.clone())
        .uri(parts.uri.clone())
        .version(Version::HTTP_11);
    if let Some(headers) = builder.headers_mut() {
        copy_request_headers(&parts.headers, headers);
    }
    let req = builder.body(full_body(body_bytes.to_vec()))?;
    let resp = sender.send_request(req).await?;
    let (resp_parts, resp_body) = resp.into_parts();
    Ok(Response::from_parts(resp_parts, resp_body.boxed()))
}

fn copy_request_headers(src: &HeaderMap, dst: &mut HeaderMap) {
    for (name, val) in src {
        if matches!(
            name.as_str(),
            "connection" | "keep-alive" | "proxy-authenticate" | "proxy-authorization" | "te" | "trailer" | "transfer-encoding" | "upgrade"
        ) {
            continue;
        }
        if let Ok(cloned) = HeaderValue::from_bytes(val.as_bytes()) {
            dst.append(name.clone(), cloned);
        }
    }
}

struct StreamCounterGuard<'a> {
    entry: &'a ConnectorEntry,
}

impl<'a> StreamCounterGuard<'a> {
    fn new(entry: &'a ConnectorEntry) -> Self {
        entry.active_streams.fetch_add(1, Ordering::Relaxed);
        Self { entry }
    }
}

impl Drop for StreamCounterGuard<'_> {
    fn drop(&mut self) {
        self.entry.active_streams.fetch_sub(1, Ordering::Relaxed);
    }
}

fn full_body(bytes: Vec<u8>) -> RespBody {
    Full::new(Bytes::from(bytes))
        .map_err(|never| match never {})
        .boxed()
}

fn empty_body() -> RespBody {
    Empty::<Bytes>::new()
        .map_err(|never| match never {})
        .boxed()
}

fn text_response(status: StatusCode, msg: &str) -> Response<RespBody> {
    Response::builder()
        .status(status)
        .header(http::header::CONTENT_TYPE, "text/plain; charset=utf-8")
        .body(full_body(msg.as_bytes().to_vec()))
        .unwrap()
}

#[cfg(test)]
mod tests {
    use super::*;
    use hyper::service::service_fn;

    #[tokio::test]
    async fn gateway_enforces_auth_and_fails_over_between_connectors() {
        let healthy_listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let healthy_addr = healthy_listener.local_addr().unwrap();
        tokio::spawn(async move {
            loop {
                if let Ok((stream, _)) = healthy_listener.accept().await {
                    tokio::spawn(async move {
                        let io = TokioIo::new(stream);
                        let _ = ServerBuilder::new(TokioExecutor::new())
                            .serve_connection(
                                io,
                                service_fn(|_req: Request<Incoming>| async move {
                                    Ok::<_, hyper::Error>(Response::new(
                                        Full::new(Bytes::from_static(b"ok-replica"))
                                            .map_err(|never| match never {})
                                            .boxed(),
                                    ))
                                }),
                            )
                            .await;
                    });
                }
            }
        });

        let dead_listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let dead_addr = dead_listener.local_addr().unwrap();
        drop(dead_listener);

        let state = Arc::new(RelayGatewayState::new(
            "relay-1".to_owned(),
            "gw-secret-123".to_owned(),
        ));
        let mut map = HashMap::new();
        map.insert(
            42u64,
            vec![
                Arc::new(ConnectorEntry::new(
                    "conn-dead".into(),
                    "svc-dead".into(),
                    100,
                    vec![dead_addr],
                )),
                Arc::new(ConnectorEntry::new(
                    "conn-live".into(),
                    "svc-live".into(),
                    100,
                    vec![healthy_addr],
                )),
            ],
        );
        state.replace_connectors(map).await;

        let gw_listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let gw_addr = gw_listener.local_addr().unwrap();
        tokio::spawn(async move {
            loop {
                if let Ok((stream, _)) = gw_listener.accept().await {
                    let st = state.clone();
                    tokio::spawn(async move {
                        let io = TokioIo::new(stream);
                        let _ = ServerBuilder::new(TokioExecutor::new())
                            .serve_connection_with_upgrades(
                                io,
                                service_fn(move |req| {
                                    let st = st.clone();
                                    async move { Ok::<_, hyper::Error>(handle_request(st, req).await) }
                                }),
                            )
                            .await;
                    });
                }
            }
        });

        let client = reqwest::Client::new();
        let unauth = client
            .get(format!("http://{gw_addr}/hello"))
            .header(HEADER_OPENFLARE_TUNNEL_ID, "42")
            .header(HEADER_OPENFLARE_RELAY_AUTH, "wrong")
            .send()
            .await
            .unwrap();
        assert_eq!(unauth.status(), StatusCode::FORBIDDEN);

        let ok_resp = client
            .get(format!("http://{gw_addr}/hello"))
            .header(HEADER_OPENFLARE_TUNNEL_ID, "42")
            .header(HEADER_OPENFLARE_RELAY_AUTH, "gw-secret-123")
            .send()
            .await
            .unwrap();
        assert_eq!(ok_resp.status(), StatusCode::OK);
        assert_eq!(ok_resp.text().await.unwrap(), "ok-replica");
    }
}
