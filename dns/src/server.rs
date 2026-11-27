use crate::cache::{AnswerCache, AnswerCacheKey, CachedAnswer, PrefixRrl};
use crate::config::ServeArgs;
use crate::control::{register_if_needed, spawn_control_loops};
use crate::engine::{
    HysteresisTracker, evaluate_and_select, match_domain_pattern, match_policy, normalize_domain,
};
use crate::geo::{GeoLookup, client_prefix_key};
use crate::metrics::DnsMetrics;
use crate::protocol::SnapshotEnvelope;
use crate::snapshot::{SnapshotStore, pull_snapshot};
use anyhow::{Context, Result};
use hickory_proto::op::{Edns, Metadata, OpCode, ResponseCode};
use hickory_proto::rr::rdata::opt::{ClientSubnet, EdnsCode, EdnsOption};
use hickory_proto::rr::{Name, RData, Record, RecordType, rdata};
use hickory_server::Server;
use hickory_server::net::runtime::Time;
use hickory_server::server::{Request, RequestHandler, ResponseHandler, ResponseInfo};
use hickory_server::zone_handler::MessageResponseBuilder;
use reqwest::Client;
use socket2::{Domain, Protocol as SockProtocol, Socket, Type};
use std::net::{IpAddr, SocketAddr};
use std::str::FromStr;
use std::sync::Arc;
use std::sync::atomic::Ordering;
use std::time::{Duration, Instant};
use tokio::net::{TcpListener, UdpSocket};
use tracing::{error, info, warn};

pub struct SmartDnsHandler {
    pub store: Arc<SnapshotStore>,
    pub geo: Arc<GeoLookup>,
    pub rrl: Arc<PrefixRrl>,
    pub cache: Arc<AnswerCache>,
    pub hysteresis: Arc<HysteresisTracker>,
    pub metrics: Arc<DnsMetrics>,
}

impl SmartDnsHandler {
    pub fn new(
        store: Arc<SnapshotStore>,
        geo: Arc<GeoLookup>,
        rrl: Arc<PrefixRrl>,
        cache: Arc<AnswerCache>,
        hysteresis: Arc<HysteresisTracker>,
        metrics: Arc<DnsMetrics>,
    ) -> Self {
        Self {
            store,
            geo,
            rrl,
            cache,
            hysteresis,
            metrics,
        }
    }

    async fn send_status_response<R: ResponseHandler>(
        &self,
        request: &Request,
        mut response_handle: R,
        rcode: ResponseCode,
        reply_edns: Option<&Edns>,
    ) -> ResponseInfo {
        let mut meta = Metadata::response_from_request(&request.metadata);
        meta.authoritative = true;
        meta.recursion_available = false;
        meta.response_code = rcode;

        let mut builder = MessageResponseBuilder::from_message_request(request);
        if let Some(edns) = reply_edns {
            builder.edns(edns);
        }
        let resp = builder.build_no_records(meta);
        send_or_servfail(request, &mut response_handle, resp).await
    }

    async fn send_records_response<R: ResponseHandler>(
        &self,
        request: &Request,
        mut response_handle: R,
        rcode: ResponseCode,
        answers: &[Record],
        soa: &[Record],
        reply_edns: Option<&Edns>,
    ) -> ResponseInfo {
        let mut meta = Metadata::response_from_request(&request.metadata);
        meta.authoritative = true;
        meta.recursion_available = false;
        meta.response_code = rcode;

        let mut builder = MessageResponseBuilder::from_message_request(request);
        if let Some(edns) = reply_edns {
            builder.edns(edns);
        }
        let resp = builder.build(
            meta,
            answers.iter(),
            std::iter::empty::<&Record>(),
            soa.iter(),
            std::iter::empty::<&Record>(),
        );
        send_or_servfail(request, &mut response_handle, resp).await
    }
}

#[async_trait::async_trait]
impl RequestHandler for SmartDnsHandler {
    async fn handle_request<R: ResponseHandler, T: Time>(
        &self,
        request: &Request,
        response_handle: R,
    ) -> ResponseInfo {
        let start = Instant::now();
        self.metrics.queries_total.fetch_add(1, Ordering::Relaxed);

        let Ok(info) = request.request_info() else {
            self.metrics.queries_servfail.fetch_add(1, Ordering::Relaxed);
            return self
                .send_status_response(request, response_handle, ResponseCode::FormErr, None)
                .await;
        };

        if info.metadata.op_code != OpCode::Query {
            self.metrics.queries_servfail.fetch_add(1, Ordering::Relaxed);
            return self
                .send_status_response(request, response_handle, ResponseCode::NotImp, None)
                .await;
        }

        let (client_ip, reply_edns) = extract_client_and_reply_edns(request);
        let prefix_key = client_prefix_key(client_ip);
        let now = Instant::now();

        if !self.rrl.allow(prefix_key, now) {
            self.metrics
                .queries_ratelimited
                .fetch_add(1, Ordering::Relaxed);
            let out = self
                .send_status_response(
                    request,
                    response_handle,
                    ResponseCode::Refused,
                    reply_edns.as_ref(),
                )
                .await;
            self.metrics.record_latency(start.elapsed());
            return out;
        }

        let snap = self.store.guard();
        if snap.raw.version == 0 && snap.policies.is_empty() {
            self.metrics.queries_servfail.fetch_add(1, Ordering::Relaxed);
            let out = self
                .send_status_response(
                    request,
                    response_handle,
                    ResponseCode::ServFail,
                    reply_edns.as_ref(),
                )
                .await;
            self.metrics.record_latency(start.elapsed());
            return out;
        }

        let qtype = info.query.query_type();
        let cache_key = if qtype == RecordType::A || qtype == RecordType::AAAA {
            let key = AnswerCacheKey {
                version: snap.raw.version,
                qname: info.query.name().clone(),
                qtype,
                prefix: prefix_key,
            };
            self.metrics.cache_lookups.fetch_add(1, Ordering::Relaxed);
            if let Some(hit) = self.cache.get_valid(&key, now) {
                self.metrics.cache_hits.fetch_add(1, Ordering::Relaxed);
                self.metrics.queries_success.fetch_add(1, Ordering::Relaxed);
                let out = self
                    .send_records_response(
                        request,
                        response_handle,
                        hit.rcode,
                        &hit.answers,
                        &hit.soa,
                        reply_edns.as_ref(),
                    )
                    .await;
                self.metrics.record_latency(start.elapsed());
                return out;
            }
            Some(key)
        } else {
            None
        };

        let qname_orig = Name::from(info.query.name().clone());
        let clean_qname = normalize_domain(&info.query.name().to_string());

        if !is_authoritative_query(&snap.raw, &clean_qname) {
            self.metrics.queries_nxdomain.fetch_add(1, Ordering::Relaxed);
            let out = self
                .send_status_response(
                    request,
                    response_handle,
                    ResponseCode::Refused,
                    reply_edns.as_ref(),
                )
                .await;
            self.metrics.record_latency(start.elapsed());
            return out;
        }

        if qtype == RecordType::SOA {
            let answers: Vec<Record> = build_soa_record(&clean_qname, &snap.raw)
                .into_iter()
                .collect();
            self.metrics.queries_success.fetch_add(1, Ordering::Relaxed);
            let out = self
                .send_records_response(
                    request,
                    response_handle,
                    ResponseCode::NoError,
                    &answers,
                    &[],
                    reply_edns.as_ref(),
                )
                .await;
            self.metrics.record_latency(start.elapsed());
            return out;
        }

        if qtype == RecordType::NS {
            let ttl = snap.raw.default_ttl.max(60) as u32;
            let ns_records: Vec<Record> = snap
                .raw
                .ns_hosts
                .iter()
                .filter_map(|host| {
                    parse_fqdn(host).map(|target| {
                        Record::from_rdata(qname_orig.clone(), ttl, RData::NS(rdata::NS(target)))
                    })
                })
                .collect();
            let soa: Vec<Record> = if ns_records.is_empty() {
                build_soa_record(&clean_qname, &snap.raw)
                    .into_iter()
                    .collect()
            } else {
                Vec::new()
            };
            self.metrics.queries_success.fetch_add(1, Ordering::Relaxed);
            let out = self
                .send_records_response(
                    request,
                    response_handle,
                    ResponseCode::NoError,
                    &ns_records,
                    &soa,
                    reply_edns.as_ref(),
                )
                .await;
            self.metrics.record_latency(start.elapsed());
            return out;
        }

        if qtype != RecordType::A && qtype != RecordType::AAAA {
            let soa: Vec<Record> = build_soa_record(&clean_qname, &snap.raw)
                .into_iter()
                .collect();
            self.metrics.queries_success.fetch_add(1, Ordering::Relaxed);
            let out = self
                .send_records_response(
                    request,
                    response_handle,
                    ResponseCode::NoError,
                    &[],
                    &soa,
                    reply_edns.as_ref(),
                )
                .await;
            self.metrics.record_latency(start.elapsed());
            return out;
        }

        let (qtype_str, answer_family) = if qtype == RecordType::AAAA {
            ("AAAA", "ipv6")
        } else {
            ("A", "ipv4")
        };

        let Some(policy) = match_policy(&snap.raw, &snap.policies, &clean_qname) else {
            self.metrics.queries_nxdomain.fetch_add(1, Ordering::Relaxed);
            let soa: Vec<Record> = build_soa_record(&clean_qname, &snap.raw)
                .into_iter()
                .collect();
            let out = self
                .send_records_response(
                    request,
                    response_handle,
                    ResponseCode::NXDomain,
                    &[],
                    &soa,
                    reply_edns.as_ref(),
                )
                .await;
            self.metrics.record_latency(start.elapsed());
            return out;
        };

        let client = self.geo.lookup_client(client_ip, answer_family);
        match evaluate_and_select(
            &snap.oracle,
            &snap.raw.nodes,
            policy,
            &client,
            &clean_qname,
            qtype_str,
            Some(&self.hysteresis),
            now,
        )
        .await
        {
            Ok(selection) => {
                let ttl = selection.ttl.max(5);
                let mut answers = Vec::with_capacity(selection.answers.len());
                for sel in &selection.answers {
                    match (qtype, sel.ip) {
                        (RecordType::A, IpAddr::V4(v4)) => {
                            answers.push(Record::from_rdata(
                                qname_orig.clone(),
                                ttl,
                                RData::A(rdata::A(v4)),
                            ));
                        }
                        (RecordType::AAAA, IpAddr::V6(v6)) => {
                            answers.push(Record::from_rdata(
                                qname_orig.clone(),
                                ttl,
                                RData::AAAA(rdata::AAAA(v6)),
                            ));
                        }
                        _ => {}
                    }
                }
                let soa: Vec<Record> = if answers.is_empty() {
                    build_soa_record(&clean_qname, &snap.raw)
                        .into_iter()
                        .collect()
                } else {
                    Vec::new()
                };
                let cache_ttl_secs = (ttl as u64).clamp(5, 30);
                let cached = Arc::new(CachedAnswer {
                    rcode: ResponseCode::NoError,
                    answers,
                    soa,
                    expires_at: now + Duration::from_secs(cache_ttl_secs),
                });
                if let Some(key) = cache_key {
                    self.cache.insert(key, cached.clone());
                }
                self.metrics.queries_success.fetch_add(1, Ordering::Relaxed);
                let out = self
                    .send_records_response(
                        request,
                        response_handle,
                        cached.rcode,
                        &cached.answers,
                        &cached.soa,
                        reply_edns.as_ref(),
                    )
                    .await;
                self.metrics.record_latency(start.elapsed());
                out
            }
            Err(err) => {
                warn!("SmartDNS evaluation error for {}: {}", clean_qname, err);
                self.metrics.queries_servfail.fetch_add(1, Ordering::Relaxed);
                let out = self
                    .send_status_response(
                        request,
                        response_handle,
                        ResponseCode::ServFail,
                        reply_edns.as_ref(),
                    )
                    .await;
                self.metrics.record_latency(start.elapsed());
                out
            }
        }
    }
}

async fn send_or_servfail<'q, 'a, R, A, N, S, D>(
    request: &'q Request,
    response_handle: &mut R,
    response: hickory_server::zone_handler::MessageResponse<'q, 'a, A, N, S, D>,
) -> ResponseInfo
where
    R: ResponseHandler,
    A: Iterator<Item = &'a Record> + Send + 'a,
    N: Iterator<Item = &'a Record> + Send + 'a,
    S: Iterator<Item = &'a Record> + Send + 'a,
    D: Iterator<Item = &'a Record> + Send + 'a,
{
    match response_handle.send_response(response).await {
        Ok(info) => info,
        Err(err) => {
            error!("failed sending DNS response: {}", err);
            let mut meta = Metadata::response_from_request(&request.metadata);
            meta.response_code = ResponseCode::ServFail;
            ResponseInfo::from(hickory_proto::op::Header {
                metadata: meta,
                counts: Default::default(),
            })
        }
    }
}

fn extract_client_and_reply_edns(request: &Request) -> (IpAddr, Option<Edns>) {
    let mut client_ip = request.src().ip();
    let Some(req_edns) = request.edns.as_ref() else {
        return (client_ip, None);
    };

    let mut reply_edns = Edns::new();
    reply_edns.set_max_payload(req_edns.max_payload().clamp(512, 1232));
    reply_edns.set_version(0);

    if let Some(EdnsOption::Subnet(ecs)) = req_edns.option(EdnsCode::Subnet) {
        let src_prefix = ecs.source_prefix();
        if src_prefix == 0 {
            let reply_ecs = ClientSubnet::new(ecs.addr(), 0, 0);
            reply_edns.options_mut().insert(EdnsOption::Subnet(reply_ecs));
        } else {
            client_ip = ecs.addr();
            let scope_prefix = match ecs.addr() {
                IpAddr::V4(_) => 24,
                IpAddr::V6(_) => 48,
            };
            let reply_ecs = ClientSubnet::new(ecs.addr(), src_prefix, scope_prefix);
            reply_edns.options_mut().insert(EdnsOption::Subnet(reply_ecs));
        }
    }

    (client_ip, Some(reply_edns))
}

fn is_authoritative_query(env: &SnapshotEnvelope, clean_qname: &str) -> bool {
    let deleg = normalize_domain(&env.delegation_domain);
    if !deleg.is_empty() {
        if clean_qname == deleg || clean_qname.ends_with(&format!(".{deleg}")) {
            return true;
        }
    }
    for route in &env.domain_routes {
        if normalize_domain(&route.target_host) == clean_qname
            || route
                .member_domains
                .iter()
                .any(|pat| match_domain_pattern(pat, clean_qname))
        {
            return true;
        }
    }
    for p in &env.policies {
        if p.scope_type.eq_ignore_ascii_case("domains")
            && p.scope_domains
                .iter()
                .any(|pat| match_domain_pattern(pat, clean_qname))
        {
            return true;
        }
    }
    // If no delegation domain is configured, fall back to allowing domain queries only when
    // policies exist without restriction.
    deleg.is_empty()
        && env
            .policies
            .iter()
            .any(|p| p.is_default || p.scope_type.eq_ignore_ascii_case("all") || p.scope_type.is_empty())
}

fn parse_fqdn(raw: &str) -> Option<Name> {
    let trimmed = normalize_domain(raw);
    if trimmed.is_empty() {
        return None;
    }
    Name::from_str(&format!("{trimmed}.")).ok()
}

fn build_soa_record(clean_qname: &str, env: &SnapshotEnvelope) -> Option<Record> {
    let zone = {
        let deleg = normalize_domain(&env.delegation_domain);
        if deleg.is_empty() {
            clean_qname.to_string()
        } else {
            deleg
        }
    };
    let owner = parse_fqdn(&zone)?;
    let mname = env
        .ns_hosts
        .first()
        .and_then(|s| parse_fqdn(s))
        .or_else(|| parse_fqdn(&format!("ns1.{zone}")))?;
    let rname = parse_fqdn(&format!("hostmaster.{zone}"))?;
    let serial = env.version.max(1) as u32;
    let min_ttl = env.default_ttl.max(60) as u32;
    Some(Record::from_rdata(
        owner,
        min_ttl,
        RData::SOA(rdata::SOA::new(mname, rname, serial, 300, 60, 604800, min_ttl)),
    ))
}

fn bind_udp_reuseport(addr: SocketAddr) -> Result<UdpSocket> {
    let domain = Domain::for_address(addr);
    let sock = Socket::new(domain, Type::DGRAM, Some(SockProtocol::UDP))?;
    sock.set_reuse_address(true)?;
    #[cfg(unix)]
    sock.set_reuse_port(true)?;
    if addr.is_ipv6() {
        let _ = sock.set_only_v6(false);
    }
    sock.set_nonblocking(true)?;
    sock.bind(&addr.into())?;
    let std_sock: std::net::UdpSocket = sock.into();
    Ok(UdpSocket::from_std(std_sock)?)
}

fn bind_tcp_reuseaddr(addr: SocketAddr) -> Result<TcpListener> {
    let domain = Domain::for_address(addr);
    let sock = Socket::new(domain, Type::STREAM, Some(SockProtocol::TCP))?;
    sock.set_reuse_address(true)?;
    if addr.is_ipv6() {
        let _ = sock.set_only_v6(false);
    }
    sock.set_nonblocking(true)?;
    sock.bind(&addr.into())?;
    sock.listen(1024)?;
    let std_listener: std::net::TcpListener = sock.into();
    Ok(TcpListener::from_std(std_listener)?)
}

pub async fn run_serve(args: ServeArgs) -> Result<()> {
    let listen_addr: SocketAddr = args
        .listen
        .parse()
        .with_context(|| format!("invalid listen address {:?}", args.listen))?;

    let http_client = Client::builder()
        .timeout(Duration::from_secs(10))
        .gzip(true)
        .build()
        .context("build reqwest client")?;

    let token = register_if_needed(&http_client, &args).await?;
    let store = Arc::new(SnapshotStore::new(Some(&args.cache_dir)));

    if let Err(err) = pull_snapshot(&http_client, &args.controller_url, &token, &store).await {
        let (ver, _) = store.current_version_and_etag();
        if ver > 0 {
            warn!(
                "initial snapshot pull failed ({}); continuing with cached snapshot v{}",
                err, ver
            );
        } else {
            warn!(
                "initial snapshot pull failed ({}); starting with empty snapshot until controller reconnects",
                err
            );
        }
    }

    let geo = Arc::new(GeoLookup::load_from_dir(&args.geoip_dir));
    let rrl = Arc::new(PrefixRrl::new(args.rrl_qps, args.rrl_burst));
    let cache = Arc::new(AnswerCache::new(65_536));
    let hysteresis = Arc::new(HysteresisTracker::new(65_536));
    let metrics = Arc::new(DnsMetrics::new());

    spawn_control_loops(
        http_client,
        args.controller_url.clone(),
        token,
        args.listen.clone(),
        args.public_addr.clone(),
        args.snapshot_interval_secs,
        store.clone(),
        metrics.clone(),
    );

    let handler = SmartDnsHandler::new(store, geo, rrl, cache, hysteresis, metrics);
    let mut server = Server::new(handler);

    let workers = if args.workers == 0 {
        std::thread::available_parallelism()
            .map(|n| n.get())
            .unwrap_or(4)
            .clamp(1, 32)
    } else {
        args.workers.clamp(1, 64)
    };

    #[cfg(unix)]
    let udp_count = workers;
    #[cfg(not(unix))]
    let udp_count = 1usize;

    for idx in 0..udp_count {
        let sock = bind_udp_reuseport(listen_addr)
            .with_context(|| format!("bind UDP socket #{idx} on {listen_addr}"))?;
        server.register_socket(sock);
    }

    let tcp_listener = bind_tcp_reuseaddr(listen_addr)
        .with_context(|| format!("bind TCP listener on {listen_addr}"))?;
    server.register_listener(tcp_listener, Duration::from_secs(5), 32);

    info!(
        "openflare-dns listening on {} (udp_workers={}, rrl={}/{}qps)",
        listen_addr, udp_count, args.rrl_qps, args.rrl_burst
    );

    let shutdown = server.shutdown_token().clone();
    tokio::spawn(async move {
        let _ = tokio::signal::ctrl_c().await;
        info!("received shutdown signal, stopping openflare-dns");
        shutdown.cancel();
    });

    server
        .block_until_done()
        .await
        .context("DNS server exited with error")?;
    Ok(())
}
