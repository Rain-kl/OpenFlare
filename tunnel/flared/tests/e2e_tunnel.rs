use bytes::Bytes;
use http::{Request, StatusCode};
use http_body_util::{BodyExt, Full};
use hyper::body::Incoming;
use hyper::service::service_fn;
use hyper_util::rt::{TokioExecutor, TokioIo};
use hyper_util::server::conn::auto::Builder as ServerBuilder;
use openflare_relay::gateway::{handle_request, RelayGatewayState};
use openflare_relay::rathole_mgr::RatholeRelayManager;
use openflare_tunnel_common::protocol::{
    RelayRuntimeConfigResponse, RelayServiceSpec, TunnelConnectorAttachmentGrant,
    TunnelConnectorConfigResponse, TunnelHTTPRouteRule, TunnelRelayCandidate,
    HEADER_OPENFLARE_RELAY_AUTH, HEADER_OPENFLARE_ROUTE_ID, HEADER_OPENFLARE_TUNNEL_ID,
};
use openflared::ingress::spawn_ingress_server;
use openflared::rathole_pool::RatholeClientPool;
use std::net::TcpListener as StdTcpListener;
use std::sync::Arc;
use std::time::Duration;
use tokio::net::TcpListener;
use tokio::sync::broadcast;

const TEST_NOISE_PRIV: &str = "dwdtCnMYpX08FsFyUbJmRd9ML4frwJkqsXf7pR25LCo=";
const TEST_NOISE_PUB: &str = "hSDwCYkwp1R0i33ctD73Wg2/Og0mOBr066SpjqqbTmo=";

fn free_port() -> u16 {
    StdTcpListener::bind("127.0.0.1:0")
        .unwrap()
        .local_addr()
        .unwrap()
        .port()
}

#[tokio::test]
async fn end_to_end_http_tunnel_through_in_process_noise_rathole() {
    let temp = tempfile::tempdir().unwrap();
    let (shutdown_tx, _) = broadcast::channel(4);

    // 1. Private HTTP origin behind openflared
    let origin_listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let origin_addr = origin_listener.local_addr().unwrap();
    let mut origin_shutdown = shutdown_tx.subscribe();
    tokio::spawn(async move {
        loop {
            tokio::select! {
                _ = origin_shutdown.recv() => break,
                res = origin_listener.accept() => {
                    if let Ok((stream, _)) = res {
                        tokio::spawn(async move {
                            let io = TokioIo::new(stream);
                            let _ = ServerBuilder::new(TokioExecutor::new())
                                .serve_connection(
                                    io,
                                    service_fn(|_req: Request<Incoming>| async move {
                                        Ok::<_, hyper::Error>(Response::new(
                                            Full::new(Bytes::from_static(b"hello-through-noise-rathole"))
                                                .map_err(|never| match never {})
                                                .boxed(),
                                        ))
                                    }),
                                )
                                .await;
                        });
                    }
                }
            }
        }
    });

    // 2. Embedded openflared HTTP ingress
    let (ingress_state, ingress_addr) = spawn_ingress_server(shutdown_tx.subscribe())
        .await
        .unwrap();
    let route = TunnelHTTPRouteRule {
        route_id: 77,
        site_name: "private-app".into(),
        domains: vec!["private.example.com".into()],
        target_addr: origin_addr.to_string(),
        target_protocol: "http".into(),
        origin_host: String::new(),
        options_json: "{}".into(),
    };
    ingress_state
        .update_config(std::slice::from_ref(&route), &[], &["169.254.0.0/16".into()])
        .await
        .unwrap();

    // 3. OpenFlare Relay (Rathole Noise server + HTTP Gateway)
    let transport_port = free_port();
    let gw_secret = "test-gw-secret-e2e".to_owned();
    let gw_state = Arc::new(RelayGatewayState::new(
        "relay-e2e".to_owned(),
        gw_secret.clone(),
    ));
    let relay_mgr = Arc::new(RatholeRelayManager::new(
        temp.path().join("relay-state"),
        gw_state.clone(),
    ));

    let relay_cfg = RelayRuntimeConfigResponse {
        id: 1,
        relay_id: "relay-e2e".into(),
        pool_id: 1,
        name: "e2e-pop".into(),
        public_addr: "127.0.0.1".into(),
        transport_port,
        ws_port: 0,
        gateway_addr: "127.0.0.1".into(),
        gateway_port: 0,
        gateway_tls: false,
        gateway_secret: gw_secret.clone(),
        supported_transports: vec!["noise".into()],
        noise_private_key: TEST_NOISE_PRIV.into(),
        noise_public_key: TEST_NOISE_PUB.into(),
        draining: false,
        lease_ttl_seconds: 90,
        updated_at_unix: 1,
        services: vec![RelayServiceSpec {
            service_name: "http_t7_c1".into(),
            service_type: "http_ingress".into(),
            tunnel_id: 7,
            tunnel_public_id: "tun-e2e".into(),
            connector_id: "c1".into(),
            token: "ses_e2e_secret".into(),
            transport: "noise".into(),
            listen_port: 0,
            state: "active".into(),
        }],
    };
    relay_mgr.apply_config(&relay_cfg).await.unwrap();

    let gw_listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let gw_addr = gw_listener.local_addr().unwrap();
    let mut gw_shutdown = shutdown_tx.subscribe();
    let gw_state_clone = gw_state.clone();
    tokio::spawn(async move {
        loop {
            tokio::select! {
                _ = gw_shutdown.recv() => break,
                res = gw_listener.accept() => {
                    if let Ok((stream, _)) = res {
                        let st = gw_state_clone.clone();
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
            }
        }
    });

    // 4. OpenFlared Client Pool (Rathole Noise client)
    let client_pool = Arc::new(RatholeClientPool::new(
        temp.path().join("flared-state"),
        ingress_addr,
        "tun_e2e_access".into(),
    ));
    let conn_cfg = TunnelConnectorConfigResponse {
        tunnel_id: 7,
        tunnel_public_id: "tun-e2e".into(),
        name: "e2e-tun".into(),
        config_revision: 1,
        enabled: true,
        pool_id: 1,
        ha_relay_count: 1,
        preferred_transport: "noise".into(),
        fallback_transports: vec!["noise".into()],
        client_proxy_url: String::new(),
        allowed_target_cidrs: Vec::new(),
        blocked_target_cidrs: vec!["169.254.0.0/16".into()],
        candidate_relays: vec![TunnelRelayCandidate {
            id: 1,
            relay_id: "relay-e2e".into(),
            pool_id: 1,
            name: "e2e-pop".into(),
            public_addr: "127.0.0.1".into(),
            transport_port,
            ws_port: 0,
            gateway_addr: "127.0.0.1".into(),
            gateway_port: gw_addr.port(),
            gateway_tls: false,
            health_path: "/_openflare/health".into(),
            supported_transports: vec!["noise".into()],
            noise_public_key: TEST_NOISE_PUB.into(),
            geo_name: String::new(),
            geo_latitude: None,
            geo_longitude: None,
            zone_tag: String::new(),
            weight: 100,
            draining: false,
        }],
        attachments: vec![TunnelConnectorAttachmentGrant {
            relay_id: 1,
            relay_public_id: "relay-e2e".into(),
            service_name: "http_t7_c1".into(),
            session_token: "ses_e2e_secret".into(),
            transport: "noise".into(),
            state: "active".into(),
            lease_expires_at_unix: 9999999999,
        }],
        routes: vec![route.clone()],
        services: Vec::new(),
    };
    client_pool.reconcile(&conn_cfg).await.unwrap();

    // 5. Wait for in-process Rathole Noise handshake and assert E2E response
    let http_client = reqwest::Client::new();
    let mut succeeded = false;
    for _ in 0..40 {
        let resp = http_client
            .get(format!("http://{gw_addr}/ping"))
            .header(HEADER_OPENFLARE_TUNNEL_ID, "7")
            .header(HEADER_OPENFLARE_ROUTE_ID, "77")
            .header(HEADER_OPENFLARE_RELAY_AUTH, &gw_secret)
            .send()
            .await;
        if let Ok(r) = resp {
            if r.status() == StatusCode::OK {
                assert_eq!(r.text().await.unwrap(), "hello-through-noise-rathole");
                succeeded = true;
                break;
            }
        }
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
    assert!(succeeded, "E2E HTTP request over in-process Noise Rathole tunnel did not succeed in time");

    // 6. Verify gateway auth rejection
    let unauth = http_client
        .get(format!("http://{gw_addr}/ping"))
        .header(HEADER_OPENFLARE_TUNNEL_ID, "7")
        .header(HEADER_OPENFLARE_ROUTE_ID, "77")
        .header(HEADER_OPENFLARE_RELAY_AUTH, "invalid")
        .send()
        .await
        .unwrap();
    assert_eq!(unauth.status(), StatusCode::FORBIDDEN);

    // 7. Verify dynamic CIDR policy block takes effect across live tunnel
    ingress_state
        .update_config(&[route], &[], &["127.0.0.0/8".into()])
        .await
        .unwrap();
    let blocked = http_client
        .get(format!("http://{gw_addr}/ping"))
        .header(HEADER_OPENFLARE_TUNNEL_ID, "7")
        .header(HEADER_OPENFLARE_ROUTE_ID, "77")
        .header(HEADER_OPENFLARE_RELAY_AUTH, &gw_secret)
        .send()
        .await
        .unwrap();
    assert_eq!(blocked.status(), StatusCode::FORBIDDEN);

    let _ = shutdown_tx.send(true);
    client_pool.shutdown_all().await;
    relay_mgr.shutdown().await;
}

use hyper::Response;
