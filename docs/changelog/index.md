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

### 新增

- 新增可配置的 FRPS 内置 WebUI 开关和监听端口。在数据库 w_system_configs 中新增 `relay_frps_web_ui_enabled` 与 `relay_frps_web_ui_port`，支持通过后台系统设置页进行图形化管理与动态同步至 Relay 节点。
- 将 TLS 证书续签逻辑接入 Asynq 异步任务框架。新增单证书续期任务 `of_ssl_single_renew`（`openflare:ssl_single_renew`），支持在管理后台查看每步的申请状态和详细日志，并提供失败重试能力。

### 修复

- 修复代理路由详情页点击“发布配置”时，同时弹出配置差异对话框和确认发布对话框导致重叠的问题：点击发布时不再展示配置差异，直接进行确认发布。
- 修复配置版本发布到 Agent 后 `openresty -t` 因 `proxy_cache_path` 使用 `/var/cache/openresty` 导致非 root 用户 `mkdir` 失败的问题：发布快照与渲染将 `/var/` 下路径规范为 `__OPENFLARE_PROXY_CACHE_PATH__`，Agent 应用时落地为 `data_dir/var/cache/openflare_proxy` 并兼容重写已发布配置中的旧路径。
- 修复配置版本发布到 Agent 后 `openresty -t` 因证书私钥无法解析而失败的问题。根因是发布快照生成 `certs/{id}.key` 时直接写入库内加密的 `KeyPEM`（`enc:v1:`），未解密为 PEM；现与证书详情接口一致，发布前通过 `OpenKeyPEM` 解密后再下发。
- 修复 `/api/v1/d/option` 批量更新 OpenResty 等业务配置不生效的问题。根本原因是 option 模块在读写时做了 PascalCase 与 snake_case 的机械转换（如 `OpenRestyEventsUse` → `open_resty_events_use`），与 `w_system_configs` 中实际 key（`openresty_events_use`）不一致，更新写入了错误的幽灵配置行。现改为 API 直接使用与数据库一致的 snake_case key，并同步更新前端性能调优与运维设置页。
- 修复 PostgreSQL 数据库执行迁移时报 `duplicate key value violates unique constraint "goose_db_version_pkey"` 导致迁移中断的问题。根本原因：`goose_db_version.id` 自增序列落后于表内 `MAX(id)`（常见于从 dump 恢复或历史迁移以显式 id 复制版本记录后），goose 记录新版本号时自增 id 与既有行冲突。修复方式：在 `goose.Up` 前对 PostgreSQL 执行 `setval` 重新对齐 `goose_db_version` 的 id 序列。
- 修复 openflared（Tunnel Client）WebSocket 连接在 Cloudflare 代理环境下频繁收到 EOF 断连的问题。根本原因：服务端 `read_pump` 仅在收到 WebSocket 协议层 Pong 帧时刷新读超时，而客户端（`golang.org/x/net/websocket`）以 JSON 应用层 `{"type":"pong"}` 响应 ping，服务端 90s 读超时到期后主动关闭连接，客户端收到 EOF 并进入无限重连循环。修复方式：在 `clientPongType` 分支中同步调用 `conn.SetReadDeadline` 刷新超时。
- 修复 openflared frpc 子进程异常退出（`exit status 1`）时缺乏详细诊断信息的问题。现捕获 frpc stderr 并在进程退出时将其输出记录到结构化日志 `stderr` 字段，便于排查配置格式错误、Auth Token 鉴权失败、relay 端不可达等具体原因。
- 修复 Relay 节点启动时在双栈网络环境可能上报 IPv6 地址，导致 Tunnel frpc 客户端无法连接 frps 的问题。强化 `pkg/geoip.HTTPOutboundIPStrategy` 在回退到双栈客户端后仍优先返回 IPv4 地址，确保 Relay 心跳上报的 IP 与 frpc 连接兼容。
- 修复 WAF 黑白名单判定时，白名单作为严格准入控制导致黑名单逻辑失效的问题。现将白名单逻辑调整为信任放行（Bypass/Allow），命中的请求直接放行，未命中的请求继续进入黑名单等防护模块判定。
- 修复 WAF IP 组手动编辑和配置版本发布后，未向 Agent 触发 WebSocket 实时广播导致配置变更不能即时生效的问题。

### 变更

- 将 OpenFlare 配置体系从独立的 `of_options` 表统一迁移至标准系统配置框架 `w_system_configs`（SystemConfig），全部归类为业务类型（`type=business`）。涵盖 Agent（心跳间隔、发现令牌、更新仓库等）、UptimeKuma 集成、GeoIP、数据库自动清理及全部 OpenResty 主配置项共 48 项。业务代码统一改为通过 `repository.GetSystemConfigByKey`/`GetBoolByKey`/`GetIntByKey` 读取，移除进程级内存快照 `OptionMap` 与启动时热重载机制，配置变更经 Redis 缓存失效实现动态生效。已存在的同义配置（如 `password_login_enabled`、`smtp_host`）不重复迁移，旧系统遗留的 `SystemName`、`Footer`、`HomePageLink`、`About` 等无用项一并清理；公开状态接口 `/api/v1/d/status` 相应移除 `system_name`、`home_page_link`、`footer_html` 字段。数据迁移完成后通过 goose 迁移 `202606220005` 删除遗留的 `of_options` 表。
- 优化并统一 `agent`、`relay`、`flared` 的 IP 探测与上报逻辑，均复用公用 `nodeip` 包；在未指定 `node_ip` 时实现心跳 Tick 动态探测上报。
- 优化 `pkg/geoip.GetOutboundIP` 出口 IP 探测策略，优先通过 `tcp4` 建立 HTTP 连接以获得 IPv4 公网地址，并在纯 IPv6/无 IPv4 路由环境下自动降级为双栈 `tcp` 握手。
- 优化 `agent` 系统指纹缓存算法，计算指纹时排除 `UptimeSeconds` 和 `ReportedAtUnix` 动态字段，防止周期心跳时不断触发冗余完整的系统 Profile 数据上报。

- 彻底移除废弃的 GitHub OAuth 和微信登录相关遗留设置项（包括 `GitHubOAuthEnabled`、`GitHubClientId`、`GitHubClientSecret`、`WeChatAuthEnabled` 等），从公开状态接口 `/api/v1/d/status` 移除这些字段的返回。

- 前端路由调整：将 TLS 证书和 DNS 账号的路由地址移出 `/websites`（分别变更为顶级路由 `/certificates` 和 `/dns-accounts`），将 WAF IP 组的路由地址移出 `/waf`（变更为顶级路由 `/ip-groups`）。

- 前端页面鉴权改为默认私域：除 `/login`、`/register`、`/callback` 外，未登录访问任意页面（含数据看板 `/`）均重定向至登录页。

- 重构优化：收敛 `flared` 和 `relay` 客户端模块中重复声明的 `APIResponse` 结构体，统一通过类型别名复用 `pkg/protocol.APIResponse`。

### 修复

- 修复由于生成的 Docker 安装/部署命令硬编码拉取 `:latest` 镜像，导致使用 v3 新版协议的控制端与 v2 旧版协议的 Relay 节点无法通信的问题。前端改用动态获取当前控制端版本（serverVersion）并自动拉取与当前控制端匹配的 `:beta` 或具体版本镜像。

- 修复点击「打开 FRPS WebUI」跳转到 `about:blank#blocked`：在 Relay 节点详情页的 WebUI 磁贴卡片中增加展示具体 URL 地址，并提供一键复制按钮。解决因 Chrome 浏览器限制从公网安全源（HTTPS）跨域直接访问本地/私有网络 HTTP 端口（Private Network Access 限制）而导致新页面打开被拦截的问题。

- 修复 Docker 部署模式下 Agent 无法自更新：修改 Docker 启动脚本 `agent-entrypoint.sh`，在降权前将二进制文件所在目录 `/usr/local/bin` 及二进制文件自身的属主赋予 `openflare`，解决容器内自更新写入时报 `permission denied` 的问题。

- 修复 Agent 升级版本比对逻辑：使用统一的 `pkg/utils.CompareVersions` 对比版本，正确处理预览/预发布版本（如 `v3.0.0-beta` 升级到 `v3.0.0-beta.1`），避免升级按钮非预期禁用的问题。

- 修复 Agent 以 `openflare` 非 root 运行时 OpenResty `-t`/reload 失败：nginx `pid` 与 `client_body_temp`/`proxy_temp` 等临时目录改写入 `data_dir/var/run` 与 `data_dir/var/cache/nginx`（`__OPENFLARE_PID_PATH__` / `__OPENFLARE_NGINX_CACHE_DIR__` 占位符），不再使用 OpenResty 安装目录下不可写路径。

- 修复 OpenResty 响应泄露版本号：默认主配置模板与 safe fallback 模板补充 `server_tokens off;`，隐藏 `Server` 头与错误页中的 nginx/OpenResty 版本信息。

- 修复 Agent 与 OpenResty worker 权限不一致导致 Pages/WAF 等静态资源 Permission denied：引入共享运行时用户 `openflare`（Agent 进程、OpenResty worker、文件属主统一）；Docker 入口脚本在启动前修正 volume 属主并降权；本地 systemd 服务以 `openflare` 运行并授予 `CAP_NET_BIND_SERVICE`；`data_dir` 与 `pages_dir` 等路径在同步/Apply 时统一 `chown` 与 `0755/0644` 规范化。

- 修复 Pages 站点根路径 `/` 访问异常：OpenResty 渲染增加 `location = /` 精确匹配；未启用 SPA Fallback 时直接提供入口文件（`index` 指令在 `try_files ... =404` 场景下不生效）；启用 SPA Fallback 时避免 `try_files $uri $uri/ /index.html` 因 `$uri/` 命中站点根目录触发内部重定向循环而返回 500。

- 修复代理路由详情认证配置 Tab：移除 PoW 配置（PoW 仅在 WAF 规则组中设置）；保留 Basic Auth 保存能力；移除页头重复的「保存当前分区」按钮。

- 修复 Pages 路由发布失败并报 `pages module is not available`：配置快照发布流程补齐 Pages 项目激活部署解析与 `pages_deployment` 写入。

- 修复仪表盘与节点详情「24 小时网络趋势」误按速率展示：改为 OpenResty 入/出站小时流量与近 24 小时总量摘要，Y 轴与 tooltip 自动换算 B/KB/MB/GB。

- 修复 Pages 上传或节点同步时报 `pages file size out of bounds`：允许 ZIP 包内的 0 字节文件，并兼容未声明解压大小的 ZIP 条目。

- 修复 Agent 在 OpenResty 配置 checksum 已一致时跳过 Pages 部署包下载，导致 `deployments/{id}/releases` 为空、站点文件未落地：在 state 中缓存 Pages 部署引用；周期同步通过 `GET /api/v1/agent/pages/deployments/:id/hash` 对比 upload SHA-256，仅在哈希变化或本地 release 未就绪时下载 ZIP，避免重复拉取完整配置与部署包。

- 收敛 Pages 部署包读取路径：`upload` 域新增 `GetActiveUpload` / `OpenStoredUpload` / `ActiveUploadHash` / `ResolveLocalFile` / `IngestFromLocalPath` 门面；遗留 `artifact_path` 回填与本地路径解析迁入 upload 域；`OpenDeploymentPackage` 改为返回 `DeploymentPackage`（`io.ReadCloser` + 元数据），不再向 Agent Handler 泄漏 `storage.Object`。

- 修复节点详情 OpenResty 连接数与吞吐显示为「—」：节点可观测 API 将 OpenResty 观测数据合并进 `metric_snapshots`；指标文案改为「请求/分钟」（近 60 秒窗口），连接数为 0 时正常显示 0。

- 修复仪表盘「24 小时请求趋势」摘要误显示当前小时请求量/错误量：改为汇总近 24 小时总量。

- 修复 Pages 部署包上传报「请求超时，请稍后重试」：上传请求使用独立 10 分钟超时（覆盖默认 15 秒），并在大文件上传完成后提示服务端处理中。

- 修复应用日志异常膨胀：Agent 配置同步加锁避免并发重复上报，成功且版本/checksum 未变时跳过重复 apply 日志；Server 入库前对相同成功记录去重；Flared 配置未变更时不再上报 apply 日志。

- 修复应用日志页「清空」无效果：原按钮仅重置筛选；新增「清空日志」入口并对接 `/api/v1/d/apply-logs/cleanup`，支持确认后删除全部记录。

- 配置版本列表按 `created_at` 倒序展示，最新发布版本固定显示在列表顶部。

- 修复 WAF 规则组保存/绑定网站时报 `of_waf_rule_group_bindings_pkey` 冲突：PostgreSQL 在迁移导入显式 ID 后同步绑定表序列，并在写入前自动校正序列。

- 修复 PostgreSQL 启动迁移失败：`of_waf_rule_group_bindings` 序列表为空时 `setval(0)` 越界，改为空表重置为 1、有数据时对齐 `MAX(id)`。

- 修复 WAF 规则组 PoW 策略发布后边缘不生效：统一 WAF 绑定站点名与 OpenResty 路由 `site_name` 解析逻辑，并为所有已启用网站生成 `site_rule_groups` 条目（含仅依赖全局规则组的站点）。

- 修复 WAF PoW 已启用但挑战页不弹出：`pow_enabled=true` 且 `pow_config` 为空时补齐默认配置写入 `waf_config.json`，OpenResty 按全局+已绑定规则组解析 PoW 注入，Lua 对空配置使用运行时默认值。

- 修复 Agent 使用 volume 映射时 PoW/WAF 运行时配置无法加载：OpenResty worker（`nobody`）对 `0700` 父目录无法遍历导致 `waf_config.json` 虽为 `0644` 仍不可读；Apply 后强制修正 `data_dir` 至运行时目录链为 `0755`，PoW Lua 在不可读时输出 WARN。

- 收敛子代理站点标识双轨逻辑：新增 `routeidentity` 统一包，`proxy_route`、`config_version`、`uptimekuma`、`flared` 与 OpenResty 渲染共用 `ResolveSiteName` / `DecodeDomains`；移除废弃 `RenderPoWConfig`；PoW Lua 与 WAF 一致仅依赖 `$openflare_waf_site`。

- 修复全球态势板在仅有 `geo_name`（如 mmdb 的 Germany）而无经纬度时误用美国 fallback 坐标的问题；按国家名/ISO 匹配地图质心。

- 修复 Agent 心跳上报公网 IP 后节点地理位置未自动更新：进程启动时按 `GeoIPProvider` 初始化 `pkg/geoip`，`mmdb` 模式从内置 GeoLite2 种子到 `data/`，并在 Relay 心跳同步地理位置。

- 修复 Agent 启动时 Pages 部署包下载失败：Pages 部署包统一下载走 upload 文件存储框架，部署记录持久化 `upload_id`，legacy `artifact_path` 仅用于一次性回填 upload。

- 修复登录 Cap 人机验证：前端 `cap-solver` 与 Cap 路由测试对齐 `b3a55d4` 之后的统一 API 信封 `{ error_msg, data }`，避免 `challenge` 解构失败。

- 修复配置版本快照/发布预览侧栏无法滚动：内容区改为 `flex-1 min-h-0 overflow-y-auto`，与项目内其他可滚动 Sheet 布局一致。

- 修复 Agent CI/Docker 构建：将 `GeoLite2-Country.mmdb` 提交至仓库作为兜底，构建前优先尝试 `scripts/fetch-agent-geoip-mmdb.sh` 拉取最新库，远程失败时回退使用已提交文件。

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
