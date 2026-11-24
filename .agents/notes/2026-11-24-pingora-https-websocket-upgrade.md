# Pingora HTTPS 回源 WebSocket 升级与长连接保活

1. **HTTPS 上游强制协商 `HTTP/1.1` 并隔离连接池**：Pingora 的 `proxy_h2` 路径不支持 HTTP/2 WebSocket（RFC 8441），且性能策略的 `upstream_alpn`（`h2h1`/`h2`）仅对 HTTPS 上游生效，导致开启 `h2h1`/`h2` 时 HTTPS 源站的 `Upgrade`/`Connection` 头被剥离而 HTTP 源站不受影响。`upstream.rs` 对 HTTP/1.1 `Upgrade: websocket` 请求强制设置 `peer.options.alpn = ALPN::H1`，并在 `peer.group_key` 置位 `1 << 63` 以隔离 `HttpPeer` 连接池哈希，避免降级普通流量的 ALPN 选择。
2. **豁免长连接读取超时与请求体累计上限**：WebSocket 升级前清空回源 `peer.options.read_timeout`；收到 `101 Switching Protocols` 后在 `filter_response` 清空下游 `read_timeout` 并提前返回（保留 PoW 会话 Cookie 注入）；`request_body_filter` 跳过 `session.was_upgraded()` 请求，避免累计上行流量达到 64 MiB 时触发 413 错误。
3. **保留源站 TLS 验证选项**：对 WebSocket `HttpPeer` 覆盖 `alpn` 与 `read_timeout` 时保留原有 `verify_cert`、`verify_hostname` 及写超时配置，确保自签 HTTPS 上游在关闭证书与主机名校验后可正常握手与收发数据。
