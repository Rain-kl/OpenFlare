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

* `proxy_routes` 增加 HTTPS 相关字段：`enable_https`、`cert_id`、`redirect_http`
* 渲染器根据字段生成 HTTPS `server` 块（443 端口），并可选生成 HTTP → HTTPS 重定向块
* 控制面托管证书并下发到节点本地，支持手动导入与文件导入

**2.5.2 域名管理与证书托管**

* 新增 `managed_domains` 表：管理业务域名，支持精确域名与通配符域名（如 `*.example.com`）
* 新增 `tls_certificates` 表：保存证书与私钥，支持手动粘贴导入和证书文件上传导入
* 控制面新增证书管理与域名管理页面
* 在反代规则编辑时，输入域名后自动匹配可用证书（包含通配符匹配）

**2.5.3 Agent 管理与自动发现**

* 管理端支持手工创建节点、编辑节点名、删除节点
* `nodes` 表增加 Agent 鉴权 Token 与自动发现 Token 字段
* 首次接入不再依赖全局环境变量 Token，而是依赖管理端为节点生成的自动发现 Token
* Agent 首次注册成功后，Server 下发节点专属 Agent Token，Agent 本地完成 Token 置换
* Agent 默认自动探测主机名与 IP，也允许通过配置覆盖

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
* 节点分组与差异化下发
* 对象存储、消息队列、Redis、Prometheus
* 证书自动签发（ACME）
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
* `cert_id` — 关联托管证书 ID（nullable，未启用 HTTPS 时可为空）
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

第二版沿用第一版字段，不新增分组字段。

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

第二版沿用第一版字段，不新增分组字段。

### 5.4 apply_logs（第一版）

节点应用记录。

建议字段：

* `id`
* `node_id`
* `version`
* `result`
* `message`
* `created_at`

### 5.5 tls_certificates（第二版新增）

证书托管表，用于保存证书与私钥内容。

建议字段：

* `id`
* `name` — 证书名称（唯一）
* `cert_pem` — 证书 PEM 内容
* `key_pem` — 私钥 PEM 内容
* `not_before` — 证书生效时间
* `not_after` — 证书过期时间
* `remark`
* `created_at`
* `updated_at`

### 5.6 managed_domains（第二版新增）

域名管理表，用于维护可选域名及其默认证书关系。

建议字段：

* `id`
* `domain` — 域名（支持精确域名和 `*.example.com`）
* `cert_id` — 关联 `tls_certificates.id`（nullable）
* `enabled`
* `remark`
* `created_at`
* `updated_at`

### 5.7 nodes（第二版扩展）

节点表在第二版增加节点管理与自动发现字段。

新增字段建议：

* `agent_token` — 节点专属 Agent Token，用于注册完成后的正式鉴权
* `discovery_token` — 自动发现 Token，仅用于首次接入

约束：

* `agent_token` 与 `discovery_token` 都应为随机生成值
* `discovery_token` 仅用于首次接入，注册成功后应失效或清空
* 删除节点后，该节点关联的 Token 必须立即失效

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

第一版不在控制中心管理证书，第二版开始支持证书托管。

约定如下：

* 第一版：Nginx 的监听端口、证书、TLS 相关配置由节点本地预先准备
* 第二版：控制中心托管证书并在配置下发时生成对应证书文件与 HTTPS 配置引用
* 第二版：反代规则可通过 `cert_id` 绑定证书，并支持 HTTP → HTTPS 重定向

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

* `GET /api/tls-certificates/` — 证书列表
* `POST /api/tls-certificates/` — 手动导入证书（粘贴 PEM）
* `POST /api/tls-certificates/import-file` — 证书文件导入
* `PUT /api/tls-certificates/:id` — 更新证书备注/状态
* `DELETE /api/tls-certificates/:id` — 删除证书
* `GET /api/managed-domains/` — 域名列表
* `POST /api/managed-domains/` — 创建域名并可绑定默认证书
* `PUT /api/managed-domains/:id` — 更新域名配置
* `DELETE /api/managed-domains/:id` — 删除域名
* `GET /api/tls-certificates/match?domain=` — 按输入域名返回匹配证书（支持 `*.example.com`）
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

* Agent 正式鉴权改为查 `nodes.agent_token`
* 首次注册使用 `nodes.discovery_token`
* 不再依赖全局环境变量 Agent Token
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
* 证书选择（自动匹配候选证书，支持通配符）
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

* 发布前展示配置预览与变更摘要

### 12.4 节点页（第一版，已实现）

展示：

* 节点名
* IP
* 在线状态
* 当前版本
* 最后心跳时间
* 最近错误

### 12.5 应用记录页（第一版，已实现）

展示：

* 节点
* 版本
* 成功/失败
* 错误信息
* 时间

### 12.6 节点管理页（第二版增强）

展示：

* 节点名
* Node ID
* 自动发现 Token（仅待接入节点展示）
* 在线状态
* 当前版本
* 最后心跳时间
* 最近错误

动作：

* 创建节点
* 编辑节点名
* 删除节点

### 12.7 证书管理页（第二版新增）

展示：

* 证书名称
* 有效期（起止时间）
* 绑定域名数量
* 备注

动作：

* 手动导入证书（粘贴 PEM）
* 文件导入证书
* 删除证书

### 12.8 域名管理页（第二版新增）

展示：

* 域名（支持 `*.example.com`）
* 绑定证书
* 是否启用
* 备注

动作：

* 创建域名
* 绑定/更换证书
* 删除域名

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
    tls_certificate.go # 证书管理
    managed_domain.go  # 域名管理
    node.go            # 节点管理
  model/
    tls_certificate.go # TLSCertificate 模型
    managed_domain.go  # ManagedDomain 模型
  service/
    tls_certificate.go # 证书导入与匹配逻辑
    managed_domain.go  # 域名管理逻辑
    node.go            # 节点管理与自动发现逻辑
    renderer.go        # 抽离渲染逻辑（HTTPS 支持扩展）
  middleware/
    agent-auth.go      # 改为查节点专属 Token 验证
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

* `sync`: 拉取包含 HTTPS 与证书引用的渲染配置并应用
* `nginx`: 写入控制面托管证书生成的本地文件并参与 `nginx -t` / reload

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
2. 域名管理与证书托管（managed_domains/tls_certificates + 证书导入 + 自动匹配）
3. Agent 管理（节点 CRUD + discovery token + 节点专属 agent token）
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

* HTTPS 支持由控制面托管证书，但只支持导入，不做自动签发与自动续期
* 第二版不做节点分组，所有节点继续消费同一份激活版本
* 节点专属 Token 不做额外权限分级，第二版仅区分 discovery token 与 agent token 两种用途
* 路由自定义头不做模板变量，只支持静态 key-value，避免过早引入 DSL
* 配置预览只展示渲染结果，不实际验证 Nginx 语法，真实校验仍由 Agent 完成

第二版成功标准：

```text
HTTPS 路由可生效 + 控制面可托管证书并按域名自动匹配（含通配符）+ 节点可通过 discovery token 自动接入并完成 token 置换 + 发布前可预览变更
```
