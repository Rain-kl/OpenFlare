# Pingora HTTPS 回源名称

普通代理路由使用 HTTPS IP 上游且 `origin_host` 留空时，`select_upstream` 使用已匹配路由的请求域名作为 `HttpPeer` 的 TLS 名称。连接仍使用上游配置中的 IP 和端口，HTTP `Host` 沿用现有透传规则。显式 `origin_host` 优先，域名上游留空时仍使用上游域名作为 TLS 名称。

启用源站证书和名称校验时，TLS 源站须提供覆盖访问域名的受信任证书。此前依赖默认 TLS 虚拟主机的 IP 上游会开始发送域名 SNI，可能选中不同的源站虚拟主机。Pingora 的连接池按 SNI 区分连接。

现有边缘运行时冒烟测试使用受信任的本地 TLS 源站，验证同一通配符路由的两个具体子域名，以及显式 Host 和域名上游的兼容行为。
