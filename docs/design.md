# OpenFlare 设计基线

本文档定义 OpenFlare `1.0.0` 之后仍然有效的产品边界、系统结构与长期约束。第六版已经完成并并入正式版；过程性设计不再在这里维护。

## 1. 产品定位

OpenFlare 是一套自托管的 OpenResty 控制面，面向单团队或单组织内部运维场景，解决反向代理配置、节点同步、证书托管与基础观测的统一管理问题。

当前稳定能力包括：

* 反代规则管理
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

## 2. 范围边界

当前明确不做：

* 多租户
* CDN SaaS 化能力
* GeoDNS、全球调度、智能选路
* WAF、Bot 管理、限流平台化
* 灰度百分比发布、按节点差异化配置
* 对象存储、消息队列、Prometheus、ClickHouse、Kafka 等前置基础设施
* 通用日志检索平台、APM、调用链系统、任意 BI 报表
* 证书自动签发与自动续期
* 平台化抽象对象，如 `zone`、`origin_pool`、`policy`、`deployment`

边界补充：

* OpenResty 代理缓存只作为当前反代链路优化能力存在，不扩展为独立缓存产品
* 源站对象只用于沉淀可复用的回源地址，不扩展为带健康检查、权重、策略编排的 `origin_pool`
* 主配置文件由 Server 统一渲染并由 Agent 受控写入，不支持节点侧手工编辑后回传合并
* 节点观测聚焦运营与运维排障所需的摘要、趋势和受控窗口数据，不提供长期日志托管

新增能力如果超出上述边界，先更新本文档，再进入实现。

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

* 一个域名只对应一条 `proxy_routes` 规则
* `origins` 只保存源站地址、展示名与备注，不承载协议、端口、路径、权重或健康检查策略
* `proxy_routes` 可选关联一个 `origins` 记录，用于复用源站地址；规则仍保存完整 `origin_url` 快照以参与渲染与版本快照
* `proxy_routes` 至少包含一个上游地址；为兼容历史数据保留 `origin_url` 主上游字段，也允许在同一规则内补充多个上游做负载均衡
* `proxy_routes` 上游统一渲染为带 keepalive 的 named `upstream`；单上游可附带 base path 或 query 并在 `proxy_pass` 中追加，多上游仍限定为纯 `scheme://host[:port]`
* `proxy_routes.origin_host` 为可选字段，用于回源时覆盖 `Host` 请求头；未设置时默认透传访问域名
* `proxy_routes.domain` 必须唯一
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
