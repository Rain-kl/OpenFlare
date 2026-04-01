# OpenFlare 设计基线

本文档定义 OpenFlare `1.0.0` 之后仍然有效的产品边界、系统结构与长期约束。第六版已经完成并并入正式版；过程性设计不再在这里维护。

## 1. 产品定位

OpenFlare 是一套自托管的 OpenResty 控制面，面向单团队或单组织内部运维场景，解决反向代理配置、节点同步、证书托管与基础观测的统一管理问题。

当前稳定能力包括：

* 反代规则管理
* 网站级配置与多域名绑定
* 源站管理与复用
* 配置预览、发布、激活与回滚
* Agent 注册、心跳、同步、应用结果上报
* OpenResty 主配置模板、性能参数与缓存参数托管
* HTTPS/TLS 与域名资产管理
* 节点请求聚合、资源快照、健康事件与看板展示
* 节点管理、令牌体系、部署与更新链路
* 基于 Next.js 的正式管理端前端

默认工作方式：

* 所有节点消费同一份全局激活版本
* Server 保存配置与状态，不直接 SSH 管理节点
* Agent 是节点侧唯一受控落地入口

## 3. 技术基线

### 3.1 Server

`openflare_server` 继续作为单体控制面：

* Gin
* GORM
* SQLite / PostgreSQL
* 现有登录与 Session 体系
* 托管 `openflare_server/web` 静态构建产物

### 3.2 Agent

`openflare_agent` 继续作为 Go 单体程序：

* 单二进制
* 节点本地执行
* `openresty_path` 优先
* 未配置 `openresty_path` 时默认使用 Docker OpenResty

### 3.3 Frontend

`openflare_server/web` 是正式前端基线：

* Next.js App Router
* React 19
* TypeScript
* Tailwind CSS
* 静态导出后由 Go Server 托管

## 4. 总体架构

```text
OpenFlare Server (Gin + SQLite/PostgreSQL + Web UI)
        |
        | HTTP API / Config Pull
        v
OpenFlare Agent (register / heartbeat / sync / apply / update)
        |
        v
Local OpenResty or Docker OpenResty
        |
        v
Origin
```

职责分工：

* Server 负责配置、版本、节点、设置、证书、管理端 UI 与聚合查询
* Agent 负责本地写入、校验、reload、回滚、自更新与轻量采集
* 发布通过“生成完整版本并激活”完成
* 历史版本不可变
* heartbeat 响应返回激活版本摘要，Agent 仅在不一致时拉取完整配置

## 5. 核心对象

当前有效实体：

* `proxy_routes`
* `origins`
* `config_versions`
* `nodes`
* `node_system_profiles`
* `apply_logs`
* `tls_certificates`
* `managed_domains`
* `node_request_reports`
* `node_access_logs`
* `node_metric_snapshots`
* `traffic_analytics_rollups`
* `node_health_events`

稳定约束：

* `proxy_routes` 从“单域名规则”升级为“网站配置”聚合对象；一条记录对应一个网站，可绑定一个或多个域名，并共享一组站点级配置
* `proxy_routes.site_name` 是网站的业务唯一标识；新建时默认取 `domains[0]`，后续允许独立维护，不随域名改动自动重写
* `proxy_routes.domains` 至少包含一个域名，且 `domains[0]` 作为主域名；任一域名全局只能属于一个 `proxy_routes`
* 为兼容历史数据，迁移期可保留 `proxy_routes.domain` 作为 `domains[0]` 的镜像字段，但业务读写与后续扩展必须以 `site_name` + `domains` 为准
* `origins` 只保存源站地址、展示名与备注，不承载协议、端口、路径、权重或健康检查策略
* `proxy_routes` 可选关联一个 `origins` 记录，用于复用源站地址；规则仍保存完整 `origin_url` 快照以参与渲染与版本快照
* `proxy_routes` 至少包含一个上游地址；为兼容历史数据保留 `origin_url` 主上游字段，也允许在同一规则内补充多个上游做负载均衡
* `proxy_routes` 上游统一渲染为带 keepalive 的 named `upstream`；单上游可附带 base path 或 query 并在 `proxy_pass` 中追加，多上游仍限定为纯 `scheme://host[:port]`
* `proxy_routes.origin_host` 为可选字段，用于回源时覆盖 `Host` 请求头；未设置时默认透传访问域名
* 网站级流量限制、反向代理与缓存配置当前按站点共享，不在同一网站内做域名级差异化配置；但 HTTPS 允许在同一站点内按域名绑定证书
* `proxy_routes.domain_cert_ids` 用于记录与 `domains` 平行的域名证书绑定；值为 `0` 表示该域名不启用 HTTPS，仅保留 HTTP
* 发布渲染时，带证书的域名按证书分组输出独立 `443 ssl` `server` 块；未绑定证书的域名不得被自动带入 HTTPS
* 发布渲染时必须将 `proxy_routes.domains` 中的全部域名一并纳入同一站点配置，避免同站点在版本快照中被拆散
* 所有上游地址都必须为合法 `http://` 或 `https://`
* `config_versions` 必须保存完整快照、渲染结果与 `checksum`
* 全局同时只能有一个激活版本
* 回滚通过重新激活旧版本实现
* `nodes` 只承载控制面状态与低频摘要，不承载高频观测事实
* 指标、趋势和访问分析优先使用服务端聚合结果，而不是前端临时统计
* 访问明细只保留受控时间窗口，不演变成通用日志平台

## 6. 发布模型

标准链路：

```text
修改规则 -> 预览/查看 diff -> 发布 -> 生成完整配置版本 -> 激活版本 -> Agent 拉取 -> 本地应用 -> 上报结果
```

发布规则：

1. 读取全部启用的 `proxy_routes`
2. 读取 Server 侧 OpenResty 主配置与结构化参数
3. 渲染完整 OpenResty 配置
4. 计算 `checksum`
5. 写入 `config_versions`
6. 切换激活版本
7. Agent 在后续 heartbeat 中发现并应用

版本号格式固定为 `YYYYMMDD-NNN`。

## 7. 模块边界

### 7.1 `openflare_server`

负责：

* 管理端 UI 与 API
* Agent API
* 配置渲染与版本发布
* 数据存储与聚合查询
* OpenResty 主配置模板、性能参数与缓存参数管理

### 7.2 `openflare_agent`

负责：

* 首次注册与凭证置换
* 周期性心跳与同步
* 主配置、路由配置、证书与 Lua 资源写入
* 执行 `openresty -t` / `openresty -s reload`
* 失败回滚
* 对已失败并回退的目标版本做本地熔断，直到控制面出现新的激活版本
* 节点观测采集与结果上报

### 7.3 `openflare_server/web`

负责：

* 管理端页面、布局、交互与主题
* 总览、节点详情、规则、版本、节点、证书、域名、用户与设置页面
* 统一请求层与前端状态管理

## 8. 文档维护原则

* 产品范围或系统边界变化时更新本文档
* 已完成阶段不再以“版本计划”形式回填
* 新阶段开始前，先补设计，再进入实现
* 涉及网站级规则改造的详细需求与实施顺序，见 [docs/website-configuration-redesign.md](./website-configuration-redesign.md)
