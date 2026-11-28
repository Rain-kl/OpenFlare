# 部署 OpenFlare Relay

`openflare-relay` 是基于 Rust 与内嵌 `rathole` 引擎构建的公网隧道中继节点，负责终结 `openflared` 加密隧道、提供 L7 Relay Gateway 与转发 L4 TCP/UDP 端口。架构与调度模型见 [内网穿透隧道设计](../design/tunnel-design.md)。

## 接入条件

- **引导令牌**：在控制台 **内网穿透** > **中继节点** 复制自动发现命令或创建中继后复制专属 `oftok1_` 引导令牌。
- **开放端口**：
  - `7443/tcp`：`rathole` Noise / TCP 隧道入口（供 `openflared` 连接）。
  - `7444/tcp`：`rathole` WebSocket 隧道回退入口。
  - `8443/tcp`：Relay Gateway 与 `/_openflare/health` 探测端口（供 Edge 节点与 `openflared` 连接）。
  - **L4 服务端口**：若启用 L4 转发，放行对应公开 TCP/UDP 端口。
- **可用区与共置（推荐）**：设置 `zone_tag` 便于跨故障域分布；与 Edge 同机部署时填写 `colocated_edge_node_id`。

## Linux 主机部署

`openflare-relay` 与 `openflared` 使用独立 `tunnel-v*` 发布通道（Linux `amd64`/`arm64`）。使用内嵌控制端地址的 `oftok1_` 令牌一键安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-relay.sh | sudo bash -s -- \
  --token "<OFTOK1_BOOTSTRAP_TOKEN>"
```

常用安装参数（完整 CLI 参数与环境变量见 [配置项参考](../reference/configuration.md)）：

- `--server-url URL`：使用原始令牌时指定控制端地址（`oftok1_` 令牌可省略）。
- `--public-addr HOST_OR_IP` / `--gateway-addr HOST_OR_IP`：显式通告给连接器或 Edge 的地址。
- `--zone-tag TAG` / `--colocated-edge-node-id ID`：故障域标签与同机 Edge 节点 ID。
- `--transport-port` / `--ws-port` / `--gateway-port`：自定义监听端口（默认 `7443`、`7444`、`8443`）。
- `--version tunnel-vX.Y.Z` / `--install-dir` / `--state-dir` / `--no-service`：指定版本、目录或跳过 systemd 注册。

## Docker 容器部署

运行官方镜像 `ghcr.io/rain-kl/openflare-relay`（非 root 用户 `10001`）：

```bash
docker run -d --name openflare-relay --restart unless-stopped \
  -p 7443:7443 \
  -p 7444:7444 \
  -p 8443:8443 \
  -e OPENFLARE_RELAY_TOKEN="<OFTOK1_BOOTSTRAP_TOKEN>" \
  -v openflare-relay-data:/var/lib/openflare-relay \
  ghcr.io/rain-kl/openflare-relay:latest
```

承担 L4 TCP/UDP 转发时，补充对应 `-p` 端口映射或使用 `--network host`。

## 验证与卸载

查看服务日志并在控制台 **内网穿透** > **中继节点** 确认状态为 **在线**：

```bash
sudo systemctl status openflare-relay
sudo journalctl -u openflare-relay -n 50 --no-pager
```

卸载 systemd 服务（追加 `--remove-state` 同时删除身份与状态目录）：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-relay.sh | sudo bash
```

升级与双槽位热更新见 [升级与维护](./upgrade.md)。
