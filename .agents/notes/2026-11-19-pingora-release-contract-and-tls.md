# Pingora 独立发布契约与 TLS 热更新

## 背景

第一阶段 `openflare-edge` 只能消费 OpenResty 的 `source_config_json`，不能承载 TLS 证书资产，主控也没有独立 Pingora 发布组。第二阶段需要在同一业务快照下生成可校验的专用载荷，并让 Rust 节点以原子快照完成路由与证书热更新。

## 决策

1. 复用 `of_config_versions`，以 `engine = pingora` 与 `is_active` 形成独立发布组。`pingora_enabled` 默认关闭，关闭时发布流程不生成 Pingora 记录，OpenResty 现有行为保持不变。
2. 专用契约为 `schema_version=1`、`contract_version=1`，包含路由、域名证书引用、HTTPS/HTTP2/重定向语义和证书资产。`payload_json` 按原始 UTF-8 字节计算 SHA-256，节点先校验摘要再解析，证书在应用前预解析为 `X509/PKey`。
3. Rust 节点注册时声明 `engine=pingora` 与契约版本；心跳只接受 Pingora 发布组的元数据，并调用 `/api/v1/agent/pingora/config-versions/active` 拉取。应用日志回传引擎、发布序号和载荷摘要。
4. TLS 监听使用 Pingora `TlsSettings::with_callbacks`，回调按 SNI 从当前快照选择证书，并通过 `enable_h2()` 启用 H2/H1 ALPN；HTTP 监听保持 HTTP/1.1。证书与路由处在同一个 `RwLock<Option<AppliedConfig>>` 快照内，替换一次即整体生效，解析失败时保留旧快照。

## 已知边界

第二阶段只对齐路由、域名、上游、HTTPS、HTTP/2 与重定向语义。WAF、PoW、站点认证、限流、Pages 和缓存仍在后续阶段对照 OpenResty 逐项接入。
