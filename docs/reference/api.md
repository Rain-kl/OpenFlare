# API 与协议参考

你会学到：OpenFlare 的 OpenAPI / Swagger 规范定义文件与在线交互文档入口、接口认证机制、统一响应信封结构、核心 API 路由分组以及开发者调用示例。

---

## 规范源文件与交互控制台

OpenFlare 后端基于 [Swaggo](https://github.com/swaggo/swag) 根据源码注释自动维护 OpenAPI 2.0 / Swagger 规范文档。

### 1. 静态规范定义文件

规范文件位于仓库的 `api/swagger/` 目录下，可直接导入 Postman、Apifox、Insomnia 等 API 协作工具或用于自动化客户端生成：

- **JSON 格式**：`api/swagger/swagger.json`
- **YAML 格式**：`api/swagger/swagger.yaml`

每次在业务代码中修改 Handler 注释、结构体或路由后，通过根目录 Makefile 重新生成规范：

```bash
make swagger
# 等价于执行 tools/codegen/swagger.sh
```

### 2. 在线 Swagger UI

服务启动且处于**非生产环境**（`app.env` 为 `development` 或 `testing`，即 `APP_ENV != production`）时，后端会自动挂载 Swagger UI 交互式控制台：

- **访问地址**：`http://<SERVER_ADDR>/api/swagger/index.html`（若修改了 `app.api_prefix`，请将 `/api` 替换为实际前缀）
- **功能**：在线调试全部接口、查看请求与响应模型定义、测试 Header 认证。

> **说明**：生产环境（`app.env: production`）为保障安全默认关闭 `/swagger/*` 路由挂载。

---

## 认证机制 (Authentication)

OpenFlare 支持两种标准客户端认证方式，以及专用于节点守护进程的接入认证。

### 1. Access Token (API 密钥)

适用于脚本自动化、CI/CD 流水线及第三方系统集成。

- **获取途径**：管理员登录控制台后，在「个人中心」或「系统管理 → API 密钥」中创建访问密钥。
- **传递方式**：推荐在 HTTP 请求头中通过 `Authorization: Bearer <TOKEN>` 传递；亦兼容 `X-Access-Token: <TOKEN>` 请求头。

```http
GET /api/v1/d/dashboard/overview HTTP/1.1
Host: panel.example.com
Authorization: Bearer of_pat_xxxxxxxxxxxxxxxxxxxxxxxx
Accept: application/json
```

### 2. Session Cookie (浏览器会话)

适用于控制台前端交互与用户网页端操作。

- **认证流程**：通过 `POST /api/v1/oauth/login` 提交用户名与密码，或通过 OIDC (SSO) 第三方免密流程完成登录。
- **Cookie 结构**：登录成功后，服务端会在响应头中签发 Set-Cookie。Cookie 键名由配置项 `app.session_cookie_name`（默认 `openflare_session_id`）决定。
- **有效期**：由 `app.session_age`（默认 86400 秒）或系统配置 `login_session_ttl_hours` 约束。

### 3. 边缘节点专用协议凭证 (Node Protocols)

OpenFlare Edge 节点、Relay 中继节点与 OpenFlared 隧道客户端通过独立凭证与控制面建立 WebSocket 及 HTTP 通信：

- **Discovery Token**：新节点首次接入时的自动注册凭证，由系统配置 `agent_discovery_token` 控制。
- **Node Agent Token**：节点完成注册后由控制面颁发的专属凭证，用于心跳上报与配置快照拉取。
- **Tunnel Token**：OpenFlared 客户端专属隧道接入凭证。

---

## 统一响应信封结构

OpenFlare 所有 HTTP API 均采用统一的 JSON 信封结构（Envelope），遵循以下规范。

### 1. 成功响应 (HTTP 200 OK)

成功调用时 HTTP 状态码始终为 `200`，`error_msg` 为空字符串，业务载荷位于 `data` 字段中：

```json
{
  "error_msg": "",
  "data": {
    "site_name": "OpenFlare",
    "version": "1.0.0"
  }
}
```

若操作无返回实体（例如删除或更新成功），`data` 字段返回 `null`：

```json
{
  "error_msg": "",
  "data": null
}
```

### 2. 分页列表响应

涉及集合列表查询时，`data` 字段包含 `total`（总记录数）与 `results`（数据项数组）：

```json
{
  "error_msg": "",
  "data": {
    "total": 42,
    "results": [
      {
        "id": "194729103847291024",
        "name": "example-zone",
        "status": "active"
      }
    ]
  }
}
```

### 3. 错误响应 (HTTP 4xx / 5xx)

当请求校验失败、凭证无效或服务端发生异常时，接口返回对应的标准 HTTP 错误状态码（非 HTTP 200），`error_msg` 携带小驼峰命名（camelCase）的业务错误标识，`data` 始终为 `null`：

```json
{
  "error_msg": "unauthorized",
  "data": null
}
```

常见状态码与错误标识说明：

| HTTP 状态码 | 业务场景 | 常见 `error_msg` 示例 |
| --- | --- | --- |
| `400 Bad Request` | 请求参数绑定失败、格式不合规 | `errBindParamsFailed`, `errInvalidInput` |
| `401 Unauthorized` | 未登录、Token 缺失、无效或已过期 | `unauthorized`, `errTokenExpired` |
| `403 Forbidden` | 权限不足（如非管理员访问管理端接口） | `forbidden`, `errAdminRequired` |
| `404 Not Found` | 请求的目标资源不存在 | `recordNotFound`, `userNotFound` |
| `500 Internal Server Error` | 数据库读写异常或底层服务故障 | `errInternalServer` |
| `503 Service Unavailable` | 日志数据库正在执行迁移切换 | `日志数据库迁移中，暂不可写` |

---

## 核心 API 路由分组

OpenFlare 控制面路由按功能模块进行清晰分层：

### 1. 用户与认证路由

| 路由路径 | 请求方法 | 权限要求 | 接口说明 |
| --- | --- | --- | --- |
| `/api/v1/oauth/login` | `POST` | 公开 | 用户密码登录 |
| `/api/v1/oauth/logout` | `POST` | 登录会话 | 退出当前登录并销毁会话 |
| `/api/v1/oauth/authorize` | `GET` | 公开 | 发起 OIDC 第三方登录跳转 |
| `/api/v1/oauth/callback` | `GET` | 公开 | 处理 OIDC 登录成功回调 |
| `/api/v1/user/profile` | `GET` | 登录会话 | 获取当前登录用户的个人资料 |
| `/api/v1/user/password` | `POST` | 登录会话 | 修改当前用户密码 |
| `/api/v1/user/access-tokens` | `GET`/`POST` | 登录会话 | 查询与生成个人 Access Token |

### 2. 平台管理路由 (`/api/v1/admin/*`)

| 路由路径 | 请求方法 | 权限要求 | 接口说明 |
| --- | --- | --- | --- |
| `/api/v1/admin/users` | `GET`/`POST` | 管理员 | 平台用户列表与用户创建 |
| `/api/v1/admin/users/:id` | `PUT`/`DELETE` | 管理员 | 编辑用户信息、禁用账号或删除用户 |
| `/api/v1/admin/configs` | `GET`/`POST` | 管理员 | 读取与热更新运行时系统配置 (`w_system_configs`) |
| `/api/v1/admin/database/export`| `GET` | 管理员 | 导出数据库一致性数据备份快照 |
| `/api/v1/admin/tasks` | `GET` | 管理员 | 查看 Asynq 异步任务队列与执行状态 |

### 3. OpenFlare 控制台业务路由 (`/api/v1/d/*`)

| 业务域 | 基础路由路径 | 接口说明 |
| --- | --- | --- |
| 仪表盘 | `/api/v1/d/dashboard/overview` | 全局带宽、QPS、请求量统计与节点健康度概览 |
| 域名区 (Zones) | `/api/v1/d/zones` | 域名区增删改查、DNS 记录配置、Cloudflare 接入同步 |
| 源站集群 (Origins) | `/api/v1/d/origins` | 业务真实源站服务器与健康检查探针配置 |
| 反向代理路由 | `/api/v1/d/proxy-routes` | 站点路由规则、回源负载均衡分发策略 |
| 边缘节点 | `/api/v1/d/nodes` | OpenFlare Edge 节点列表、状态监控、重启指令下发 |
| 性能策略 | `/api/v1/d/performance/policies` | 边缘节点运行时、回源连接池、HTTP/2 及缓存策略模板 |
| WAF 防护 | `/api/v1/d/waf` | WAF 规则组配置、攻击检测模式（CRS）、IP 黑白名单组 |
| 人机验证 (PoW) | `/api/v1/d/pow` | 本地算力防爆破难题下发策略与保护开关 |
| Pages 静态站点 | `/api/v1/d/pages` | 静态项目创建、部署包上传 (`.zip` / `.tar.gz`)、发布历史 |
| TLS 证书 | `/api/v1/d/certificates` | 自定义 SSL/TLS 证书上传、Let's Encrypt 自动化申请管理 |

### 4. 边缘节点通信路由 (`/api/v1/agent/*`, `/api/v1/relay/*`, `/api/v1/tunnel/*`)

| 路由路径 | 协议 | 鉴权方式 | 接口说明 |
| --- | --- | --- | --- |
| `/api/v1/agent/nodes/register` | HTTP POST | Discovery Token | 边缘节点首次接入自动注册 |
| `/api/v1/agent/nodes/heartbeat` | HTTP POST | Node Agent Token | 边缘节点状态与系统指标周期上报 |
| `/api/v1/agent/ws` | WebSocket | Node Agent Token | 实时长连接，用于快照下发与控制指令推流 |
| `/api/v1/relay/ws` | WebSocket | Relay Agent Token | 中继节点实时控制流 |
| `/api/v1/tunnel/ws` | WebSocket | Tunnel Token | 内网穿透客户端隧道会话保持与配置同步 |

---

## 开发者调用示例

### 1. 使用 Access Token 获取仪表盘概览

```bash
curl -fsSL -X GET "https://panel.example.com/api/v1/d/dashboard/overview" \
  -H "Authorization: Bearer of_pat_9a8b7c6d5e4f3a2b1c" \
  -H "Accept: application/json"
```

返回示例：

```json
{
  "error_msg": "",
  "data": {
    "total_nodes": 4,
    "online_nodes": 4,
    "total_zones": 12,
    "today_requests": 1428570,
    "today_bytes": 10737418240
  }
}
```

### 2. 分页查询边缘节点列表

```bash
curl -fsSL -X GET "https://panel.example.com/api/v1/d/nodes/page?page=1&page_size=50&include_monitoring=true" \
  -H "Authorization: Bearer of_pat_9a8b7c6d5e4f3a2b1c" \
  -H "Accept: application/json"
```

控制台的节点列表和卡片视图均使用 50 条分页。`search` 匹配名称、节点标识、IP、地区或操作系统名称。点击延迟摘要可打开该节点的延迟与丢包图表，通过任务名称筛选曲线，并查看 1 小时至 30 天的记录。

`include_monitoring=true` 为当前页的节点返回 `monitoring`。其中 `system` 为操作系统名称，`metric` 为在线节点最近 5 分钟内的最新资源快照，`net_in` 和 `net_out` 分别以字节每秒表示下载和上传速率。`ping` 按任务顺序返回最多 3 个已绑定任务，每个任务最多包含 12 条按时间升序排列的样本。延迟值以毫秒计，负值表示探测失败。部分监控数据暂时无法读取时，响应仍返回节点列表，并标记 `monitoring_unavailable`。

### 3. 创建反向代理路由规则

```bash
curl -fsSL -X POST "https://panel.example.com/api/v1/d/proxy-routes" \
  -H "Authorization: Bearer of_pat_9a8b7c6d5e4f3a2b1c" \
  -H "Content-Type: application/json" \
  -d '{
    "zone_id": "194729103847291024",
    "name": "api-service",
    "pattern": "/api/*",
    "origin_id": "194729103847291055",
    "websocket_enabled": true
  }'
```

---

## 相关文档

- [配置项参考](./configuration.md)：Server、Agent 与边缘节点参数
- [命令与脚本参考](./cli.md)：Makefile、CLI 二进制启动与构建
- [架构设计规范](../design/architecture.md)：控制面与数据面通信模型
