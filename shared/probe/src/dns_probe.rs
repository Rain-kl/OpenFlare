use crate::ping::split_host_port;
use crate::types::{DnsQueryRequest, DnsQueryResult, DnsRecord};
use std::net::{IpAddr, Ipv4Addr, Ipv6Addr, SocketAddr};
use std::str::FromStr;
use std::sync::atomic::{AtomicU16, Ordering};
use std::sync::OnceLock;
use std::time::{Duration, Instant};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpStream, UdpSocket};

const DEFAULT_DNS_TIMEOUT_MS: u64 = 5_000;
const MIN_DNS_TIMEOUT_MS: u64 = 500;
const MAX_DNS_TIMEOUT_MS: u64 = 15_000;
const MAX_NAME_POINTER_JUMPS: usize = 16;

const TYPE_A: u16 = 1;
const TYPE_NS: u16 = 2;
const TYPE_CNAME: u16 = 5;
const TYPE_SOA: u16 = 6;
const TYPE_PTR: u16 = 12;
const TYPE_MX: u16 = 15;
const TYPE_TXT: u16 = 16;
const TYPE_AAAA: u16 = 28;
const TYPE_SRV: u16 = 33;
const TYPE_OPT: u16 = 41;
const TYPE_CAA: u16 = 257;
const EDNS_OPTION_ECS: u16 = 8;

fn next_query_id() -> u16 {
    static QUERY_COUNTER: OnceLock<AtomicU16> = OnceLock::new();
    let counter = QUERY_COUNTER.get_or_init(|| AtomicU16::new(std::process::id() as u16));
    counter.fetch_add(1, Ordering::Relaxed)
}

#[derive(Debug, Clone, Copy)]
struct ParsedECS {
    family: u16,
    prefix_len: u8,
    addr_bytes: [u8; 16],
    wire_bytes: usize,
}

/// Executes an RFC 1035 DNS lookup against the specified resolver with optional EDNS0 Client Subnet.
/// Automatically falls back from UDP to TCP when the response sets `TC=1`, enforcing SSRF validation.
pub async fn execute_dns_query(req: &DnsQueryRequest) -> DnsQueryResult {
    execute_dns_query_with_policy(req, true).await
}

pub(crate) async fn execute_dns_query_with_policy(
    req: &DnsQueryRequest,
    enforce_safety: bool,
) -> DnsQueryResult {
    let domain = req.domain.trim().trim_end_matches('.');
    let record_type = normalize_qtype_name(&req.record_type);
    let resolver_input = if req.resolver.trim().is_empty() {
        "1.1.1.1:53"
    } else {
        req.resolver.trim()
    };

    if domain.is_empty() && req.domain.trim() != "." {
        return dns_error(
            &req.domain,
            &record_type,
            resolver_input,
            "DNS domain cannot be empty".to_owned(),
        );
    }

    let Some(qtype) = parse_qtype(&record_type) else {
        return dns_error(
            domain,
            &record_type,
            resolver_input,
            format!("unsupported DNS record type: {}", req.record_type),
        );
    };

    let ecs = match req
        .ecs_subnet
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
    {
        Some(raw) => match parse_ecs_subnet(raw) {
            Ok(parsed) => Some(parsed),
            Err(err) => return dns_error(domain, &record_type, resolver_input, err),
        },
        None => None,
    };

    let resolver_addr = match resolve_dns_server_addr(resolver_input, enforce_safety).await {
        Ok(addr) => addr,
        Err(err) => return dns_error(domain, &record_type, resolver_input, err),
    };

    let tx_id = next_query_id();
    let packet = match build_dns_query(tx_id, domain, qtype, ecs) {
        Ok(pkt) => pkt,
        Err(err) => return dns_error(domain, &record_type, resolver_input, err),
    };

    let timeout = Duration::from_millis(
        req.timeout_ms
            .unwrap_or(DEFAULT_DNS_TIMEOUT_MS)
            .clamp(MIN_DNS_TIMEOUT_MS, MAX_DNS_TIMEOUT_MS),
    );

    let started = Instant::now();
    let raw_response = match tokio::time::timeout(timeout, query_udp(resolver_addr, &packet)).await
    {
        Ok(Ok(bytes)) => bytes,
        Ok(Err(err)) => {
            return dns_error(
                domain,
                &record_type,
                resolver_input,
                format!("DNS UDP query failed: {err}"),
            );
        }
        Err(_) => {
            return dns_error(
                domain,
                &record_type,
                resolver_input,
                format!("DNS query timed out after {}ms", timeout.as_millis()),
            );
        }
    };

    let mut parsed = match parse_dns_response(tx_id, &raw_response, started, false) {
        Ok(resp) => resp,
        Err(err) => return dns_error(domain, &record_type, resolver_input, err),
    };

    if parsed.truncated {
        match tokio::time::timeout(timeout, query_tcp(resolver_addr, &packet)).await {
            Ok(Ok(tcp_bytes)) => {
                parsed = match parse_dns_response(tx_id, &tcp_bytes, started, true) {
                    Ok(resp) => resp,
                    Err(err) => return dns_error(domain, &record_type, resolver_input, err),
                };
            }
            Ok(Err(err)) => {
                parsed.error = Some(format!("DNS TCP fallback failed: {err}"));
            }
            Err(_) => {
                parsed.error = Some("DNS TCP fallback timed out".to_owned());
            }
        }
    }

    parsed.domain = if domain.is_empty() {
        ".".to_owned()
    } else {
        domain.to_owned()
    };
    parsed.record_type = record_type;
    parsed.resolver = resolver_addr.to_string();
    parsed
}

async fn resolve_dns_server_addr(raw: &str, enforce_safety: bool) -> Result<SocketAddr, String> {
    let trimmed = raw.trim();
    if let Ok(sock) = SocketAddr::from_str(trimmed) {
        if enforce_safety {
            crate::safety::validate_target_ip(sock.ip())?;
        }
        return Ok(sock);
    }
    if let Ok(ip) = IpAddr::from_str(trimmed) {
        if enforce_safety {
            crate::safety::validate_target_ip(ip)?;
        }
        return Ok(SocketAddr::new(ip, 53));
    }
    let (host, port) = if trimmed.contains(':') {
        split_host_port(trimmed)
    } else {
        (trimmed.to_owned(), 53)
    };
    if host.is_empty() {
        return Err("DNS resolver host cannot be empty".to_owned());
    }
    if enforce_safety {
        return crate::safety::resolve_public_socket_addr(&host, port).await;
    }
    let mut addrs = tokio::net::lookup_host((host.as_str(), port))
        .await
        .map_err(|err| format!("failed to resolve DNS server {trimmed}: {err}"))?;
    addrs
        .next()
        .ok_or_else(|| format!("no IP addresses found for DNS server {trimmed}"))
}

async fn query_udp(addr: SocketAddr, packet: &[u8]) -> Result<Vec<u8>, std::io::Error> {
    let bind_addr: SocketAddr = if addr.is_ipv6() {
        SocketAddr::new(IpAddr::V6(Ipv6Addr::UNSPECIFIED), 0)
    } else {
        SocketAddr::new(IpAddr::V4(Ipv4Addr::UNSPECIFIED), 0)
    };
    let sock = UdpSocket::bind(bind_addr).await?;
    sock.connect(addr).await?;
    sock.send(packet).await?;
    let mut buf = vec![0u8; 4096];
    let n = sock.recv(&mut buf).await?;
    buf.truncate(n);
    Ok(buf)
}

async fn query_tcp(addr: SocketAddr, packet: &[u8]) -> Result<Vec<u8>, std::io::Error> {
    let mut stream = TcpStream::connect(addr).await?;
    let len = u16::try_from(packet.len()).map_err(|_| {
        std::io::Error::new(std::io::ErrorKind::InvalidInput, "DNS packet too large")
    })?;
    stream.write_all(&len.to_be_bytes()).await?;
    stream.write_all(packet).await?;
    stream.flush().await?;

    let mut len_buf = [0u8; 2];
    stream.read_exact(&mut len_buf).await?;
    let resp_len = usize::from(u16::from_be_bytes(len_buf));
    let mut resp = vec![0u8; resp_len];
    stream.read_exact(&mut resp).await?;
    Ok(resp)
}

fn normalize_qtype_name(raw: &str) -> String {
    let up = raw.trim().to_ascii_uppercase();
    if up.is_empty() {
        "A".to_owned()
    } else {
        up
    }
}

fn parse_qtype(name: &str) -> Option<u16> {
    match name {
        "A" => Some(TYPE_A),
        "NS" => Some(TYPE_NS),
        "CNAME" => Some(TYPE_CNAME),
        "SOA" => Some(TYPE_SOA),
        "PTR" => Some(TYPE_PTR),
        "MX" => Some(TYPE_MX),
        "TXT" => Some(TYPE_TXT),
        "AAAA" => Some(TYPE_AAAA),
        "SRV" => Some(TYPE_SRV),
        "CAA" => Some(TYPE_CAA),
        _ => None,
    }
}

fn qtype_to_str(qtype: u16) -> String {
    match qtype {
        TYPE_A => "A".to_owned(),
        TYPE_NS => "NS".to_owned(),
        TYPE_CNAME => "CNAME".to_owned(),
        TYPE_SOA => "SOA".to_owned(),
        TYPE_PTR => "PTR".to_owned(),
        TYPE_MX => "MX".to_owned(),
        TYPE_TXT => "TXT".to_owned(),
        TYPE_AAAA => "AAAA".to_owned(),
        TYPE_SRV => "SRV".to_owned(),
        TYPE_CAA => "CAA".to_owned(),
        other => format!("TYPE{other}"),
    }
}

fn rcode_to_str(rcode: u8) -> String {
    match rcode {
        0 => "NOERROR".to_owned(),
        1 => "FORMERR".to_owned(),
        2 => "SERVFAIL".to_owned(),
        3 => "NXDOMAIN".to_owned(),
        4 => "NOTIMP".to_owned(),
        5 => "REFUSED".to_owned(),
        other => format!("RCODE_{other}"),
    }
}

fn parse_ecs_subnet(raw: &str) -> Result<ParsedECS, String> {
    let (ip_str, prefix_opt) = match raw.split_once('/') {
        Some((ip, bits)) => {
            let parsed_bits = bits
                .trim()
                .parse::<u8>()
                .map_err(|_| format!("invalid ECS prefix length in {raw}"))?;
            (ip.trim(), Some(parsed_bits))
        }
        None => (raw.trim(), None),
    };

    let ip = IpAddr::from_str(ip_str).map_err(|_| format!("invalid ECS IP address: {raw}"))?;
    match ip {
        IpAddr::V4(v4) => {
            let prefix_len = prefix_opt.unwrap_or(24);
            if prefix_len > 32 {
                return Err(format!("invalid IPv4 ECS prefix length: /{prefix_len}"));
            }
            let mask = if prefix_len == 0 {
                0u32
            } else {
                !0u32 << (32 - u32::from(prefix_len))
            };
            let masked = (u32::from(v4) & mask).to_be_bytes();
            let mut addr_bytes = [0u8; 16];
            addr_bytes[..4].copy_from_slice(&masked);
            Ok(ParsedECS {
                family: 1,
                prefix_len,
                addr_bytes,
                wire_bytes: usize::from(prefix_len).div_ceil(8),
            })
        }
        IpAddr::V6(v6) => {
            let prefix_len = prefix_opt.unwrap_or(56);
            if prefix_len > 128 {
                return Err(format!("invalid IPv6 ECS prefix length: /{prefix_len}"));
            }
            let mask = if prefix_len == 0 {
                0u128
            } else {
                !0u128 << (128 - u32::from(prefix_len))
            };
            let masked = (u128::from(v6) & mask).to_be_bytes();
            Ok(ParsedECS {
                family: 2,
                prefix_len,
                addr_bytes: masked,
                wire_bytes: usize::from(prefix_len).div_ceil(8),
            })
        }
    }
}

fn build_dns_query(
    tx_id: u16,
    domain: &str,
    qtype: u16,
    ecs: Option<ParsedECS>,
) -> Result<Vec<u8>, String> {
    let mut buf = Vec::with_capacity(64);
    buf.extend_from_slice(&tx_id.to_be_bytes());
    buf.extend_from_slice(&0x0100u16.to_be_bytes()); // standard query, RD=1
    buf.extend_from_slice(&1u16.to_be_bytes()); // QDCOUNT=1
    buf.extend_from_slice(&0u16.to_be_bytes()); // ANCOUNT=0
    buf.extend_from_slice(&0u16.to_be_bytes()); // NSCOUNT=0
    buf.extend_from_slice(&1u16.to_be_bytes()); // ARCOUNT=1 (OPT RR)

    encode_qname(domain, &mut buf)?;
    buf.extend_from_slice(&qtype.to_be_bytes());
    buf.extend_from_slice(&1u16.to_be_bytes()); // QCLASS = IN

    // RFC 6891 EDNS0 OPT pseudo-RR
    buf.push(0); // root owner name
    buf.extend_from_slice(&TYPE_OPT.to_be_bytes());
    buf.extend_from_slice(&4096u16.to_be_bytes()); // UDP payload size
    buf.extend_from_slice(&0u32.to_be_bytes()); // extended RCODE & flags

    if let Some(ecs) = ecs {
        let opt_data_len = 4 + ecs.wire_bytes;
        let rdlen = u16::try_from(4 + opt_data_len)
            .map_err(|_| "EDNS0 ECS option length overflow".to_owned())?;
        buf.extend_from_slice(&rdlen.to_be_bytes());
        buf.extend_from_slice(&EDNS_OPTION_ECS.to_be_bytes());
        buf.extend_from_slice(&(opt_data_len as u16).to_be_bytes());
        buf.extend_from_slice(&ecs.family.to_be_bytes());
        buf.push(ecs.prefix_len);
        buf.push(0); // scope prefix-length = 0
        buf.extend_from_slice(&ecs.addr_bytes[..ecs.wire_bytes]);
    } else {
        buf.extend_from_slice(&0u16.to_be_bytes());
    }

    Ok(buf)
}

fn encode_qname(domain: &str, out: &mut Vec<u8>) -> Result<(), String> {
    let trimmed = domain.trim().trim_end_matches('.');
    if trimmed.is_empty() {
        out.push(0);
        return Ok(());
    }
    if trimmed.len() > 253 {
        return Err("DNS domain name exceeds 253 bytes".to_owned());
    }
    for label in trimmed.split('.') {
        if label.is_empty() || label.len() > 63 {
            return Err(format!("invalid DNS label length in domain {trimmed}"));
        }
        out.push(label.len() as u8);
        out.extend_from_slice(label.as_bytes());
    }
    out.push(0);
    Ok(())
}

fn parse_dns_response(
    expected_id: u16,
    buf: &[u8],
    started: Instant,
    used_tcp: bool,
) -> Result<DnsQueryResult, String> {
    if buf.len() < 12 {
        return Err("truncated DNS response header".to_owned());
    }
    let id = u16::from_be_bytes([buf[0], buf[1]]);
    if id != expected_id {
        return Err("mismatched DNS transaction ID".to_owned());
    }
    let flags = u16::from_be_bytes([buf[2], buf[3]]);
    let aa = (flags & 0x0400) != 0;
    let tc = (flags & 0x0200) != 0;
    let rcode = (flags & 0x000f) as u8;

    let qdcount = usize::from(u16::from_be_bytes([buf[4], buf[5]]));
    let ancount = usize::from(u16::from_be_bytes([buf[6], buf[7]]));

    let mut offset = 12usize;
    for _ in 0..qdcount {
        let (_, next) = read_name(buf, offset)?;
        if next + 4 > buf.len() {
            return Err("truncated DNS question section".to_owned());
        }
        offset = next + 4;
    }

    let mut records = Vec::with_capacity(ancount);
    for _ in 0..ancount {
        let (name, next) = read_name(buf, offset)?;
        if next + 10 > buf.len() {
            return Err("truncated DNS answer header".to_owned());
        }
        let rtype = u16::from_be_bytes([buf[next], buf[next + 1]]);
        let _rclass = u16::from_be_bytes([buf[next + 2], buf[next + 3]]);
        let ttl = u32::from_be_bytes([buf[next + 4], buf[next + 5], buf[next + 6], buf[next + 7]]);
        let rdlength = usize::from(u16::from_be_bytes([buf[next + 8], buf[next + 9]]));
        let rdata_start = next + 10;
        let rdata_end = rdata_start
            .checked_add(rdlength)
            .ok_or_else(|| "DNS RDLENGTH overflow".to_owned())?;
        if rdata_end > buf.len() {
            return Err("truncated DNS answer rdata".to_owned());
        }
        let value = decode_rdata(buf, rtype, rdata_start, rdata_end)?;
        records.push(DnsRecord {
            name: if name.is_empty() {
                ".".to_owned()
            } else {
                name
            },
            record_type: qtype_to_str(rtype),
            ttl,
            value,
        });
        offset = rdata_end;
    }

    Ok(DnsQueryResult {
        domain: String::new(),
        record_type: String::new(),
        resolver: String::new(),
        rcode: rcode_to_str(rcode),
        authoritative: aa,
        truncated: tc,
        used_tcp,
        latency_ms: started.elapsed().as_secs_f64() * 1000.0,
        records,
        error: None,
    })
}

fn read_name(packet: &[u8], start: usize) -> Result<(String, usize), String> {
    let mut labels = Vec::new();
    let mut pos = start;
    let mut next_offset = None;
    let mut jumps = 0usize;

    loop {
        if pos >= packet.len() {
            return Err("unexpected end of DNS name".to_owned());
        }
        let len = packet[pos];
        if len == 0 {
            if next_offset.is_none() {
                next_offset = Some(pos + 1);
            }
            break;
        }
        if (len & 0xc0) == 0xc0 {
            if pos + 1 >= packet.len() {
                return Err("truncated DNS compression pointer".to_owned());
            }
            let ptr = usize::from(u16::from_be_bytes([len & 0x3f, packet[pos + 1]]));
            if ptr >= packet.len() {
                return Err("DNS compression pointer out of bounds".to_owned());
            }
            if next_offset.is_none() {
                next_offset = Some(pos + 2);
            }
            jumps += 1;
            if jumps > MAX_NAME_POINTER_JUMPS {
                return Err("DNS compression pointer loop detected".to_owned());
            }
            pos = ptr;
            continue;
        }
        if (len & 0xc0) != 0 {
            return Err("unsupported DNS label encoding".to_owned());
        }
        let label_len = usize::from(len);
        let label_start = pos + 1;
        let label_end = label_start + label_len;
        if label_end > packet.len() {
            return Err("DNS label exceeds packet boundary".to_owned());
        }
        labels.push(String::from_utf8_lossy(&packet[label_start..label_end]).into_owned());
        pos = label_end;
    }

    Ok((labels.join("."), next_offset.unwrap_or(pos)))
}

fn decode_rdata(packet: &[u8], rtype: u16, start: usize, end: usize) -> Result<String, String> {
    let slice = &packet[start..end];
    match rtype {
        TYPE_A if slice.len() == 4 => {
            Ok(Ipv4Addr::new(slice[0], slice[1], slice[2], slice[3]).to_string())
        }
        TYPE_AAAA if slice.len() == 16 => {
            let mut octets = [0u8; 16];
            octets.copy_from_slice(slice);
            Ok(Ipv6Addr::from(octets).to_string())
        }
        TYPE_NS | TYPE_CNAME | TYPE_PTR => {
            let (target, _) = read_name(packet, start)?;
            Ok(if target.is_empty() {
                ".".to_owned()
            } else {
                target
            })
        }
        TYPE_MX if slice.len() >= 3 => {
            let pref = u16::from_be_bytes([slice[0], slice[1]]);
            let (exchange, _) = read_name(packet, start + 2)?;
            Ok(format!("{pref} {exchange}"))
        }
        TYPE_TXT => {
            let mut pos = 0usize;
            let mut parts = Vec::new();
            while pos < slice.len() {
                let l = usize::from(slice[pos]);
                pos += 1;
                if pos + l > slice.len() {
                    return Err("truncated DNS TXT chunk".to_owned());
                }
                parts.push(String::from_utf8_lossy(&slice[pos..pos + l]).into_owned());
                pos += l;
            }
            Ok(parts.join(""))
        }
        TYPE_SOA => {
            let (mname, next1) = read_name(packet, start)?;
            let (rname, next2) = read_name(packet, next1)?;
            if next2 + 20 > end {
                return Err("truncated DNS SOA record".to_owned());
            }
            let tail = &packet[next2..end];
            let serial = u32::from_be_bytes([tail[0], tail[1], tail[2], tail[3]]);
            let refresh = u32::from_be_bytes([tail[4], tail[5], tail[6], tail[7]]);
            let retry = u32::from_be_bytes([tail[8], tail[9], tail[10], tail[11]]);
            let expire = u32::from_be_bytes([tail[12], tail[13], tail[14], tail[15]]);
            let minimum = u32::from_be_bytes([tail[16], tail[17], tail[18], tail[19]]);
            Ok(format!(
                "{mname} {rname} {serial} {refresh} {retry} {expire} {minimum}"
            ))
        }
        TYPE_SRV if slice.len() >= 7 => {
            let priority = u16::from_be_bytes([slice[0], slice[1]]);
            let weight = u16::from_be_bytes([slice[2], slice[3]]);
            let port = u16::from_be_bytes([slice[4], slice[5]]);
            let (target, _) = read_name(packet, start + 6)?;
            Ok(format!("{priority} {weight} {port} {target}"))
        }
        TYPE_CAA if slice.len() >= 2 => {
            let flags = slice[0];
            let tag_len = usize::from(slice[1]);
            if 2 + tag_len > slice.len() {
                return Err("truncated DNS CAA record".to_owned());
            }
            let tag = String::from_utf8_lossy(&slice[2..2 + tag_len]);
            let val = String::from_utf8_lossy(&slice[2 + tag_len..]);
            Ok(format!("{flags} {tag} \"{val}\""))
        }
        _ => Ok(format!("hex:{}", hex_encode(slice))),
    }
}

fn hex_encode(bytes: &[u8]) -> String {
    const HEX: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(bytes.len() * 2);
    for &b in bytes {
        out.push(HEX[(b >> 4) as usize] as char);
        out.push(HEX[(b & 0x0f) as usize] as char);
    }
    out
}

fn dns_error(domain: &str, record_type: &str, resolver: &str, message: String) -> DnsQueryResult {
    DnsQueryResult {
        domain: domain.to_owned(),
        record_type: record_type.to_owned(),
        resolver: resolver.to_owned(),
        rcode: "ERROR".to_owned(),
        authoritative: false,
        truncated: false,
        used_tcp: false,
        latency_ms: 0.0,
        records: Vec::new(),
        error: Some(message),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encodes_edns_ecs_option_with_masked_bits() {
        let ecs = parse_ecs_subnet("223.5.5.99/24").expect("ecs");
        assert_eq!(ecs.family, 1);
        assert_eq!(ecs.prefix_len, 24);
        assert_eq!(&ecs.addr_bytes[..3], &[223, 5, 5]);
        let pkt = build_dns_query(0x1234, "edge.openflare.cc", TYPE_A, Some(ecs)).expect("pkt");
        assert_eq!(u16::from_be_bytes([pkt[0], pkt[1]]), 0x1234);
        assert_eq!(u16::from_be_bytes([pkt[10], pkt[11]]), 1); // ARCOUNT=1
                                                               // Verify ECS option code 0x0008 and masked bytes [223, 5, 5] are present in OPT RDATA.
        assert!(pkt.windows(5).any(|w| w == [0x00, 0x08, 0x00, 0x07, 0x00]));
        assert!(pkt.windows(3).any(|w| w == [223, 5, 5]));
    }

    #[test]
    fn decodes_compression_pointers_and_guards_loops() {
        // Malformed self-pointing compression loop at offset 12: 0xc0, 0x0c
        let mut malformed = vec![0u8; 14];
        malformed[12] = 0xc0;
        malformed[13] = 0x0c;
        let err = read_name(&malformed, 12).unwrap_err();
        assert!(err.contains("loop"));

        // Valid DNS response carrying ANCOUNT=1 A record using compression pointer to question QNAME.
        let mut resp = Vec::new();
        resp.extend_from_slice(&0x4242u16.to_be_bytes());
        resp.extend_from_slice(&0x8400u16.to_be_bytes()); // response, AA=1, NOERROR
        resp.extend_from_slice(&1u16.to_be_bytes());
        resp.extend_from_slice(&1u16.to_be_bytes());
        resp.extend_from_slice(&0u16.to_be_bytes());
        resp.extend_from_slice(&0u16.to_be_bytes());
        encode_qname("www.example.com", &mut resp).unwrap();
        resp.extend_from_slice(&TYPE_A.to_be_bytes());
        resp.extend_from_slice(&1u16.to_be_bytes());
        // Answer pointing back to offset 12 (www.example.com)
        resp.extend_from_slice(&[0xc0, 12]);
        resp.extend_from_slice(&TYPE_A.to_be_bytes());
        resp.extend_from_slice(&1u16.to_be_bytes());
        resp.extend_from_slice(&120u32.to_be_bytes());
        resp.extend_from_slice(&4u16.to_be_bytes());
        resp.extend_from_slice(&[203, 0, 113, 10]);

        let parsed = parse_dns_response(0x4242, &resp, Instant::now(), false).expect("parse");
        assert!(parsed.authoritative);
        assert_eq!(parsed.rcode, "NOERROR");
        assert_eq!(parsed.records.len(), 1);
        assert_eq!(parsed.records[0].name, "www.example.com");
        assert_eq!(parsed.records[0].value, "203.0.113.10");
        assert_eq!(parsed.records[0].ttl, 120);
    }

    #[tokio::test]
    async fn udp_truncation_falls_back_to_tcp() {
        let udp = UdpSocket::bind("127.0.0.1:0").await.expect("bind udp");
        let tcp = tokio::net::TcpListener::bind(udp.local_addr().unwrap())
            .await
            .expect("bind tcp");
        let addr = udp.local_addr().unwrap();

        let server_task = tokio::spawn(async move {
            let mut req_buf = [0u8; 512];
            let (n, peer) = udp.recv_from(&mut req_buf).await.unwrap();
            assert!(n >= 12);
            let id = [req_buf[0], req_buf[1]];
            let mut tc_resp = vec![id[0], id[1], 0x83, 0x80, 0, 0, 0, 0, 0, 0, 0, 0];
            tc_resp[2] = 0x82; // response + TC=1
            udp.send_to(&tc_resp, peer).await.unwrap();

            let (mut stream, _) = tcp.accept().await.unwrap();
            let mut len_bytes = [0u8; 2];
            stream.read_exact(&mut len_bytes).await.unwrap();
            let qlen = usize::from(u16::from_be_bytes(len_bytes));
            let mut tcp_req = vec![0u8; qlen];
            stream.read_exact(&mut tcp_req).await.unwrap();

            let mut ok_resp = vec![tcp_req[0], tcp_req[1], 0x81, 0x80, 0, 0, 0, 1, 0, 0, 0, 0];
            encode_qname("openflare.cc", &mut ok_resp).unwrap();
            ok_resp.extend_from_slice(&TYPE_A.to_be_bytes());
            ok_resp.extend_from_slice(&1u16.to_be_bytes());
            ok_resp.extend_from_slice(&60u32.to_be_bytes());
            ok_resp.extend_from_slice(&4u16.to_be_bytes());
            ok_resp.extend_from_slice(&[198, 51, 100, 7]);
            let rlen = (ok_resp.len() as u16).to_be_bytes();
            stream.write_all(&rlen).await.unwrap();
            stream.write_all(&ok_resp).await.unwrap();
        });

        let result = execute_dns_query_with_policy(
            &DnsQueryRequest {
                domain: "openflare.cc".into(),
                record_type: "A".into(),
                resolver: addr.to_string(),
                ecs_subnet: Some("198.51.100.0/24".into()),
                timeout_ms: Some(3000),
            },
            false,
        )
        .await;
        server_task.await.unwrap();

        assert!(result.used_tcp);
        assert!(result.error.is_none());
        assert_eq!(result.records.len(), 1);
        assert_eq!(result.records[0].value, "198.51.100.7");
    }

    #[tokio::test]
    async fn ssrf_blocks_private_dns_resolver() {
        for resolver in ["127.0.0.1:53", "169.254.169.254:53", "100.64.0.1:53"] {
            let res = execute_dns_query(&DnsQueryRequest {
                domain: "example.com".into(),
                record_type: "A".into(),
                resolver: resolver.into(),
                ecs_subnet: None,
                timeout_ms: Some(500),
            })
            .await;
            assert!(
                res.error.as_deref().is_some_and(|e| e.contains("SSRF")),
                "expected SSRF error for resolver {resolver}, got {:?}",
                res.error
            );
        }
    }
}
