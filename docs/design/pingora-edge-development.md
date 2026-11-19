# Pingora 边缘节点开发说明

Pingora 开发线交付独立的 `openflare-edge` 二进制。节点启动后直接连接 OpenFlare Server，按 [Pingora 边缘节点设计](./pingora-edge-design.md) 完成注册、配置同步与流量服务。Go Agent 和 OpenResty 使用现有开发、测试及发布流程。两条线可以同时交付。

首阶段先对齐对外功能行为。Rust 节点使用 Pingora 默认性能参数，主控暂不新增 Pingora 调优页面、配置字段或运行档位。现有功能的默认行为通过契约样例固定，避免将 OpenResty 的进程数、事件模型和缓冲区参数映射到 Rust 节点。

## 代码位置与依赖

在 OpenFlare 源码仓库新增 `edge/pingora/`，使用单独的 Cargo 清单和锁文件。主控适配随 Go Server 提交。通过跨语言 JSON 样例校验协议，Rust 构建无需 Go 工具链。

| 位置 | 实现职责 |
| --- | --- |
| `edge/pingora/src/main.rs` | 解析启动参数，装配任务和监听器，管理就绪、信号及退出。 |
| `edge/pingora/src/control/` | 注册、凭证持久化、HTTPS 心跳、WebSocket、配置与资源下载、应用结果上报。 |
| `edge/pingora/src/config/` | 载荷解码、语义校验、资源准备、持久化和内存快照发布。 |
| `edge/pingora/src/proxy/` | Pingora 服务、TLS、路由、上游连接、代理响应及静态内容服务。 |
| `edge/pingora/src/policy/` | 策略编译、访问认证、WAF、IP 组、GeoIP、PoW 和限流。 |
| `edge/pingora/src/telemetry/` | 运行状态、访问明细、有界缓冲及上报。 |
| `edge/pingora/tests/fixtures/` | Go 主控生成的协议样例，以及路由、策略与资源的行为用例。 |

初期以 Pingora 0.9.1 为依赖基线，将 TLS 后端和相关原生依赖纳入和目标平台相同的构建验证。缓存接入 `pingora-cache` 的公开接口，适配代码留在节点侧。锁定可验证的依赖组合，后续升级 Pingora 时逐项运行契约和代理回归用例。上游已有的能力使用其正式版本，确有需要时为明确的接口缺口维护小范围补丁并跟进上游。 [Pingora 0.9.1 发布](https://github.com/cloudflare/pingora/releases/tag/0.9.1) · [Pingora 缓存接口状态](https://github.com/cloudflare/pingora/tree/0.9.1#notable-crates-in-this-workspace)

## 主控接入与兼容性

当前协议定义在 `internal/openflare/protocol/agent.go`。主控处理逻辑在 `internal/apps/openflare/agent/`，Go 端实现在 `internal/edge/agent/`。新增能力沿用节点凭证鉴权与响应信封，既有字段、端点和默认行为保持兼容。

| 现有调用 | Rust 接入方式 |
| --- | --- |
| `POST /api/v1/agent/nodes/register` | 复用节点注册、`X-Agent-Token` 与 `registration_key` 语义。增加可选的引擎、契约版本和能力描述，主控在首次绑定时记录引擎。 |
| `POST /api/v1/agent/nodes/heartbeat` 与 `GET /api/v1/agent/ws` | 复用心跳与变更通知，按节点引擎及发布组返回或推送目标配置。Rust 状态包含节点版本、运行健康、目标配置和实际版本。 |
| `GET /api/v1/agent/config-versions/active` | 保留旧契约。新增专供 Pingora 的活跃配置端点，例如 `GET /api/v1/agent/pingora/config-versions/active`，通过认证节点定位发布组和载荷。 |
| `POST /api/v1/agent/apply-logs` | 复用应用结果上报。为 Pingora 补充发布序号与载荷摘要，用于关联同一配置的多次发布，并确保迟到结果不覆盖新版本。 |
| Pages 下载与 WAF IP 组同步 | 在各自功能阶段复用现有资源传输语义，并按认证节点的发布范围校验资源访问。 |

配置回包包含 `schema_version`、`release_sequence`、`config_version`、`payload_sha256` 和 `payload_json`。后者承载 `required_capabilities`、`routes` 和 `assets`，按收到的字符串 UTF-8 字节计算 SHA-256，解码前先与可信回包中的摘要比对。契约样例固定这两层的字段与编码。记录 `desired`、`applied` 和 `last_error`，使控制台能区分配置未收齐、应用失败和已切换。节点发现无法识别的必需能力时保持已生效版本，并报告明确原因。

部署顺序为主控兼容层先行，随后启用 Pingora 试点。回归用例同时覆盖旧 Go Agent 接入新主控，以及 Rust 节点收到旧主控响应时给出兼容性错误。已部署的 Rust 节点在主控回退期间继续使用本地快照。新增 API 的实现按仓库约定更新 Swagger 文档。

## 构建与运行接口

以下命令定义计划交付的接口。初期支持 Linux `amd64` 和 `arm64`，在干净环境验证运行依赖及低权限启动。

```bash
cargo build --locked --release --manifest-path edge/pingora/Cargo.toml
cargo test --locked --manifest-path edge/pingora/Cargo.toml

openflare-edge run \
  --server-url https://control.example.com \
  --token-type discovery \
  --token-file /etc/openflare-edge/bootstrap-token \
  --state-dir /var/lib/openflare-edge
```

`--token-file` 读取接入凭证。预分配节点使用 `--token-type node`。凭证文件及持久化身份仅向运行用户开放，注册成功后后续启动读取已保存身份。`--state-dir` 保存身份、配置快照和状态，不作为填写路由或规则的配置入口。服务进程使用普通用户运行，系统服务按需授予绑定 80/443 的权限。测试和试点可使用高端口，或在不同宿主机部署两种引擎，避免端口冲突。

## 独立发布

`openflare-edge` 使用自己的 SemVer、发布工作流和下载源，节点版本从 Cargo 包版本生成。建议在同一源码仓库保留代码，由 CI 将产物发布到独立发行仓库 `OpenFlare-Edge`。发行仓库名称作为实施建议。每次发行生成带版本及平台的独立归档，例如 `openflare-edge_v0.1.0_linux_amd64.tar.gz`，归档包含二进制及许可，发行清单记录摘要和最低兼容的主控协议版本。

现有 Go 更新器同时使用 `releases/latest` 和列表查询，预发布清理工作流也会扫描发行版列表。独立发行源应在引入 Rust 产物时一起建立，防止 Go 更新器选择 Rust 发行版或清理任务删除对应产物。Rust 版本也独立于 `Cargo.lock` 中的 Pingora 版本，两者分别记录，便于排查依赖问题。

发布由专用 `edge-v*` 工作流触发，产物发布在当前仓库 Release，但版本查询只筛选 `edge-v*`，不使用通用 `v*` 的 latest 语义。安装脚本校验归档摘要、写入版本化目录并通过 `current` 相对符号链接切换。低权限 systemd 服务仅授予 `CAP_NET_BIND_SERVICE`，并限制文件系统、内核接口、命名空间和系统调用范围。平滑升级使用 Pingora `SIGQUIT` 与升级 Unix Socket 交接监听 FD，新进程完成配置恢复后再接管流量；升级失败时安装脚本切回上一版本并重启。发布完成的判据包括新进程使用预期配置成功服务，上一版二进制与可读取的有效快照在回退窗口内可用。

## 实现顺序与验收

| 阶段 | 交付内容 | 验收条件 |
| --- | --- | --- |
| 1. 接入与最小代理 | Rust 进程、主控兼容层、独立试点发布组、HTTPS 注册与心跳、单域名 HTTP/1.1 代理。 | 干净 Linux 节点以单二进制接入。重复注册保持节点身份。主控故障或节点重启后由最后有效快照继续服务。试点发布不改变 Go Agent 的目标版本。 |
| 2. 代理行为与配置热生效 | TLS 与 SNI、HTTP/2、动态域名和上游、流式代理、响应处理、配置通知与原子快照。 | 已落地主控 Pingora 独立发布组、专用契约与校验摘要，Rust 节点支持精确与单层通配域名、TLS/SNI 动态证书、TLS ALPN H2/H1、HTTP/1.1 监听和快照热更新。路由、源站、HTTPS、HTTP/2 与重定向语义从同一业务快照派生并用 Go 契约测试对照 OpenResty。 |
| 3. 安全策略与业务能力 | WAF、增量 IP 组、GeoIP、PoW、站点认证、限流、Pages、结构化访问明细、上游支持的缓存行为。 | 已落地契约 v2 的策略字段：同一业务快照中的 Basic Auth、站点认证、PoW 策略、限流合并结果、Pages 部署参数和 WAF 图进入 Pingora payload；Rust 节点实现 Basic Auth 校验、站点认证 Cookie/登录签名、WAF 图执行、IP 组引用、Country GeoIP 匹配、PoW 路径策略判定、挑战签发验证与会话、请求速率限制和 Pages 安全路径解析。Pages 包下载采用流式 SHA-256 校验，支持 ZIP、tar、tar.gz、tar.xz、tar.bz2 和 7z，解包受文件数、单文件大小与总大小约束，写入就绪 marker 后通过临时符号链接原子切换 `current`。静态文件已支持入口文件、SPA 回退、常见 MIME 类型和 Pages API 路径重写。GeoIP 同时读取 City 与 Country 数据库，优先使用 City 的国家与一级行政区，City 不可用时回落 Country；PoW 使用与 OpenResty 相同的模板配置、内建 shield/classic 源模板、静态资源和 boot JSON 契约，并按 `Accept-Language` 输出中文或英文挑战页；Pingora `logging` 回调采集结构化访问明细，经心跳 `access_logs` 上报主控并复用既有日志入库链路。上游缓存行为仍在后续补齐。 |
| 4. 节点交付与试点扩容 | `amd64` 与 `arm64` 独立版本、低权限运行、进程异常恢复、平滑升级与回滚。 | 已落地独立 `edge-v*` 发布工作流、双架构归档、低权限 systemd 单元、GeoIP 自动下载、持久访问明细队列、异常自动重启和 Pingora FD 平滑升级。升级失败时通过保留版本目录自动回滚。功能对照用例继续使用同一业务快照验证 OpenResty 与 Pingora。 |

## 双引擎行为对照

在同一测试环境中运行两类节点，用同一业务快照和可控源站重放测试用例。比较客户端收到的状态码、关键响应头和正文，同时校验源站实际收到的请求。时间戳、挑战随机值和引擎专属标识按各自语义断言。每项能力同时覆盖默认配置、显式开启和显式关闭的行为。

| 功能 | 对照结果 |
| --- | --- |
| 路由、TLS 与代理 | 精确和通配域名的匹配顺序、未知域名响应、HTTPS 重定向、SNI 证书选择、可信代理后的真实客户端地址、Host 和路径透传、多源站故障处理。 |
| 请求与响应 | 默认请求体限制、WebSocket 握手与转发、SSE 持续传输、上传与大响应、压缩协商、自定义源站错误页及原状态码保留。 |
| 安全策略 | WAF 规则组顺序、IP 组和 GeoIP 匹配、放行与阻止结果、PoW 路径及白名单、挑战验证与会话、站点认证、限流和未授权请求是否到达源站。 |
| Pages 与观测 | 静态文件与入口页、SPA 回退、API 路径重写与鉴权、部署切换与回滚、访问日志字段和流量统计口径。 |
| 缓存 | 按上游支持范围验证启用与绕过、缓存键隔离、命中、过期及回源失败行为。以可确认的能力作为发布边界。 |

HTTP/3 随上游能力评估，首版按 HTTP/1.1 和 HTTP/2 验收。使用默认参数记录基础运行指标，吞吐、延迟与内存的对比压测和调优安排在功能对齐之后。

验证范围随改动选择。在 Rust 目录运行 `cargo fmt --check`、`cargo clippy --locked --all-targets -- -D warnings` 和 `cargo test --locked`。涉及节点协议和主控发布的改动，增加 Go/Rust 契约测试并运行现有 Go Agent 集成用例。修改请求处理或缓存时运行对应的双引擎对照用例。
