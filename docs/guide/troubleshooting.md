# 故障排查

你会学到：如何按症状排查 OpenFlare Server、数据库、登录、OpenFlare Edge 和配置同步问题。

排查时先确认问题发生在哪一层：浏览器、Server、数据库、OpenFlare Edge、源站或 DNS。节点收到 WebSocket 通知后会重新获取配置清单，也会通过周期心跳重新同步。

## 快速定位

| 现象 | 先看哪里 |
| --- | --- |
| 管理端打不开 | Server 容器或进程日志、端口监听 |
| 登录异常 | 默认账号、Session Cookie、Server 日志 |
| 数据无法保存 | 数据库连接、SQLite 文件权限、PostgreSQL 健康状态 |
| Agent 离线 | Agent 日志、Token、Server 地址、网络连通性 |
| 保存后节点未更新 | 节点心跳、应用记录、控制端快照编译日志 |
| OpenFlare Edge 配置应用失败 | 应用记录、Edge 日志、证书、源站地址、端口占用 |
| 访问分析无数据 | 节点心跳、Edge 访问日志队列和控制端日志 |
| 静态资源总不命中缓存 | 全局/站点缓存开关、策略扩展名、配置是否已发布、访问日志 `cache_status`、源站 Set-Cookie / Cache-Control |

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
| 数据库连接失败 | 检查 `DB_HOST`、`DB_PORT`、`DB_USERNAME`、`DB_PASSWORD`、`DB_NAME`、`DB_SSL_MODE` 是否一致 |
| SQLite 无法创建文件 | 检查 `SQLITE_PATH` 所在目录是否存在且可写 |
| 端口被占用 | 修改 `PORT` 或 `--port`，或停止占用端口的进程 |

## 管理端打不开或空白

1. 确认 Server 正在监听：

```bash
curl -I http://127.0.0.1:3000
```

2. 检查浏览器访问地址是否与反向代理配置一致。

## 默认账号无法登录

默认账号是 `admin` / `12345678`。首次登录后如果已经修改密码，应使用修改后的密码。

排查步骤：

1. 确认连接的是预期数据库，避免 `SQLITE_PATH` 或 `DB_HOST` / `DB_NAME` 指向了另一个环境。
2. 查看 Server 日志中使用的是 `sqlite` 还是 `postgres`。
3. 在浏览器开发者工具中确认管理端 API 请求已正确携带 Session Cookie。
4. 清理浏览器缓存及 Cookie 后重新登录。

### 应急重置管理员密码

忘记 `admin` 账户密码时，使用 `reset-passwd` 命令重置（支持 SQLite 与 PostgreSQL）：

```bash
go run ./cmd/server reset-passwd --user admin --password your-new-password
```

若使用 SQLite，建议先停止 Server 进程再执行，避免数据库文件锁冲突。未指定 `--password` 时命令会生成随机密码并输出到终端。重置成功后请立即登录并修改密码。

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

## 保存后节点未更新

1. 确认站点已启用，且节点的最近心跳时间持续更新。
2. 查看节点详情的「应用记录」。节点会报告候选配置的错误，并继续使用最后一份健康快照。
3. 控制端无法编译时，查看控制端日志中的 `refresh live edge snapshot`，修正配置后保存。
4. 节点环境修复后，可在节点详情执行「强制同步」。节点重启也会清除失败标记并重试最新修订。

## OpenFlare Edge 配置应用失败

使用 `systemctl status openflare-edge` 查看服务状态，并通过 `journalctl -u openflare-edge` 查看节点拉取、校验、应用和监听日志。节点的 `ready.json` 包含实际加载的站点版本和内容摘要。


## HTTPS 不生效

1. 确认证书已经上传或托管。
2. 确认网站配置中对应域名已经绑定证书。
3. 确认站点和证书绑定已保存。
4. 查看应用记录是否成功。
5. 用 `curl` 查看证书和状态码：

```bash
curl -Iv https://your-domain
```

没有绑定证书的域名不会被自动加入 HTTPS 配置，这是预期行为。

## 访问分析没有数据

1. 确认 `openflare-edge` 服务正在运行，且节点心跳正常。
2. 检查节点日志是否有内存队列丢弃、批次落盘失败或心跳确认失败的告警。落盘失败时检查状态目录的写入权限和磁盘空间。
3. 确认 Server 侧没有因数据库清理策略删除对应时间窗口数据。队列与磁盘保留上限见[访问日志写入](./performance-policies.md#访问日志写入)。

## 边缘缓存命中率异常

访问日志中缓存三态：**命中**（HIT/STALE/REVALIDATED/UPDATING）、**回源**（MISS/EXPIRED）、**未缓存**（BYPASS 或空，请求时未进入可缓存路径或响应未入库）。设计说明见 [边缘缓存策略设计](../design/edge-cache-design.md)。

### 检查清单

1. 节点性能策略中的缓存开关已开启。
2. 站点 **缓存** 已启用，策略与路径匹配（「标准静态资源」只覆盖内置扩展名，**不含** HTML/JSON；`.js.map` 的扩展名是 `map`，在默认表内）。  
3. 对应节点的「应用记录」显示最新配置已成功应用。
4. 请求方法为 **GET**（非 GET 一律不缓存）。  
5. 源站未对目标 URL 返回 **`Set-Cookie`**（有则不会写入边缘）。  
6. 源站未声明 **`Cache-Control: private` / `no-store`**（共享缓存不会存）。  
7. 浏览器 DevTools「禁用缓存」只影响浏览器；边缘是否 HIT 看访问日志 `cache_status`，不要只看 Network 面板。

### 常见误解

| 现象 | 说明 |
| --- | --- |
| `static` 下 `/api/foo` 或 `/index.html` 未缓存 | 预期行为（扩展名不在默认可缓存表） |
| 策略为 `all` 后 HTML 被串用户 | 源站未禁止共享缓存；改回 `static` 或给动态响应加 `private`/`no-store` |
| 带 `?v=` 的 URL 命中率低 | 默认缓存键含完整 `$request_uri`，query 不同即不同对象 |
| 第一次 MISS、第二次仍 MISS | 查源站是否每次 `Set-Cookie`、是否 `private`，或节点磁盘/缓存 inactive 过短 |

### 期望行为（对齐 Cloudflare 默认）

* 带登录 Cookie 的用户访问 `/_app/**/*.js` 等静态资源：**可以 HIT**。  
* 响应带 `Set-Cookie` 或 `private`：**不入库**。  
* 无源站缓存头的可缓存状态码：使用默认 Edge TTL（如 200 约 120 分钟）。

