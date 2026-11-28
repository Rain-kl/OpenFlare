# 命令与脚本

`openflare-server`、`openflare` CLI、`openflare-edge`、`openflare-dns`、`openflare-relay`、`openflared`、控制台前端、Swagger 与文档站的命令行参数、构建与质量检查指令。

> 除非另有说明，所有构建与校验命令均在**代码仓库根目录**执行。

---

## 控制面服务 (`openflare-server`)

### 1. 命令行子命令与参数

编译产物位于 `bin/openflare-server`（亦可直接通过 `go run ./cmd/server` 运行）：

```bash
# 查看版本信息
./bin/openflare-server --version

# 单进程融合模式（默认启动 API + Worker + Scheduler）
./bin/openflare-server all

# 分离进程启动
./bin/openflare-server api          # 仅启动控制面 HTTP/WS API
./bin/openflare-server worker       # 仅启动 Asynq 异步任务 Worker
./bin/openflare-server scheduler    # 仅启动 Cron 定时调度器

# 重置指定账号密码
./bin/openflare-server reset-passwd --user admin --password "NewPassword123!"

# 预检并在单事务中重加密库内凭证（用于轮换 app.session_secret）
./bin/openflare-server rotate-credentials --new-secret-file ~/.openflare-new-session-secret
./bin/openflare-server rotate-credentials --new-secret-file ~/.openflare-new-session-secret --apply
```

`reset-passwd` 参数：
- `--user <string>`：待重置用户名；省略时交互输入。
- `--password <string>`：新密码；省略时自动生成 16 位强随机密码输出。

指定自定义配置文件路径：

```bash
CONFIG_PATH=/etc/openflare/config.yaml ./bin/openflare-server all
```

### 2. 构建与测试指令

```bash
# 仅编译后端服务（产物：bin/openflare-server）
make build-backend

# 编译嵌入前端静态资源的单文件二进制
make build-embedded

# 运行后端单元测试与集成测试
GOCACHE=/tmp/openflare-go-cache go test ./...
```

---

## 运维 CLI (`openflare`)

面向脚本与运维自动化的 CLI 工具（`cmd/openflare`），支持管理站点、域名区、证书、WAF 规则与探测任务，使用方式详见 [CLI 与 MCP 自动化](../guide/automation.md)。

```bash
# 编译运维 CLI 二进制
go build -o bin/openflare ./cmd/openflare

# 登录控制面并查看支持的命令与操作清单
openflare auth login --server https://openflare.example.com
openflare connection info
openflare operations list
```

---

## 边缘网关 (`openflare-edge`)

OpenFlare Edge 基于 Rust 与 Cloudflare Pingora 框架实现，二进制名称为 `openflare-edge`。

### 1. 子命令与启动参数

```bash
# 运行边缘代理服务
openflare-edge run \
  --server-url "https://panel.example.com" \
  --token-type "node" \
  --token-file "/etc/openflare-edge/node.token" \
  --state-dir "/var/lib/openflare-edge" \
  --http-listen "0.0.0.0:80" \
  --tls-listen "0.0.0.0:443"

# 升级模式启动（要求本地现有有效快照恢复后运行）
openflare-edge run --upgrade \
  --server-url "https://panel.example.com" \
  --token-type "node" \
  --token-file "/etc/openflare-edge/node.token" \
  --state-dir "/var/lib/openflare-edge" \
  --http-listen "0.0.0.0:80" \
  --tls-listen "0.0.0.0:443"

# 查询协议版本、WAF 规则目录，以及运行网络探测隔离 Worker
openflare-edge config-sync-version
openflare-edge self-update-version
openflare-edge attack-rule-catalog
openflare-edge probe-worker --idle-timeout-secs 300
```

`openflare-edge run` 主要参数：

| 参数名称 | 类型 | 是否必填 | 默认值 | 作用说明 |
| --- | --- | --- | --- | --- |
| `--server-url` | `string` | 是 | 无 | OpenFlare 控制面访问地址 |
| `--token-type` | `string` | 否 | `discovery` | 认证令牌类型：`discovery`（首次注册）或 `node`（节点专属凭据） |
| `--token-file` | `path` | 是 | 无 | 认证令牌本地文件路径 |
| `--state-dir` | `path` | 是 | 无 | 持久化状态与快照目录（如 `/var/lib/openflare-edge`） |
| `--http-listen` | `string` | 否 | `127.0.0.1:8080` | HTTP 监听地址（systemd 安装默认为 `0.0.0.0:80`） |
| `--tls-listen` | `string` | 否 | 空 | HTTPS/TLS 监听地址（systemd 安装默认为 `0.0.0.0:443`） |
| `--upgrade` | `bool` | 否 | `false` | 升级模式，存在快照时强制校验并恢复配置 |

### 2. 构建、检查与测试指令

```bash
# 编译 OpenFlare Edge Release 二进制（产物：edge/target/release/openflare-edge）
make build-edge

# 格式校验、Clippy 严格检查、单元测试与运行时冒烟验证
cargo fmt --manifest-path edge/Cargo.toml --check
cargo clippy --manifest-path edge/Cargo.toml --all-targets -- -D warnings
cargo test --manifest-path edge/Cargo.toml --locked
python3 edge/tests/runtime_smoke.py
```

---

## 智能 DNS 节点 (`openflare-dns`)

`openflare-dns` 基于 Rust 实现于 `dns/` 目录，可独立运行或由 Server 托管拉起。

### 1. 子命令与参数

```bash
# 启动独立权威 SmartDNS 服务
openflare-dns serve \
  --controller-url "https://panel.example.com" \
  --discovery-token "<DISCOVERY_TOKEN>" \
  --listen "0.0.0.0:53" \
  --geoip-dir "/var/lib/openflare-dns/geoip" \
  --cache-dir "/var/lib/openflare-dns"

# 运行解析吞吐基准测试
openflare-dns bench
```

完整 CLI 参数与 `OPENFLARE_DNS_*` 环境变量覆盖列表见 [配置项清单](./configuration.md#openflare-dns-参数与环境变量)。

### 2. 构建与测试指令

```bash
# 编译 openflare-dns（产物：bin/openflare-dns）
make build-dns

# 运行 SmartDNS 单元测试
make test-dns
```

---

## 流量中继端 (`openflare-relay`)

`openflare-relay` 基于 Rust 实现于 `tunnel/relay`，负责公网隧道中继与 L4/L7 流量转发。

### 1. 运行与参数

```bash
openflare-relay \
  --server-url "https://panel.example.com" \
  --bootstrap-token "<RELAY_TOKEN>" \
  --state-dir "/var/lib/openflare-relay" \
  --public-addr "203.0.113.10" \
  --transport-port 7443 \
  --ws-port 7444 \
  --gateway-port 8443
```

完整启动参数与 `OPENFLARE_RELAY_*` 环境变量参见 [配置项清单](./configuration.md#relay-参数与环境变量)。

### 2. 编译与测试指令

```bash
# 产物：bin/openflare-relay
make build-relay

# 运行隧道工作区单元测试（与 openflared 共用）
make test-tunnel
```

---

## 穿透客户端 (`openflared`)

`openflared` 基于 Rust 实现于 `tunnel/flared`，部署在受保护的内网主机主动连接公共中继。

### 1. 运行与参数

```bash
openflared \
  --server-url "https://panel.example.com" \
  --token-file "/var/lib/openflared/bootstrap-token" \
  --state-dir "/var/lib/openflared"
```

完整启动参数与环境变量参见 [配置项清单](./configuration.md#openflared-参数与环境变量)。

### 2. 编译指令

```bash
# 产物：bin/openflared
make build-flared
```

---

## 统一构建与工程质量门禁

### 1. 全组件一键编译

```bash
# 编译全套二进制（Server, Edge, Relay, OpenFlared, SmartDNS）
make build-all
```

### 2. 自动化代码格式化与整理

```bash
make format
```

自动调用 `goimports` 整理 Go 导入包并格式化 `frontend/` 代码。

### 3. 代码质量门禁检查 (Code Check)

```bash
make code-check
```

按顺序执行：
1. **架构分层防线**：校验 `internal/model` 不越权依赖 `db.DB` 或 Redis；
2. **规则 ID 一致性校验**：核对 WAF 攻击规则 ID 元数据；
3. **Go 代码静态检查**：执行 `golangci-lint run`；
4. **前端工程质量检查**：校验 PoW 静态资源、合并 i18n 片段并校验完整性、运行 `tsc --noEmit` 与零警告 ESLint。

---

## Swagger 文档生成

后端 API 注释变更后重新生成 OpenAPI 2.0 / Swagger 文件：

```bash
make swagger
# 生成产物：api/swagger/swagger.json, api/swagger/swagger.yaml
```

非生产环境下可访问在线交互界面：`http://localhost:3000/api/swagger/index.html`。

---

## 前端与文档站开发

### 1. 控制台前端 (`frontend/`)

```bash
cd frontend

# 安装依赖
pnpm install

# 启动本地开发热更新服务器
pnpm dev

# 独立编译嵌入产物（输出至 frontend/out）
pnpm build:embed

# 执行前端质量检查
pnpm lint
pnpm tsc --noEmit --jsx preserve
pnpm check:i18n
```

### 2. 文档站 (`docs/`)

```bash
cd docs

# 安装依赖
pnpm install

# 启动本地文档实时预览服务器
pnpm dev

# 构建静态文档产物
pnpm build
```
