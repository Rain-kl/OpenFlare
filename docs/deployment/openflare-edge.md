# 部署 OpenFlare Edge

OpenFlare Edge（`openflare-edge`）是 OpenFlare 的边缘代理与安全网关，支持 Linux `amd64`/`arm64` 主机与 Docker 容器部署。双进程模型、全内存态 TLS 私钥、配置同步与自更新机制见 [OpenFlare Edge 架构设计](../design/pingora-edge-design.md)。

## 接入条件

- **控制端连通**：节点须能通过 HTTPS 访问 OpenFlare Server 地址。
- **入站端口**：Linux 主机开放 `80/tcp` 与 `443/tcp`；Docker 容器内默认监听 `8080` 与 `8443`，需映射到宿主机 `80` 与 `443`。
- **认证令牌**（在控制台 **节点管理** 节点详情点击 **边缘节点部署** 可直接复制预填命令）：
  - **专属令牌（`node`）**：在 **节点管理** 预建节点后获取，绑定特定节点配置，推荐生产使用。
  - **发现令牌（`discovery`）**：在 **系统设置** > **OpenFlare** 获取，首次启动时自动登记为新节点。

## Linux 主机安装

### 一键安装

使用专属令牌（`--token-type node`）安装并启动 `openflare-edge.service`：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type node \
  --token '<NODE_TOKEN>' \
  --http-listen 0.0.0.0:80 \
  --tls-listen 0.0.0.0:443
```

使用全局发现令牌自动注册时，将 `--token-type` 设为 `discovery`：

```bash
sudo bash install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type discovery \
  --token '<DISCOVERY_TOKEN>'
```

本地源码仓库或解压包内可直接执行 `sudo bash scripts/install-openflare-edge.sh ...`。

### 常用安装参数

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `--server-url URL` | OpenFlare Server 控制端地址（必填） | 无 |
| `--token TOKEN` | 节点或发现令牌（与 `--token-file` 二选一） | 无 |
| `--token-file PATH` | 从本地文件读取令牌 | 无 |
| `--token-type TYPE` | 令牌类型：`node` 或 `discovery` | `discovery` |
| `--version TAG` | 指定 `edge-v*` 发布标签（如 `edge-v0.1.6`） | 最新稳定版 |
| `--repo OWNER/NAME` | GitHub 发布仓库 | `Rain-kl/OpenFlare` |
| `--http-listen ADDR` | HTTP 监听地址 | `0.0.0.0:80` |
| `--tls-listen ADDR` | HTTPS/TLS 监听地址（传 `none` 禁用 TLS） | `0.0.0.0:443` |
| `--install-dir DIR` | 二进制安装目录 | `/opt/openflare-edge` |
| `--state-dir DIR` | 状态与快照目录 | `/var/lib/openflare-edge` |
| `--no-service` | 仅安装文件，不创建 systemd 服务 | `false` |

## Docker 容器部署

运行官方静态镜像 `ghcr.io/rain-kl/openflare-edge`（支持 `:latest` 与 `:canary`）：

```bash
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  --cap-add=NET_RAW \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<NODE_TOKEN>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

容器关键选项：

- **必须保留 `--cap-add=NET_RAW`**：用于隔离探测子进程执行原生 ICMP Ping 与路由追踪。
- **令牌文件挂载**：可用 `OPENFLARE_EDGE_TOKEN_FILE`（默认 `/run/secrets/openflare-edge-token`，需对 UID `10001` 可读）替代明文 `OPENFLARE_EDGE_TOKEN`，两者互斥。
- **监听与状态目录**：`OPENFLARE_EDGE_HTTP_LISTEN`（默认 `0.0.0.0:8080`）、`OPENFLARE_EDGE_TLS_LISTEN`（默认 `0.0.0.0:8443`，设为 `none` 禁用）与 `OPENFLARE_EDGE_STATE_DIR`（默认 `/var/lib/openflare-edge`）可按需覆盖；数据卷持久化节点身份与快照。

## 验证与卸载

检查本地进程与控制台状态：

```bash
# Linux systemd 主机
/opt/openflare-edge/current --version
sudo systemctl status openflare-edge
sudo journalctl -u openflare-edge -n 50 --no-pager

# Docker 容器
docker logs -f openflare-edge
```

在控制台 **节点管理** > **节点列表** 确认节点在线并已同步配置版本，再按服务器规格在 **性能策略** 绑定策略。

卸载 systemd 服务与程序文件（追加 `--remove-state` 可同时清除状态目录与 GeoIP 库）：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-edge.sh | sudo bash
```

节点升级与回滚流程见 [升级与维护](./upgrade.md)。
