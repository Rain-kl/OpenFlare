# OpenFlare Edge 架构设计

OpenFlare Edge（`openflare-edge`）是基于 Pingora 构建的边缘代理与安全网关，负责统一承载 HTTP/HTTPS 反向代理、TLS 终止、WAF 流编排与 OWASP CRS 检测、PoW 人机验证、站点认证、请求限流、TinyUFO 响应缓存、Pages 静态托管及原生网络质量观测。控制面通过版本化清单协议（`config_sync v1`）向边缘节点分发配置，节点校验后在内存中完成原子切换，无需重载进程或中断现有连接。

---

## 1. 双进程模型与守护容灾

`openflare-edge` 在单个二进制内采用控制面监督线程与 Pingora 数据面协同运行的双进程架构（`edge/src/main.rs`、`edge/src/control/`、`edge/src/proxy/`）：

```text
systemd (Type=notify, Restart=always, RestartSec=5)
  └─ openflare-edge supervisor（控制面）
       ├─ 心跳 / WebSocket 长连接、配置与资源同步、TLS 内存私钥拉取
       ├─ 公网 IP 探测、Pages 包拉取与原子解压、访问日志 spool 投递
       ├─ 每 1s 探测监听器存活状态并输出 ready.json
       ├─ 隔离探测子进程：openflare-edge probe-worker（按需启用 CAP_NET_RAW）
       └─ Pingora 数据面工作进程
            ├─ HTTP / HTTPS (SNI + ALPN HTTP/2) 监听器（2s 优雅排空窗口）
            ├─ WAF / PoW / Auth / RateLimit / EdgeCache / Origin Error Page
            └─ 直连反代、tunnel-routing.v1 L7 穿透网关转发、Pages 本地服务
```

* **控制循环互斥与就绪通知**：控制面通过 `$STATE_DIR/control.lock` 的文件锁（`flock`）确保同一状态目录同时仅存在一个活跃控制环路；当 HTTP 与 TLS 监听器全部进入监听状态后，写入 `$STATE_DIR/ready.json`（包含 `pid` 与 `binary_version`），同时作为 systemd `Type=notify` 与 Docker `HEALTHCHECK` 的就绪凭据。
* **存活恢复与无缝重载**：控制面持续检测数据面存活与监听状态，数据面异常退出时自动重新拉起；控制面进程异常退出时由 systemd 以 5 秒间隔自动重启。收到线程数或监听器等启动期变更时，Pingora 数据面以 2 秒连接排空宽限期（`grace_period_seconds = 2`）优雅退出，并在非自更新切换场景下通过 `exec()`（剥离 `--upgrade` 参数）重新绑定监听套接字，确保新套接字选项生效。
* **本地快照与冷启动恢复**：节点运行期将已校验快照持久化至 `$STATE_DIR/snapshot.json`（及上一健康版本 `snapshot.previous.json`）。控制面失联期间，运行中节点继续沿用内存路由与私钥提供服务；若节点在控制面不可达时冷启动，HTTP 路由与静态策略可从本地快照立即恢复，待控制面恢复连通后再按需拉取 HTTPS 私钥恢复 TLS 握手，无需重启进程。

---

## 2. 进程加固与最小权限模型

* **Linux 内核与进程级加固**：`edge/src/main.rs` 启动执行体在 Linux 平台调用 `prctl(PR_SET_DUMPABLE, 0)` 与 `setrlimit(RLIMIT_CORE, 0)`，禁止调试附加（`ptrace`）与核心转储文件生成，阻断从内存转储提取敏感凭据。
* **systemd 服务沙箱（`scripts/install-openflare-edge.sh`）**：安装脚本创建专用非特权系统账号 `openflare-edge` 运行服务，仅授予低端口绑定能力 `CAP_NET_BIND_SERVICE` 与主动探测所需的 `CAP_NET_RAW`（仅限隔离的 `probe-worker` 子进程使用），并强制启用 `NoNewPrivileges=true`、`PrivateTmp=true`、`ProtectSystem=strict`、`ProtectHome=true`、`ProtectKernelTunables=true`、`ProtectKernelModules=true`、`ProtectControlGroups=true`、`RestrictSUIDSGID=true`、`LockPersonality=true`、`MemoryDenyWriteExecute=true`、`RestrictNamespaces=true`、`SystemCallFilter=@system-service`、`LimitCORE=0` 与 `UMask=0077`。
* **状态目录最小读写面**：状态目录 `/var/lib/openflare-edge` 收紧为 `0750` 属主 `openflare-edge:openflare-edge`，作为运行时唯一可写路径存放 `identity.json`（`0600`）、`snapshot.json`（`0600`）、`performance-policy.json`（`0600`）、`tunnel-routing.json`（`0600`）、GeoIP 数据库、Pages 发布目录及 `access-logs.jsonl` 缓冲文件。
* **静态 musl 容器镜像（`docker/Dockerfile.edge`）**：基于 Alpine 构建零动态库依赖的静态可执行文件，默认以非 root 用户 `10001:10001` 运行，仅对可执行文件授予 `cap_net_raw+p` 文件能力；配合容器运行时 `--cap-add=NET_RAW` 后，仅主动探测子进程按需提权，代理主线程始终保持最小权限。

---

## 3. 全内存态 TLS 私钥与紧急隔离

自配置契约 v4（`CONTRACT_VERSION = 4`，声明能力 `shared-snapshot-v1`、`tls-sni-h2-v1`、`policy-runtime-v1`、`tls-memory-keys-v1`）起，OpenFlare Edge 取消所有边缘磁盘私钥落盘：

1. **快照脱敏落盘**：控制面下发配置快照及节点本地持久化 `snapshot.json` 仅保留证书公钥链（`cert_pem`）与 SHA-256 指纹（`cert_sha256`），不包含 `key_pem`。
2. **内存密钥环与 `zeroize` 擦除**：节点通过 `POST /api/v1/agent/tls/keys`（响应禁用 HTTP 缓存）按活跃发布范围拉取私钥，解析并存入内存容器 `CertificateKeyRing`（按 `(cert_id, cert_sha256)` 索引）。临时 PEM 缓冲包裹于 `zeroize::Zeroizing<String>`，解析完成后自动归零；每次快照切换调用 `retain_for()` 清理已下线或已轮换的旧密钥。
3. **证书下发审计与紧急隔离（`of_node_cert_deliveries`）**：控制面记录每台节点曾经在内存拉取过的证书公钥指纹、释放序号与下发次数。发现节点主机疑似失陷时，管理员可在控制台触发「紧急隔离」：
   * 立即吊销并轮换节点 `X-Agent-Token`，失效节点认证缓存；
   * 通过 WebSocket 推送 `quarantined` 指令，节点调用 `CertificateKeyRing::clear()` 立即清零内存中的全部私钥；
   * 自动将绑定该节点的 DNS 指向分组切换至备用节点（`FailoverQuarantinedNode`）；
   * 按需自动轮换受影响范围内的 ACME 证书，并对旧证书发起 `CRLReasonKeyCompromise` 吊销。

---

## 4. 统一配置同步与局部故障隔离

节点通过周期心跳 `POST /api/v1/agent/nodes/heartbeat` 与控制面同步配置清单，并在连接 WebSocket 后接收 `config_changed` 事件实时唤醒；启用 Redis 时唤醒事件跨控制面副本广播，断线重连后由心跳自动收敛。

| 资源键 | 数据内容 | 同步方式 |
| :--- | :--- | :--- |
| `performance.v1` | 当前节点的性能策略参数或默认配置 | 清单内联 JSON |
| `sites.v1` | 站点路由、证书公钥、WAF 编译图、响应页面与 Pages 引用 | 修订号 + SHA-256，按需拉取 |
| `tunnel-routing.v1` | L7 内网穿透隧道与可用区健康中继 Gateway 端点列表 | 按摘要按需拉取 |
| `waf.ip-group.v1/<id>` | 生效快照引用的 WAF IP 组规则集 | 小对象内联，大对象按需拉取 |

资源提供者在 `internal/apps/openflare/agent/config_sync.go` 统一注册，内联限制为单资源 `8 KiB`、单份清单 `32 KiB`。节点统一调用 `GET /api/v1/agent/config/pull?key=<资源键>&sha256=<摘要>` 拉取远端资源（若控制端资源已变更则返回 HTTP `409` 触发重读），忽略未知资源键。

* **状态对账与局部回退**：心跳同时上报 `latest_config_version` 与 `current_version`。候选快照须通过摘要校验、结构解码、证书公钥指纹一致性及本地 Pages 资源就绪检查后才原子切换流量。若校验或应用失败，节点保持最后一份健康快照继续服务，并在后续心跳跳过同一失败修订的重拉，直至新修订发布、节点重启或控制端触发强制同步。

---

## 5. 性能策略、HTTP 响应缓存与自更新

### 5.1 运行时性能策略

每个节点可显式绑定一条性能策略（未绑定时采用内置默认策略），持久化为 `$STATE_DIR/performance-policy.json`。

* **动态参数热更新**：回源超时、TLS 校验与 ALPN 协议（`h1` / `h2h1` / `h2`）、部分缓存参数及连接池参数通过内存替换应用并隔离旧连接池；
* **启动期参数重载**：修改工作线程数（`threads` / `threads_auto`）、每监听描述符任务数（`listener_tasks_per_fd`）、`SO_REUSEPORT`、`TCP_FASTOPEN`、DSCP 或内存/磁盘缓存容量上限时，节点触发平滑重载并由 `exec()` 重新应用监听器与线程池参数。

### 5.2 基于 TinyUFO 的 HTTP 响应缓存

`edge/src/proxy/cache.rs` 通过 [pingap-cache](https://github.com/vicanso/pingap) 接入 TinyUFO 存储后端：

* **内存与可选磁盘模式**：默认启用内存存储（`memory://openflare?max_size={capacity}MiB&mode=compact`，默认容量 `128 MiB`、单对象上限 `16 MiB`）；启用磁盘缓存时写入 `$STATE_DIR/http-cache`（目录权限 `0700`、`levels=2:2`），复用 TinyUFO 作为热页层，后台 `DiskCleanup` 任务按 `cache_disk_inactive_seconds`（默认 48 小时）定期清理失活缓存文件；磁盘初始化异常时自动回退内存模式（磁盘与内存配置参数详见[性能策略指南](../guide/performance-policies.md#缓存存储设置)）。
* **缓存键隔离与隐私**：缓存主键由站点快照摘要、单对象大小上限、协议、Host 与完整请求 URI 哈希生成，确保存储文件名与写入日志仅保留不可逆散列 ID，不同快照或限额之间互不串扰。

### 5.3 访问日志缓冲与磁盘 Spool

请求访问明细先写入内存有界队列 `AccessLogBuffer`（容量 `2048` 条、`16 MiB` 信号量预算），每满 `256` 条或间隔 `100 ms` 批量刷写至带文件锁（`access-logs.lock`）的 `$STATE_DIR/access-logs.jsonl`（权限 `0600`）。磁盘 spool 严格限制在 `2048` 条与 `4 MiB` 上限内，防止断网期间磁盘写满；心跳每次最多提取 `1024` 条上报并在控制面返回 HTTP 200 后截断已确认前缀。

### 5.4 公网 IP 探测与自更新通道

* **公网 IP 识别（`edge/src/control/node_ip.rs`）**：启动时及每 60 秒优先扫描本地网卡地址，过滤全部内网及保留 CIDR（`0.0.0.0/8`、`10.0.0.0/8`、`100.64.0.0/10`、`127.0.0.0/8`、`169.254.0.0/16`、`172.16.0.0/12`、`192.0.0.0/24`、`192.0.2.0/24`、`192.88.99.0/24`、`192.168.0.0/16`、`198.18.0.0/15`、`198.51.100.0/24`、`203.0.113.0/24`、`224.0.0.0/4`、`240.0.0.0/4` 及对应 IPv6 保留前缀）；网卡无公网地址时依次向 `https://api.ip.sb/ip`、`https://ip.me`、`https://api.ip2location.io/` 与控制端 `GET /api/geoip` 探测并校验公网可达性，全部不可达时回退上报 `0.0.0.0`。
* **三槽位安全自更新（`edge/src/control/update/mod.rs`）**：在受 systemd 托管（存在 `NOTIFY_SOCKET` 与 `INVOCATION_ID`）的非 root Linux（`x86_64` / `aarch64`）环境上，支持以 6 小时间隔检查 GitHub `edge-v*` 发行版或响应控制台手动下发的更新指令。下载包须通过 HTTPS 白名单重定向、大小限制（压缩包 128 MiB / 解压 256 MiB）、SHA-256 摘要校验与协议版本探测（`config-sync-version` / `self-update-version`），随后沿 `edge-update-next → edge-update-trial → edge-update-active` 完成试运行与提交；试运行超时（15 分钟）自动回滚至稳定版本。

---

## 6. 独立发布与版本注入架构

OpenFlare Edge 采用与 Server、Relay 及 DNS 解耦的独立发布线（`edge-v*`）：

* **编译期版本注入**：CI 工作流（`.github/workflows/build-release-openflare-edge.yml`、`publish-release-openflare-edge.yml`、`build-image-openflare-edge.yml`）通过环境变量 `OPENFLARE_EDGE_VERSION` 注入语义化版本（剥离 `edge-v` 前缀，例如标签 `edge-v0.1.6` 对应运行时版本 `0.1.6`，本地 `cargo build` 默认显示 `dev`），并在所有发布编译中使用 `cargo build --locked --release` 锁定依赖树。
* **多架构归档与镜像**：针对 Linux `amd64` 与 `arm64` 构建静态链接 musl 的归档包（含 `openflare-edge`、`VERSION`、`SHA256SUMS`、许可证与维护脚本）及 Alpine 运行时镜像 `ghcr.io/<owner>/openflare-edge`。向 `canary` 分支推送或手动调度 Canary 构建时，按下一修订号叠加工作流编号生成 `edge-v<X.Y.Z+1>-canary.<run_number>` 预发布标签；正式渠道与默认安装脚本仅筛选稳定版 `edge-vX.Y.Z`。

---

## 7. 代码模块结构

| 模块 | 职责 |
| :--- | :--- |
| `edge/src/main.rs`、`runtime/` | CLI 子命令（`run`、`probe-worker`、`config-sync-version`、`self-update-version`、`attack-rule-catalog`）、Linux 进程加固、监听就绪与性能/隧道路由状态管理 |
| `edge/src/protocol/` | 控制面快照契约解码（`CONTRACT_VERSION = 4`）与配置类型 |
| `edge/src/control/` | 节点注册与 `identity.json` 持久化、心跳、WebSocket 事件消费、`config_sync`、`node_ip` 探测与三槽位自更新 |
| `edge/src/config.rs` | 候选快照校验、本地冷恢复、`CertificateKeyRing` 全内存私钥管理与原子切换 |
| `edge/src/proxy/` | Pingora 监听与 2s 优雅退出、TLS SNI/ALPN、请求管线、直连与 `tunnel-routing.v1` 回源、TinyUFO 缓存与源站错误页 |
| `edge/src/policy/` | 站点认证、Basic Auth、PoW 校验、WAF DAG 与 `zentinel-modsec` CRS 执行、限流与 Service Worker |
| `edge/src/pages/`、`geoip.rs`、`telemetry/` | Pages 拉取校验与 Range/ETag 服务、GeoIP MMDB 自动更新、`AccessLogBuffer`/`AccessLogSpool`、`HostCollector` 与实时日志流 |
| `shared/probe/` | `ProbeSupervisor` 与 `probe-worker` 隔离探测执行器、双向 SSRF 校验及发包预算 |
| `internal/openflare/edgeconfig/`、`internal/apps/openflare/agent/` | 控制面快照编译、`/tls/keys` 私钥下发与 `config_sync v1` 资源提供者 |
