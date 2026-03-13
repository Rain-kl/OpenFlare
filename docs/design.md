# ATSFlare 设计基线

## 1. 文档目的

本文档只保留 ATSFlare 当前有效的产品边界、系统结构与稳定约束。

当前状态：

* 第一版、第二版、第三版均已完成
* 前端改造已完成，`atsf_server/web` 新版工程已成为正式基线
* 第五版（0.5.x）已立项，目标聚焦 OpenResty 反代与缓存性能优化
* 已完成阶段的实现细节以代码与 Git 历史为准，不再在本文档中维护过程性设计

---

## 2. 产品定位

ATSFlare 当前定位为内部自用的反向代理控制面，不面向外部租户提供 CDN SaaS 能力。

当前核心能力：

* 反代规则管理
* 配置预览、发布、激活与回滚
* Agent 注册、心跳、同步、应用结果上报
* Agent 上报 OpenResty 运行健康状态与错误摘要
* OpenResty 配置写入、校验、reload 与失败回滚
* Server 向 Agent 下发受限运行指令（当前仅支持 OpenResty 重启）
* Server 统一管理 OpenResty 主配置模板与性能优化参数
* 反向代理链路的连接、缓冲、超时、压缩与缓存性能优化
* HTTPS/TLS 路由支持
* 证书托管与域名管理
* 节点管理、节点专属 `agent_token`、全局 `discovery_token`
* 配置变更摘要
* Agent 运行参数下发
* Agent 自我更新、一键部署、正式版默认更新与手动 preview 更新
* Server 版本检查、正式版默认自升级、手动 preview 检查升级与手动上传二进制确认升级
* 新版管理端 UI、主题切换与统一交互框架

默认工作方式：

* 所有节点消费同一份全局激活版本
* 控制面保存配置与状态，不直接 SSH 管理机器
* Agent 是节点侧唯一落地入口

---

## 3. 范围边界

当前明确不做：

* 多租户
* WAF、限流防护平台化、Bot 管理
* 节点分组、灰度百分比发布、按节点差异化下发
* Redis、消息队列、对象存储、Prometheus 等新基础设施前置依赖
* 平台化缓存产品能力、分层缓存、mid-tier
* 证书自动签发与自动续期
* 审批流、审计中台、Purge 平台化能力
* 平台化抽象对象，如 `zone`、`origin_pool`、`policy`、`deployment`

第五版边界补充：

* 允许在 OpenResty 单节点层面引入受控的代理缓存能力，但只服务于当前反代链路优化，不扩展为独立缓存产品
* 主配置文件由 Server 统一生成并由 Agent 受控落地，不开放节点侧手改后再回传合并
* 性能优化参数统一在 Server 配置，不支持按节点分叉不同性能模板
* 第五版不开放任意自定义 Nginx/OpenResty 片段上传，不提供任意指令执行入口, 但是需要保留拓展能力, 为后续开放准备
* 允许在管理端提供 OpenResty 主配置模板编辑能力，但模板必须保留 ATSFlare 预留占位符，确保性能参数、证书目录与受管路由 include 继续由 Server 统一渲染

新增能力超出上述边界时，必须先更新本文档，再进入实现。

---

## 4. 技术基线

### 4.1 Server

`atsf_server` 继续作为单体控制面：

* Gin
* GORM
* SQLite
* 现有 ATSFlare 登录体系
* 托管 `atsf_server/web` 静态构建产物
* 托管 OpenResty 主配置模板、性能参数与缓存参数

### 4.2 Agent

`atsf_agent` 继续作为 Go 单体程序：

* 单二进制
* 节点本地执行
* `openresty_path` 优先
* 未配置 `openresty_path` 时默认使用 Docker OpenResty
* 生成资源默认落在 `./data`，可由 `data_dir` 覆盖
* 负责接管 OpenResty 主配置文件与受管 include 文件

### 4.3 Frontend

`atsf_server/web` 作为正式管理端前端基线：

* Next.js App Router
* React 19
* TypeScript
* Tailwind CSS
* 静态导出，继续由 Go Server 托管

---

## 5. 总体架构

```text
ATSFlare Server (Gin + SQLite + Web UI)
        |
        | HTTP API / Config Pull
        v
ATSFlare Agent (register / heartbeat / sync / apply / update)
        |
        v
  Local OpenResty or Docker OpenResty
        |
        v
      Origin
```

职责分工：

* Server 负责配置、版本、节点、设置、管理端 UI 以及 OpenResty 主配置模板渲染
* Agent 负责本地落盘、校验、reload、回滚、自更新，不负责维护独立于 Server 的主配置真相
* 发布通过“生成完整版本并激活”完成
* 历史版本不可变
* Agent 常规轮询以 heartbeat 为主；heartbeat 响应返回当前激活版本的 `version` 与 `checksum` 摘要，Agent 仅在发现不一致时再拉取完整配置

---

## 6. 核心对象

当前有效实体：

* `proxy_routes`：域名到源站的反向代理规则
* `config_versions`：完整发布快照与渲染结果
* `nodes`：节点状态、版本、凭证与 Agent 设置相关状态
* `apply_logs`：节点应用版本结果
* `tls_certificates`：托管证书与私钥
* `managed_domains`：域名资产及默认证书关系

稳定约束：

* 一个域名只对应一个 `origin_url`
* `proxy_routes.domain` 必须唯一
* `origin_url` 必须为合法 `http://` 或 `https://`
* `config_versions` 必须保存完整快照、渲染结果与 `checksum`
* 全局同时只能有一个激活版本
* 回滚通过重新激活旧版本实现
* 激活版本中的 OpenResty 渲染结果必须包含主配置与路由配置的统一快照
* 域名与证书匹配同时支持精确匹配与通配符匹配
* 节点专属 `agent_token` 必须可立即失效
* OpenResty 性能优化参数与缓存参数统一由 Server 设置管理，不允许节点侧形成额外配置源
* OpenResty 主配置模板允许编辑，但必须保留系统要求的占位符，不能绕过结构化参数校验与受管 include 注入
* Agent 只应用受控主配置文件，不提供任意配置片段拼接入口

---

## 7. 发布模型

标准链路：

```text
修改规则 -> 预览/查看 diff -> 发布 -> 生成完整配置版本 -> 激活版本 -> Agent 拉取 -> 本地应用 -> 上报结果
```

发布规则：

1. 读取全部启用的 `proxy_routes`
2. 读取 Server 侧受管 OpenResty 性能参数与缓存参数
3. 渲染完整 OpenResty 配置
4. 计算 `checksum`
5. 写入 `config_versions`
6. 切换激活版本
7. Agent 在后续同步中发现并应用

版本规则：

* 版本号格式：`YYYYMMDD-NNN`
* 版本不可变
* 节点只拉取当前激活版本

---

## 8. 模块边界

### 8.1 `atsf_server`

负责：

* 管理端 UI 与 API
* Agent API
* 数据存储
* 配置渲染
* OpenResty 主配置模板管理
* OpenResty 性能参数与缓存参数管理
* 发布与激活
* 节点状态与设置管理

### 8.2 `atsf_agent`

负责：

* 首次注册与凭证置换
* 周期性心跳与同步
* 运行参数接收
* 主配置文件、路由配置与必要证书文件写入
* 执行 `openresty -t` / `openresty -s reload`
* 失败回滚
* 自我更新
* 应用结果上报

### 8.3 `atsf_server/web`

负责：

* 管理端页面、布局、交互与主题
* 规则、版本、节点、证书、域名、用户、设置、性能等页面
* 统一请求层与前端状态管理

---

## 9. 接口域

管理端接口当前覆盖：

* `proxy-routes`
* `config-versions`
* `nodes`
* `apply-logs`
* `tls-certificates`
* `managed-domains`
* `users`
* `settings`
* `update`

Agent 接口当前覆盖：

* 注册
* 心跳
* 获取激活版本
* 上报应用结果
* 通过心跳回传 OpenResty 健康状态并接收受限运行指令

统一约束：

* 管理端与 Agent API 均使用 JSON
* Agent API 固定放在 `/api/agent/*`
* Agent 鉴权统一使用 `X-Agent-Token`
* OpenResty 性能优化相关配置通过现有设置域统一管理，不新增节点直连配置入口

---

## 10. 文档维护原则

后续只维护当前有效基线：

* 产品范围或系统边界变化时更新本文档
* 已完成阶段的步骤不再回填为长期计划
* 新阶段开始前，先补设计，再进入实现
