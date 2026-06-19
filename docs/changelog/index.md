---
sidebar: false
---

# 更新日志

本文件记录 OpenFlare 每个版本的重要变更。

格式基于 [Keep a Changelog](http://keepachangelog.com/)，版本号遵循 [语义化版本](http://semver.org/)。

## 重大变更

> [!IMPORTANT]
> 2.3.2 开始使用 JWT_SECRET 环境变量替代 SESSION_SECRET 进行管理端 API 的 JWT 签名密钥管理。SESSION_SECRET 将会在之后的版本中逐步废弃，请务必尽快迁移到 JWT_SECRET。


## [Unreleased]

### 移除

- 移除 Wavelet About 界面相关代码：删除 `AboutService` 与 `GET /api/v1/d/about` 接口（页面从未实现，属冗余遗留）。
- 移除 Wavelet 中从旧系统迁移但未实装的全局 API / Web / 敏感接口限流选项（`GlobalApiRateLimit*`、`GlobalWebRateLimit*`、`CriticalRateLimit*`）及相关校验与数据库种子。
- 移除 `internal/apps/openflare/legacy/` 控制台兼容层、`compat/auth.go`（`OpenFlare-Token` JWT 桥接）及前端 `legacy-base.service.ts`；`openflare-server/web` 不再能对接当前 Wavelet 后端管理 API。
- 移除旧 OpenFlare 服务升级兼容层（`/api/v1/openflare/update/*`）；统一使用 Wavelet 框架 `admin/updater`（`/api/v1/admin/update`）。

### 新增

- Wavelet 总览仪表盘图表对齐旧前端 ECharts 样式：24 小时请求/容量趋势补充请求量与错误量展示，新增网络与磁盘趋势、Top 节点榜单、来源分布、状态码分布与 Top Domain 板块。
- Wavelet 总览仪表盘迁移旧前端「全球态势板」：世界地图展示节点健康与来源国家热度，并保留在线覆盖、流量与容量摘要指标。
- Wavelet 全球态势板视觉对齐系统风格：改用 `Card` / `Badge` / `Progress` 与虚线边框指标卡，地图配色与 tooltip 跟随主题 token。
- Wavelet 全球态势板布局重构：关键指标前置为摘要栅格，地图与容量诊断并排展示，移动端自动堆叠。
- 修复 Wavelet 全球态势板 ECharts 地图在紧凑布局下容器尺寸为 0 导致无法渲染的问题。
- Wavelet 全球态势板地图容器调整为约 2:3（宽:高）比例，避免世界地图横向过度拉伸。
- Wavelet WAF / 代理规则认证实装完整 PoW 配置面板：支持难度、算法、TTL 与路径/IP/UA 黑白名单编辑。
- Wavelet WAF 新建/编辑规则组改用右侧 Sheet 抽屉，替代居中弹窗，与站点绑定等操作保持一致的 shadcn 侧栏交互。

- 将 `openflare-server` 后端业务域迁移至 `Wavelet/internal/apps/openflare/`；管理控制台 API 挂载于 `/api/v1/d/*`，节点协议挂载于 `/api/v1/agent|relay|tunnel/*`。
- 新增 OpenFlare 业务表 goose 迁移（`of_options`、`of_origins`、`of_proxy_routes`、`of_nodes`、`of_waf_*`、`of_tls_*`、`of_config_versions`、`of_pages_*`、`of_apply_logs` 等）。
- 重叠职能复用 Wavelet 内置用户/OAuth/Cap/认证源能力；新增 `integration` 包覆盖认证、核心链路、安全、Agent 协议集成测试。
- 修复 Wavelet 引入 `openflare` 模块后 `gomodule/redigo v2.0.0+incompatible` 导致 `redistore` 编译失败的问题。
- Wavelet 后端默认监听端口由 `:8000` 调整为 `:3000`，与旧 OpenFlare Server 保持一致。
- 新增 OpenFlare 可观测性表 goose 迁移（`of_node_system_profiles`、`of_node_metric_snapshots`、`of_node_request_reports`、`of_node_health_events`、`of_node_obs_openresty`、`of_node_obs_frps`、`of_node_access_logs`）。
- Agent heartbeat 恢复可观测性数据持久化（系统画像、指标快照、流量报表、健康事件等）。
- Wavelet 默认数据库名由 `wavelet` 调整为 `openflare`（PostgreSQL、ClickHouse、SQLite 后备路径同步更新）。
- Wavelet 默认 PostgreSQL `application_name` 由 `wavelet-server` 调整为 `openflare-server`，Redis 键前缀由 `wavelet:` 调整为 `openflare:`。
- 实装 Agent WAF IP 组同步（heartbeat `waf_ip_groups` 增量下发与 `/api/v1/agent/waf/ip-groups/sync`）。
- 实装 Pages Agent 部署包下载（`/api/v1/agent/pages/deployments/:deployment_id/package` 二进制响应）。

- 实装访问日志单表查询层（列表、折叠、IP 汇总/趋势、地域统计）及 `(node_id, logged_at)` 复合索引。
- 扩展 Relay/Tunnel heartbeat 载荷与可观测性持久化（frps 观测、健康事件）；新增 `of_node_obs_frpc` 单表。
- Agent heartbeat 恢复 Geo 自动更新、访问日志地域解析与 90 天保留清理；对齐 config `support_files` 过滤规则。
- OAuth 登录/绑定统一由 Wavelet `/api/v1/oauth/:source/*` 提供；旧 `/api/oauth/*` 快捷路径未保留。
- 新增 `internal/apps/openflare/tasks/` 集中承载 OpenFlare 定时/后台任务业务逻辑；调度已迁入 Wavelet Asynq 异步任务框架（`async_tasks.go` + `w_schedules` 种子迁移），含数据库可观测性自动清理、WAF IP 组周期同步、UptimeKuma 同步、ACME 证书自动续期。
- Pages 部署包上传改为通过 Wavelet `upload.Ingest` 写入本地文件存储框架，部署记录关联 `upload_id`；Agent 下载改为从存储后端流式读取。
- 实装数据库可观测性手动/自动清理、WAF IP 组订阅/自动同步与测试接口、UptimeKuma 监控同步、TLS ACME 申请/续期（lego DNS-01）。
- 修复 Wavelet Agent WebSocket 未处理 `status` 消息导致 WS 模式下 `last_seen_at` 停止更新、节点超时显示离线的问题；列表「最近心跳」恢复显示「WS 已连接」。
- 修复节点「强制同步」仍为 stub 导致始终返回「节点不在线或通过 WebSocket 发送同步指令失败」的问题。
- 将 OpenFlare 管理控制台从 `openflare-server/web` 迁移至 `Wavelet/frontend/`，复用 Wavelet shadcn 组件与 Session 鉴权；Service 层对接 `/api/v1/d/*`（`OpenFlareBaseService`）。
- 实装节点、代理规则（6 Section）、配置发布、WAF、网站/证书/DNS、Pages、源站、访问/应用日志、仪表盘、性能调优等业务页面；Admin 设置新增 OpenFlare 运维扩展 Tab。
- 修复静态导出构建中 `useSearchParams` 未包裹 Suspense 的问题；`pnpm build:embed` 全量 46 页通过。
- 补全前端迁移收尾：新增 `/openflare/about` 关于页与 `AboutService`；补全 `UpdateService` 升级流程（在线升级、手动上传、WebSocket 日志流）及顶栏版本升级入口。
- 前端 UI 打磨：节点/WAF/DNS/配置清理等 Dialog 补 RHF+Zod；代理规则详情与访问日志 Tab 补 error 态；`pnpm build:embed` 增至 47 静态页。
- 修复开发模式下 `/api/*` 尾斜杠引发 Next.js 308 与 Gin 301 循环重定向：前端 Service 与 `api-client` 请求拦截器规范化路径、强制浏览器同源代理、Next `skipTrailingSlashRedirect`、后端 `RegisterCollection` 双路径注册；Access Token 管理员权限校验对齐 `token_admin`。
- 补全 Wavelet 边缘节点详情数据看板：迁移运行诊断摘要、系统信息、实时资源、网络流量、24 小时请求/容量/网络/磁盘趋势、请求结构分布与健康事件时间线。
- 节点详情页改为 Tabs 布局（概览 / 数据看板 / 配置与部署），统一 Edge/Relay/Tunnel 详情壳层并支持 `tab` URL 参数深链。
- 节点详情 24 小时网络/磁盘 IO 趋势图对齐原版 ECharts 样式，恢复系列摘要卡与平滑面积折线图。
- 侧栏新增「网站管理」折叠组（shadcn Collapsible + SidebarMenuSub），收纳网站、证书、DNS、源站入口。
- 移除网站、证书、DNS 页面右上角互相跳转按钮，改由侧栏折叠组统一导航。
- 侧栏导航改为 `openflareSidebarNav` 单一配置源，折叠组位置由数组顺序决定，移除 `slice(0, 4)` 硬编码切分。
- 侧栏新增「安全性」折叠组，收纳 WAF 与 IP 组入口；同步移除两页右上角互相跳转按钮。
- OpenFlare 前端路由去除 `/openflare` 前缀，业务页面直接挂载于 `/`（如 `/nodes`、`/websites`）；侧栏移除「首页」「我的文件」；旧路径 `/openflare/*`、`/home` 永久重定向至新路径。

### 变更

- Wavelet API 路径统一：管理端由 `/api/v1/openflare/*` 调整为 `/api/v1/d/*`；Agent/Relay/Tunnel 协议路由分别迁移至 `/api/v1/agent/*`、`/api/v1/relay/*`、`/api/v1/tunnel/*`（原 `/api/flared/*`）。同步更新 Wavelet 前端服务层与 `openflare-agent`、`openflare-relay`、`openflared` 客户端连接端点。
- Agent/Relay/Tunnel 协议 API 响应格式对齐 Wavelet `{error_msg, data}`：服务端移除 `compat` 包，业务错误改为 HTTP 4xx + `error_msg`；同步更新 `openflare-agent`、`openflare-relay`、`openflared` HTTP 客户端解析逻辑。
- OpenFlare 管理端权限模型对齐 Wavelet：取消旧系统 Admin/Root 三级角色区分，统一以 `user.IsAdmin` 为管理门槛；Access Token 访问敏感接口（Option、Update 等）须 `token_admin=true`；权限不足返回 HTTP 404 + `error_msg`，参数错误返回 HTTP 400 + `error_msg`（`apiutil.AdminMiddlewares` = `oauth.LoginRequired` + `admin.LoginAdminRequired`）。
- 移除 Wavelet 顶栏 OpenFlare 服务端版本入口按钮；版本升级能力保留在 Admin 设置 OpenFlare 运维 Tab。
- 将 Wavelet 默认上游仓库调整为 `Rain-kl/OpenFlare`，站点名称、邮件模板、前端默认标题与页脚品牌统一初始化为 OpenFlare；新增 goose 迁移回填既有环境的旧 Wavelet 默认值。
- Wavelet WAF IP 组列表新增「查看」操作，支持在查看弹窗中浏览当前 IP 并移除单条 IP；自动类型同步裁剪 `ext_ips` 以与 `ip_list` 保持一致。
- 系统默认关闭用户自主注册（`registration_enabled` / `password_register_enabled`），默认开启登录 Cap 人机验证（`cap_login_enabled`）。
- 将 OpenFlare 业务 API 路由注册从 `internal/apps/openflare/legacy` 迁移至 `internal/router/openflare`，与 Wavelet 路由分层规范对齐。
- 移除 `internal/router/openflare` 中与 Wavelet `/api/v1` 重复的 legacy 认证路由（用户登录/注册、OAuth、Cap、认证源等），统一由 Wavelet 内置用户/OAuth/Cap 模块提供。
- OpenFlare 路由注册统一收敛至 `internal/router/v1/openflare/`（`register_*.go` + `RegisterV1Routes`）；`internal/apps/openflare/*/routers.go` 仅保留 Handler 与业务逻辑。
- 将 OpenFlare 路由注册包从 `internal/router/openflare/` 迁移至 `internal/router/v1/openflare/`，与 v1 路由分层目录结构对齐。
- OpenFlare 管理端路由注册从 `RegisterCustomRoutes` 移至 `v1.RegisterV1Routes`，API 前缀由 `/api/v1/custom/openflare` 调整为 `/api/v1/openflare`。
- OpenFlare 管理控制台 API 统一迁移至 `/api/v1/d/*`，响应格式对齐 Wavelet `{error_msg, data}` + `response.Abort*`。
- 节点协议路由迁移至 `/api/v1/agent|relay|tunnel/*`（`flared` 更名为 `tunnel`）；协议层响应格式统一为 Wavelet `{error_msg, data}` + `response.Abort*`，移除 `compat` 遗留信封。
- 控制台鉴权改为 `apiutil.AdminMiddlewares()`（`oauth.LoginRequired` + `admin.LoginAdminRequired`；Session / Access Token）。
- 前端 OpenFlare Service 层切换为 `OpenFlareBaseService`（`BaseService` + `/api/v1/d/*`）。
- 为 OpenFlare 管理端 API 补充 Swagger 注解（约 99 个端点）。
- 移除已无引用的 `internal/apps/openflare/auth/`、`compat/routes.go`。

## [v2.3.4] - 2026-06-17

### 变更

- 访问日志列表查询将分页与计数下推到数据库执行，避免百万级数据全量加载到内存。
- 访问日志 `total_ip` 统计改为 SQL `UNION` + `COUNT(*)` 下推执行，分片计数与分页查询并行化。
- 访问日志折叠视图、IP 汇总与趋势改为 SQL `GROUP BY` 聚合；过滤条件改为 `node_id` 精确匹配及其他字段前缀匹配以利用索引。
- 标准化 Server Go 目录结构，引入 `cmd/server`、`openflare-server/internal` 与根级 `pkg` 分层，并拆分原 `utils` 公共能力包。

## [v2.3.3] - 2026-06-06

### 新增

- 新增密码登录人机验证（基于 Proof-of-Work 和无感浏览器检测的 Cap 验证码防护）
- 新增后端 PoW 校验服务，实现 FNV-1a/XORShift PRNG 难题生成、验证及 JWT 难题校验算法，支持基于路由路径参数 `scope` 进行验证流的强校验与安全隔离
- 新增线程安全的内存 TTL 核销缓存，支持高并发与 Single-use 难题令牌防重放
- 新增 Gin 拦截中间件与参数化路由 `/api/cap/:scope/challenge` 和 `/api/cap/:scope/redeem`，登录接口 `POST /api/user/login` 自动从 HTTP 请求头校验 `X-Cap-Token` 并放行
- 前端登录页集成 cap-widget 组件，配置 `/api/cap/login/` 隔离端点按需加载 CDN 脚本，实现静默 PoW 求解与令牌提交
- 管理后台系统设置页“登录与注册开关”中新增“启用登录人机验证”开关，支持热更新全局防护状态
- 新增 Agent 交互式安装向导，支持选择本地安装和 Docker 运行模式；未传参数时自动进入交互菜单
- 新增 Docker 运行模式的智能环境检查，检测到未安装 Docker 时支持一键在线安装，中国大陆环境支持多镜像源自动测速优选与加速器配置
- 新增 Agent 交互式卸载向导，支持选择本地卸载和 Docker 容器卸载模式；未传参数时自动进入交互菜单

### 变更

- 重构 `install-agent.sh` 安装脚本与 `uninstall-agent.sh` 卸载脚本以兼容交互式导引、非交互式命令行参数及 Docker 部署/卸载参数（`--docker`/`--method docker`）
- 重构 Go 包依赖结构为统一模块（Monorepo），模块命名为 `github.com/rain-kl/openflare`
- 移除各子目录下独立的 `go.mod`/`go.sum` 文件，统一由根目录 `go.mod` 进行全局依赖管理与依赖版本锁定
- 替换全仓库 Go源文件中的内部引用路径，由本地相对路径迁移为标准 GitHub 绝对导入路径
- 适配 Docker 镜像构建，所有组件镜像的 Dockerfile 调整为基于根目录的上下文编译
- 更新 GitHub release 自动化发布流水线，适配全新 monorepo 包结构与符号信息注入路径
- 简化并重构数据库历史迁移校验逻辑，将版本 2 至 6 的中间校验函数合并到基线校验函数 `validateDatabaseSchemaV7` 中，消除冗余代码
- 重构数据库历史迁移校验架构，引入基于 GORM 反射解析（`schema.Parse`）的通用自动表结构校验，彻底废弃老版本中大量手动编写的 `HasTable`/`HasColumn` 结构字段存在性检测代码

---

## [v2.3.2] - 2026-06-04

### 说明

> [!IMPORTANT]
> 2.3.2 开始使用 JWT_SECRET 环境变量替代 SESSION_SECRET 进行管理端 API 的 JWT 签名密钥管理。SESSION_SECRET 将会在之后的版本中逐步废弃，请务必尽快迁移到 JWT_SECRET。

### 新增

- 新增 `JWT_SECRET` 环境变量，专用于管理端 API JWT 签名密钥；生产环境必须显式配置
- 新增 VitePress 更新日志页面（`docs/changelog/index.md`），记录所有版本变更历史

### 变更

- 管理端 API 鉴权框架迁移至 `gin-jwt`
- 认证方式变更为 Headers 认证.
- `JWT_SECRET` 优先于 `SESSION_SECRET` 用于 JWT 签名；未配置时回退到 `SESSION_SECRET`，向下兼容
- 屏蔽手动升级入口（`/api/update/manual-upload`、`/api/update/manual-upgrade`），前端隐藏对应 UI 组件

---

## [v2.3.1] - 2026-06-03

### 变更

- 屏蔽手动升级入口，前端隐藏对应 UI 组件
- POW 与 WAF 规则合并, 统一逻辑处理

---

## [v2.3.0] - 2026-06-03

### 新增

- WAF IP 组支持订阅模式，可从远程文本或 JSON 源定时同步
- 新增 Pages 静态站点托管，支持 SPA fallback 路由配置
- Agent 实现 WebSocket 实时推送，Server 发布配置后立即通知在线 Agent

### 变更

- Agent 数据面与 OpenResty 合并为集成镜像部署方式
- 访问日志与观测数据支持数据库分片，按 ID 分片替代原有逻辑

---

## [v2.2.8] - 2026-06-03

### 修复

- 修复多域名部署场景下跨域认证绕过安全漏洞

---

## [v2.2.6] - 2026-06-02

### 新增

- 新增 Uptime Kuma 集成，支持自动同步监控任务
- WAF 新增 PoW（工作量证明）防护能力，可配置有效期

### 变更

- 内网穿透支持 TunnelRelay 中继节点（frps），新增 OpenFlared 客户端（frpc）

---

## [v2.2.5] - 2026-06-02

### 新增

- 新增 WAF 自动 IP 组，支持基于 Expr 规则定时聚合请求日志更新名单
- WAF IP 组黑白名单支持直接引用 IP 组对象

### 变更

- WAF 规则组与网站解耦，支持全局规则组和自定义规则组独立管理

---

## [v2.2.4] - 2026-06-02

### 新增

- WAF 规则组新增拦截返回配置 Tab

### 修复

- 修复 WAF 配置发布后部分规则不生效的问题

---

## [v2.2.3] - 2026-06-02

### 新增

- 新增 WAF 安全防护模块，支持 IP 黑白名单和地域拦截规则

---

## [v2.2.2] - 2026-06-01

### 变更

- 观测数据支持按时间窗口自动清理，新增数据库自动清理调度器

---

## [v2.2.1] - 2026-06-01

### 修复

- 修复仪表板概览数据压缩与规范化问题

---

## [v2.2.0] - 2026-06-01

### 新增

- 新增 TLS 证书转换为 ACME 托管证书的接口（`/convert-acme`）
- 新增 ACME 账号与 DNS 账号管理页面
- 支持 Let's Encrypt 自动申请与续期

---

## [v2.1.1] - 2026-06-01

### 变更

- Agent 架构调整，采用集成镜像方式内置 OpenResty

---

## [v2.0.3] - 2026-05-31

### 修复

- 修复版本号生成逻辑，确保使用当日最大序列号

---

## [v2.0.1] - 2026-05-30

### 修复

- 修复 GitHub 登录逻辑异常

---

## [v2.0.0] - 2026-05-30

### 新增

- 全面重构发布模型，引入配置版本不可变快照机制
- 支持配置版本回滚（重新激活旧版本）
- 新增 `source_config_json` 与 `support_files` 供 Agent 获取完整配置包
- 新增节点专属 Agent Token 与 Discovery Token 双轨鉴权

### 变更

- 数据库迁移框架切换至 goose，统一管理版本升级步骤
- Agent API 与管理端 API 鉴权完全分离

---

## [v1.9.3] - 2026-05-30

### 修复

- 修复节点 IP 自动探测逻辑，优先使用公网地址

---

## [v1.9.2] - 2026-05-29

### 变更

- Agent 心跳超时后自动退回 HTTP 轮询模式

---

## [v1.9.1] - 2026-05-29

### 修复

- 修复 Agent WebSocket 升级失败时的重连逻辑

---

## [v1.9.0] - 2026-05-29

### 新增

- Agent 支持 WebSocket 长连接，Server 发布后实时推送配置变更

---

## [v1.8.0] - 2026-05-26

### 新增

- 支持自定义 DNS 解析器（`OpenRestyResolvers`）
- 新增历史配置快照清理功能

### 变更

- CORS 配置支持动态源与凭证
- 上游统一渲染为命名 `upstream` 并启用 keepalive

---

## [v1.7.0] - 2026-05-25

### 新增

- 新增 ACME 和 DNS 账号管理功能，支持证书申请与续期

### 变更

- 移除新用户注册功能
- 更新 Go 版本要求至 1.25+

---

## [v1.6.1] - 2026-05-13

### 修复

- 修复个人设置页无法查看第三方认证源及解绑功能

---

## [v1.6.0] - 2026-05-13

### 新增

- 支持 OIDC 单点登录（SSO）

---

## [v1.5.0] - 2026-04-25

### 新增

- 集成 PoW（Anubis）防护，支持有效期配置

---

## [v1.4.0] - 2026-04-01

### 新增

- 支持域名级别独立绑定 TLS 证书，每个域名可单独选择证书
- 新增批量更新配置项接口
- 新增 Agent 卸载脚本

### 变更

- 禁用新用户自助注册
- 默认服务器块新增 HTTPS 握手拒绝支持

---

## [v1.3.2] - 2026-03-30

### 新增

- 网站配置支持多域名绑定与共享设置
- 新增抽屉式规则创建组件

---

## [v1.3.1] - 2026-03-20

### 新增

- 新增源站管理功能，支持源站创建、更新与删除

### 变更

- 重构代理路由页面，优化输入组件与样式

---

## [v1.3.0] - 2026-03-19

### 新增

- 新增数据库观测数据手动和自动清理策略
- 节点访问日志支持数据库分片，按 ID 分片

### 变更

- 数据库版本管理与迁移逻辑重构

---

## [v1.2.0] - 2026-03-19

### 新增

- 支持多上游地址负载均衡
- 新增缓存策略配置（路径前缀、精确路径）
- 节点健康事件清理功能

### 变更

- 上游渲染改为命名 upstream 并启用 keepalive
- 更新 HTTPS 配置，启用 reuseport 与 epoll 事件模型

---

## [v1.1.2] - 2026-03-18

### 变更

- HTTPS 启用 HTTP/2 支持

---

## [v1.1.1] - 2026-03-18

### 新增

- 新增获取配置版本详情 API

### 变更

- 仪表板概览数据结构优化，添加压缩与规范化

---

## [v1.1.0] - 2026-03-18

### 新增

- 新增应用日志分页查询与清理功能
- 新增访问日志 IP 汇总与趋势查询
- 新增 OpenResty DNS 解析器指令支持
- Docker 部署支持在运行中容器内执行 reload

### 修复

- 修复应用结果警告逻辑
- Lua 和证书文件管理重构，优化文件同步与清理机制

---

## [v1.0.2] - 2026-03-17

### 新增

- 支持 PostgreSQL 数据库，添加数据库迁移逻辑
- 新增 Docker Compose 配置，支持 PostgreSQL 联动部署

### 变更

- 多个管理端 API 请求方法从 PUT/DELETE 统一改为 POST

---

## [v1.0.1] - 2026-03-16

### 新增

- 新增 `origin_host` 字段，支持覆盖回源请求的 Host 头

### 修复

- 修复代理配置中 SSL 服务器名称和主机头覆盖逻辑

---

## [v1.0.0] - 2026-03-15

OpenFlare 首个正式版本发布。

### 新增

- 管理端 UI、管理 API、Agent API 基础功能
- 反向代理配置管理与 OpenResty 配置渲染
- 配置版本发布与 Agent 同步
- TLS 证书导入与管理
- 节点注册、心跳与状态观测
- SQLite 数据库支持
