# SmartDNS Phase 1：网络情报与隔离探测底座

## 背景与目标

为支撑后续权威智能 DNS（SmartDNS）依据访客源 IP、ASN 与实时链路质量进行最优边缘节点调度，第一阶段先落地以下三层基础能力：
1. **多源网络情报数据库（`pkg/netintel` + `internal/apps/openflare/netintel`）**：整合 GeoIP/ASN MMDB 与 `bgp.tools` 路由表/AS 标签（运行期可选 CAIDA AS Relationships），提供无锁原子的 IP/ASN/前缀/AS 邻接查询与 Redis 广播热重载。
2. **被动 TCP RTT 采集与质量融合矩阵（`internal/apps/openflare/netquality`）**：Edge 在下游日志采样路径仅于有界队列尚有容量时读取 Linux `TCP_INFO` 与 TLS 握手耗时；控制面无锁识别 `client_asn` 并经 1 分钟内存分桶异步落盘（`of_network_quality_samples`），由 `of_netquality_rollup` 按 6 小时半衰期融合被动 TCP、主动 Traceroute 与 AS 路径先验生成 `of_network_quality_matrix`。
3. **跨进程隔离的主动网络探测工具（`shared/probe` + `edge` + `internal/apps/openflare/nettools`）**：为避免网络探针阻塞或崩溃波及反向代理主线程，所有主动探测全部隔离在 `openflare-edge probe-worker` 独立操作系统子进程内运行，通过有界 NDJSON 管道与 `ProbeSupervisor` 通信并在崩溃/超时时自动恢复。

## 关键实现与踩坑总结

- **`trippy-core = "0.13.0"` API 适配**：
  - `Builder::new(ip)` 使用 `.max_ttl(u8)` 而非 `max_hops`。
  - `Protocol::Udp` 必须设置 `.port_direction(PortDirection::FixedSrc(Port(33434)))`，`Protocol::Tcp` 必须设置 `.port_direction(PortDirection::FixedDest(Port(port)))`。
  - 并发探测需保留 `.drop_privileges(false)`，否则首个探测丢权后同进程后续原始套接字探测会失去 `CAP_NET_RAW`。
- **子进程隔离边界**：
  - `probe-worker` 开启 `PR_SET_PDEATHSIG(SIGTERM)`、`PR_SET_DUMPABLE = 0`、`RLIMIT_CORE = 0` 与 `nice +10`，并强制日志输出到 `stderr`，保证 `stdout` 纯净作为 NDJSON IPC 通道。
  - `worker::run_probe_worker` 将 `tokio::spawn` 用于并发执行并传参 owned `std::io::stdin()` / `stdout()`（不能传 `!Send` 的 `.lock()`）。
- **测试基础设施 Hook 对齐**：
  - 向 `logstore` 新增异步批写表（`of_network_quality_samples`）时，需同步补充 `internal/testhelper/test_helper.go` 中 `SetupLogStoresForTest` 的测试即刷 Hook（`QueueNetworkQualitySample`）与 `AutoMigrate`，否则异步批写测试无法立即回读样本。
- **许可与合规边界**：
  - `scripts/fetch-netintel-dbs.sh`、Dockerfile 及 GitHub Release 工作流绝不打包受再分发限制的 CAIDA 数据，仅保留运行期 `netintel_caida_enabled=true` 的显式开关；前端对 `bgp.tools` 数据展示保留明确署名（`Data powered by bgp.tools`）。
