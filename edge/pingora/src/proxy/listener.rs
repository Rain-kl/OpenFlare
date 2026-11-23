use super::{DataPlaneState, ProxyService};
use crate::{
    config::{CertificateKeyRing, SnapshotStore},
    control::Runtime,
    runtime::performance::seconds,
};
use async_trait::async_trait;
use pingora::{
    apps::HttpServerOptions,
    listeners::{tls::TlsSettings, TlsAccept},
    protocols::http::v2::server::default_h2_options,
    proxy::http_proxy_service,
    server::{
        configuration::{Opt, ServerConf},
        RunArgs, Server,
    },
    tls::{
        ext,
        ssl::{NameType, SslRef},
    },
};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};

struct DynamicCertificateCallback {
    state: SnapshotStore,
    keys: CertificateKeyRing,
}

#[async_trait]
impl TlsAccept for DynamicCertificateCallback {
    async fn certificate_callback(&self, ssl: &mut SslRef) {
        let sni = ssl
            .servername(NameType::HOST_NAME)
            .map(str::to_owned)
            .unwrap_or_default();
        if sni.is_empty() {
            tracing::warn!("TLS connection has no SNI; rejecting certificate selection");
            return;
        };
        let Some(config) = self.state.current() else {
            tracing::warn!(sni, "TLS handshake arrived before configuration was ready");
            return;
        };
        let Some((cert_id, certificate)) = config.certificate_for_host(&sni) else {
            tracing::warn!(sni, "no TLS certificate matches SNI");
            return;
        };
        let Some(private_key) = self.keys.get(cert_id, &certificate.cert_sha256) else {
            tracing::warn!(
                sni,
                cert_id,
                "TLS private key is not hydrated in memory; refusing handshake"
            );
            return;
        };
        if let Err(error) = ext::ssl_use_certificate(ssl, &certificate.cert) {
            tracing::error!(sni, error = %error, "failed to install TLS certificate");
            return;
        }
        for intermediate in &certificate.chain {
            if let Err(error) = ext::ssl_add_chain_cert(ssl, intermediate) {
                tracing::error!(sni, %error, "failed to install TLS intermediate");
            }
        }
        if let Err(error) = ext::ssl_use_private_key(ssl, &private_key) {
            tracing::error!(sni, error = %error, "failed to install TLS private key");
        }
    }
}

pub fn run_server(state: DataPlaneState, control: Runtime, upgrade: bool) -> anyhow::Result<bool> {
    let DataPlaneState {
        shared,
        keys,
        state_dir,
        listeners,
        access_logs,
    } = state;
    let pid_file = state_dir.join("openflare-edge.pid");
    let geo_ip = control.geo_ip();
    let performance = control.performance();
    let active_policy = performance.current();
    let startup = active_policy.policy.settings.as_ref().map(|s| &s.startup);
    let mut conf = ServerConf {
        // Drain active requests briefly before re-executing the node for startup-only settings.
        grace_period_seconds: Some(2),
        daemon: false,
        working_directory: Some(state_dir.clone()),
        error_log: Some(
            state_dir
                .join("openflare-edge.log")
                .to_string_lossy()
                .into_owned(),
        ),
        pid_file: pid_file.to_string_lossy().into_owned(),
        upgrade_sock: state_dir
            .join("openflare-edge-upgrade.sock")
            .to_string_lossy()
            .into_owned(),
        ..ServerConf::default()
    };
    if let Some(startup) = startup {
        startup.configure_server(&mut conf);
    }
    tracing::info!(
        threads = conf.threads,
        upstream_keepalive_pool_size = conf.upstream_keepalive_pool_size,
        "starting Pingora workers"
    );
    let cache = super::cache::EdgeCache::open(&state_dir, startup)?;
    let disk_cleanup = cache.disk_cleanup();
    let mut server = Server::new_with_opt_and_conf(
        Some(Opt {
            upgrade,
            ..Opt::default()
        }),
        conf,
    );
    let bootstrap = server.bootstrap_as_a_service();
    let mut service = http_proxy_service(
        &Arc::clone(&server.configuration),
        ProxyService {
            cache,
            state: shared.clone(),
            rate_limiter: Default::default(),
            login_limiter: Default::default(),
            connections: Default::default(),
            state_dir,
            pow: Default::default(),
            geo_ip,
            access_logs,
            performance: performance.clone(),
        },
    );
    if let Some(startup) = startup {
        let mut options = HttpServerOptions::default();
        options.h2c = startup.h2c;
        options.keepalive_request_limit =
            (startup.keepalive_request_limit != 0).then_some(startup.keepalive_request_limit);
        options.h2_idle_timeout = seconds(startup.h2_idle_timeout);

        let mut h2 = default_h2_options();
        h2.max_concurrent_streams(startup.max_concurrent_streams)
            .initial_window_size(startup.initial_window_size)
            .initial_connection_window_size(startup.initial_connection_window_size)
            .max_frame_size(startup.max_frame_size)
            .max_header_list_size(startup.max_header_list_size)
            .header_table_size(startup.header_table_size)
            .max_send_buffer_size(startup.max_send_buffer_size);

        let proxy = service
            .app_logic_mut()
            .expect("configure Pingora proxy before starting");
        proxy.server_options = Some(options);
        proxy.h2_options = Some(h2);
        service.add_tcp_with_settings(
            &listeners.http.to_string(),
            startup.socket_options(listeners.http.is_ipv6()),
        );
    } else {
        service.add_tcp(&listeners.http.to_string());
    }
    if let Some(listen) = listeners.tls {
        let mut settings = TlsSettings::with_callbacks(Box::new(DynamicCertificateCallback {
            state: shared.clone(),
            keys,
        }))?;
        let alpn_config = shared.clone();
        settings.set_alpn_select_callback(move |ssl, offered| {
            let allow_h2 = ssl.servername(NameType::HOST_NAME).is_some_and(|sni| {
                alpn_config
                    .current()
                    .is_some_and(|snapshot| snapshot.allows_http2(sni))
            });
            let supported: &[u8] = if allow_h2 {
                b"\x02h2\x08http/1.1"
            } else {
                b"\x08http/1.1"
            };
            pingora::tls::ssl::select_next_proto(supported, offered)
                .ok_or(pingora::tls::ssl::AlpnError::NOACK)
        });
        settings.set_offload_threadpool_from_server_conf(&server.configuration);
        service.add_tls_with_settings(
            &listen.to_string(),
            startup.map(|s| s.socket_options(listen.is_ipv6())),
            settings,
        );
    }
    if let Some(cleanup) = disk_cleanup {
        server.add_service(pingora::services::background::background_service(
            "disk-cache-cleanup",
            cleanup,
        ));
    }
    let proxy = server.add_service(service);
    proxy.add_dependency(&bootstrap);
    let control = server.add_service(pingora::services::background::background_service(
        "control", control,
    ));
    control.add_dependency(&proxy);
    let reloading = Arc::new(AtomicBool::new(false));
    server.run(RunArgs {
        shutdown_signal: Box::new(performance.shutdown_watch(reloading.clone())),
    });
    Ok(reloading.load(Ordering::Acquire))
}
