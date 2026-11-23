# 部署 Relay（内网穿透中继节点）

`openflare-relay` 是基于 Rust 与内嵌 `rathole` 引擎构建的高性能公网中继节点程序。它负责终结内网 `openflared` 连接器的 Noise / WebSocket / TCP 加密隧道连接，运行 Relay Gateway（默认端口 `8443`）供 OpenFlare Edge 转发 L7 流量，并直接监听已分配的 L4 TCP/UDP 服务端口。

---

## 前置条件

在部署中继节点前，请确认以下环境与网络条件：

1. **获取引导凭据**：
   - 在 OpenFlare 控制台进入 **「内网穿透」** 页面，在「中继节点」页签复制 **自动发现命令**（使用全局 `oftok1_` 引导令牌一键注册到默认池），或新建指定中继池的 Relay 记录并复制专属部署命令。
2. **网络端口规划**：
   - `7443/tcp`：默认 `rathole` Noise / TCP 加密隧道监听端口（须对需要接入的 `openflared` 公网出向放行）。
   - `7444/tcp`：默认 `rathole` WebSocket 穿透回退端口（用于在受限网络环境下承载 WebSocket 隧道）。
   - `8443/tcp`：Relay Gateway 内部转发端口（须向同机或跨机关联的 OpenFlare Edge 节点放行；若中继同时直接承接 L4 服务，还须按需放行对应的 L4 TCP/UDP 服务端口）。
3. **可用区与 Edge 共置规划（推荐）**：
   - 建议为每个中继设置清晰的 `zone_tag`（可用区标签），便于客户端在多中继高可用调度时自动分散故障域。
   - 若 Relay 与某台 OpenFlare Edge 部署在同一台主机，可在控制台配置其 `colocated_edge_node_id`，以优先路由本地网关回环流量。

---

## Linux 主机部署（推荐）

官方提供独立的一键安装、更新与卸载脚本（基于 `tunnel-v*` 独立发布通道），默认创建非特权系统账号 `openflare-relay` 并托管于 systemd：

### 1. 一键安装

使用控制台生成的引导令牌（`oftok1_...`，已内嵌控制面地址）直接安装并启动 systemd 服务：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-relay.sh | sudo bash -s -- \
  --token "<OFTOK1_BOOTSTRAP_TOKEN>"
```

常用可选参数（可通过环境变量或脚本参数传入）：

- `--server-url URL`：使用原始 Token 时显式指定控制面地址（使用 `oftok1_` 引导令牌时可省略）。
- `--public-addr HOST_OR_IP`：显式声明中继对外通告的公网地址/域名。
- `--gateway-addr HOST_OR_IP`：显式声明供 OpenFlare Edge 访问网关的内部/内网地址。
- `--zone-tag TAG`：指定所属故障域标签（如 `hk-az1`）。
- `--colocated-edge-node-id ID`：关联同机部署的 Edge 节点 ID。
- `--transport-port 7443` / `--ws-port 7444` / `--gateway-port 8443`：自定义中继监听端口。
- `--version tunnel-vX.Y.Z`：锁定安装指定 `tunnel-v*` 版本。

安装脚本会自动将二进制部署至 `/opt/openflare-relay/current`，并将令牌与运行状态以受限权限（目录 `0750` / 状态文件 `0600`）存放于 `/var/lib/openflare-relay`。

### 2. 日常运维、手动更新与卸载

```bash
# 查看服务状态与实时日志
sudo systemctl status openflare-relay
sudo journalctl -u openflare-relay -f

# 手动更新到最新稳定版本或 canary 预发布版本
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-relay.sh | sudo bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-relay.sh | sudo bash -s -- canary

# 卸载服务（如需同时清除身份与状态文件，追加 --remove-state）
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-relay.sh | sudo bash -s -- --remove-state
```

> [!NOTE]
> 在 systemd 非 root 托管环境中，控制端控制台也可下发双槽位热更新指令（`next → trial → active`），`openflare-relay` 会自动拉取并校验 `SHA256SUMS` 与二进制签名输出后平滑重启升级。

---

## Docker 容器部署

官方多架构静态 Alpine 镜像为 `ghcr.io/rain-kl/openflare-relay`，使用 `tunnel-v*` 标签构建（默认 UID/GID `10001` 非 root 运行）：

```bash
docker pull ghcr.io/rain-kl/openflare-relay:latest
docker rm -f openflare-relay 2>/dev/null || true

docker run -d --name openflare-relay --restart unless-stopped \
  -p 7443:7443 \
  -p 7444:7444 \
  -p 8443:8443 \
  -e OPENFLARE_RELAY_TOKEN="<OFTOK1_BOOTSTRAP_TOKEN>" \
  -v openflare-relay-data:/var/lib/openflare-relay \
  ghcr.io/rain-kl/openflare-relay:latest
```

如中继节点需要直接承接额外的 L4 TCP/UDP 转发端口，请在 `docker run` 中补充对应的 `-p` 端口映射或按需使用 `--network host`。

---

## 命令行与环境变量参考

| CLI 参数 | 环境变量 | 说明 | 默认值 |
| :--- | :--- | :--- | :--- |
| `--token` | `OPENFLARE_RELAY_TOKEN` | 中继专属 Token、自动发现 Token 或 `oftok1_` 引导令牌 | — |
| `--token-file` | `OPENFLARE_RELAY_TOKEN_FILE` | 从安全文件读取令牌 | — |
| `--server-url` | `OPENFLARE_SERVER_URL` | OpenFlare Server 地址（使用 `oftok1_` 时可选） | — |
| `--state-dir` | `OPENFLARE_RELAY_STATE_DIR` | 身份凭据、Noise 密钥与热更新槽位持久化目录 | `/var/lib/openflare-relay` |
| `--name` | `OPENFLARE_RELAY_NAME` | 注册时显示的中继名称 | 系统主机名 |
| `--public-addr` | `OPENFLARE_RELAY_PUBLIC_ADDR` | 通告给内网连接器的公网连接地址 | 自动探测 |
| `--gateway-addr` | `OPENFLARE_RELAY_GATEWAY_ADDR` | 通告给 OpenFlare Edge 的网关连接地址 | 回落 `public_addr` |
| `--zone-tag` | `OPENFLARE_RELAY_ZONE_TAG` | 故障域标签 | 空 |
| `--colocated-edge-node-id` | `OPENFLARE_RELAY_COLOCATED_EDGE_NODE_ID` | 同机共置的 Edge 节点 ID | 空 |
| `--transport-port` | `OPENFLARE_RELAY_TRANSPORT_PORT` | Noise/TCP 隧道监听端口 | `7443` |
| `--ws-port` | `OPENFLARE_RELAY_WS_PORT` | WebSocket 隧道回退监听端口 | `7444` |
| `--gateway-port` | `OPENFLARE_RELAY_GATEWAY_PORT` | Relay HTTP/WS 网关监听端口 | `8443` |

---

## 状态验证

部署启动后，进入控制台 **「内网穿透 → 中继节点」** 页签并点开中继详情：
- 确认有效状态为 **在线（Online）**、WebSocket 控制流处于已连接状态，且已上报 Noise 公钥与中继版本号。
- 若存在已连接的隧道，可在「拓扑概览」中核对各连接器的接入传输协议（Noise / WebSocket / TCP）与探测延迟。
