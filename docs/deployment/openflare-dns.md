---
title: OpenFlare SmartDNS 部署
description: 独立 Rust 权威解析节点 openflare-dns 的发布版本、Linux 一键安装、Canary 通道与 Docker 容器化部署说明
---

# OpenFlare SmartDNS 部署

OpenFlare SmartDNS（`openflare-dns`）是基于 Rust 与 `hickory-server` 构建的高性能独立权威 DNS 解析节点，与 OpenFlare 控制面配合完成委派子域解析、网络质量加权调度及离线的快照灾备。

有关控制台全局配置、委派验证及 JDM 调度策略编排的使用说明，请参阅 [SmartDNS 使用指南](../guide/smartdns.md)。

---

## 发布包与版本管理

`openflare-dns` 独立于服务端与 `openflare-edge` 进行版本发布与容器构建：

| 发布通道 | 触发条件与标签格式 | Docker 镜像标签 | 说明 |
| --- | --- | --- | --- |
| **正式版（Stable）** | 推送 `dns-vX.Y.Z` 标签（例如 `dns-v0.1.0`） | `ghcr.io/rain-kl/openflare-dns:latest` 与 `:dns-vX.Y.Z` | 面向生产环境的稳定发布版本 |
| **Canary 预览版** | 推送至 `canary` 分支或无参数手动触发工作流，生成 `dns-vX.Y.Z-canary.<run_number>` | `ghcr.io/rain-kl/openflare-dns:canary` 与 `:dns-vX.Y.Z-canary.<run_number>` | 包含最新开发提交的预发布测试版本 |

Release 归档包命名为 `openflare-dns_<VERSION>_linux_<arch>.tar.gz`（支持 `amd64` 与 `arm64`），压缩包内包含 `openflare-dns` 二进制、`SHA256SUMS`、`VERSION`、`LICENSE` 及配套脚本。

---

## Linux 主机部署

### 1. 一键安装脚本

在具备公网可达性的 Linux（`x86_64` / `aarch64`）服务器上，可通过控制台 **「DNS → SmartDNS → 权威实例」** 复制预填充令牌与控制面地址的一键命令，或手动执行以下安装步骤。

**使用全局 Discovery Token 自动注册并安装正式版：**

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && \
  sudo bash install-openflare-dns.sh \
    --controller-url https://panel.example.com \
    --discovery-token <DISCOVERY_TOKEN> \
    --public-addr 203.0.113.53 \
    --listen 0.0.0.0:53
```

**安装 Canary 预览版：**

在脚本参数开头或选项中添加 `canary`、`--canary` 或 `--channel canary` 即可拉取最近发布的 SmartDNS Canary 预发布版本：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && \
  sudo bash install-openflare-dns.sh canary \
    --controller-url https://panel.example.com \
    --discovery-token <DISCOVERY_TOKEN> \
    --public-addr 203.0.113.53 \
    --listen 0.0.0.0:53
```

**使用预注册实例专属凭证：**

如果已在控制台提前创建实例，可使用 `--token-type instance --token <ACCESS_TOKEN> --instance-id <INSTANCE_ID>`（或 `--access-token <ACCESS_TOKEN>`）进行精确身份绑定。

### 2. 常用安装参数

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `canary` / `--canary` / `--channel` | 发布通道（`stable` 或 `canary`） | `stable` |
| `--controller-url`（别名 `--server-url`） | OpenFlare 控制面访问地址（必填） | 无 |
| `--discovery-token` / `--access-token` / `--token` | 引导凭证或实例专属凭证 | 无 |
| `--token-file` | 从指定文件路径读取凭证（避免出现在命令行参数中） | 无 |
| `--token-type` | 配合 `--token` / `--token-file` 使用：`discovery` 或 `instance` | `discovery` |
| `--instance-id` / `--name` | 显式实例 ID 与节点展示名称 | 自动生成 |
| `--public-addr` | 节点对外宣告的公网地址 | 无 |
| `--listen` | UDP/TCP 监听地址与端口 | `0.0.0.0:53` |
| `--workers` | `SO_REUSEPORT` UDP 工作线程数（`0` 表示自动探测） | `0` |
| `--rrl-qps` / `--rrl-burst` | 每前缀响应速率限制（RRL）稳态速率与突发容量 | `1000` / `2000` |
| `--snapshot-interval` | 快照同步周期（秒） | `10` |
| `--version` | 指定安装精确的 `dns-v*` 发布标签（与 `canary` 互斥） | 最新正式版 |
| `--install-dir` / `--state-dir` / `--geoip-dir` | 安装目录、状态数据目录与 GeoIP 库目录 | `/opt/openflare-dns`、`/var/lib/openflare-dns` |
| `--skip-geoip` | 跳过安装时自动拉取开源 GeoIP MMDB 数据库 | 否 |
| `--no-service` | 仅安装二进制与凭证文件，不创建 `systemd` 服务 | 否 |

安装脚本执行流程：
1. 校验发行版架构并从 GitHub Releases 下载对应归档包与 `SHA256SUMS`，验证完整性并检查 `--version` 输出；
2. 创建受限系统用户 `openflare-dns`（`nologin`），并将引导令牌以 `0600` 权限写入 `<state-dir>/bootstrap-token`；
3. 尝试执行 `fetch-netintel-dbs.sh` 拉取本地 GeoIP MMDB 库（失败不阻断安装，节点后续仍可通过控制面或其他方式同步）；
4. 写入 `/etc/systemd/system/openflare-dns.service`，启动服务并轮询 `<state-dir>/ready.json` 中的进程 PID 与 `binary_version` 直至就绪。

---

## 权限与安全加固

`install-openflare-dns.sh` 生成的 `systemd` 服务以最小权限原则运行：
- 使用独立无登录系统账户 `openflare-dns`，不以 `root` 身份长期运行解析进程；
- 通过 `AmbientCapabilities=CAP_NET_BIND_SERVICE` 与 `CapabilityBoundingSet=CAP_NET_BIND_SERVICE` 授予低端口（如 `53`）绑定能力，同时启用 `NoNewPrivileges=true`、`ProtectSystem=strict`、`ProtectHome=true`、`ProtectKernelTunables=true`、`ProtectKernelModules=true`、`RestrictNamespaces=true` 与 `SystemCallFilter=@system-service`；
- 引导凭证写入独立受保护文件（`UMask=0077`）并通过 `--token-file` 传入，注册成功后节点会将分配的持久化凭证原子写入 `<state-dir>/identity.json`（`0600` 权限），后续重启自动使用该身份向控制面恢复会话。

---

## Docker 容器部署

### 1. 直接运行容器镜像

官方镜像 `ghcr.io/rain-kl/openflare-dns` 基于 Alpine Linux 构建为静态链接二进制，默认以非特权用户 `10001:10001` 运行，并已预先附加 `cap_net_bind_service+ep`。由于默认配置在容器内监听 `0.0.0.0:5353`，宿主机只需将 `53/udp` 与 `53/tcp` 映射至容器 `5353` 端口即可提供标准 DNS 服务：

```bash
docker run -d --name openflare-dns --restart unless-stopped \
  -p 53:5353/udp \
  -p 53:5353/tcp \
  -e OPENFLARE_DNS_CONTROLLER_URL=https://panel.example.com \
  -e OPENFLARE_DNS_DISCOVERY_TOKEN=<DISCOVERY_TOKEN> \
  -e OPENFLARE_DNS_PUBLIC_ADDR=203.0.113.53 \
  -v openflare_dns_data:/var/lib/openflare-dns \
  ghcr.io/rain-kl/openflare-dns:latest
```

> 如需运行预览版本，将镜像标签替换为 `:canary`；若容器内需要使用高并发 `SO_REUSEPORT` 多队列且希望减少 NAT 转发开销，可按主机网络需求改用 `--network host` 并设置 `-e OPENFLARE_DNS_LISTEN=0.0.0.0:53`。

### 2. Docker Compose 部署

项目根目录的 `docker-compose.yaml` 已包含 `dns` 服务定义。配置 `.env` 后可直接单独启动：

```bash
export OPENFLARE_DNS_CONTROLLER_URL=https://panel.example.com
export OPENFLARE_DNS_DISCOVERY_TOKEN=<DISCOVERY_TOKEN>
export OPENFLARE_DNS_PUBLIC_ADDR=203.0.113.53
docker compose up -d dns
```

默认将本地 `./data/smartdns` 挂载至 `/var/lib/openflare-dns`，并通过宿主机 `${OPENFLARE_DNS_PORT:-53}` 端口转发至容器 `5353/udp` 与 `5353/tcp`。

### 3. 容器环境变量参考

| 环境变量 | 说明 | 默认值 |
| --- | --- | --- |
| `OPENFLARE_DNS_CONTROLLER_URL` | OpenFlare 控制面访问地址（兼容回落 `OPENFLARE_SERVER_URL`） | 无（必填） |
| `OPENFLARE_DNS_DISCOVERY_TOKEN` | 全局 Discovery 注册凭证（与 `OPENFLARE_DNS_TOKEN`、`OPENFLARE_DNS_TOKEN_FILE` 互斥） | 无 |
| `OPENFLARE_DNS_TOKEN` | 引导或实例专属凭证，默认按 `instance` 类型处理 | 无 |
| `OPENFLARE_DNS_TOKEN_TYPE` | 配合 `OPENFLARE_DNS_TOKEN` / `OPENFLARE_DNS_TOKEN_FILE` 使用（`discovery` 或 `instance`） | `discovery`（配合 `OPENFLARE_DNS_TOKEN` 时默认 `instance`） |
| `OPENFLARE_DNS_TOKEN_FILE` | 挂载的外部凭证文件路径 | `/run/secrets/openflare-dns-token` |
| `OPENFLARE_DNS_INSTANCE_ID` / `OPENFLARE_DNS_NAME` | 显式实例 ID 与节点展示名称 | 自动生成 |
| `OPENFLARE_DNS_PUBLIC_ADDR` | 节点对外宣告公网地址 | 无 |
| `OPENFLARE_DNS_LISTEN` | 容器内 DNS 监听地址 | `0.0.0.0:5353` |
| `OPENFLARE_DNS_CACHE_DIR` | 状态、身份文件与就绪探针目录 | `/var/lib/openflare-dns` |
| `OPENFLARE_DNS_GEOIP_DIR` | GeoIP MMDB 库目录 | `/var/lib/openflare-dns/geoip` |
| `OPENFLARE_DNS_WORKERS` / `OPENFLARE_DNS_RRL_QPS` / `OPENFLARE_DNS_RRL_BURST` / `OPENFLARE_DNS_SNAPSHOT_INTERVAL_SECS` | 分别对应工作线程数、RRL QPS/Burst 阈值与快照同步周期（秒） | 与二进制默认值一致 |

容器镜像内置 `HEALTHCHECK`，每 10 秒检查 `${OPENFLARE_DNS_CACHE_DIR}/ready.json` 是否存在且非空，进程收到 `SIGTERM` 退出时会自动清理该就绪文件。

---

## 升级与维护

### 升级与通道切换

`update-openflare-dns.sh` 仅原子替换二进制文件，自动保留 `<state-dir>/identity.json`、`last-snapshot.bin`、GeoIP 数据及现有 `systemd` 服务配置，并在重启后等待 `ready.json` 就绪：

```bash
# 升级至最新正式版
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-dns.sh -o update-openflare-dns.sh && \
  sudo bash update-openflare-dns.sh

# 升级或切换至最新 Canary 预览版
sudo bash update-openflare-dns.sh canary

# 回滚或指定安装某个版本标签
sudo bash update-openflare-dns.sh --version dns-v0.1.0
```

### 卸载节点

```bash
# 停止并移除 systemd 服务与二进制文件（保留状态与身份目录）
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-dns.sh -o uninstall-openflare-dns.sh && \
  sudo bash uninstall-openflare-dns.sh

# 彻底清理包含身份状态、离线快照与 GeoIP 数据在内的所有目录
sudo bash uninstall-openflare-dns.sh --remove-state
```
