# ATSFlare MVP 设计文档

## 1. 目标

先做一个能用的版本，不做平台化过度设计。第一版只解决 3 件事：

* 配置发布与同步
* 节点心跳检测
* Nginx 反向代理配置下发

系统定位是内部自用的控制面，不是面向外部租户的 CDN SaaS。

---

## 2. 第一版范围

### 要做

* Web 管理端维护反代规则
* 配置发布生成版本
* Agent 定时同步并应用配置
* Agent 控制本机 Nginx 校验与 reload
* 节点注册、心跳、在线状态展示
* 展示每个节点当前生效版本和最近一次应用结果

### 不做

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

第一版只保留最少的数据模型。

### 5.1 proxy_routes

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

### 5.2 config_versions

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

### 5.3 nodes

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

### 5.4 apply_logs

节点应用记录。

建议字段：

* `id`
* `node_id`
* `version`
* `result`
* `message`
* `created_at`

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

### 11.1 管理端 API

* `GET /api/routes`
* `POST /api/routes`
* `PUT /api/routes/:id`
* `DELETE /api/routes/:id`
* `GET /api/versions`
* `POST /api/versions/publish`
* `POST /api/versions/:id/activate`
* `GET /api/nodes`
* `GET /api/apply-logs`

### 11.2 Agent API

* `POST /api/agent/register`
* `POST /api/agent/heartbeat`
* `GET /api/agent/version/active`
* `GET /api/agent/versions/:version`
* `POST /api/agent/apply-result`

### 11.3 鉴权方案

管理端：

* 直接沿用 gin-template 的登录态

Agent：

* 第一版使用预共享 Token
* 例如请求头 `X-Agent-Token`
* 后续再升级 mTLS

---

## 12. 页面设计

第一版只保留最少页面。

### 12.1 登录页

沿用 gin-template 现有登录。

### 12.2 反代规则页

展示和编辑：

* 域名
* 源站地址
* 是否启用
* 备注

### 12.3 发布版本页

展示：

* 版本号
* 发布时间
* 发布人
* 是否当前激活

动作：

* 立即发布
* 激活旧版本

### 12.4 节点页

展示：

* 节点名
* IP
* 在线状态
* 当前版本
* 最后心跳时间
* 最近错误

### 12.5 应用记录页

展示：

* 节点
* 版本
* 成功/失败
* 错误信息
* 时间

---

## 13. 代码组织建议

### Server

建议直接在现有 `atsf_server` 下新增以下内容：

```text
atsf_server/
  controller/
    route.go
    version.go
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
    publisher.go
    renderer.go
```

### Agent

建议新建独立目录：

```text
atsf_agent/
  cmd/agent/main.go
  internal/config/config.go
  internal/heartbeat/heartbeat.go
  internal/sync/sync.go
  internal/nginx/nginx.go
  internal/state/state.go
```

---

## 14. 开发顺序

按下面的顺序最稳。

### 第一阶段

先把 Server 跑通：

* 建表
* 反代规则 CRUD
* 发布版本表
* 版本渲染逻辑

### 第二阶段

做 Agent 最小闭环：

* 注册
* 心跳
* 拉取激活版本
* 写入 Nginx 路由配置
* `nginx -t && nginx -s reload`
* 应用结果上报

### 第三阶段

补管理端页面：

* 规则页
* 版本页
* 节点页
* 应用日志页

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
