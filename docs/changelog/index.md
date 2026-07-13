---
sidebar: false
---

# 更新日志

本文件记录 OpenFlare 每个版本的重要变更。

格式基于 [Keep a Changelog](http://keepachangelog.com/)，版本号遵循 [语义化版本](http://semver.org/)。

## 重大变更

> [!IMPORTANT]
> 
> 3.1.2 版本更新了 CLickHouse 部署配置。
> 
> 3.0.0 版本为 Wavelet 平台迁移与架构重构版本，涉及数据库表结构、环境变量以及前后端底层架构的重大变更。请务必在升级前备份数据库，并且更新到 V2.3.4。
> 目前已知的兼容性问题：
> - Pages 无法迁移, 升级前请先手动下载并备份 Pages 静态站点的 ZIP 包，升级后重新创建。
> - 性能调优参数重置, 升级后请重新配置

## [unreleased]

### 新增

- 新增 `make prettier`，统一使用 `gofmt` 与项目固定版本的 Prettier 自动格式化前后端源码。
- WAF 规则支持可视化 DAG 编排、版本冲突保护和有序路由绑定，发布时编译为 OpenResty 纯内存运行图。
- WAF IP 组支持 checksum 驱动的 Worker 内存热刷新，并补充 City MMDB 地区匹配数据源。

### 变更

- WAF 规则编辑器缩小编排区高度和首次适配缩放比例，默认显示更多画布上下文。
- 移除 WAF 规则旧固定黑白名单、地域名单与 PoW 数据库字段；升级后需在发布前重新编排规则。
- Agent 现同时内嵌 Country 与 City MMDB，首次启动仅从程序内初始化缺失文件，网络下载只用于后续周期更新。

### 修复

- GORM SQL 语句改为仅在全局 `debug` 日志级别输出；生产默认日志级别不再记录查询文本，慢查询与执行错误也不包含 SQL 参数，避免节点访问令牌等敏感信息写入应用日志。
- 新增 `redis.maint_notifications` 启动开关并默认关闭，平台与 Asynq Redis 客户端仅在显式开启时进行 maintenance notifications 自动协商，避免不支持该命令的 Redis 服务持续输出握手 fallback 警告。
- WAF 规则编辑器支持通过按钮或键盘删除普通节点和连线，节点属性栏改为选中节点后按需显示，并修复拖动节点时因 React Flow 初始化状态丢失导致的画面闪烁与 error 015。
- WAF 规则编辑器新增启用/停用控制，并移除已废弃的规则组侧站点绑定入口；站点规则顺序统一在反代路由详情中管理。
- 修复 Agent 心跳无法从活动 WAF 运行图发现 IP 组引用的问题，并对发布与同步的完整 IP 组快照增加 20 MiB 聚合容量保护。
- 修复 Agent 增量同步长期保留已取消引用的 WAF IP 组、最终阻塞合法热更新的问题；配置同步现按活动引用集合权威收敛，实时广播仅更新本地现存组。
- 修复 Pages 部署文件清单前端请求路径与后端路由不一致导致的 404 错误。

## [v3.2.0] - 2026-07-12

### 新增

- Zone 概览页新增 Cloudflare 风格流量图：支持 24 小时 / 7 天 / 30 天，展示唯一访问者、请求总数与已提供数据趋势。
- 新增第一阶段 Zone 与正规化 Zone 域名数据库表及路由绑定模型，为后续以稳定 ID 管理网站与域名关联提供基础。
- 新增 Zone 管理 API 与显式历史域名导入命令，使用公共后缀列表验证注册根域和域名归属。

### 修改

- 调整数据库自动清理设置文案，明确自动清理按 ClickHouse 表 TTL 执行，并提示访问日志 90 天、其它观测数据 30 天的保留下限。
- 重构并清理了分析仓统计层 `node_access_log_stats.go` 和 `openflare_access_log.go` 之间的重复模型，使用底层 type aliases 简化了类型转换和拷贝逻辑。
- 配置快照、OpenResty 渲染、Tunnel 与 Uptime Kuma 监控改为从 Zone 域名绑定读取域名 and 证书，移除对反代路由旧域名/证书字段的运行时回退。
- 管理端网站入口改为 Zone 列表与 `/websites/:zoneId` 详情（概览 / 域名 / 路由 / 证书 / 设置），反代路由通过 Zone 域名选择器绑定。
- 反代路由 API 以 `zone_domain_ids` / `zone_domains` 为唯一域名与证书关联来源，不再接受或返回路由内嵌域名/证书字段。

### 移除

- 移除托管域名（managed-domains）管理 API 与前端 `WebsiteService`；请改用 Zone / Zone 域名 API。
- 移除 Zone、Zone 域名、反代路由、WAF 规则组与 IP 组的备注字段（前后端与数据库列同步删除）；证书与源站备注保留。

### 修复

- 修复 Agent 上报访问日志的 `bytes_sent` 在 Server 入库链路丢失，导致 Zone 概览“已提供的数据总计”长期为 0 的问题。
- 修复嵌入式静态前端访问 `/websites/:zoneId` 时回退到首页 HTML，导致 Zone 详情页显示总览并触发 React hydration error 的问题。
- Docker ClickHouse 性能配置改为单文件挂载，避免覆盖镜像内置的 Docker 网络监听配置，导致宿主机无法通过 8123/9000 访问服务。

## [v3.1.2] - 2026-07-10

### 修复

- 修复节点/仪表盘 24 小时容量、网络、磁盘 IO 趋势在 ClickHouse 限流查询下几乎为空的问题：改为基于小时级聚合与计数器 delta 统计，避免仅依赖最近有限条原始快照导致历史时段全空。
- 降低静置时 ClickHouse CPU：可观测/访问日志 batchwriter 启用 `MinBatchSize` 与 `MaxFlushWait`，减少心跳小 part 写入；Docker `performance.xml` 收紧小规格后台 merge 池。
- ClickHouse 清理语义：按保留天数仅 `MATERIALIZE` 表 DDL TTL，`deleted_count` 不再伪报删除；短于表 TTL 的保留请求被拒绝。
- 可观测 dedup 仅在入队成功后保留，flush 失败释放键并短重试；审计 writer 增加 `MaxFlushWait`；`/admin/status/clickhouse` 暴露 batch writer 队列深度/丢弃/flush 错误。
- model 层通过 hooks 写入 CH，去除对 `chwriter` 的直接依赖。
- Dashboard 每节点最新指标改为 `LIMIT 1 BY node_id`；新增 metric/openresty 小时预聚合表；读路径按小时 merge（rollup 窗口完整时仅走预聚合，不足时用 raw 补洞），并提供历史 backfill 迁移。
- 小规格默认连接池下调；`async_insert_busy_timeout` 调至 2s；`of_node_traffic_hourly` 增加 30 天 TTL，UV 改为峰值窗口估计并修正前端文案。
- Docker ClickHouse：`performance.xml` 下调 merge free-entry 阈值以兼容小 `background_pool`（避免 25.x 启动 Code 36）。

### 文档

- 同步 `.env.example` 与 `config.example.yaml`；ClickHouse 服务端配置改为 curl `performance.xml` 到 `./config/clickhouse` 后整目录挂载至 `config.d`（不要放入 listen 配置）。

## [v3.1.1] - 2026-07-06

### 修改

- 将 `cap_login_enabled` 默认值由 `true` 变更为 `false`，默认关闭登录界面 PoW 人机验证。

## [v3.1.0] - 2026-07-04

### 修改

- 修复 ClickHouse TTL 迁移：`DateTime64` 时间列通过 `toDateTime()` 转换后再设置 TTL，避免 goose 启动报错；移除 `MODIFY ORDER BY`（ClickHouse 不允许将排序键缩短至短于隐式主键前缀）。
- ClickHouse 遗留治理 Phase 2：保留期清理改为 TTL `MATERIALIZE TTL`（全量清理使用 `TRUNCATE`），消除定时 `ALTER DELETE` mutation；移除 GORM 双连接池并统一 `ChConn` 读路径；查询侧去除 `trim(remote_addr)`；`wait_for_async_insert` 调整为 1；新增 `/admin/status/clickhouse` 运维指标与 `of_node_traffic_hourly` 预聚合 MV。
- ClickHouse 写入路径优化：移除 Agent 心跳路径中的同步 `ALTER DELETE` 保留清理；`batchwriter` 新增 `MinBatchSize` 抑制过小批次定时 flush；可观测 writer 批次提升至 500、flush 间隔 5s，并为 OpenResty/FRPS/FRPC 补全去重。
- ClickHouse 客户端启用 `async_insert` 异步写入缓冲，并调高 `block_buffer_size` 与连接池默认值，降低小 part 与连接争用。
- Dashboard 与节点可观测 API 消除无 `LIMIT` 全表扫描、增加短 TTL 内存缓存，前端轮询间隔分别调整为 60s/30s。
- 访问日志与 WAF IP 组同步改为 ClickHouse 侧聚合与 SQL 分页，默认查询窗口限制为近 7 天，浏览器分布查询增加 Top 100 限制。
- ClickHouse 分析表新增 TTL 自动过期策略：`w_user_access_logs` 180 天、`of_node_access_logs` 90 天，其余节点观测与聚合表 30 天。
- 节点访问日志写入 ClickHouse 时对 `remote_addr` 执行 `TrimSpace` 规范化，避免首尾空白影响 IP 汇总统计。
- 数据库自动清理任务新增 OpenResty、FRPS、FRPC 观测表清理目标。
- Docker 部署为 ClickHouse 服务增加 `nofile` ulimits 与 `docker/clickhouse/config.d/performance.xml` 性能配置挂载，限制 `max_concurrent_queries`、`background_pool_size` 与 `background_merges_mutations_concurrency_ratio`，降低高负载下的合并与查询争用。
- 审计访问日志写入 ClickHouse 时仅保留安全相关请求头（Authorization、Cookie、X-Forwarded-For、X-Real-IP、User-Agent、Content-Type），敏感头字段以 SHA-256 摘要脱敏，并将序列化后的 headers 载荷上限收紧至 2KB，减小 `w_user_access_logs` 行宽与 merge CPU 开销。
- 隐藏侧边栏“文档库”分组中的“规范示例”与“接口文档”，并将“使用文档”及其他相关页面的文档链接统一跳转至外部文档 https://open-flare.pages.dev/
- 修复全局搜索数据源覆盖不全的问题，补全了所有核心业务控制台页面（节点、规则、域名、证书、DNS、源站、WAF、IP组、Pages、版本发布、访问日志、应用记录和性能调优）及缺失的管理员专有页面（存储、数据、推送、日志）的搜索检索支持。
- 修复系统自更新（Updater）检测上游 GitHub Action Release 时，因资产包名称前缀（`openflare-server`）与仓库名不完全一致导致匹配失败并报错“未找到兼容的 Release”的问题。
- 修复系统设置页面（`/admin/settings`）基于 URL `tab` 参数的定位逻辑，补全缺失的 `openflare-ops` (OpenFlare) Tab，且在不带参数时默认选中 OpenFlare 选项卡。
- 移除系统设置中 OpenFlare 标签页下的“版本信息”卡片及对应的升级管理弹窗逻辑。

## [v3.0.2] - 2026-06-30

### 修复

- 修复 PostgreSQL 自增主键序列在历史数据迁移（INSERT 指定显式 ID）后与实际数据不同步的问题，通过新增全局序列同步脚本一键重置所有相关表的自增计数器。

## [v3.0.1] - 2026-06-30

### 新增

- 新增管理后台用户个人信息编辑与重置密码功能。
- 新增用户列表邮箱列展示以及基于邮箱的搜索过滤。
- 后端新增 `reset-passwd` 命令行工具，支持通过命令行直接重置用户密码。

### 修复

- 修复添加 DNS 账号时因直接传递 class static 方法作为 React Query 的 mutationFn 导致 JavaScript 丢失 `this` 上下文报错 `this.post is not a function` 的问题。
- 修复侧边栏一级菜单项当前页面字体颜色被硬编码为 `#6366F1` 的问题，改用 CSS 主题变量 `text-sidebar-primary`，以保证在多主题系统下的色彩一致性。
- 修复默认（Default）主题因遗漏声明 `destructive-foreground` 变量，导致删除按钮（如确认删除证书弹窗）在某些状态下渲染为黑底黑字而无法阅读的问题。
- 修复 Cobra 命令行初始化注册逻辑，确保所有应用运行模式（All, API, Worker, Scheduler）都正确注册为 Cobra 子命令。
- 优化系统设置页面的色彩定义，移除硬编码的 Indigo 靛蓝色以适配多主题切换。

## [v3.0.0] - 2026-06-27

### 升级与迁移注意事项

> [!WARNING]
> 本次重构涉及数据库表结构以及环境变量的重大变更，老版本务必从 v2.3.4 最新版本升级迁移，否则可能导致数据库结构不兼容或管理端 API 无法访问。
> 升级前务必备份数据库

### 重大重构说明

本项目近期完成了**前后端底层架构的重大迁移与重构**，将原有的独立控制端重构为基于 **Wavelet 统一开发框架** 的全新架构：
- **后端重构**：全面接入 Wavelet 服务平台，收敛并复用了标准的用户管理、安全验证（PoW/邮件验证）、RAM L1 缓存以及 Redis 订阅发布同步机制。配置体系从原 `of_options` 物理表完全迁移合并至标准系统配置框架 `w_system_configs`（类型归为 `business` 业务级配置），废弃原进程级 `OptionMap` 热重载。
- **前端重构**：管理后台前端使用 Next.js App Router、TypeScript 与 Tailwind CSS（基于 shadcn/ui 组件库与 Wavelet 设计风格）进行了完全重写，提供了更具呼吸感和一致性的用户界面，优化了配置版本预览与发布体验。
- **架构解耦**：将原有“站点 (Site)”配置体系拆分为 **「网站管理 -> 域名列表」**（处理域名与证书绑定）与 **「规则管理」**（处理反向代理、静态托管、WAF 和缓存等路由匹配规则）两个维度，极大地提升了复杂拓扑配置的灵活性。内网穿透隧道也统一作为 `tunnel_client` 类型节点整合进了 **「节点管理」** 中。


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
