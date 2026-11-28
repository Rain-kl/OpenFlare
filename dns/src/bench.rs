use crate::cache::{AnswerCache, PrefixRrl};
use crate::config::BenchArgs;
use crate::engine::HysteresisTracker;
use crate::geo::GeoLookup;
use crate::metrics::DnsMetrics;
use crate::protocol::{
    CompactNodePrior, CompactQualityEntry, CompactQualitySnapshot, SnapshotDomainRoute,
    SnapshotEnvelope, SnapshotNode, SnapshotPolicy, SnapshotTag,
};
use crate::server::SmartDnsHandler;
use crate::snapshot::{CompiledSnapshot, SnapshotStore};
use anyhow::{Context, Result};
use hickory_proto::op::{Edns, Message, MessageType, Metadata, OpCode, Query, ResponseCode};
use hickory_proto::rr::rdata::opt::{ClientSubnet, EdnsOption};
use hickory_proto::rr::{Name, RData, Record, RecordType, rdata};
use hickory_proto::serialize::binary::BinEncodable;
use hickory_server::Server;
use hickory_server::net::runtime::Time;
use hickory_server::server::{Request, RequestHandler, ResponseHandler, ResponseInfo};
use hickory_server::zone_handler::MessageResponseBuilder;
use std::net::{Ipv4Addr, SocketAddr};
use std::str::FromStr;
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::{Duration, Instant};
use tokio::net::UdpSocket;

struct BareBenchHandler {
    record: Record,
}

impl BareBenchHandler {
    fn new(name: &str, ip: Ipv4Addr) -> Result<Self> {
        let owner = Name::from_str(name)?;
        let record = Record::from_rdata(owner, 60, RData::A(rdata::A(ip)));
        Ok(Self { record })
    }
}

#[async_trait::async_trait]
impl RequestHandler for BareBenchHandler {
    async fn handle_request<R: ResponseHandler, T: Time>(
        &self,
        request: &Request,
        mut response_handle: R,
    ) -> ResponseInfo {
        let mut meta = Metadata::response_from_request(&request.metadata);
        meta.authoritative = true;
        meta.response_code = ResponseCode::NoError;
        let answers = [self.record.clone()];
        let resp = MessageResponseBuilder::from_message_request(request).build(
            meta,
            answers.iter(),
            std::iter::empty::<&Record>(),
            std::iter::empty::<&Record>(),
            std::iter::empty::<&Record>(),
        );
        match response_handle.send_response(resp).await {
            Ok(info) => info,
            Err(_) => {
                let mut m = Metadata::response_from_request(&request.metadata);
                m.response_code = ResponseCode::ServFail;
                ResponseInfo::from(hickory_proto::op::Header {
                    metadata: m,
                    counts: Default::default(),
                })
            }
        }
    }
}

pub fn build_bench_snapshot() -> SnapshotEnvelope {
    let jdm = serde_json::json!({
        "nodes": [
            { "id": "in1", "type": "inputNode", "name": "Request" },
            {
                "id": "sw1",
                "type": "switchNode",
                "name": "Country",
                "content": {
                    "hitPolicy": "first",
                    "statements": [
                        { "id": "c_cn", "condition": "client.country == 'CN'" },
                        { "id": "c_def", "condition": "" }
                    ]
                }
            },
            {
                "id": "expr_cn",
                "type": "expressionNode",
                "name": "CNRoute",
                "content": {
                    "expressions": [
                        { "id": "mode", "key": "mode", "value": "\"smart\"" },
                        { "id": "f_tags", "key": "filters.tags", "value": "[\"edge\"]" },
                        { "id": "f_countries", "key": "filters.countries", "value": "[\"CN\"]" },
                        { "id": "ttl", "key": "ttl", "value": "30" },
                        { "id": "max_answers", "key": "max_answers", "value": "1" }
                    ]
                }
            },
            {
                "id": "expr_def",
                "type": "expressionNode",
                "name": "DefaultRoute",
                "content": {
                    "expressions": [
                        { "id": "mode", "key": "mode", "value": "\"smart\"" },
                        { "id": "f_tags", "key": "filters.tags", "value": "[\"edge\"]" },
                        { "id": "ttl", "key": "ttl", "value": "30" },
                        { "id": "max_answers", "key": "max_answers", "value": "1" }
                    ]
                }
            },
            { "id": "out1", "type": "outputNode", "name": "Response" }
        ],
        "edges": [
            { "id": "e1", "sourceId": "in1", "targetId": "sw1" },
            { "id": "e2", "sourceId": "sw1", "sourceHandle": "c_cn", "targetId": "expr_cn" },
            { "id": "e3", "sourceId": "sw1", "sourceHandle": "c_def", "targetId": "expr_def" },
            { "id": "e4", "sourceId": "expr_cn", "targetId": "out1" },
            { "id": "e5", "sourceId": "expr_def", "targetId": "out1" }
        ]
    });

    SnapshotEnvelope {
        version: 1,
        etag: "bench-v1".to_string(),
        generated_at: "2026-11-27T00:00:00Z".to_string(),
        delegation_domain: "sdns.example.com".to_string(),
        ns_hosts: vec!["ns1.sdns.example.com".to_string()],
        default_ttl: 30,
        nodes: vec![
            SnapshotNode {
                node_id: "node-cn".to_string(),
                name: "CN Edge".to_string(),
                ipv4: "203.0.113.10".to_string(),
                ipv6: String::new(),
                status: "online".to_string(),
                quarantined: false,
                network_group: "chinanet".to_string(),
                asn: 4134,
                country: "CN".to_string(),
                geo_latitude: Some(31.23),
                geo_longitude: Some(121.47),
                capacity_bandwidth_mbps: 10000,
                capacity_connections: 50000,
                capacity_weight: 100,
                cpu_usage_percent: 25.0,
                memory_usage_percent: 40.0,
                bandwidth_mbps: 1200.0,
                active_connections: 3000,
                tags: vec![SnapshotTag {
                    slug: "edge".to_string(),
                    name: "Edge".to_string(),
                    effective_weight: 1.0,
                }],
            },
            SnapshotNode {
                node_id: "node-us".to_string(),
                name: "US Edge".to_string(),
                ipv4: "203.0.113.20".to_string(),
                ipv6: String::new(),
                status: "online".to_string(),
                quarantined: false,
                network_group: "global".to_string(),
                asn: 13335,
                country: "US".to_string(),
                geo_latitude: Some(37.77),
                geo_longitude: Some(-122.42),
                capacity_bandwidth_mbps: 10000,
                capacity_connections: 50000,
                capacity_weight: 100,
                cpu_usage_percent: 20.0,
                memory_usage_percent: 35.0,
                bandwidth_mbps: 900.0,
                active_connections: 2200,
                tags: vec![SnapshotTag {
                    slug: "edge".to_string(),
                    name: "Edge".to_string(),
                    effective_weight: 1.0,
                }],
            },
        ],
        policies: vec![SnapshotPolicy {
            id: 1,
            name: "Bench Policy".to_string(),
            scope_type: "all".to_string(),
            scope_domains: Vec::new(),
            strategy_type: "smart".to_string(),
            priority: 100,
            ttl: 30,
            max_answers: 1,
            is_default: true,
            fallback_strategy: "lowest_load".to_string(),
            fallback_node_ids: Vec::new(),
            jdm_json: jdm.to_string(),
            revision: 1,
        }],
        domain_routes: vec![SnapshotDomainRoute {
            target_host: "bench.sdns.example.com".to_string(),
            policy_id: 1,
            group_id: 1,
            member_domains: vec!["bench.example.com".to_string()],
        }],
        quality: Some(CompactQualitySnapshot {
            schema_version: 1,
            generation: 1,
            generated_at: "2026-11-27T00:00:00Z".to_string(),
            provider_name: "bench".to_string(),
            entries: vec![CompactQualityEntry {
                node_id: "node-us".to_string(),
                client_asn: 0,
                client_country: "US".to_string(),
                client_region: String::new(),
                ip_family: "ipv4".to_string(),
                level: "country".to_string(),
                effective_rtt_ms: 18.0,
                observed_rtt_ms: 18.0,
                predicted_rtt_ms: 18.0,
                rtt_p95_ms: 25.0,
                p95_weight: 0.35,
                dispersion_ms: 4.0,
                loss_rate: 0.0,
                confidence: 0.9,
                quality_state: "healthy".to_string(),
                explore_priority: 0.0,
                geo_base_rtt_ms: 16.0,
                geo_base_known: true,
                source_tier: "asn_country".to_string(),
            }],
            node_defaults: [(
                "node-us".to_string(),
                CompactNodePrior {
                    network_group: "global".to_string(),
                    asn: 13335,
                    lat: Some(37.77),
                    lon: Some(-122.42),
                    median_extra: 5.0,
                },
            )]
            .into_iter()
            .collect(),
        }),
    }
}

fn build_query_packet(id: u16, fqdn: &str, client_v4: Ipv4Addr) -> Result<Vec<u8>> {
    let mut msg = Message::new(id, MessageType::Query, OpCode::Query);
    let name = Name::from_str(fqdn)?;
    msg.add_query(Query::query(name, RecordType::A));
    let mut edns = Edns::new();
    edns.set_max_payload(1232);
    edns.options_mut()
        .insert(EdnsOption::Subnet(ClientSubnet::new(
            client_v4.into(),
            24,
            0,
        )));
    msg.set_edns(edns);
    let mut buf = Vec::with_capacity(96);
    let mut encoder = hickory_proto::serialize::binary::BinEncoder::new(&mut buf);
    msg.emit(&mut encoder)?;
    Ok(buf)
}

async fn run_udp_workers<F>(
    addr: SocketAddr,
    packets: Arc<Vec<Vec<u8>>>,
    concurrency: usize,
    next_index: F,
) -> Result<Duration>
where
    F: Fn(usize, &mut usize) -> Option<usize> + Send + Sync + 'static,
{
    let next_index = Arc::new(next_index);
    let mut handles = Vec::with_capacity(concurrency);
    let start = Instant::now();

    for worker_id in 0..concurrency {
        let sock = UdpSocket::bind("127.0.0.1:0").await?;
        sock.connect(addr).await?;
        let packets = packets.clone();
        let next_index = next_index.clone();
        handles.push(tokio::spawn(async move {
            let mut recv_buf = [0u8; 512];
            let mut seq = worker_id;
            while let Some(idx) = next_index(packets.len(), &mut seq) {
                sock.send(&packets[idx]).await?;
                tokio::time::timeout(Duration::from_secs(2), sock.recv(&mut recv_buf))
                    .await
                    .context("UDP bench recv timeout")??;
            }
            Ok::<_, anyhow::Error>(())
        }));
    }

    for handle in handles {
        handle.await.context("bench worker join")??;
    }
    Ok(start.elapsed())
}

/// Prime each unique packet once across workers so warmup avoids cold-key stampedes.
async fn prime_udp(addr: SocketAddr, packets: Arc<Vec<Vec<u8>>>, concurrency: usize) -> Result<()> {
    let next = AtomicUsize::new(0);
    run_udp_workers(addr, packets, concurrency, move |len, _| {
        let idx = next.fetch_add(1, Ordering::Relaxed);
        (idx < len).then_some(idx)
    })
    .await?;
    Ok(())
}

async fn blast_udp(
    addr: SocketAddr,
    packets: Arc<Vec<Vec<u8>>>,
    total_queries: usize,
    concurrency: usize,
) -> Result<Duration> {
    let remaining = AtomicUsize::new(total_queries);
    run_udp_workers(addr, packets, concurrency, move |len, seq| {
        let prev = remaining.fetch_sub(1, Ordering::Relaxed);
        if prev == 0 || prev > total_queries {
            return None;
        }
        let idx = *seq % len;
        *seq = seq.wrapping_add(1);
        Some(idx)
    })
    .await
}

pub async fn run_bench(args: BenchArgs) -> Result<()> {
    let concurrency = args.concurrency.max(1);
    let queries = args.queries.max(concurrency * 10);

    let packets: Arc<Vec<Vec<u8>>> = Arc::new(
        (0..64u8)
            .map(|i| {
                build_query_packet(
                    1000 + i as u16,
                    "bench.sdns.example.com.",
                    Ipv4Addr::new(198, 51, i, 1),
                )
            })
            .collect::<Result<Vec<_>>>()?,
    );

    // 1. Bare hickory-server baseline.
    let bare_sock = UdpSocket::bind("127.0.0.1:0").await?;
    let bare_addr = bare_sock.local_addr()?;
    let mut bare_server = Server::new(BareBenchHandler::new(
        "bench.sdns.example.com.",
        Ipv4Addr::new(203, 0, 113, 10),
    )?);
    bare_server.register_socket(bare_sock);

    blast_udp(bare_addr, packets.clone(), 256, concurrency.min(16)).await?;
    let bare_elapsed = blast_udp(bare_addr, packets.clone(), queries, concurrency).await?;
    bare_server.shutdown_gracefully().await?;

    // 2. SmartDNS handler (L1 AnswerCache + SnapshotStore + QualityOracle + zen-engine + GeoLookup + RRL bypassed).
    let snap = CompiledSnapshot::from_envelope(build_bench_snapshot());
    let store = Arc::new(SnapshotStore::with_snapshot(snap));
    let geo = Arc::new(GeoLookup::empty());
    let rrl = Arc::new(PrefixRrl::new(0, 0));
    let cache = Arc::new(AnswerCache::new(65_536));
    let hysteresis = Arc::new(HysteresisTracker::new(65_536));
    let metrics = Arc::new(DnsMetrics::new());

    let sdns_handler = SmartDnsHandler::new(store, geo, rrl, cache, hysteresis, metrics.clone());
    let sdns_sock = UdpSocket::bind("127.0.0.1:0").await?;
    let sdns_addr = sdns_sock.local_addr()?;
    let mut sdns_server = Server::new(sdns_handler);
    sdns_server.register_socket(sdns_sock);

    prime_udp(sdns_addr, packets.clone(), concurrency.min(16)).await?;
    blast_udp(sdns_addr, packets.clone(), 256, concurrency.min(16)).await?;
    let sdns_elapsed = blast_udp(sdns_addr, packets.clone(), queries, concurrency).await?;
    sdns_server.shutdown_gracefully().await?;

    let bare_qps = queries as f64 / bare_elapsed.as_secs_f64();
    let sdns_qps = queries as f64 / sdns_elapsed.as_secs_f64();
    let ratio = sdns_qps / bare_qps;

    println!(
        "SmartDNS Bench (queries={queries}, concurrency={concurrency}):\n  bare_hickory_qps = {:.1} ({:.1} ms)\n  smartdns_qps     = {:.1} ({:.1} ms)\n  ratio            = {:.3} ({:.1}%)\n  cache_hits       = {} / {}",
        bare_qps,
        bare_elapsed.as_secs_f64() * 1000.0,
        sdns_qps,
        sdns_elapsed.as_secs_f64() * 1000.0,
        ratio,
        ratio * 100.0,
        metrics.cache_hits.load(Ordering::Relaxed),
        metrics.cache_lookups.load(Ordering::Relaxed),
    );

    if ratio < 0.70 {
        anyhow::bail!(
            "SmartDNS QPS ratio {:.3} is below required 0.70 threshold ({:.1} vs {:.1})",
            ratio,
            sdns_qps,
            bare_qps
        );
    }

    Ok(())
}
