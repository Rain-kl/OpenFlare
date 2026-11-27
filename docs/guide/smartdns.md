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

1. 进入左侧栏 **「DNS 管理 → SmartDNS」**。
2. 在 **「委派设置」** 页签配置：
   - **委派子域（Delegation Domain）**：例如 `sdns.example.com`。
   - **权威 NS 主机名列表**：例如 `ns1.sdns.example.com`、`ns2.sdns.example.com`。
   - **高级运行参数**中的默认 TTL 与快照同步周期。
3. 保存委派设置。在托管域名的上级 DNS（如 Cloudflare）完成 `NS` 委派记录与对应胶水 `A/AAAA` 记录配置后，点击 **「验证 NS 委派解析」** 确认公网解析生效。
4. 切换至 **「权威实例」** 页签：
   - 点击 **「接入独立实例」**，选择发布通道（**正式版** / **Canary 预览版**）以及部署方式（**Linux 一键脚本** / **Docker**），复制控制面生成的 Discovery 模式命令或预注册实例后的专属凭证命令在目标节点运行；
   - Linux 主机也支持在安装或升级命令末尾附加 `canary` 参数（或 `--channel canary`）切换至 Canary 预览通道，例如：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && \
  sudo bash install-openflare-dns.sh \
    --controller-url https://panel.example.com \
    --token-type discovery \
    --token <DISCOVERY_TOKEN> \
    --public-addr 203.0.113.53 \
    --listen 0.0.0.0:53
```

   - 使用容器化部署时，可通过标准 Docker 镜像 `ghcr.io/rain-kl/openflare-dns:latest`（或 `:canary`），或在项目根目录通过 `docker compose up -d dns` 启动独立 `dns` 服务。详细端口映射、环境变量与系统权限说明请查看 [SmartDNS 部署文档](../deployment/openflare-dns.md)。

---

## 3. 编排调度策略与决策流

打开 **「调度策略」**，点击「新增策略」或策略名称进入独立编辑页。在「基本设置」中指定名称、匹配范围和回退方式。

- **预设策略（Smart / Load Balance）**：直接调整延迟权重（`w_lat`）、负载权重（`w_load`）、地理权重（`w_geo`）与筛选条件（国家、节点标签、线路分组）。预设策略和可视化流程中的调度动作均支持搜索并多选，节点标签和线路分组取自控制台已有配置。
- **可视化流程**：在画布拖动端口连线，或在右侧属性面板选择分支与目标节点。从画布工具栏将条件或动作节点拖到画布。键盘操作时，在工具栏选中节点类型后按 Enter 放置。选中节点或连线后可按 Delete 或 Backspace 移除，删除节点会一并移除其连线。选择条件节点后可增删和排序分支。使用「仅执行首个匹配分支」时，将空条件作为最后的默认分支。表达式可以引用 `client.country`、`client.region`、`client.asn`、`client.ip_family`、`domain` 和 `qtype`。点击「校验决策流」检查当前流程并查看生成的 JDM JSON。
- **JDM JSON**：在「基本设置」选择 JDM JSON 模式，或从已校验流程的预览进入 JSON 编辑。
- 点击策略行内的 **「模拟」** 可直接使用该策略，或打开页面右上角的 **「决策模拟器」** 输入域名、查询类型（`A` / `AAAA`）及模拟客户端 IP，直观查看客户端网络归属画像、JDM 命中的分支及所有候选节点的 4 阶段打分明细。

---

## 4. 在 Cloudflare DNS 指向中使用 SmartDNS

1. 在 **「DNS 管理 → Cloudflare 连接」** 选择已有 DNS 账号或输入独立 API Token，点击 **「保存并测试」**。连接验证成功后可同步记录。
2. 打开 **「DNS 管理 → DNS 解析 → 指向分组」**，新增或编辑分组。
3. 将指向方式切换为 **「SmartDNS 调度」**，选择目标 SmartDNS 调度策略，可选自定义目标主机名（留空将自动在 `delegation_domain` 下生成唯一子域）。
4. SmartDNS 指向分组固定采用 **仅 DNS 的 CNAME** 同步模式，成员域名的 Cloudflare 代理开关将被锁定为关闭，确保访客真实 ECS 子网直达 OpenFlare SmartDNS 权威节点。

在 **「DNS 解析 → 同步记录」** 查看域名或分组的同步结果与失败详情。分组详情支持按域名搜索、批量移动和调整代理状态。
