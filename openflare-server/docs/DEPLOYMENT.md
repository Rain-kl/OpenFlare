# wavelet 部署指南

本文档详细介绍了 **wavelet** 脚手架系统在不同业务阶段的部署方案，涵盖从**最小化单机部署**到**最大化高可用分布式部署**的全生命周期架构。

---

## 一、 系统组件概览

在部署系统前，请了解各运行组件及其角色：

| 组件名称 | 运行命令/形式 | 职责说明 | 必选/可选 |
| :--- | :--- | :--- | :--- |
| **HTTP API 服务** | `bin/wavelet api` | 接收并处理前端及第三方的 RESTful API 请求 | **必选** |
| **异步任务工作进程** | `bin/wavelet worker` | 消费并处理异步队列任务（如邮件发送、清理上传文件等） | **必选** |
| **定时任务调度器** | `bin/wavelet scheduler` | 定时向 Redis 队列下发 Cron 任务（仅负责触发，不负责执行） | **必选** |
| **前端服务 (Node.js)** | `pnpm start` | 提供 React/Next.js 页面服务（在分离部署时使用） | 分离模式必选 |
| **PostgreSQL** | 关系型主数据库 | 存储用户、系统配置、认证源、任务执行记录等核心数据 | **必选** |
| **Redis** | 缓存与消息队列中间件 | 存储 Session 会话、临时缓存以及 Asynq 异步任务队列数据 | **必选** |
| **ClickHouse** | 分析型数据库 | 存储历史数据同步或进行高性能分析 | 可选 |
| **对象存储 (S3)** | 兼容 S3 的云存储/私有云 | 存放用户上传的静态文件、图片等 | 可选 |

---

## 二、 部署配置准备

系统在启动前会从当前目录加载 `config.yaml` 配置文件。
生产环境部署前，请复制 `config.example.yaml` 为 `config.yaml`，并至少确认以下关键参数的配置：

```yaml
app:
  env: "production"                      # 生产环境标识
  addr: ":8000"                          # API 服务监听端口
  session_secret: "prod-random-secret"   # 极其重要的加密密钥，首发启动后不可更改
  session_domain: ".yourdomain.com"      # 跨域共享 Session 时需配置

database:
  host: "db.yourdomain.com"
  port: 5432
  username: "postgres"
  password: "YOUR_DB_PASSWORD"
  database: "refreshing"

redis:
  addrs:
    - "redis.yourdomain.com:6379"
  password: "YOUR_REDIS_PASSWORD"
```

---

## 三、 方案一：最小部署 — 单机嵌入式极简版 (推荐)

此部署方案将**前端静态网页全部直接打入 Go 后端二进制文件中**，极大地简化了部署运维，是中小型应用、内部系统、SaaS 早期阶段的首选。

### 📊 架构设计
- **服务载体**：单台云服务器 (1核2G 即可)。
- **依赖服务**：在一台机器上启动轻量级 PostgreSQL 与 Redis（可采用 Docker 部署）。
- **进程管理**：在一台机器上直接拉起打包好的 Go 单文件，并分别运行 `api`、`worker`、`scheduler` 进程。
- **前端托管**：Go 服务直接在 8000 端口承载前端的所有页面，不需要额外配置 Node.js 生产服务器。

### 🛠️ 步骤说明

#### 1. 单机依赖服务初始化 (使用 Docker Compose)
在机器上准备以下 `docker-compose.yml` 快速启动 PostgreSQL 和 Redis：
```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15-alpine
    container_name: refreshing-db
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: YOUR_DB_PASSWORD
      POSTGRES_DB: refreshing
    ports:
      - "5432:5432"
    volumes:
      - ./data/pg:/var/lib/postgresql/data
    restart: always

  redis:
    image: valkey/valkey:8.0-alpine
    container_name: refreshing-redis
    command: valkey-server --requirepass YOUR_REDIS_PASSWORD
    ports:
      - "6379:6379"
    volumes:
      - ./data/redis:/data
    restart: always
```
执行命令启动：
```bash
docker compose up -d
```

#### 2. 前后端一键嵌入式打包
在开发或编译机上，运行编译指令：
```bash
make build-embedded
```
该命令会自动完成前端的静态编译导出 (`frontend/out`)、复制到 Go 后端目录，最后使用 `-tags embed_frontend` 生成后端单文件：
- 产物路径：`bin/wavelet`

#### 3. 进程管理 (使用 Systemd)
将 `bin/wavelet` 拷贝到生产服务器 `/usr/local/bin/wavelet`，并为 `api`、`worker` 和 `scheduler` 配置 Systemd 管理服务。

新建 API 进程服务文件 `/etc/systemd/system/wavelet-api.service`：
```ini
[Unit]
Description=Refreshing API Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/app
ExecStart=/usr/local/bin/wavelet api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```
同理，新建 Worker 服务 `/etc/systemd/system/wavelet-worker.service`（将命令改为 `wavelet worker`），以及 Scheduler 服务 `/etc/systemd/system/wavelet-scheduler.service`（将命令改为 `wavelet scheduler`）。

启动并启用所有服务：
```bash
systemctl daemon-reload
systemctl enable --now refreshing-api refreshing-worker refreshing-scheduler
```

#### 4. 配置 Nginx 证书
配置 Nginx 作为反向代理并启用 HTTPS 证书：
```nginx
server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate /path/to/cert.crt;
    ssl_certificate_key /path/to/cert.key;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## 四、 方案二：标准部署 — 前后端物理分离架构

此方案中前端与后端彻底解耦。前端采用 SSR/ISR (Next.js Node 服务) 运行，后端采用独立的 API 服务运行。

### 📊 架构设计
- **前端部署**：单独部署到 Node.js 托管环境（如多台前端机器或 Vercel/Cloudflare Pages）。
- **后端部署**：多台后端云服务器，统一指向云数据库 RDS 与云缓存 Redis。
- **通信方式**：前后端通过 Nginx 规则路由或独立域名（如 `app.yourdomain.com` 访问前端，`api.yourdomain.com` 访问后端）进行跨域通信。

### 🛠️ 步骤说明

#### 1. 部署后端 Go 服务
1. 编译后端：
   ```bash
   go build -o bin/wavelet main.go
   ```
2. 在后端服务器上，同样使用 Systemd 或 Docker 守护启动 `wavelet api`、`wavelet worker` 和 `wavelet scheduler`。
3. 配置后端 Nginx 将客户端 API 请求（如 `/api/...`）反向代理至后端绑定的端口（如 `:8000`）。

#### 2. 部署前端 Next.js 服务
1. 前端服务器环境确保已安装 Node.js 和 pnpm。
2. 安装依赖并编译生产版本：
   ```bash
   cd frontend
   pnpm install
   pnpm build
   ```
3. 使用 PM2 守护前端 Node.js 服务运行。新建 `ecosystem.config.js`：
   ```javascript
   module.exports = {
     apps: [
       {
         name: 'refreshing-frontend',
         script: 'node_modules/next/dist/bin/next',
         args: 'start -p 3000',
         instances: 'max',
         exec_mode: 'cluster',
         env: {
           NODE_ENV: 'production',
           WAVELET_BACKEND_URL: 'https://api.yourdomain.com'
         }
       }
     ]
   };
   ```
   启动前端服务：
   ```bash
   pm2 start ecosystem.config.js
   ```

#### 3. 跨域与 Cookie 说明
- 若前后端使用**不同子域名**部署（例如 `app.yourdomain.com` 和 `api.yourdomain.com`），必须在 `config.yaml` 中将 `app.session_domain` 显式设置为顶级域名（`.yourdomain.com`），以确保 Session Cookie 可以在子域间顺利透传。
- 在跨域状态下，前端请求必须配置 `withCredentials: true`，API 端的跨域中间件（`corsMiddleware`）会自动将该域添加至允许源中。

---

## 五、 方案三：最大部署 — 企业级高可用分布式架构 (Max)

当系统面临高并发流量、海量后台任务或极高的可用性要求时，需要将所有组件拆分为无状态水平扩容，并引入高可用的云基础设施。

### 📊 架构设计图
```
                        ┌────────────────────────┐
                        │    域名 / 负载均衡器    │
                        │     (SLB / Cloudflare) │
                        └──────────┬─────────────┘
                                   │
                ┌──────────────────┴──────────────────┐
                ▼                                     ▼
     ┌─────────────────────┐               ┌─────────────────────┐
     │   前端集群          │               │   后端 API 集群     │
     │  (Next.js Node)     │               │   (Go 无状态实例)    │
     │  [弹性扩容 / 8台+]   │               │   [弹性扩容 / 8台+]  │
     └─────────────────────┘               └──────────┬──────────┘
                                                      │
             ┌────────────────────────────────────────┼────────────────────────────────────────┐
             ▼                                        ▼                                        ▼
   ┌───────────────────┐                    ┌───────────────────┐                    ┌───────────────────┐
   │  异步 Worker 集群 │                    │  定时 Scheduler   │                    │  S3 对象存储集群  │
   │  (多节点并发处理)  │                    │ (主备模式，限单节点)│                    │(R2/MinIO/AWS S3)  │
   └─────────┬─────────┘                    └─────────┬─────────┘                    └───────────────────┘
             │                                        │
             └───────────────────┬────────────────────┘
                                 │
             ┌───────────────────┴────────────────────┐
             ▼                                        ▼
   ┌───────────────────────────────────┐    ┌───────────────────────────────────┐
   │          Redis 哨兵/集群           │    │       PG 主从读写分离集群          │
   │      (高可用缓存/Asynq 队列)       │    │       (RDS Primary-Replica)       │
   └───────────────────────────────────┘    └───────────────────────────────────┘
```

### ⚙️ 最大部署配置要点

#### 1. 数据库高可用 (主从读写分离)
在 `config.yaml` 中配置 `database` 的主库写与从库读：
```yaml
database:
  enabled: true
  host: "pg-primary.yourdomain.com" # 主库地址（写）
  port: 5432
  username: "postgres"
  password: "YOUR_DB_PASSWORD"
  database: "refreshing"
  # 配置读写分离只读副本（GORM 自动轮询读，支持配置多个从库）
  replicas:
    - host: "pg-replica-1.yourdomain.com"
      port: 5432
      username: "postgres"
      password: "YOUR_DB_PASSWORD"
    - host: "pg-replica-2.yourdomain.com"
      port: 5432
      username: "postgres"
      password: "YOUR_DB_PASSWORD"
```

#### 2. Redis 高可用 (哨兵/Sentinel 或集群)
- **Sentinel 哨兵模式**：通过配置 `redis.master_name` 启用，SDK 会自动监视 Master 的主备切换。
- **Cluster 集群模式**：将 `redis.cluster_mode` 设为 `true`，并提供所有集群节点的 `addrs`。
```yaml
redis:
  addrs:
    - "redis-node-1.yourdomain.com:6379"
    - "redis-node-2.yourdomain.com:6379"
    - "redis-node-3.yourdomain.com:6379"
  cluster_mode: true
```

#### 3. 对象存储与缓存分离 (S3 + Local Cache)
高可用集群下，本地文件系统不再可共享。文件存储必须启用 S3 兼容服务，并在多节点间开启本地高速磁盘缓存加速读取：
```yaml
s3:
  enabled: true
  endpoint: "https://your-r2-or-s3-id.r2.cloudflarestorage.com"
  region: "auto"
  bucket: "refreshing-assets"
  access_key_id: "YOUR_S3_KEY"
  secret_access_key: "YOUR_S3_SECRET"
  local_cache:
    enabled: true                     # 开启本地磁盘缓存
    cache_dir: "/data/s3_cache"       # 本地高性能 SSD 挂载点
```

#### 4. 后端进程横向拆分部署
- **API 集群**：启动数十个甚至上百个 `wavelet api` 无状态容器。它们可以通过负载均衡器直接挂载，支持随时弹性缩容扩容。
- **Worker 集群**：启动多个 `wavelet worker` 容器。因为 `Asynq` 基于 Redis 分布式处理，多个 Worker 进程可以安全地同时运行并竞抢同一队列的异步任务，自动保障任务的并发吞吐能力。
- **Scheduler 独占**：**【注意】** 为避免重复触发定时 Cron 任务，`wavelet scheduler` 定时调度器进程**同一时间应仅运行单个活跃实例**（主备高可用可以通过容器平台的单实例保障或 K8s Job 机制来限制实例数为 1）。

#### 5. ClickHouse 高并发同步
在大数据量、高频支付结算场景下，开启 ClickHouse 以接收系统的历史数据同步，通过定时器把 PostgreSQL 的压力转移到 ClickHouse 列式存储中。
```yaml
clickhouse:
  enabled: true
  hosts:
    - "ch-node-1.yourdomain.com:9000"
    - "ch-node-2.yourdomain.com:9000"
```

#### 6. OpenTelemetry 分布式链路追踪
最大部署架构必须引入链路追踪（Jaeger 或 OTel Collector）以便排查节点间请求延迟或网络问题。
在生产环境，通过配置 OTel 将 Span 发送至公共日志分析平台。
```yaml
otel:
  sampling_rate: 0.05    # 开启 5% 的流量追踪采样率以减少开销
```

---

## 六、 部署方案对比与选择建议

| 指标维度 | 方案一：最小单机嵌入版 | 方案二：标准前后端分离版 | 方案三：最大高可用分布式版 |
| :--- | :--- | :--- | :--- |
| **支持流量/并发** | 1,000 ~ 5,000 QPS (视机器性能) | 5,000 ~ 20,000 QPS | 20,000 ~ 100,000+ QPS (无限扩展) |
| **服务器数量** | 1 台 | 3 ~ 5 台 | 10 台以上集群 |
| **运维复杂度** | 极简 (只需部署一个程序) | 中等 (需维护 Node 和 Go 两套环境) | 较高 (K8s/多组件集群维护) |
| **适合场景** | 个人项目、内部系统、SaaS 早期起步 | 正常线上运营项目、有中等规模团队 | 大型企业级应用、高并发核心交易系统 |
