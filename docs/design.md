# ATSFlare MVP 设计文档

## 1. 目标

先做一个能用的版本，不做平台化过度设计。第一版只解决 3 件事：

* 配置发布与同步
* 节点心跳检测
* Nginx 反向代理配置下发

系统定位是内部自用的控制面，不是面向外部租户的 CDN SaaS。

---

## 2. 第一版范围（已完成）

### 已做

* Web 管理端维护反代规则
* 配置发布生成版本
* Agent 定时同步并应用配置
* Agent 控制本机 Nginx 校验与 reload
* 节点注册、心跳、在线状态展示
* 展示每个节点当前生效版本和最近一次应用结果

### 不做（第一版）

* 多租户
* WAF、限流、Bot、防刷
* 灰度发布、节点分组、分批发布
* 对象存储、消息队列、Redis、Prometheus
* 复杂缓存策略管理
* 证书托管与自动签发
* Purge、中台审计、审批流
* mid-tier / 分层缓存

第一版默认所有节点消费同一份全量配置，不做差异化下发。

---

## 2.5 第二版范围

在 MVP 闭环稳定运行的基础上，第二版聚焦以下增量能力。

### 要做

**2.5.1 HTTPS/TLS 支持**

* `proxy_routes` 增加 HTTPS 相关字段：`enable_https`、`ssl_cert_path`、`ssl_key_path`
* 渲染器根据字段生成 HTTPS `server` 块（443 端口），并可选生成 HTTP → HTTPS 重定向块
* 证书文件仍由节点本地预先准备，控制面只记录路径，不托管证书

**2.5.2 节点分组与差异化下发**

* 新增 `node_groups` 表：管理分组（如 staging、production）
* `nodes` 增加 `group_id` 字段，节点可归属某个分组
* 发布时可选择目标分组，生成面向该分组的版本
* 不指定分组时，默认行为与第一版相同（全量下发）
* Agent 在心跳时携带自身分组信息，Server 按分组返回对应激活版本

**2.5.3 Agent Token 管理**

* 新增 `agent_tokens` 表：支持创建多个命名 Token，记录备注、创建人、过期时间
* 认证中间件改为查表验证，不再依赖单个全局环境变量
* 提供 Token CRUD 管理 API 及前端页面
* 旧的全局 Token 环境变量作为引导 Token，仅在数据库无 Token 记录时生效（bootstrap 模式）

**2.5.4 路由增强**

* `proxy_routes` 增加 `custom_headers` 字段（JSON 格式），支持每条路由追加自定义 `proxy_set_header` 指令
* 渲染器按 `custom_headers` 内容注入到对应 `server` 块

**2.5.5 配置预览与变更摘要**

* 新增"配置预览"接口：在不实际发布的情况下，返回基于当前启用规则渲染的 Nginx 配置
* 新增"变更摘要"接口：对比当前激活版本与新渲染结果，返回新增、删除、修改的域名列表
* 前端发布页接入预览与变更摘要，让管理员在点击发布前确认变化

### 仍不做（第二版）

* 多租户
* WAF、限流、Bot、防刷
* 对象存储、消息队列、Redis、Prometheus
* 证书托管与自动签发
* Purge、中台审计、审批流
* mid-tier / 分层缓存
* 复杂缓存策略配置

---

## 3. 技术约束

### Server

控制中心直接基于现有 `atsf_server` 的 `gin-template` 工程开发：

* Web 框架：Gin
* ORM：GORM
* 前端：沿用现有 web 管理端
* 鉴权：沿用 gin-template 登录体系

### 数据库

只使用 SQLite，不引入其他中间件：

* 不配置 `SQL_DSN`，直接走项目现有 SQLite 初始化逻辑
* 不配置 `REDIS_CONN_STRING`，会退化为 cookie session

### Agent

Agent 使用 Go 单体程序：

* 单二进制
* systemd 管理
* 优先调用独立 Nginx，而不是依赖系统全局 Nginx
* 显式配置 `nginx_path` 时，直接调用该路径下的 Nginx
* 未配置 `nginx_path` 时，默认通过 Docker 运行独立 Nginx 容器
* 管理本机 Nginx 路由配置文件和 reload
* Agent 生成资源默认统一落在 `./data`，也允许通过单个基路径配置覆盖
* Agent 启动时会校验本地路由文件哈希与控制面激活版本是否一致
* Docker 模式启动时会重建独立 Nginx 容器，避免复用故障容器

### Nginx 管理边界

第一版只管理最核心的反代映射：

* 重点生成独立的 Nginx 路由配置文件，例如 `/etc/nginx/conf.d/atsflare_routes.conf`
* `nginx.conf`、TLS 证书、缓存细节、upstream 高级配置先保持节点本地静态配置
* Agent 可以管理独立安装路径下的 Nginx，或者独立 Docker Nginx 容器

也就是说，MVP 先把 Nginx 当成“可集中配置的反向代理”，不是完整网关平台。

---

## 4. 总体架构

```text
                ┌────────────────────────────┐
                │       ATSFlare Server      │
                │  gin-template + SQLite     │
                │  Admin UI + Admin API      │
                └──────────────┬─────────────┘
                               │
                     HTTP API / Config Pull
                               │
            ┌──────────────────┴──────────────────┐
            │                                     │
   ┌────────▼────────┐                   ┌────────▼────────┐
   │ Nginx Agent 1   │                   │ Nginx Agent N   │
   │ heartbeat/sync  │                   │ heartbeat/sync  │
   │ nginx reload    │                   │ nginx reload    │
   └────────┬────────┘                   └────────┬────────┘
            │                                     │
      ┌─────▼─────┐                         ┌─────▼─────┐
      │   Nginx   │                         │   Nginx   │
      │ reverse   │                         │ reverse   │
      │  proxy    │                         │  proxy    │
      └─────┬─────┘                         └─────┬─────┘
            │                                     │
            └──────────────► Origin ◄────────────┘
```

设计原则只有 3 条：

* Server 只保存配置和节点状态，不直接 SSH 改机器
* Agent 是唯一的落地入口
* 所有发布都是“新版本生效”，不是在线修改当前文件

---

## 5. 核心对象

### 5.1 proxy_routes（第一版）

反代规则表，控制 `Host -> Origin` 映射。

建议字段：

* `id`
* `domain`
* `origin_url`
* `enabled`
* `remark`
* `created_at`
* `updated_at`

约束：

* `domain` 唯一
* `origin_url` 必须是合法的 `http://` 或 `https://`
* 第一版一条域名只对应一个源站，不做源站池

第二版新增字段：

* `enable_https` — 是否启用 HTTPS（bool，默认 false）
* `ssl_cert_path` — 节点本地证书文件路径（string）
* `ssl_key_path` — 节点本地私钥文件路径（string）
* `redirect_http` — 是否将 HTTP 重定向到 HTTPS（bool，默认 false）
* `custom_headers` — 自定义 `proxy_set_header` 指令（JSON 格式，存字符串）

### 5.2 config_versions（第一版）

发布版本表，保存不可变快照。

建议字段：

* `id`
* `version`
* `snapshot_json`
* `rendered_config`
* `checksum`
* `is_active`
* `created_by`
* `created_at`

说明：

* `snapshot_json` 保存发布时的完整规则快照
* `rendered_config` 保存渲染后的 Nginx 路由配置
* 第一版直接存 SQLite，不单独上对象存储

第二版新增字段：

* `group_id` — 关联目标分组（nullable，null 表示全量发布）

### 5.3 nodes（第一版）

节点表，保存当前状态。

建议字段：

* `id`
* `node_id`
* `name`
* `ip`
* `agent_version`
* `nginx_version`
* `status`
* `current_version`
* `last_seen_at`
* `last_error`
* `created_at`
* `updated_at`

第二版新增字段：

* `group_id` — 所属节点分组（nullable，无分组时为 null）

### 5.4 apply_logs（第一版）

节点应用记录。

建议字段：

* `id`
* `node_id`
* `version`
* `result`
* `message`
* `created_at`

### 5.5 node_groups（第二版新增）

节点分组表，用于差异化下发。

建议字段：

* `id`
* `name` — 分组名称，唯一（如 `staging`、`production`）
* `remark` — 备注
* `created_at`
* `updated_at`

### 5.6 agent_tokens（第二版新增）

Agent Token 管理表，替代全局单一 Token。

建议字段：

* `id`
* `token` — Token 值，唯一，不可变
* `name` — Token 备注名称
* `created_by` — 创建人
* `expires_at` — 过期时间（nullable，null 表示永不过期）
* `is_active` — 是否有效
* `created_at`
* `updated_at`

---

## 6. 配置发布模型

第一版不做增量发布，也不做 bundle 文件仓库。

发布逻辑：

1. 管理员在后台修改 `proxy_routes`
2. 点击“发布”
3. Server 校验规则
4. Server 根据当前全部启用规则渲染出完整 Nginx 路由配置
5. 生成新 `config_versions` 记录
6. 将该版本标记为当前激活版本
7. Agent 下一次心跳或轮询时发现新版本并拉取

### 版本原则

* 一个版本就是一份完整快照
* 版本不可变
* 节点只拉取当前激活版本
* 回滚本质上是重新激活旧版本

### 版本号建议

```text
20260309-001
20260309-002
```

### 发布校验

发布前至少做以下检查：

* `domain` 不能为空
* `origin_url` 合法
* 不允许重复域名
* 至少存在 1 条启用规则

---

## 7. Nginx 配置策略

第一版只生成独立的 Nginx 路由配置文件，这样最简单，也最容易验证。

### 规则映射

```conf
server {
    listen 80;
    server_name www.example.com;

    location / {
        proxy_pass http://10.0.0.10:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_pass http://10.0.0.20:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### HTTPS 处理

第一版不在控制中心管理证书。

约定如下：

* Nginx 的监听端口、证书、TLS 相关配置由节点本地预先准备
* 控制中心只负责反代映射
* 如果节点已经具备 HTTPS 接入能力，后续可以扩展生成 HTTPS `server` 块

### 缓存处理

第一版不开放缓存策略配置：

* 是否开启缓存由节点静态配置决定
* 控制中心不管理 TTL、Header 改写、缓存规则

---

## 8. Server 模块设计

控制中心仍然是单体应用，不拆服务。

### 8.1 管理端模块

* 登录鉴权
* 反代规则 CRUD
* 发布版本管理
* 节点状态页面
* 应用日志查看

### 8.2 Agent API 模块

* 节点注册
* 心跳上报
* 获取当前激活版本
* 下载指定版本配置
* 上报应用结果

### 8.3 渲染模块

职责很简单：

* 从 `proxy_routes` 读取全部启用规则
* 按固定模板拼出 Nginx 路由配置
* 计算 checksum
* 写入 `config_versions`

这层不要引入复杂 DSL，第一版直接围绕 `domain -> origin_url` 即可。

---

## 9. Agent 模块设计

Agent 做成一个 Go 单体进程即可。

### 9.1 本地职责

* 读取本地配置
* 定时心跳
* 拉取新版本
* 覆盖 Nginx 路由配置文件
* 执行 `nginx -t` 和 `nginx -s reload`
* 上报应用结果
* 保存本地最近成功版本

### 9.2 建议的本地文件

* `/etc/atsf-agent/config.yaml`
* `/var/lib/atsf-agent/state.json`
* `/etc/nginx/conf.d/atsflare_routes.conf`
* `/etc/nginx/conf.d/atsflare_routes.conf.bak`

### 9.3 最小工作流

```text
1. Agent 启动
2. 读取或生成 node_id
3. 上报 heartbeat
4. 获取当前激活版本元数据
5. 若版本变更，则下载 rendered_config
6. 备份旧路由配置文件
7. 写入新路由配置文件
8. 调用 `nginx -t`
9. 校验通过后执行 `nginx -s reload`
10. 记录结果并上报
11. 进入下一轮
```

### 9.4 失败处理

第一版只做最基本的容错：

* 拉取失败：继续使用本地旧配置
* 配置校验或 reload 失败：恢复备份文件并再次校验后 reload
* Server 不可用：不影响 Nginx 继续转发

---

## 10. 心跳与在线状态

心跳不单独搞复杂监控系统，直接走业务表。

### 心跳内容

Agent 每次上报：

* `node_id`
* `name`
* `ip`
* `agent_version`
* `nginx_version`
* `current_version`
* `last_apply_result`
* `timestamp`

### 状态判定

建议规则：

* 15 秒一次心跳
* 超过 45 秒未上报记为 `offline`
* 最近一次应用失败但仍有心跳，记为 `warning`
* 正常心跳且版本一致，记为 `online`

---

## 11. API 设计

### 11.1 管理端 API（第一版，已实现）

* `GET /api/proxy-routes/`
* `POST /api/proxy-routes/`
* `PUT /api/proxy-routes/:id`
* `DELETE /api/proxy-routes/:id`
* `GET /api/config-versions/`
* `GET /api/config-versions/active`
* `POST /api/config-versions/publish`
* `PUT /api/config-versions/:id/activate`
* `GET /api/nodes/`
* `GET /api/apply-logs/`

### 11.2 Agent API（第一版，已实现）

* `POST /api/agent/nodes/register`
* `POST /api/agent/nodes/heartbeat`
* `GET /api/agent/config-versions/active`
* `POST /api/agent/apply-logs`

### 11.3 第二版新增管理端 API

* `GET /api/node-groups/` — 分组列表
* `POST /api/node-groups/` — 创建分组
* `PUT /api/node-groups/:id` — 更新分组
* `DELETE /api/node-groups/:id` — 删除分组
* `GET /api/agent-tokens/` — Token 列表
* `POST /api/agent-tokens/` — 创建 Token
* `DELETE /api/agent-tokens/:id` — 撤销 Token
* `GET /api/config-versions/preview` — 预览当前启用规则的渲染结果（不写库）
* `GET /api/config-versions/diff` — 对比当前激活版本与待发布的变更摘要

### 11.4 鉴权方案

管理端：

* 直接沿用 gin-template 的登录态

Agent（第一版）：

* 预共享 Token，请求头 `X-Agent-Token`，Token 值来自环境变量

Agent（第二版）：

* Token 改为查 `agent_tokens` 表验证
* 环境变量 Token 仅作 bootstrap 引导 Token，数据库有记录时不再使用
* 后续可升级 mTLS

---

## 12. 页面设计

### 12.1 登录页

沿用 gin-template 现有登录。

### 12.2 反代规则页（第一版，已实现）

展示和编辑：

* 域名
* 源站地址
* 是否启用
* 备注

第二版新增字段：

* 是否启用 HTTPS
* SSL 证书路径
* SSL 私钥路径
* 是否 HTTP → HTTPS 重定向
* 自定义请求头（JSON 编辑器）

### 12.3 发布版本页（第一版，已实现）

展示：

* 版本号
* 发布时间
* 发布人
* 是否当前激活

动作：

* 立即发布
* 激活旧版本

第二版新增：

* 发布目标分组选择（可选，不选则全量）
* 发布前展示配置预览与变更摘要

### 12.4 节点页（第一版，已实现）

展示：

* 节点名
* IP
* 在线状态
* 当前版本
* 最后心跳时间
* 最近错误

第二版新增：

* 所属分组

### 12.5 应用记录页（第一版，已实现）

展示：

* 节点
* 版本
* 成功/失败
* 错误信息
* 时间

### 12.6 Token 管理页（第二版新增）

展示：

* Token 名称
* 创建人
* 过期时间
* 是否有效

动作：

* 创建 Token
* 撤销 Token

### 12.7 节点分组页（第二版新增）

展示：

* 分组名称
* 备注
* 该分组下节点数

动作：

* 创建分组
* 编辑分组
* 删除分组

---

## 13. 代码组织建议

### Server（第一版，已实现）

```text
atsf_server/
  controller/
    proxy_route.go
    config_version.go
    node.go
    agent.go
  model/
    proxy_route.go
    config_version.go
    node.go
    apply_log.go
  router/
    api-router.go
  service/
    proxy_route.go
    config_version.go
    agent.go
```

### Server（第二版新增）

```text
atsf_server/
  controller/
    node_group.go      # 节点分组 CRUD
    agent_token.go     # Token 管理
  model/
    node_group.go      # NodeGroup 模型
    agent_token.go     # AgentToken 模型
  service/
    node_group.go      # 分组逻辑
    agent_token.go     # Token 创建与验证
    renderer.go        # 抽离渲染逻辑（HTTPS 支持扩展）
  middleware/
    agent-auth.go      # 改为查表验证
```

### Agent（第一版，已实现）

```text
atsf_agent/
  cmd/agent/main.go
  internal/config/config.go
  internal/heartbeat/service.go
  internal/sync/service.go
  internal/nginx/manager.go
  internal/state/state.go
  internal/httpclient/client.go
  internal/protocol/agent_api.go
```

### Agent（第二版）

第二版 Agent 无需新增模块，只需在现有模块内扩展：

* `config`: 新增 `group_id` 配置项
* `heartbeat`: 心跳请求中携带 `group_id`
* `sync`: 按分组获取对应激活版本（Server 端路由区分）

---

## 14. 开发顺序

### 第一版（已完成）

1. Server 建表、AutoMigrate
2. 反代规则 CRUD 与发布逻辑
3. Agent API 与节点状态表
4. Agent 同步、落盘、reload、回滚
5. 管理端页面
6. 联调和部署文档

### 第二版（当前阶段）

按以下顺序执行，前项完成后再推进下一项：

1. HTTPS/TLS 支持（ProxyRoute 扩展字段 + 渲染器 + 前端表单）
2. Agent Token 管理（agent_tokens 表 + 中间件改造 + 前端 Token 管理页）
3. 节点分组与差异化下发（node_groups 表 + 发布分组逻辑 + Agent 携带 group_id）
4. 路由增强（custom_headers 字段 + 渲染器注入 + 前端表单）
5. 配置预览与变更摘要（preview 接口 + diff 接口 + 前端发布确认弹窗）

---

## 15. 关键取舍

第一版故意做这些取舍：

* 不抽象 zone、origin pool、policy 这些平台概念
* 不做复杂发布编排，所有节点统一拉当前版本
* 不管理 Nginx 全部配置，只先管独立生成的路由配置文件
* 不引入 Redis、MQ、对象存储，先把单机 SQLite 跑起来
* 不为了“以后可能会用到”提前把系统拆复杂

只要这版能稳定完成下面这条链路，就算成功：

```text
后台改规则 -> 点击发布 -> Agent 拉到新版本 -> Nginx reload -> 节点状态可见
```

这就是当前阶段最需要的 MVP。

### 第二版取舍

* HTTPS 支持不托管证书，只记录本地路径，避免引入证书存储和签发复杂度
* 节点分组不做跨分组继承，每个分组独立一套激活版本，降低理解负担
* Token 管理不做细粒度权限（如只读 Token），第二版所有 Token 权限一致
* 路由自定义头不做模板变量，只支持静态 key-value，避免过早引入 DSL
* 配置预览只展示渲染结果，不实际验证 Nginx 语法，真实校验仍由 Agent 完成

第二版成功标准：

```text
HTTPS 路由可生效 + 节点可按分组差异化下发 + Token 可在界面管理 + 发布前可预览变更
```
