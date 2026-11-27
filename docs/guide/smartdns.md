# SmartDNS 智能权威解析使用指南

OpenFlare SmartDNS（Phase 2）结合控制面实时感知的网络质量矩阵、节点负载容量与视觉化 JDM 规则引擎，为域名提供低延迟、支持 EDNS0 Client Subnet（ECS）的高性能权威解析能力，并支持两种运行形态：

- **独立 Rust 高性能实例（`openflare-dns`）**：基于 `hickory-server`、`zen-engine`、`moka` 与 `SO_REUSEPORT` 多核 UDP/TCP 监听，单核缓存命中压测可达裸 `hickory-server` 吞吐的 84%–87%。
- **控制面内嵌 Go 兜底引擎（`embedded_go`）**：在配置启用 `smartdns.enabled: true` 时随机启动，适合轻量测试与过渡期本地权威解析验证。

---

## 1. 准备工作：配置节点容量与路由标签

1. 进入控制台 **「节点管理 → 节点列表」**，点击右上角 **「标签管理」** 创建路由分类标签（如 `optimized`、`high-bandwidth` 等，可设定默认权重乘数 `0.01 ~ 100`）。
2. 编辑边缘节点时填写：
   - **带宽上限（Mbps）**、**并发连接上限** 与 **容量基准权重**（默认 `100`，`0` 表示不限制）；
   - **SmartDNS 路由标签**：勾选所属标签，可选填写单节点权重覆盖值。

---

## 2. 配置 SmartDNS 委派域名与权威实例

1. 进入左侧栏 **「DNS → SmartDNS」**。
2. 在 **「总览与全局设置」** 页签配置：
   - **委派子域（Delegation Domain）**：例如 `sdns.example.com`。
   - **权威 NS 主机名列表**：例如 `ns1.sdns.example.com`、`ns2.sdns.example.com`。
   - **默认 TTL** 与 **快照同步周期**。
3. 在托管域名的上级 DNS（如 Cloudflare）完成 `NS` 委派记录与对应胶水 `A/AAAA` 记录配置后，点击 **「验证 NS 委派解析」** 确认公网解析生效。
4. 切换至 **「权威实例」** 页签：
   - 可直接在页面中选择发布通道（**正式版** / **Canary 预览版**）以及部署方式（**Linux 一键脚本** / **Docker**），复制控制面生成的 Discovery 模式命令或预注册实例后的专属凭证命令在目标节点运行；
   - Linux 主机也支持在安装或升级命令末尾附加 `canary` 参数（或 `--channel canary`）切换至 Canary 预览通道，例如：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && \
  sudo bash install-openflare-dns.sh \
    --controller-url https://panel.example.com \
    --discovery-token <DISCOVERY_TOKEN> \
    --public-addr 203.0.113.53 \
    --listen 0.0.0.0:53
```

   - 使用容器化部署时，可通过标准 Docker 镜像 `ghcr.io/rain-kl/openflare-dns:latest`（或 `:canary`），或在项目根目录通过 `docker compose up -d dns` 启动独立 `dns` 服务。详细端口映射、环境变量与系统权限说明请查看 [SmartDNS 部署文档](../deployment/openflare-dns.md)。

---

## 3. 编排调度策略与决策流

切换至 **「调度策略与流程」** 页签创建或编辑策略：

- **预设策略（Smart / Load Balance）**：直接调整延迟权重（`w_lat`）、负载权重（`w_load`）、地理权重（`w_geo`）与筛选条件（国家、节点标签、线路分组）。
- **可视化 JDM 决策流**：使用流程图画布连接 `Input → Condition (Switch) → Action (Expression) → Output`，支持基于 `client.country`、`client.region`、`client.asn`、`client.ip_family`、`domain`、`qtype` 编写分支条件并实时校验编译结果。
- 完成后点击页面右上角 **「决策模拟器」** 输入域名、查询类型（`A` / `AAAA`）及模拟客户端 IP，直观查看客户端网络归属画像、JDM 命中的分支及所有候选节点的 4 阶段打分明细。

---

## 4. 在 Cloudflare DNS 指向中使用 SmartDNS

1. 打开 **「DNS → DNS 解析」**，新增或编辑指向分组。
2. 将指向方式切换为 **「SmartDNS 调度」**，选择目标 SmartDNS 调度策略，可选自定义目标主机名（留空将自动在 `delegation_domain` 下生成唯一子域）。
3. 注意：SmartDNS 指向分组固定采用 **仅 DNS 的 CNAME** 同步模式，成员域名的 Cloudflare 代理开关将被锁定为关闭，确保访客真实 ECS 子网直达 OpenFlare SmartDNS 权威节点。
