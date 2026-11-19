use super::{DataPlaneState, ProxyService};
use crate::{config::SnapshotStore, control::Runtime};
use async_trait::async_trait;
use pingora::{
    listeners::{tls::TlsSettings, TlsAccept},
    proxy::http_proxy_service,
    server::{
        configuration::{Opt, ServerConf},
        Server,
    },
    tls::{
        ext,
        ssl::{NameType, SslRef},
    },
};
use std::sync::Arc;

struct DynamicCertificateCallback {
    state: SnapshotStore,
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
        let Some(certificate) = config.certificate_for_host(&sni) else {
            tracing::warn!(sni, "no TLS certificate matches SNI");
            return;
        };
        if let Err(error) = ext::ssl_use_certificate(ssl, &certificate.cert) {
            tracing::error!(sni, error = %error, "failed to install TLS certificate");
        }
        for intermediate in &certificate.chain {
            if let Err(error) = ext::ssl_add_chain_cert(ssl, intermediate) {
                tracing::error!(sni, %error, "failed to install TLS intermediate");
            }
        }
        if let Err(error) = ext::ssl_use_private_key(ssl, &certificate.key) {
            tracing::error!(sni, error = %error, "failed to install TLS private key");
        }
    }
}

pub fn run_server(state: DataPlaneState, control: Runtime, upgrade: bool) -> anyhow::Result<()> {
    let DataPlaneState {
        shared,
        state_dir,
        listeners,
        access_logs,
    } = state;
    let pid_file = state_dir.join("openflare-edge.pid");
    let geo_ip = control.geo_ip();
    let conf = ServerConf {
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
            state: shared.clone(),
            rate_limiter: Default::default(),
            login_limiter: Default::default(),
            connections: Default::default(),
            state_dir,
            pow: Default::default(),
            geo_ip,
            access_logs,
        },
    );
    service.add_tcp(&listeners.http.to_string());
    if let Some(listen) = listeners.tls {
        let mut settings = TlsSettings::with_callbacks(Box::new(DynamicCertificateCallback {
            state: shared.clone(),
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
        service.add_tls_with_settings(&listen.to_string(), None, settings);
    }
    let proxy = server.add_service(service);
    proxy.add_dependency(&bootstrap);
    let control = server.add_service(pingora::services::background::background_service(
        "control", control,
    ));
    control.add_dependency(&proxy);
    server.run_forever()
}
