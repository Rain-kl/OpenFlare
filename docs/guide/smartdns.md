# SmartDNS 智能解析

通过权威委派、区域拓扑与节点实时状态实现智能 `A`/`AAAA` 调度，支持内嵌或独立实例部署。

## 1. 部署与启用

- **内嵌模式（轻量场景）**：在 Server 配置中设置 `smartdns.enabled: true`（默认监听 `:53`）并开放宿主机 `53/UDP` 与 `53/TCP`。
- **独立模式（生产推荐）**：按 [部署 SmartDNS](../deployment/openflare-dns.md) 在专用节点部署 `openflare-dns` 并通过 Discovery Token 自动注册。

> Linux 若启用 `systemd-resolved` 占用 `53` 端口，需停用其 Stub Listener 后再启动 SmartDNS。

## 2. 控制台配置

1. 打开 **DNS 管理** > **SmartDNS** > **实例状态**，确认至少一个实例状态为 **在线**。
2. 在 **解析策略** 选择预设策略（**智能分配策略** / **容量负载均衡策略**）或新建自定义策略。
3. 在 **权威域管理** 添加全局委派主域名（如 `edge.example.com`）与 NS 主机名（如 `ns1.dns-infra.example`），调整默认 TTL（默认 `60s`）并启用。
4. 在 **代理路由** 将站点指向切换为 **SmartDNS**，或在 **全局记录** 统一绑定域名解析策略。

## 3. 注册商委派与硬性约束

| 约束项 | 说明 |
| :--- | :--- |
| NS 主机名位置 | 委派记录中的 NS 主机名必须在被委派子域之外；若使用主域自身 NS，需配置 Glue 记录 |
| 站点绑定方式 | 将站点域名通过 **仅 DNS**（关闭上游代理）的 `CNAME` 指向被委派子域（如 `www.example.com -> www.edge.example.com`） |
| 传输协议 | SmartDNS 实例与防火墙必须同时放行 `53/UDP` 与 `53/TCP` |
| Apex 根域名 | 标准 DNS 不允许根域直接设置 `CNAME`，需依赖注册商 ALIAS/ANAME/CNAME 展平或转用支持厂商的直接托管 |
| 启用顺序 | 先确认实例在线并通过本地解析验证，再向注册商添加 `NS` 委派记录 |

## 4. 策略编排变量（可选）

在 **解析策略** DAG 中可结合客户端与请求属性定义过滤、权重（`latency`、`load`、`geo`）及标签偏好（`use_tag_weights`）：

| 变量 | 说明 | 示例 |
| :--- | :--- | :--- |
| `client.country` / `client.region` | 客户端国家（ISO 2 位）与省份/区域 | `"CN"`、`"California"` |
| `client.asn` | 客户端自治系统号 | `4134` |
| `client.ip_family` | 请求来源 IP 协议族 | `"v4"` / `"v6"` |
| `domain` / `qtype` | 查询域名与记录类型 | `"www.example.com"`、`"A"` |

## 5. 验证与排错

```sh
dig @<SMARTDNS_IP> www.edge.example.com A +short
dig +trace www.edge.example.com A
```

| 现象 | 排查方向 |
| :--- | :--- |
| 解析超时 / `SERVFAIL` | 确认 `openflare-dns` 进程在线且防火墙同时放行 `53/UDP` 与 `53/TCP` |
| 返回 `NXDOMAIN` | 检查委派域名是否已启用，以及站点路由或全局记录是否已绑定到 SmartDNS |
| 命中节点与预期不符 | 确认客户端公网 IP 具备 GeoIP/ASN 信息，并检查节点状态或策略过滤条件是否排除了目标节点 |
| UDP 正常但 TCP 失败 | 确认云安全组与宿主机防火墙允许 `53/TCP`，供大响应包与 `+trace` 降级回退 |

网络探测与节点品质来源参见 [网络质量与探测工具](./network-tools.md)，内部架构参见 [SmartDNS 架构设计](../design/smartdns-design.md)。
