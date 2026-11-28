# 部署架构与说明

OpenFlare 采用控制面与数据面解耦的分离架构。生产环境建议使用 PostgreSQL 作为 Server 主数据库，并通过配置文件或环境变量设置 `APP_SESSION_SECRET`。系统依赖 Redis 驱动 Asynq 异步任务引擎；针对海量访问日志与时序指标，可接入 ClickHouse 分析存储（判定与切换见 [日志存储解耦](../design/logstore.md)）。边缘网关（OpenFlare Edge）基于 OpenFlare Edge 构建，提供单机双进程的高可用代理能力。

## 网络拓扑与流量路径

### 1. 标准反向代理流量路径

控制面与数据面流量物理隔离：用户客户端访问直接到达 OpenFlare Edge 边缘网关，控制端仅负责规则下发与状态同步。

```text
[控制流 Control Plane]
OpenFlare Server (控制面，监听 :3000)
       ^
       | 定期拉取快照 / 心跳上报 / 日志回传 (HTTPS)
       v
OpenFlare Edge (数据面网关)

[数据流 Data Plane]
客户端浏览器 (Client / Browser)
       |
       v 请求到达 (TCP 80 / 443)
OpenFlare Edge (OpenFlare Edge: TLS 终结、WAF 规则过滤、本地缓存、Pages 托管)
       |
       v 反向代理回源 (HTTP / HTTPS)
源站服务器 (Origin Service)
```

### 2. 内网穿透流量路径

内网穿透架构由边缘网关、公网中继（Relay）与内网客户端（OpenFlared）协同工作：

```text
[控制流 Control Plane]
OpenFlare Server (控制面)
    ^                             ^
    | 配置同步与心跳               | 隧道凭证校验与心跳
    v                             v
Relay 节点 (frps 管理器)        OpenFlared (内网客户端)

[数据流 Data Plane]
客户端浏览器 (Client / Browser)
    |
    v 公网访问 (TCP 80 / 443)
OpenFlare Edge (OpenFlare Edge: 域名路由、WAF 拦截、HTTPS 证书卸载)
    |
    v 本地回环转发 (127.0.0.1:{vhostHTTPPort})
OpenFlare Relay (frps 守护进程，监听公网中继端口 :7000)
    |
    | 加密 FRP 隧道 (主动出网建立)
    v
OpenFlared 客户端 (frpc 守护进程，部署于内网机器)
    |
    v 局域网转发 (内网 IP / 端口)
内网业务服务 (Internal Service: 192.168.x.x)
```

## 硬件配置基线

| 组件 | 入门推荐 | 生产环境推荐 | 说明 |
| --- | --- | --- | --- |
| **Server 控制面** | 1 核 CPU / 2 GB 内存 / 20 GB 磁盘 | 2 核 CPU / 4 GB 内存 / 50 GB+ 磁盘 | 磁盘容量根据访问日志留存策略与指标数据量扩容 |
| **Edge 边缘网关** | 1 核 CPU / 512 MB 内存 / 2 GB 磁盘 | 2 核 CPU / 2 GB 内存 / 10 GB+ 磁盘 | 根据 OpenFlare Edge 并发代理连接量、TLS 握手压力与 WAF 吞吐扩容 |
| **OpenFlare DNS (SmartDNS)** | 1 核 CPU / 256 MB 内存 / 1 GB 磁盘 | 2 核 CPU / 512 MB 内存 / 5 GB 磁盘 | 轻量级独立权威解析节点，支持离线快照容灾与多核监听 |
| **Relay 中继节点** | 1 核 CPU / 1 GB 内存 / 5 GB 磁盘 | 2 核 CPU / 2 GB 内存 / 20 GB 磁盘 | frps 传输中继吞吐受网络带宽与并发 TCP 连接上限影响 |
| **OpenFlared 客户端** | 1 核 CPU / 256 MB 内存 / 1 GB 磁盘 | 1 核 CPU / 512 MB 内存 / 5 GB 磁盘 | 运行于内网环境，资源占用低，主要依赖内网主机网络吞吐 |

## 快速部署参考

### Docker Compose 部署 Server 控制面

仓库根目录提供包含 PostgreSQL、Redis、ClickHouse 与 Jaeger 的标准编排文件 `docker-compose.yaml`：

```bash
curl -o .env.example https://raw.githubusercontent.com/Rain-kl/OpenFlare/refs/heads/main/.env.example
cp .env.example .env
# 编辑 .env 文件，设置强随机密钥 APP_SESSION_SECRET 与数据库密码
docker compose up -d
docker compose ps
docker compose logs -f openflare
```

启动完成后，使用浏览器访问 `http://localhost:3000`。初始管理员账户为 `admin` / `12345678`。登录后请立即进入系统设置修改初始密码。详细部署说明见 [Server 部署](./server.md)。

从仓库源码构建 Server、Relay 或 OpenFlared 镜像时，Docker 会使用根目录 `go.mod` 指定的 Go 版本。使用 `make cross-build` 构建跨平台二进制文件时也遵循该版本。

### 源码编译启动 Server

如需从源码编译运行：

```bash
# 1. 编译嵌入式管理端前端静态资源
cd frontend
corepack enable
pnpm install
pnpm build:embed
cd ..

# 2. 准备配置文件并启动服务
cp config.example.yaml config.yaml
export APP_SESSION_SECRET='replace-with-a-long-random-string'
go run ./cmd/server all
```

服务默认监听 `:3000` 端口（通过 `config.yaml` 中的 `app.addr` 或环境变量 `APP_ADDR` 配置）。

### 部署边缘节点、SmartDNS 与隧道

- 边缘网关部署：参考 [OpenFlare Edge 部署指南](./openflare-edge.md)（技术规格、Linux systemd 权限加固与平滑升级）或 [边缘节点快速接入](./agent.md)（节点令牌与快速安装命令）。
- 智能权威 DNS 部署：参考 [OpenFlare SmartDNS 部署](./openflare-dns.md)（Linux 一键安装、Docker 部署与控制台远程更新）。
- 隧道基础设施部署：参考 [Relay 部署](./relay.md) 与 [OpenFlared 部署](./openflared.md)。
