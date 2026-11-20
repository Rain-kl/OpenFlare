# 命令与脚本

你会学到：OpenFlare Server、Pingora Edge、Relay、OpenFlared、管理端前端、Swagger 与文档站的命令行参数、构建编译指令与质量检查命令。

> 所有命令均在**代码仓库根目录**执行，除非另有说明。

---

## 控制面服务 (OpenFlare Server)

### 1. 命令行子命令与参数

构建生成的二进制文件位于 `bin/openflare-server`（或通过源码 `go run ./cmd/server` 执行）：

```bash
# 查看版本信息
./bin/openflare-server --version

# 启动融合单进程模式（默认启动 API + Worker + Scheduler）
./bin/openflare-server all

# 分离进程独立部署启动
./bin/openflare-server api          # 仅启动 HTTP API 服务
./bin/openflare-server worker       # 仅启动 Asynq 异步任务消费 Worker
./bin/openflare-server scheduler    # 仅启动定时任务 Cron 调度器

# 重置指定账号密码
./bin/openflare-server reset-passwd --user admin --password "NewPassword123!"
```

`reset-passwd` 参数说明：
- `--user <string>`：待重置密码的用户名；省略时进入交互式控制台输入。
- `--password <string>`：新密码；省略时系统将自动生成 16 位强随机密码并输出至终端。

指定非默认配置文件路径：

```bash
CONFIG_PATH=/etc/openflare/config.yaml ./bin/openflare-server all
```

### 2. 构建与测试指令

```bash
# 仅编译后端服务二进制（产物：bin/openflare-server）
make build-backend

# 编译包含前端静态资源的单文件完整二进制（静态资源内置打包）
make build-embedded

# 运行后端单元测试与集成测试
GOCACHE=/tmp/openflare-go-cache go test ./...
```

---

## 边缘网关 (Pingora Edge)

Pingora Edge 基于 Rust 与 Cloudflare Pingora 框架实现，二进制名称为 `openflare-edge`。

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

# 升级模式启动（检查并恢复本地现有快照，平滑迁移）
openflare-edge run --upgrade \
  --server-url "https://panel.example.com" \
  --token-type "node" \
  --token-file "/etc/openflare-edge/node.token" \
  --state-dir "/var/lib/openflare-edge" \
  --http-listen "0.0.0.0:80" \
  --tls-listen "0.0.0.0:443"

# 查询控制面配置同步协议版本号
openflare-edge config-sync-version

# 查询自动升级协议版本号
openflare-edge self-update-version

# 输出 WAF / CRS 攻击防护规则清单 (JSON 格式)
openflare-edge attack-rule-catalog
```

`openflare-edge run` 参数详表：

| 参数名称 | 类型 | 是否必填 | 默认值 | 作用说明 |
| --- | --- | --- | --- | --- |
| `--server-url` | `string` | 是 | 无 | OpenFlare 控制面访问地址（如 `https://panel.example.com`） |
| `--token-type` | `string` | 否 | `discovery` | 认证 Token 类型：首次自动注册填 `discovery`；专属节点凭据填 `node` |
| `--token-file` | `path` | 是 | 无 | 存放 Token 字符串的本地文件路径 |
| `--state-dir` | `path` | 是 | 无 | 边缘节点持久化状态与快照存储目录（如 `/var/lib/openflare-edge`） |
| `--http-listen` | `string` | 否 | `127.0.0.1:8080` | HTTP 协议监听套接字地址（生产环境通常为 `0.0.0.0:80`） |
| `--tls-listen` | `string` | 否 | 空 | HTTPS/TLS 协议监听套接字地址（生产环境通常为 `0.0.0.0:443`） |
| `--upgrade` | `bool` | 否 | `false` | 升级模式；当本地存在快照时要求成功恢复配置后方可启动 |

### 2. 构建与测试指令

```bash
# 编译 Pingora Edge Release 生产二进制
make build-edge
# 产物位于：edge/pingora/target/release/openflare-edge

# 运行 Rust 测试套件
cargo test --manifest-path edge/pingora/Cargo.toml --locked
```

---

## 流量中继端 (Relay)

Relay 负责内网穿透流量的中继分发，管理内嵌的 `frps` 进程。

### 1. 运行与参数

```bash
# 源码运行
go run ./cmd/relay -config /path/to/relay.json

# 二进制运行
./bin/openflare-relay -config /etc/openflare-relay/relay.json
```

命令行仅接受 `-config <PATH>` 参数（默认 `./relay.json`）。支持通过环境变量覆盖关键配置：
- `OPENFLARE_SERVER_URL`：控制面地址
- `OPENFLARE_AGENT_TOKEN`：中继专属认证 Token
- `OPENFLARE_DISCOVERY_TOKEN`：自动发现 Token
- `OPENFLARE_NODE_NAME`：节点名称
- `OPENFLARE_NODE_IP`：节点公网出口 IP
- `OPENFLARE_DATA_DIR`：运行时工作数据目录
- `OPENFLARE_FRPS_PATH`：系统 `frps` 二进制文件路径
- `LOG_LEVEL`：日志等级（默认 `info`）

### 2. 编译指令

```bash
make build-relay
# 产物：bin/openflare-relay
```

---

## 穿透客户端 (OpenFlared)

OpenFlared 为运行于用户内网服务器的穿透 Agent，负责管理本地 `frpc` 客户端与中继建立加密隧道。

### 1. 运行与参数

```bash
# 源码运行
go run ./cmd/flared -config /path/to/flared.json

# 二进制运行
./bin/flared -config /etc/openflared/flared.json
```

命令行仅接受 `-config <PATH>` 参数（默认 `./flared.json`）。支持通过环境变量覆盖关键配置：
- `OPENFLARE_SERVER_URL`：控制面地址
- `OPENFLARE_TUNNEL_TOKEN`：隧道接入专属 Token
- `OPENFLARE_DATA_DIR`：客户端工作数据目录
- `OPENFLARE_FRPC_PATH`：系统 `frpc` 二进制文件路径
- `LOG_LEVEL`：日志等级（默认 `info`）

### 2. 编译指令

```bash
make build-flared
# 产物：bin/flared
```

---

## 统一构建与工程质量门禁

### 1. 全组件一键编译

```bash
# 编译全套二进制 (Server, Edge, Relay, Flared)
make build-all
```

### 2. 自动化代码格式化与整理

```bash
make format
```
该指令自动调用 `goimports` 清理并对齐后端 Go 导入包，并调用 Prettier 格式化 `frontend/` 下的所有前端代码。

### 3. 代码质量门禁检查 (Code Check)

```bash
make code-check
```
门禁流水线按顺序执行以下检查：
1. **架构分层防线**：静态扫描禁止 `internal/model` 越权依赖 `db.DB` 或 Redis；
2. **规则 ID 一致性校验**：自动比对 WAF 攻击规则 ID 元数据；
3. **Go 代码静态检查**：执行 `golangci-lint run`；
4. **前端工程质量检查**：验证 PoW 静态资源一致性、合并 i18n 语言片段并执行 `next-intl` 完整性校验、TypeScript 类型检查 (`tsc --noEmit`) 与 ESLint 严格检查（零警告策略）。

---

## Swagger 文档生成

后端 API 注释更新后，重新生成 OpenAPI / Swagger 文档源文件：

```bash
make swagger
# 生成产物：docs/swagger.json, docs/swagger.yaml
```

在线控制台请在非生产环境下访问：`http://localhost:3000/api/swagger/index.html`。

---

## 前端与文档站开发

### 1. 控制台前端 (`frontend/`)

```bash
cd frontend

# 安装依赖
pnpm install

# 启动本地开发热更新服务器
pnpm dev

# 独立编译嵌入产物（生成静态文件至 frontend/out）
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
