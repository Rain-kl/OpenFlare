# 系统架构

OpenFlare 由 Server 控制面、OpenFlare Edge 数据面、OpenFlare DNS 权威节点、原生 Rust 穿透组件（`openflare-relay` 与 `openflared`）及管理端前端组成。边缘节点同步与运行细节见 [OpenFlare Edge 架构设计](./pingora-edge-design.md)。

## 流量路径概览

OpenFlare 支持四类核心数据面流量路径：

### 1. 标准反向代理路径

```text
Browser
  | HTTPS / HTTP
  v
OpenFlare Edge (TLS / WAF / PoW / Rate Limit / EdgeCache / 可选源站错误页)
  | HTTP / HTTPS
  v
Origin Server (公网或局域网直连上游)
```

### 2. L7 内网穿透路径（Edge + Relay Gateway）

```text
Browser
  | HTTPS / HTTP / WS
  v
OpenFlare Edge (tunnel-routing.v1, TLS / WAF / PoW)
  | HTTP/WS (X-OpenFlare-Gateway-Auth, 就近选择健康中继)
  v
openflare-relay Gateway (:8443)
  | 内嵌 rathole server (Noise :7443 / WebSocket :7444 / TCP)
  v
openflared (内嵌 rathole client + CIDR SSRF 校验)
  |
  v
Internal Service (内网 HTTP / HTTPS / WS 服务)
```

### 3. L4 TCP/UDP 内网穿透路径

```text
TCP / UDP Client
  |
  v
openflare-relay (公开监听的 L4 TCP/UDP 服务端口)
  | 内嵌 rathole 加密长连接 (Noise / WebSocket / TCP)
  v
openflared (CIDR SSRF 校验)
  |
  v
Internal TCP / UDP Target
```

### 4. Pages 静态托管路径

```text
Browser
  | HTTPS / HTTP
  v
OpenFlare Edge (TLS / WAF / PoW / 站点认证)
  |
  +---> [静态文件] 本地解压目录 (ETag / Range / SPA Fallback)
  |
  +---> [API 反代] 命中 api_proxy_path ---> 后端 API 服务
```

## 组件职责

| 组件 | 核心职责 | 设计参考 |
| --- | --- | --- |
| **OpenFlare Server** | 控制台 UI/API、状态持久化、站点与隧道清单编译同步、DNS 账号与多云指向调度、SmartDNS 快照与网络质量矩阵、Pages 存储、访问日志聚合、Uptime Kuma 同步与登录验证码 | [OpenFlare Edge 架构设计](./pingora-edge-design.md) / [SmartDNS 系统架构设计](./smartdns-design.md) / [多云 DNS 指向与故障切换设计](./cloudflare-pointing.md) / [边缘可观测性与流量统计架构](./observability-design.md) / [Uptime Kuma 监控同步设计](./kuma-design.md) / [登录验证码设计](./login-captcha.md) |
| **OpenFlare Edge** | 承载代理与安全管线：TLS 终结、WAF DAG 规则与 CRS 检测、PoW 挑战、站点认证、TinyUFO 缓存、Pages 服务、源站错误页与主动探测 | [OpenFlare Edge 架构设计](./pingora-edge-design.md) / [WAF 可编排规则设计](./waf-orchestration-design.md) / [PoW 防护设计](./pow-design.md) / [边缘缓存策略设计](./edge-cache-design.md) / [Pages 静态托管设计](./pages-design.md) / [源站错误页设计](./origin-error-page.md) |
| **OpenFlare DNS** | 基于 `hickory-server` 与 `zen-engine` 的独立权威解析节点，支持 ECS 感知、RRL 限流、离线快照与网络质量加权选点 | [SmartDNS 系统架构设计](./smartdns-design.md) / [网络情报与链路质量感知架构](./network-quality-design.md) |
| **OpenFlare Relay** | 公网中继进程，内嵌 `rathole` 服务端与 Relay Gateway（`:8443`），处理 L4/L7 转发、同机共置、排水迁移与热更新 | [内网穿透隧道设计](./tunnel-design.md) |
| **OpenFlared** | 内网连接器，执行多中继 RTT 探测、Closest-K 可用区调度、Make-Before-Break 切换与 CIDR SSRF 防护 | [内网穿透隧道设计](./tunnel-design.md) |

## 控制面与数据面交互

### 1. 配置编译与同步

```text
控制台保存配置 -> 编译 sites.v1 / tunnel-routing.v1 / dns snapshot
                       |
     +-----------------+-----------------+
     | (WebSocket 唤醒或周期心跳)           |
     v                                   v
[OpenFlare Edge]                  [OpenFlare DNS / Relay / OpenFlared]
按 SHA-256 拉取增量清单             拉取最新权威快照与隧道拓扑
校验并原子切换内存态配置             进程内热更新解析引擎与 rathole 隧道
```

### 2. 核心持久化表

| 领域 | 数据表 |
| --- | --- |
| **站点与路由** | `of_zones`、`of_zone_domains`、`of_proxy_routes`、`of_origins`、`of_config_versions`、`of_edge_snapshot_state`、`of_tls_certificates` |
| **多云 DNS 指向** | `of_dns_accounts`、`of_cf_connections`、`of_cf_pointing_groups`、`of_cf_pointing_members` |
| **SmartDNS 与网络质量** | `of_dns_instances`、`of_dns_policies`、`of_network_quality_matrix`、`of_as_topology_edges`、`of_traceroute_records`、`of_probe_targets`、`of_probe_target_states` |
| **Pages 静态托管** | `of_pages_projects`、`of_pages_project_sources`、`of_pages_project_source_runtime`、`of_pages_deployments`、`of_pages_deployment_files` |
| **节点与穿透** | `of_nodes`、`of_node_tags`、`of_node_tag_bindings`、`of_node_system_profiles`、`of_node_cert_deliveries`、`of_apply_logs`、`of_tunnel_relay_pools`、`of_tunnel_relays`、`of_tunnels`、`of_tunnel_connectors`、`of_tunnel_attachments`、`of_tunnel_services` |
| **WAF 与安全** | `of_waf_rule_groups`、`of_waf_ip_groups`、`of_waf_rule_group_bindings`、`of_waf_request_whitelist`、`of_attack_groups`、`of_pow_policies` |
| **观测与日志库 (`logstore`)** | `of_node_access_logs`、`of_node_metric_snapshots`、`of_node_ping_records`、`of_network_quality_samples`、`of_dns_metric_snapshots`、`w_user_access_logs` |

## 关键设计决策

| 决策 | 原因 |
| --- | --- |
| 完整版本化快照 | 为摘要校验、内存原子切换与本地冷启动恢复提供一致边界 |
| 节点主动拉取 | 控制面无需节点 SSH 权限，通过 HTTP 与 WebSocket 协同保持最终一致 |
| Zone 域名与路由分离 | Zone 管理根域与证书归属，多域名可共用同一站点路由策略 |
| 多云 DNS 账号统一适配 | `of_dns_accounts` 统一承接 ACME DNS-01、父区委派与多厂商指向记录同步 |
| 内嵌 `rathole` 隧道 | 消除外部进程包装，统一支持 Noise/WS/TCP 回退、Closest-K 容灾与 CIDR 防护 |
| 运行时与控制库解耦 | 边缘与 DNS 节点均在内存运行快照，控制面故障时数据面不中断 |
| 访问日志为流量唯一真相 | 禁止边缘预聚合流量，所有看板与 Zone 统计在 Server 统一查询聚合 |
