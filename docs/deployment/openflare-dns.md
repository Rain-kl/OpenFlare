# 部署 OpenFlare DNS

OpenFlare DNS（`openflare-dns`）是 OpenFlare SmartDNS 的独立 Rust 权威解析节点，支持 Linux `amd64`/`arm64` 主机与 Docker 容器部署。调度机制、ECS 识别与快照容灾见 [SmartDNS 系统架构设计](../design/smartdns-design.md)，控制台配置见 [SmartDNS 使用指南](../guide/smartdns.md)。

## 接入条件

- **控制端连通**：节点须能通过 HTTPS 访问 OpenFlare Server 地址。
- **入站端口**：Linux 主机开放 `53/udp` 与 `53/tcp`；容器内默认监听 `5353`，需映射到宿主机 `53`（或使用 `--network host` 并设置 `--listen 0.0.0.0:53`）。
- **认证令牌**：在控制台 **DNS 管理** > **SmartDNS** > **权威实例** 复制预填充命令（支持全局发现令牌 `discovery` 与预注册实例令牌 `instance`）。

## Linux 主机安装

### 一键安装

使用发现令牌安装稳定版（安装 Canary 预览版时追加 `canary` 参数）：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-dns.sh -o install-openflare-dns.sh && \
  sudo bash install-openflare-dns.sh \
    --controller-url https://panel.example.com \
    --discovery-token '<DISCOVERY_TOKEN>' \
    --public-addr 203.0.113.53 \
    --listen 0.0.0.0:53
```

绑定控制台预注册实例时，改用 `--token-type instance --token '<ACCESS_TOKEN>' --instance-id '<INSTANCE_ID>'`。

### 常用安装参数

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `canary` / `--canary` / `--channel` | 发布通道：`stable` 或 `canary` | `stable` |
| `--controller-url` | OpenFlare 控制端地址（必填） | 无 |
| `--discovery-token` / `--token` / `--token-file` | 发现令牌、实例令牌或令牌文件路径 | 无 |
| `--token-type` | 令牌类型：`discovery` 或 `instance` | `discovery` |
| `--instance-id` / `--name` | 实例 ID 与展示名称 | 自动生成 |
| `--public-addr` | 节点对外通告的公网地址 | 无 |
| `--listen` | UDP/TCP 监听地址 | `0.0.0.0:53` |
| `--workers` | `SO_REUSEPORT` UDP 工作线程数（`0` 为自动探测） | `0` |
| `--version` | 指定 `dns-v*` 发布标签（与 `canary` 互斥） | 最新稳定版 |
| `--install-dir` / `--state-dir` / `--geoip-dir` | 安装、状态与 GeoIP 库目录 | `/opt/openflare-dns`、`/var/lib/openflare-dns`、`<state-dir>/geoip` |
| `--skip-geoip` / `--no-service` | 跳过 GeoIP 下载或跳过 systemd 服务注册 | `false` |

### 启用控制台远程更新

新版安装脚本会自动注册 `openflare-dns-update.path` 与 `openflare-dns-update.service`。早期安装的主机可执行以下命令启用远程更新能力，之后即可在控制台单台或批量更新：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-dns.sh -o update-openflare-dns.sh
sudo bash update-openflare-dns.sh --enable-remote
sudo systemctl restart openflare-dns
```

## Docker 容器部署

运行官方镜像 `ghcr.io/rain-kl/openflare-dns`（`:latest` 或 `:canary`）：

```bash
docker run -d --name openflare-dns --restart unless-stopped \
  -p 53:5353/udp \
  -p 53:5353/tcp \
  -e OPENFLARE_DNS_CONTROLLER_URL=https://panel.example.com \
  -e OPENFLARE_DNS_DISCOVERY_TOKEN='<DISCOVERY_TOKEN>' \
  -e OPENFLARE_DNS_PUBLIC_ADDR=203.0.113.53 \
  -v openflare_dns_data:/var/lib/openflare-dns \
  ghcr.io/rain-kl/openflare-dns:latest
```

若使用仓库根目录的 `docker-compose.yaml`，设置上述环境变量后执行 `docker compose up -d dns`。全部容器环境变量见 [配置项参考](../reference/configuration.md)。

## 验证与卸载

检查本地状态并测试权威应答（`aa` 标志）：

```bash
sudo systemctl status openflare-dns
dig @203.0.113.53 sdns.example.com NS +norecurse
```

卸载 systemd 服务与程序文件（追加 `--remove-state` 同时清除身份凭据、离线快照与 GeoIP 数据）：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-dns.sh | sudo bash
```

升级与回滚流程见 [升级与维护](./upgrade.md)。
