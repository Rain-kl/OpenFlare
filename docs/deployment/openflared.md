# 部署 OpenFlared 客户端

`openflared` 是运行于内网环境（私有子网、局域网或无公网 IP 服务器）的高可用穿透连接器，基于 Rust 与内嵌 `rathole` 客户端构建。支持多连接器副本并行部署，自动向所属中继池中跨可用区最优的多个 `openflare-relay` 建立加密通道，并将通过 CIDR 安全校验的 L7 HTTP/WebSocket 与 L4 TCP/UDP 流量转发至内网目标服务。

---

## 前置条件

1. **创建隧道与获取凭证**：
   - 登录 OpenFlare 控制台，进入 **「内网穿透 → 隧道列表」**，点击创建隧道，选择所属中继池（`Pool`）、传输偏好与安全 CIDR 规则，保存后复制生成的 `oftok1_` 引导令牌或一键部署命令。
2. **出网连通性要求**：
   - 内网服务器无需开放任何公网入站端口，仅需保证能够出向访问：
     - OpenFlare Server 控制端 HTTP/WebSocket 地址（控制流与配置同步）
     - 所选中继池中各在线 `openflare-relay` 节点的 `7443`（Noise/TCP）或 `7444`（WebSocket）传输端口（以及网关探测端口 `8443`）。

---

## Linux 主机部署（推荐）

官方安装脚本会自动下载匹配架构（`amd64` / `arm64`）的静态二进制，配置非特权系统账号 `openflared`，持久化本地 `connector_id` 并以 systemd `Type=notify` 模式托管：

### 1. 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflared.sh | sudo bash -s -- \
  --token "<OFTOK1_BOOTSTRAP_TOKEN>"
```

若在同一内网需部署多个冗余副本实现双活高可用，只需在不同主机（或配合不同 `--state-dir` / `--connector-id`）运行同一条隧道的部署命令。

### 2. 日常运维、手动更新与卸载

```bash
# 查看运行状态与实时日志
sudo systemctl status openflared
sudo journalctl -u openflared -f

# 手动升级至最新稳定版或 canary 预览版
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflared.sh | sudo bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflared.sh | sudo bash -s -- canary

# 卸载（如需删除连接器 ID、令牌等本地状态，请追加 --remove-state）
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflared.sh | sudo bash -s -- --remove-state
```

---

## Docker 容器部署

官方镜像为 `ghcr.io/rain-kl/openflared`，基于 `alpine` 构建并以非 root 用户（UID/GID `10001`）运行：

```bash
docker pull ghcr.io/rain-kl/openflared:latest
docker rm -f openflared 2>/dev/null || true

docker run -d --name openflared --restart unless-stopped \
  -e OPENFLARE_TUNNEL_TOKEN="<OFTOK1_BOOTSTRAP_TOKEN>" \
  -v openflared-data:/var/lib/openflared \
  ghcr.io/rain-kl/openflared:latest
```

> [!NOTE]
> 若需直接访问宿主机 `127.0.0.1` 上监听的本地服务，请在 Linux 环境中添加 `--network host`；在默认桥接网络中，目标地址可指向容器所在同一 Docker 网络内的服务名或内网 IP（注意检查隧道配置中允许的目标 CIDR 列表）。

---

## 命令行与环境变量参考

| CLI 参数 | 环境变量 | 说明 | 默认值 |
| :--- | :--- | :--- | :--- |
| `--token` | `OPENFLARE_TUNNEL_TOKEN` | 隧道 Token 或 `oftok1_` 引导令牌 | — |
| `--token-file` | `OPENFLARE_TUNNEL_TOKEN_FILE` | 从受限文件读取令牌 | — |
| `--server-url` | `OPENFLARE_SERVER_URL` | OpenFlare Server 地址（使用 `oftok1_` 时可选） | — |
| `--state-dir` | `OPENFLARE_TUNNEL_STATE_DIR` | 存储 `connector-id`、`connector_identity.json`、rathole 配置及热更新槽位 | `/var/lib/openflared` |
| `--connector-id` | `OPENFLARE_CONNECTOR_ID` | 显式指定持久化连接器标识 | 自动生成 UUID |

---

## 运行机制与验证

启动后，`openflared` 自动执行以下流程：
1. 解析引导凭据并将身份持久化至 `$STATE_DIR/connector_identity.json`（权限 `0600`），仅当来源令牌变更时重新采用新凭据。
2. 建立控制面 WebSocket（`/api/v1/tunnel/ws`）及心跳通道，周期向控制面 `GET /api/v1/tunnel/relays/discover` 拉取池内候选健康中继。
3. 并行探测网关延迟并按 `zone_tag` 故障域选出最优的 `K` 个中继，以 Make-Before-Break 迟滞算法热切换内嵌 `rathole` 客户端隧道。
4. 对所有收到的 L7 HTTP/WS 与 L4 TCP/UDP 请求执行 CIDR SSRF 允许/拒绝名单校验（默认封锁 `169.254.169.254/32`）。

在控制台 **「内网穿透 → 隧道列表 → 隧道详情」** 中检查：
- 在线连接器（Connectors）列表中出现本实例及所属公网出口 IP、当前软件版本。
- 活跃附着关系（Attachments）展示当前已连接的中继列表、传输协议（`noise` / `websocket` / `tcp`）与 RTT 时延。
