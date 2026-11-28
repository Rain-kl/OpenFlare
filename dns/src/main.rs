mod bench;
mod cache;
mod config;
mod control;
mod engine;
mod geo;
mod metrics;
mod protocol;
mod quality;
mod server;
mod snapshot;
mod update;

use anyhow::Result;
use clap::Parser;
use config::{Cli, Command};
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> Result<()> {
    let filter = EnvFilter::try_from_default_env()
        .unwrap_or_else(|_| EnvFilter::new("info,hickory_server=warn"));
    tracing_subscriber::fmt().with_env_filter(filter).init();

    let cli = Cli::parse();
    match cli.command {
        Command::Serve(args) => server::run_serve(args).await,
        Command::Bench(args) => bench::run_bench(args).await,
    }
}

#[cfg(test)]
mod tests {
    use crate::bench::build_bench_snapshot;
    use crate::cache::{AnswerCache, PrefixRrl};
    use crate::engine::{HysteresisTracker, evaluate_and_select, match_policy};
    use crate::geo::{ClientProfile, GeoLookup};
    use crate::metrics::DnsMetrics;
    use crate::protocol::{
        CompactQualityEntry, CompactQualitySnapshot, SnapshotEnvelope, SnapshotNode, SnapshotPolicy,
    };
    use crate::quality::SnapshotQualityOracle;
    use crate::server::SmartDnsHandler;
    use crate::snapshot::{CompiledSnapshot, SnapshotStore};
    use hickory_proto::op::{Edns, Message, MessageType, OpCode, Query, ResponseCode};
    use hickory_proto::rr::rdata::opt::{ClientSubnet, EdnsCode, EdnsOption};
    use hickory_proto::rr::{Name, RData, RecordType};
    use hickory_proto::serialize::binary::{BinDecodable, BinEncodable, BinEncoder};
    use hickory_server::Server;
    use std::net::{IpAddr, Ipv4Addr, Ipv6Addr};
    use std::str::FromStr;
    use std::sync::Arc;
    use std::sync::atomic::Ordering;
    use std::time::{Duration, Instant};
    use tokio::net::UdpSocket;

    fn test_node(
        id: &str,
        ipv4: &str,
        ipv6: &str,
        country: &str,
        cpu: f64,
        quarantined: bool,
    ) -> SnapshotNode {
        SnapshotNode {
            node_id: id.to_string(),
            name: id.to_string(),
            ipv4: ipv4.to_string(),
            ipv6: ipv6.to_string(),
            status: "online".to_string(),
            quarantined,
            network_group: "edge".to_string(),
            asn: 13335,
            country: country.to_string(),
            geo_latitude: Some(1.35),
            geo_longitude: Some(103.8),
            capacity_bandwidth_mbps: 10000,
            capacity_connections: 50000,
            capacity_weight: 100,
            cpu_usage_percent: cpu,
            memory_usage_percent: cpu,
            bandwidth_mbps: 1000.0,
            active_connections: 500,
            tags: Vec::new(),
        }
    }

    #[tokio::test]
    async fn jdm_switch_overload_and_hysteresis() {
        let mut env = build_bench_snapshot();
        env.nodes = vec![
            test_node("node-hot", "203.0.113.10", "", "US", 96.0, false),
            test_node("node-good-1", "203.0.113.11", "", "US", 20.0, false),
            test_node("node-good-2", "203.0.113.12", "", "US", 22.0, false),
            test_node("node-cn-1", "203.0.113.13", "", "CN", 18.0, false),
        ];
        env.quality = Some(CompactQualitySnapshot {
            schema_version: 1,
            generation: 1,
            generated_at: "2026-11-27T00:00:00Z".to_string(),
            provider_name: "test".to_string(),
            entries: vec![
                CompactQualityEntry {
                    node_id: "node-good-1".to_string(),
                    client_asn: 0,
                    client_country: "US".to_string(),
                    client_region: String::new(),
                    ip_family: "ipv4".to_string(),
                    level: "country".to_string(),
                    effective_rtt_ms: 25.0,
                    observed_rtt_ms: 25.0,
                    predicted_rtt_ms: 25.0,
                    rtt_p95_ms: 30.0,
                    p95_weight: 0.0,
                    dispersion_ms: 3.0,
                    loss_rate: 0.0,
                    confidence: 0.9,
                    quality_state: "healthy".to_string(),
                    explore_priority: 0.0,
                    geo_base_rtt_ms: 20.0,
                    geo_base_known: true,
                    source_tier: "asn_country".to_string(),
                },
                CompactQualityEntry {
                    node_id: "node-good-2".to_string(),
                    client_asn: 0,
                    client_country: "US".to_string(),
                    client_region: String::new(),
                    ip_family: "ipv4".to_string(),
                    level: "country".to_string(),
                    effective_rtt_ms: 26.0,
                    observed_rtt_ms: 26.0,
                    predicted_rtt_ms: 26.0,
                    rtt_p95_ms: 31.0,
                    p95_weight: 0.0,
                    dispersion_ms: 3.0,
                    loss_rate: 0.0,
                    confidence: 0.9,
                    quality_state: "healthy".to_string(),
                    explore_priority: 0.0,
                    geo_base_rtt_ms: 20.0,
                    geo_base_known: true,
                    source_tier: "asn_country".to_string(),
                },
            ],
            node_defaults: Default::default(),
        });
        env.nodes.iter_mut().for_each(|n| {
            n.tags = vec![crate::protocol::SnapshotTag {
                slug: "edge".to_string(),
                name: "Edge".to_string(),
                effective_weight: 1.0,
            }]
        });

        let compiled = CompiledSnapshot::from_envelope(env);
        let client_us = ClientProfile {
            client_ip: "8.8.8.8".to_string(),
            client_prefix: "8.8.8.0/24".to_string(),
            asn: 15169,
            country_iso: "US".to_string(),
            region_code: String::new(),
            ip_family: "ipv4".to_string(),
            latitude: None,
            longitude: None,
            from_ecs: false,
        };
        let policy = match_policy(&compiled.raw, &compiled.policies, "bench.sdns.example.com")
            .expect("policy matches");
        let hyst = HysteresisTracker::new(64);
        let now = Instant::now();

        let res1 = evaluate_and_select(
            &compiled.oracle,
            &compiled.raw.nodes,
            policy,
            &client_us,
            "bench.sdns.example.com",
            "A",
            Some(&hyst),
            now,
            1_700_000_000,
        )
        .await
        .expect("select ok");
        assert_eq!(res1.answers.len(), 1);
        assert_eq!(res1.answers[0].node_id, "node-good-1");
        assert_ne!(res1.answers[0].node_id, "node-hot");

        // CN client hits switchNode `c_cn` -> `expressionNode` filters `filters.countries = ["CN"]`.
        let client_cn = ClientProfile {
            client_ip: "223.5.5.5".to_string(),
            client_prefix: "223.5.5.0/24".to_string(),
            asn: 4134,
            country_iso: "CN".to_string(),
            region_code: String::new(),
            ip_family: "ipv4".to_string(),
            latitude: None,
            longitude: None,
            from_ecs: false,
        };
        let res_cn = evaluate_and_select(
            &compiled.oracle,
            &compiled.raw.nodes,
            policy,
            &client_cn,
            "bench.sdns.example.com",
            "A",
            Some(&hyst),
            now,
            1_700_000_000,
        )
        .await
        .expect("cn select ok");
        assert_eq!(res_cn.answers.len(), 1);
        assert_eq!(res_cn.answers[0].node_id, "node-cn-1");

        // Mandatory fallback: even if all nodes are overloaded (>0.92), must still return an answer.
        let mut all_hot = build_bench_snapshot();
        all_hot.nodes = vec![test_node("node-hot", "203.0.113.99", "", "US", 99.0, false)];
        all_hot.policies = vec![SnapshotPolicy {
            id: 2,
            name: "AllHot".to_string(),
            scope_type: "all".to_string(),
            scope_domains: Vec::new(),
            strategy_type: "smart".to_string(),
            priority: 100,
            ttl: 30,
            max_answers: 1,
            is_default: true,
            fallback_strategy: "lowest_load".to_string(),
            fallback_node_ids: Vec::new(),
            jdm_json: String::new(),
            revision: 1,
        }];
        let comp_hot = CompiledSnapshot::from_envelope(all_hot);
        let res_hot = evaluate_and_select(
            &comp_hot.oracle,
            &comp_hot.raw.nodes,
            &comp_hot.policies[0],
            &client_us,
            "bench.sdns.example.com",
            "A",
            None,
            now,
            1_700_000_000,
        )
        .await
        .expect("fallback select ok");
        assert_eq!(res_hot.answers.len(), 1);
        assert_eq!(
            res_hot.answers[0].ip,
            IpAddr::V4(Ipv4Addr::new(203, 0, 113, 99))
        );
    }

    #[test]
    fn offline_snapshot_disk_persistence() {
        let dir =
            std::env::temp_dir().join(format!("openflare-dns-snap-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        let store1 = SnapshotStore::new(dir.to_str());
        let env: SnapshotEnvelope = build_bench_snapshot();
        store1.update_from_envelope(env);

        let store2 = SnapshotStore::new(dir.to_str());
        let loaded = store2.load();
        assert_eq!(loaded.raw.version, 1);
        assert_eq!(loaded.raw.delegation_domain, "sdns.example.com");
        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn quality_oracle_4_tier_lookup() {
        let snap = CompactQualitySnapshot {
            entries: vec![
                CompactQualityEntry {
                    node_id: "n1".to_string(),
                    client_asn: 4134,
                    client_country: "CN".to_string(),
                    client_region: "SH".to_string(),
                    ip_family: "ipv4".to_string(),
                    level: "region".to_string(),
                    effective_rtt_ms: 12.0,
                    observed_rtt_ms: 12.0,
                    predicted_rtt_ms: 12.0,
                    rtt_p95_ms: 18.0,
                    p95_weight: 0.35,
                    dispersion_ms: 2.0,
                    loss_rate: 0.0,
                    confidence: 0.95,
                    quality_state: "healthy".to_string(),
                    explore_priority: 0.0,
                    geo_base_rtt_ms: 10.0,
                    geo_base_known: true,
                    source_tier: "real_region".to_string(),
                },
                CompactQualityEntry {
                    node_id: "n1".to_string(),
                    client_asn: 4134,
                    client_country: "CN".to_string(),
                    client_region: String::new(),
                    ip_family: "ipv4".to_string(),
                    level: "country".to_string(),
                    effective_rtt_ms: 25.0,
                    observed_rtt_ms: 25.0,
                    predicted_rtt_ms: 25.0,
                    rtt_p95_ms: 35.0,
                    p95_weight: 0.25,
                    dispersion_ms: 5.0,
                    loss_rate: 0.0,
                    confidence: 0.7,
                    quality_state: "healthy".to_string(),
                    explore_priority: 0.0,
                    geo_base_rtt_ms: 10.0,
                    geo_base_known: true,
                    source_tier: "asn_country".to_string(),
                },
            ],
            node_defaults: [(
                "n1".to_string(),
                crate::protocol::CompactNodePrior {
                    network_group: "edge".to_string(),
                    asn: 13335,
                    lat: Some(1.35),
                    lon: Some(103.8),
                    median_extra: 4.0,
                },
            )]
            .into_iter()
            .collect(),
            ..Default::default()
        };
        let oracle = SnapshotQualityOracle::from_snapshot(Some(&snap));
        use crate::quality::QualityOracle;

        // Tier 1: measured_region
        let q1 = oracle.evaluate_node(
            "n1",
            &ClientProfile {
                asn: 4134,
                country_iso: "CN".to_string(),
                region_code: "SH".to_string(),
                ip_family: "ipv4".to_string(),
                ..Default::default()
            },
        );
        assert_eq!(q1.source_tier, "measured_region");
        assert!((q1.effective_rtt_ms - 12.0).abs() < 1e-6);

        // Tier 2: measured_country (same ASN/country, different region)
        let q2 = oracle.evaluate_node(
            "n1",
            &ClientProfile {
                asn: 4134,
                country_iso: "CN".to_string(),
                region_code: "BJ".to_string(),
                ip_family: "ipv4".to_string(),
                ..Default::default()
            },
        );
        assert_eq!(q2.source_tier, "measured_country");
        assert!((q2.effective_rtt_ms - 25.0).abs() < 1e-6);

        // Tier 3: inferred country any-ASN fallback (keeps highest-confidence CN entry: 12.0 ms)
        // and clears direct-only p95_weight / explore_priority.
        let q3 = oracle.evaluate_node(
            "n1",
            &ClientProfile {
                asn: 9999,
                country_iso: "CN".to_string(),
                region_code: String::new(),
                ip_family: "ipv4".to_string(),
                ..Default::default()
            },
        );
        assert_eq!(q3.source_tier, "inferred");
        assert!(q3.quality_score > 0.0);
        assert!((q3.effective_rtt_ms - 12.0).abs() < 1e-6);
        assert!((q3.p95_weight - 0.0).abs() < 1e-6);
        assert!((q3.explore_priority - 0.0).abs() < 1e-6);

        // Tier 4: geo_prior when no matching country/ASN entry exists
        let q4 = oracle.evaluate_node(
            "n1",
            &ClientProfile {
                asn: 9999,
                country_iso: "JP".to_string(),
                region_code: String::new(),
                ip_family: "ipv4".to_string(),
                latitude: Some(35.68),
                longitude: Some(139.69),
                ..Default::default()
            },
        );
        assert_eq!(q4.source_tier, "geo_prior");
        assert!(q4.effective_rtt_ms > 5.0);
    }

    async fn udp_query(
        addr: std::net::SocketAddr,
        qname: &str,
        qtype: RecordType,
        ecs: Option<(IpAddr, u8)>,
    ) -> Message {
        let mut msg = Message::new(4242, MessageType::Query, OpCode::Query);
        msg.add_query(Query::query(Name::from_str(qname).unwrap(), qtype));
        if let Some((ip, src_len)) = ecs {
            let mut edns = Edns::new();
            edns.set_max_payload(1232);
            edns.options_mut()
                .insert(EdnsOption::Subnet(ClientSubnet::new(ip, src_len, 0)));
            msg.set_edns(edns);
        }
        let mut wire = Vec::new();
        msg.emit(&mut BinEncoder::new(&mut wire)).unwrap();

        let sock = UdpSocket::bind("127.0.0.1:0").await.unwrap();
        sock.connect(addr).await.unwrap();
        sock.send(&wire).await.unwrap();
        let mut buf = [0u8; 2048];
        let n = tokio::time::timeout(Duration::from_secs(2), sock.recv(&mut buf))
            .await
            .expect("recv timeout")
            .expect("recv ok");
        Message::from_bytes(&buf[..n]).expect("valid DNS reply")
    }

    #[tokio::test]
    async fn udp_server_a_aaaa_ecs_ns_soa_refused_and_cache() {
        let mut env = build_bench_snapshot();
        env.nodes[0].ipv6 = "2001:db8:1::10".to_string();
        let compiled = CompiledSnapshot::from_envelope(env);
        let store = Arc::new(SnapshotStore::with_snapshot(compiled));
        let geo = Arc::new(GeoLookup::empty());
        let rrl = Arc::new(PrefixRrl::new(1000, 2000));
        let cache = Arc::new(AnswerCache::new(1024));
        let hysteresis = Arc::new(HysteresisTracker::new(1024));
        let metrics = Arc::new(DnsMetrics::new());

        let handler = SmartDnsHandler::new(store, geo, rrl, cache, hysteresis, metrics.clone());
        let sock = UdpSocket::bind("127.0.0.1:0").await.unwrap();
        let addr = sock.local_addr().unwrap();
        let mut server = Server::new(handler);
        server.register_socket(sock);

        // 1. A query with IPv4 ECS -> replies with /24 scope_prefix
        let reply_a = udp_query(
            addr,
            "bench.sdns.example.com.",
            RecordType::A,
            Some((IpAddr::V4(Ipv4Addr::new(198, 51, 100, 7)), 24)),
        )
        .await;
        assert_eq!(reply_a.metadata.response_code, ResponseCode::NoError);
        assert_eq!(reply_a.answers.len(), 1);
        let EdnsOption::Subnet(ecs_a) = reply_a
            .edns
            .as_ref()
            .expect("edns in reply")
            .option(EdnsCode::Subnet)
            .expect("ecs option in reply")
        else {
            panic!("expected Subnet option");
        };
        assert_eq!(ecs_a.scope_prefix(), 24);

        // 2. Second query with same /24 prefix hits L1 AnswerCache
        let reply_a2 = udp_query(
            addr,
            "bench.sdns.example.com.",
            RecordType::A,
            Some((IpAddr::V4(Ipv4Addr::new(198, 51, 100, 99)), 24)),
        )
        .await;
        assert_eq!(reply_a2.answers.len(), 1);
        assert_eq!(metrics.cache_hits.load(Ordering::Relaxed), 1);

        // 3. AAAA query with IPv6 ECS -> replies with /48 scope_prefix
        let reply_aaaa = udp_query(
            addr,
            "bench.sdns.example.com.",
            RecordType::AAAA,
            Some((
                IpAddr::V6(Ipv6Addr::new(0x2001, 0xdb8, 0xabcd, 1, 0, 0, 0, 1)),
                56,
            )),
        )
        .await;
        assert_eq!(reply_aaaa.metadata.response_code, ResponseCode::NoError);
        assert!(matches!(reply_aaaa.answers[0].data, RData::AAAA(_)));
        let EdnsOption::Subnet(ecs_v6) = reply_aaaa
            .edns
            .as_ref()
            .unwrap()
            .option(EdnsCode::Subnet)
            .unwrap()
        else {
            panic!("expected v6 Subnet");
        };
        assert_eq!(ecs_v6.scope_prefix(), 48);

        // 4. NS & SOA queries
        let reply_ns = udp_query(addr, "sdns.example.com.", RecordType::NS, None).await;
        assert_eq!(reply_ns.metadata.response_code, ResponseCode::NoError);
        assert_eq!(reply_ns.answers.len(), 1);

        let reply_soa = udp_query(addr, "sdns.example.com.", RecordType::SOA, None).await;
        assert_eq!(reply_soa.metadata.response_code, ResponseCode::NoError);
        assert!(matches!(reply_soa.answers[0].data, RData::SOA(_)));

        // 5. Out-of-zone query -> REFUSED
        let reply_refused = udp_query(addr, "outside.invalid.", RecordType::A, None).await;
        assert_eq!(reply_refused.metadata.response_code, ResponseCode::Refused);

        server.shutdown_gracefully().await.unwrap();
    }

    #[test]
    fn token_file_identity_and_ready_file_lifecycle() {
        use crate::config::{DNS_VERSION, ServeArgs};
        use crate::control::{
            BootstrapToken, PersistedIdentity, load_saved_identity, resolve_bootstrap_token,
            save_identity,
        };
        use crate::server::write_ready_file;

        let dir =
            std::env::temp_dir().join(format!("openflare-dns-state-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(&dir).unwrap();

        let tok_file = dir.join("bootstrap-token");
        std::fs::write(&tok_file, "  secret-token-value\n").unwrap();

        let mut args = ServeArgs {
            controller_url: "https://panel.example.com".to_string(),
            token: None,
            discovery_token: None,
            token_file: Some(tok_file.to_string_lossy().to_string()),
            token_type: "discovery".to_string(),
            instance_id: None,
            name: "test".to_string(),
            listen: "127.0.0.1:5353".to_string(),
            public_addr: String::new(),
            geoip_dir: String::new(),
            cache_dir: dir.to_string_lossy().to_string(),
            workers: 1,
            rrl_qps: 1000,
            rrl_burst: 2000,
            snapshot_interval_secs: 10,
        };
        assert_eq!(
            resolve_bootstrap_token(&args).unwrap(),
            BootstrapToken::Discovery("secret-token-value".to_string())
        );

        args.token_type = "instance".to_string();
        assert_eq!(
            resolve_bootstrap_token(&args).unwrap(),
            BootstrapToken::Instance("secret-token-value".to_string())
        );

        args.token_type = "invalid".to_string();
        assert!(resolve_bootstrap_token(&args).is_err());

        let id = PersistedIdentity {
            instance_id: "dns-abc123".to_string(),
            access_token: "tok-xyz".to_string(),
        };
        save_identity(&args.cache_dir, &id).unwrap();
        assert_eq!(load_saved_identity(&args.cache_dir), Some(id));

        let ready_path = dir.join("ready.json");
        {
            let guard = write_ready_file(&args.cache_dir, "127.0.0.1:5353".parse().unwrap())
                .unwrap()
                .unwrap();
            assert_eq!(guard.path(), ready_path.as_path());
            let body = std::fs::read_to_string(&ready_path).unwrap();
            assert!(body.contains(DNS_VERSION));
            assert!(body.contains(&std::process::id().to_string()));
        }
        assert!(!ready_path.exists());

        let _ = std::fs::remove_dir_all(&dir);
    }

    #[tokio::test]
    async fn p95_weight_tail_penalty_and_ecs_only_exploration() {
        let mut env = build_bench_snapshot();
        env.nodes = vec![
            test_node("hk-a", "203.0.113.1", "", "CN", 20.0, false),
            test_node("jp-b", "203.0.113.2", "", "JP", 20.0, false),
            test_node("us-d", "203.0.113.4", "", "US", 20.0, false),
        ];
        for n in &mut env.nodes {
            n.geo_latitude = None;
            n.geo_longitude = None;
        }
        env.policies = vec![SnapshotPolicy {
            id: 10,
            name: "SmartP95".to_string(),
            scope_type: "all".to_string(),
            scope_domains: Vec::new(),
            strategy_type: "smart".to_string(),
            priority: 100,
            ttl: 60,
            max_answers: 1,
            is_default: true,
            fallback_strategy: "lowest_load".to_string(),
            fallback_node_ids: Vec::new(),
            jdm_json: String::new(),
            revision: 1,
        }];
        env.quality = Some(CompactQualitySnapshot {
            entries: vec![
                CompactQualityEntry {
                    node_id: "hk-a".to_string(),
                    client_asn: 9808,
                    client_country: "CN".to_string(),
                    client_region: String::new(),
                    ip_family: "ipv4".to_string(),
                    level: "country".to_string(),
                    effective_rtt_ms: 30.0,
                    observed_rtt_ms: 30.0,
                    predicted_rtt_ms: 30.0,
                    rtt_p95_ms: 220.0,
                    p95_weight: 0.35,
                    dispersion_ms: 0.0,
                    loss_rate: 0.0,
                    confidence: 0.9,
                    quality_state: "healthy".to_string(),
                    explore_priority: 0.0,
                    geo_base_rtt_ms: 0.0,
                    geo_base_known: false,
                    source_tier: "measured_country".to_string(),
                },
                CompactQualityEntry {
                    node_id: "jp-b".to_string(),
                    client_asn: 9808,
                    client_country: "CN".to_string(),
                    client_region: String::new(),
                    ip_family: "ipv4".to_string(),
                    level: "country".to_string(),
                    effective_rtt_ms: 65.0,
                    observed_rtt_ms: 65.0,
                    predicted_rtt_ms: 65.0,
                    rtt_p95_ms: 80.0,
                    p95_weight: 0.35,
                    dispersion_ms: 0.0,
                    loss_rate: 0.0,
                    confidence: 0.9,
                    quality_state: "healthy".to_string(),
                    explore_priority: 0.0,
                    geo_base_rtt_ms: 0.0,
                    geo_base_known: false,
                    source_tier: "measured_country".to_string(),
                },
            ],
            ..Default::default()
        });

        let compiled = CompiledSnapshot::from_envelope(env.clone());
        let policy = &compiled.policies[0];
        let mut client = ClientProfile {
            client_ip: "113.108.1.1".to_string(),
            client_prefix: "113.108.1.0/24".to_string(),
            asn: 9808,
            country_iso: "CN".to_string(),
            region_code: String::new(),
            ip_family: "ipv4".to_string(),
            latitude: None,
            longitude: None,
            from_ecs: false,
        };
        let now = Instant::now();
        let res_p95 = evaluate_and_select(
            &compiled.oracle,
            &compiled.raw.nodes,
            policy,
            &client,
            "app.example.com",
            "A",
            None,
            now,
            1_700_000_000,
        )
        .await
        .expect("select ok");
        assert_eq!(res_p95.answers[0].node_id, "jp-b");

        // Now mark hk-a stale with high explore_priority and verify ECS-only exploration.
        let mut exp_env = env;
        exp_env.quality = Some(CompactQualitySnapshot {
            entries: vec![
                CompactQualityEntry {
                    node_id: "hk-a".to_string(),
                    client_asn: 9808,
                    client_country: "CN".to_string(),
                    client_region: String::new(),
                    ip_family: "ipv4".to_string(),
                    level: "country".to_string(),
                    effective_rtt_ms: 180.0,
                    observed_rtt_ms: 180.0,
                    predicted_rtt_ms: 30.0,
                    rtt_p95_ms: 210.0,
                    p95_weight: 0.25,
                    dispersion_ms: 0.0,
                    loss_rate: 0.0,
                    confidence: 0.2,
                    quality_state: "stale".to_string(),
                    explore_priority: 0.9,
                    geo_base_rtt_ms: 0.0,
                    geo_base_known: false,
                    source_tier: "measured_country".to_string(),
                },
                CompactQualityEntry {
                    node_id: "jp-b".to_string(),
                    client_asn: 9808,
                    client_country: "CN".to_string(),
                    client_region: String::new(),
                    ip_family: "ipv4".to_string(),
                    level: "country".to_string(),
                    effective_rtt_ms: 65.0,
                    observed_rtt_ms: 65.0,
                    predicted_rtt_ms: 65.0,
                    rtt_p95_ms: 80.0,
                    p95_weight: 0.35,
                    dispersion_ms: 0.0,
                    loss_rate: 0.0,
                    confidence: 0.9,
                    quality_state: "healthy".to_string(),
                    explore_priority: 0.0,
                    geo_base_rtt_ms: 0.0,
                    geo_base_known: false,
                    source_tier: "measured_country".to_string(),
                },
                CompactQualityEntry {
                    node_id: "us-d".to_string(),
                    client_asn: 9808,
                    client_country: "CN".to_string(),
                    client_region: String::new(),
                    ip_family: "ipv4".to_string(),
                    level: "country".to_string(),
                    effective_rtt_ms: 220.0,
                    observed_rtt_ms: 220.0,
                    predicted_rtt_ms: 210.0,
                    rtt_p95_ms: 250.0,
                    p95_weight: 0.35,
                    dispersion_ms: 0.0,
                    loss_rate: 0.0,
                    confidence: 0.8,
                    quality_state: "healthy".to_string(),
                    explore_priority: 0.0,
                    geo_base_rtt_ms: 0.0,
                    geo_base_known: false,
                    source_tier: "measured_country".to_string(),
                },
            ],
            ..Default::default()
        });
        let exp_compiled = CompiledSnapshot::from_envelope(exp_env);
        let hyst = HysteresisTracker::new(1024);

        // Without ECS: never explores.
        for i in 0..500u32 {
            client.from_ecs = false;
            client.client_prefix = format!("113.{}.{}.0/24", (i / 250) + 1, i % 250);
            let out = evaluate_and_select(
                &exp_compiled.oracle,
                &exp_compiled.raw.nodes,
                &exp_compiled.policies[0],
                &client,
                "app.example.com",
                "A",
                Some(&hyst),
                now,
                1_700_000_000,
            )
            .await
            .expect("select ok");
            assert!(!out.exploration_used);
            assert_eq!(out.answers[0].node_id, "jp-b");
        }

        // With ECS: explores hk-a for a small budget, clamps TTL <= 25, skips hysteresis.
        let hyst_ecs = HysteresisTracker::new(1024);
        let mut explored = 0usize;
        let mut explored_prefix = String::new();
        for i in 0..500u32 {
            client.from_ecs = true;
            client.client_prefix = format!("113.{}.{}.0/24", (i / 250) + 1, i % 250);
            let out = evaluate_and_select(
                &exp_compiled.oracle,
                &exp_compiled.raw.nodes,
                &exp_compiled.policies[0],
                &client,
                "app.example.com",
                "A",
                Some(&hyst_ecs),
                now,
                1_700_000_000,
            )
            .await
            .expect("select ok");
            if out.exploration_used {
                explored += 1;
                explored_prefix = client.client_prefix.clone();
                assert_eq!(out.answers[0].node_id, "hk-a");
                assert!(out.ttl <= 25);
            } else {
                assert_eq!(out.answers[0].node_id, "jp-b");
            }
        }
        assert!(explored > 0 && explored < 30, "explored={explored}");
        assert_eq!(hyst_ecs.get(&format!("10:{explored_prefix}:A"), now), None);
    }
}
