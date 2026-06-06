# 故障排查

你会学到：如何按症状排查 OpenFlare Server、数据库、登录、Agent、OpenResty、配置发布和前端构建问题。

排查时先确认问题发生在哪一层：浏览器、Server、数据库、Agent、OpenResty、源站或 DNS。OpenFlare 的配置不会直接在线写入所有节点，只有激活版本变化后，Agent 才会在 heartbeat 中发现并应用。

## 快速定位

| 现象 | 先看哪里 |
| --- | --- |
| 管理端打不开 | Server 容器或进程日志、端口监听 |
| 登录异常 | 默认账号、OPENFLARE_TOKEN、浏览器请求、Server 日志 |
| 数据无法保存 | 数据库连接、SQLite 文件权限、PostgreSQL 健康状态 |
| Agent 离线 | Agent 日志、Token、Server 地址、网络连通性 |
| 发布后节点未更新 | 激活版本、节点 heartbeat、应用记录 |
| OpenResty 应用失败 | 应用记录、Agent 日志、证书、上游地址、端口占用 |
| 访问分析无数据 | OpenResty 容器状态、观测端口、Agent 补报日志 |

## Server 无法启动

1. 查看日志：

```bash
docker compose logs -n 200 openflare
```

源码运行时查看终端输出。

2. 检查端口占用：

```bash
lsof -i :3000
```

3. 如果使用 PostgreSQL，确认数据库健康：

```bash
docker compose ps postgres
docker compose logs -n 100 postgres
```

4. 如果使用 SQLite，确认数据库文件目录可写：

```bash
ls -ld "$(dirname /path/to/openflare.db)"
```

常见原因：

| 日志或现象 | 处理 |
| --- | --- |
| 数据库连接失败 | 检查 `DSN` 中用户名、密码、主机、端口、库名和 `sslmode` |
| SQLite 无法创建文件 | 检查 `SQLITE_PATH` 所在目录是否存在且可写 |
| 端口被占用 | 修改 `PORT` 或 `--port`，或停止占用端口的进程 |

## 管理端打不开或空白

1. 确认 Server 正在监听：

```bash
curl -I http://127.0.0.1:3000
```

2. 如果是源码运行，确认已经构建前端静态产物：

```bash
cd openflare-server/web
pnpm build
```

3. 检查浏览器访问地址是否与反向代理配置一致。

4. 如果通过前端开发服务器访问，确认后端代理地址：

```bash
cd openflare-server/web
NEXT_DEV_BACKEND_URL=http://127.0.0.1:3000 pnpm dev
```

## 默认账号无法登录

默认账号是 `root` / `123456`。首次登录后如果已经修改密码，应使用修改后的密码。

排查步骤：

1. 确认连接的是预期数据库，避免 `SQLITE_PATH` 或 `DSN` 指向了另一个环境。
2. 查看 Server 日志中使用的是 `sqlite` 还是 `postgres`。
3. 在浏览器开发者工具中确认管理端 API 请求携带 `OPENFLARE_TOKEN` 请求头。
4. 清理浏览器本地存储中的旧 `openflare_token` 后重新登录。

### 应急重置管理员密码

如果忘记了 `root` 账户的密码，可以通过直接更新数据库中的密码哈希值将其重置为 `123456`（登录后请务必立即修改）：

#### 1. 若使用 SQLite 数据库
停止 Server 运行，使用 sqlite3 客户端打开数据库文件：
```bash
sqlite3 /path/to/openflare.db
```
执行以下 SQL 语句：
```sql
UPDATE users SET password_hash = '$2a$10$wN9aE3zTz83rO7R1uKlhuehJtA3c604pX4Z12B/9.5c0X337t1L4m' WHERE username = 'root';
```
输入 `.exit` 退出并重新启动 Server。

#### 2. 若使用 PostgreSQL 数据库
通过您的数据库连接工具（如 psql、pgAdmin 或 DBeaver）连接到 PostgreSQL 实例，选择对应的 `openflare` 数据库，执行以下 SQL 语句：
```sql
UPDATE users SET password_hash = '$2a$10$wN9aE3zTz83rO7R1uKlhuehJtA3c604pX4Z12B/9.5c0X337t1L4m' WHERE username = 'root';
```
执行成功后即可使用默认密码 `123456` 重新登录管理后台。

## Agent 无法注册或一直离线

在 Agent 节点执行：

```bash
curl -I http://your-server:3000
```

查看 Agent 日志：

```bash
journalctl -u openflare-agent -n 200 --no-pager
```

检查配置文件：

```bash
sed -n '1,160p' /opt/openflare-agent/agent.json
```

重点确认：

| 配置 | 说明 |
| --- | --- |
| `server_url` | 必须是 Agent 节点能访问的 Server 地址 |
| `agent_token` / `discovery_token` | 至少填写一个 |
| `heartbeat_interval` | 支持毫秒整数或 Go duration 字符串 |
| `request_timeout` | 网络较慢时可适当增大 |

如果日志提示 Token 无效，重新在管理端准备 Token 并更新 `agent.json`，然后重启：

```bash
systemctl restart openflare-agent
```

## 发布后节点没有应用新版本

按顺序检查：

1. 版本页面中是否已经激活目标版本。
2. 节点是否在线，最近心跳时间是否更新。
3. 应用记录中是否有目标版本的成功、警告或失败记录。
4. 网站配置是否启用；未启用的网站不会参与发布渲染。
5. Agent 日志是否出现拉取、校验、reload 或回滚信息。

查看 Agent 日志：

```bash
journalctl -u openflare-agent -f
```

注意：某个目标 `version + checksum` 一旦应用失败并回退，Agent 会在本地状态中阻断该目标重复应用。修正配置后需要重新发布生成新的 checksum，或激活旧版本回滚。

如果这是 Agent 首次应用配置，且本地没有历史 `nginx.conf` 可回滚，失败目标仍会被阻断，但 Agent 会尝试进入安全兜底运行态。此时应用记录和 Agent 日志会包含 `fallback runtime started`，OpenResty 对外只监听 `80` 端口并统一返回 `503` 与 `OpenFlare: No Valid Configuration`，同时保留本地 `stub_status` 健康检查入口。修正配置并重新发布新版本后，Agent 会覆盖兜底配置并恢复正常代理。

## OpenResty 应用失败

常见原因：

| 原因 | 排查 |
| --- | --- |
| 域名或 server 块冲突 | 检查同一域名是否被多个网站配置使用 |
| 上游地址不合法 | 确认所有上游都是 `http://` 或 `https://` |
| 多上游格式不符合约束 | 多上游必须是纯 `scheme://host[:port]` |
| 证书缺失或路径错误 | 检查域名是否绑定证书，以及 Agent 证书目录是否可写 |
| 端口被占用 | 检查本机 `80`、`443` 端口 |

OpenResty 配置校验：

```bash
openresty -t -c /path/to/openflare/data/etc/nginx/nginx.conf
```

OpenResty 运行状态：

```bash
ps aux | grep openresty
```

Agent 周期性健康检查通过本地 `http://127.0.0.1:<openresty_observability_port>/openflare/stub_status` 判断 OpenResty 是否存活，不会反复执行 `openresty -t`。如果节点被标记为 unhealthy，优先确认该本地观测端口是否正在监听；如果只在应用配置时出现 `host not found in upstream`，说明失败来自配置校验或 reload，而不是周期性健康探针。

实际二进制路径和主配置路径以 `agent.json` 中的 `openresty_path` 与 `main_config_path` 为准。

## HTTPS 不生效

1. 确认证书已经上传或托管。
2. 确认网站配置中对应域名已经绑定证书。
3. 确认发布并激活了新版本。
4. 查看应用记录是否成功。
5. 用 `curl` 查看证书和状态码：

```bash
curl -Iv https://your-domain
```

没有绑定证书的域名不会被自动加入 HTTPS 配置，这是预期行为。

## 访问分析没有数据

1. 确认节点已经成功应用包含观测 Lua 资源的配置。
2. 确认 OpenResty 正在运行。
3. 查看 Agent 日志是否有观测采集或补报失败信息。
4. 检查 `openresty_observability_port` 是否被占用，默认是 `18081`。
5. 确认 Server 侧没有因数据库清理策略删除对应时间窗口数据。

## 前端构建失败

执行：

```bash
cd openflare-server/web
corepack enable
pnpm install
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```

常见原因：

| 现象 | 处理 |
| --- | --- |
| pnpm 版本不一致 | 使用 `corepack enable` 后重新安装 |
| 类型错误 | 先运行 `pnpm typecheck` 定位具体文件 |
| API 类型不一致 | 检查 `lib/api/` 和 `types/` 中的响应结构 |
| E2E 失败 | 确认 Server 和前端开发服务器都已启动 |

## 文档站构建失败

```bash
cd docs
pnpm install
pnpm build
```

如果是链接错误，检查新增页面是否已经加入 `docs/config.ts` 侧边栏，或者相对链接是否指向存在的 Markdown 文件。
