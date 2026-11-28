# 部署规划与基线

OpenFlare 采用控制面与数据面分离部署，组件间网络拓扑与流量路径见 [系统架构](../design/architecture.md)。

## 运行依赖

- **Redis / Valkey（必需）**：所有部署模式均须启用 Redis，用于 Asynq 异步任务队列、缓存失效与多副本状态广播。
- **主关系库**：测试与单机部署可使用嵌入式 SQLite；生产环境建议使用 PostgreSQL。
- **ClickHouse（可选）**：大流量访问日志与指标可写入 ClickHouse；未启用时自动回落至主关系库（详见 [日志存储解耦](../design/logstore.md)）。
- **会话密钥**：生产或公网部署前须设置长随机 `APP_SESSION_SECRET`（至少 32 字节以支持 MCP）。

## 端口规划矩阵

| 组件 | 端口与协议 | 方向与范围 | 用途说明 |
| --- | --- | --- | --- |
| OpenFlare Server | `3000/tcp` | 入站（管理端与各节点） | 控制台 UI、管理 API 与节点控制接口（可通过 `APP_ADDR` 修改） |
| PostgreSQL | `5432/tcp` | 仅 Server 内网访问 | 主关系型数据库 |
| Redis / Valkey | `6379/tcp` | 仅 Server 内网访问 | Asynq 队列与实时事件广播 |
| ClickHouse（可选） | `9000/tcp`、`8123/tcp` | 仅 Server 内网访问 | 分析日志与时序指标存储 |
| Jaeger（可选） | `16686/tcp`、`4317/tcp`、`4318/tcp` | `16686` 内网面板；`4317`/`4318` 供 Server 写入 | OpenTelemetry 链路追踪收集与查询 |
| OpenFlare Edge | `80/tcp`、`443/tcp` | 公网入站（容器内为 `8080`、`8443`） | HTTP 与 HTTPS/TLS 流量入口 |
| OpenFlare DNS | `53/udp`、`53/tcp` | 公网入站（容器内为 `5353`） | 权威 DNS 解析监听端口 |
| OpenFlare Relay | `7443/tcp`、`7444/tcp` | 公网入站（供 `openflared` 连接） | `rathole` Noise/TCP（`7443`）与 WebSocket（`7444`）隧道入口 |
| OpenFlare Relay | `8443/tcp` | Edge 节点及 `openflared` 可达 | Relay HTTP/WS Gateway 与 `/_openflare/health` 探测端口 |
| OpenFlare Relay（L4） | 按服务配置 | 公网入站 | 已分配的 L4 TCP/UDP 转发端口 |
| OpenFlared | 无入站端口 | 仅出网 | 连接 Server 与同池在线 Relay 节点 |

## 硬件配置基线

| 组件 | 入门推荐 | 生产环境推荐 | 说明 |
| --- | --- | --- | --- |
| OpenFlare Server | 1 核 CPU / 2 GB 内存 / 20 GB 磁盘 | 2 核 CPU / 4 GB 内存 / 50 GB+ 磁盘 | 磁盘按日志留存周期与数据库大小扩展 |
| OpenFlare Edge | 1 核 CPU / 512 MB 内存 / 2 GB 磁盘 | 2 核 CPU / 2 GB 内存 / 10 GB+ 磁盘 | 按并发连接、TLS 握手量、WAF 与磁盘缓存调整 |
| OpenFlare DNS | 1 核 CPU / 256 MB 内存 / 1 GB 磁盘 | 2 核 CPU / 512 MB 内存 / 5 GB 磁盘 | 内存主要用于快照、应答缓存与 GeoIP 库 |
| OpenFlare Relay | 1 核 CPU / 1 GB 内存 / 5 GB 磁盘 | 2 核 CPU / 2 GB 内存 / 20 GB 磁盘 | 吞吐主要受网络带宽与并发流数影响 |
| OpenFlared | 1 核 CPU / 256 MB 内存 / 1 GB 磁盘 | 1 核 CPU / 512 MB 内存 / 5 GB 磁盘 | 运行于内网主机，资源占用较低 |

## 组件部署入口

- 控制面编排与源码启动：[Server 部署](./server.md)
- 边缘代理网关：[部署 OpenFlare Edge](./openflare-edge.md)
- 独立权威 DNS：[部署 OpenFlare DNS](./openflare-dns.md)
- 隧道中继与连接器：[部署 OpenFlare Relay](./relay.md)、[部署 OpenFlared](./openflared.md)
- 版本更新与回滚：[升级与维护](./upgrade.md)
