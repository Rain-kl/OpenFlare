# 部署 OpenFlared 客户端

你会学到：OpenFlared 客户端的职责、配置参数与环境变量、基于 Docker 运行客户端的方法，以及如何在内网服务器上通过二进制方式独立部署。

**OpenFlared** 是部署在用户内网（局域网、私有云等无法被公网直接访问的环境）的隧道客户端。它的核心职责是通过 `X-Tunnel-Token` 与控制面（OpenFlare Server）建立通信，并在本地自动拉起并管理一个或多个 **frpc (快速反向代理客户端)** 进程，从而将内网的 HTTP 流量安全、稳定地穿透至外网的中继节点。

---

## 前置条件

1. **获取 Tunnel Token**：在管理端「节点管理」中新增一个类型为 **Tunnel** 的节点，保存后进入节点详情页即可查看该节点专属的接入 Token。
2. **网络出方向权限**：内网服务器无需任何公网入方向 IP 或端口映射，但必须能够通过网络访问公网上的 **OpenFlare Server 地址** 以及对应的 **TunnelRelay 节点中继端口 (默认 7000)**。
3. **软件依赖**（仅限宿主机直接部署）：
   - 本地需有可执行的 `frpc` 二进制文件，或通过参数显式指定路径。

---

## 配置文件与环境变量

`openflared` 启动时默认会读取当前目录下的 `flared.json`。同时也完全支持通过环境变量进行覆盖。

### 配置字段详情

| JSON 字段 | 环境变量 | 说明 | 默认值 |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server 接口服务地址 | **无（必填）** |
| `tunnel_token` | `OPENFLARE_TUNNEL_TOKEN` | 隧道客户端专属认证 Token | **无（必填）** |
| `frpc_path` | `OPENFLARE_FRPC_PATH` | frpc 可执行二进制文件路径 | `"frpc"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | 本地数据与生成的 `frpc_{relayNodeID}.toml` 存放目录 | `"./data"` |
| `state_path` | - | 本地状态记录文件路径（保存最后应用的配置版本）| `"{data_dir}/flared-state.json"` |
| `heartbeat_interval`| - | 状态心跳上报周期（支持毫秒数或 Go Duration 字符串） | `10000` (10s) |
| `sync_interval` | - | 隧道配置拉取同步周期（支持毫秒数或 Go Duration 字符串） | `30000` (30s) |
| `request_timeout` | - | 接口网络请求超时时长 | `10000` (10s) |

---

## Docker 运行

Docker 部署是内网运行最简单也最安全的方式。官方的 `openflared` 镜像已经内置了客户端控制器以及 `frpc v0.69.0` 二进制运行时，无需额外搭建环境。

```bash
docker pull ghcr.io/rain-kl/openflared:latest
docker rm -f openflared 2>/dev/null || true

docker run -d --name openflared --restart unless-stopped \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_TUNNEL_TOKEN=YOUR_TUNNEL_TOKEN \
  -v openflared-data:/app/data \
  ghcr.io/rain-kl/openflared:latest
```

---

## 启动与验证

### 1. 自动同步逻辑

启动成功后，OpenFlared 将执行以下工作流：
- **心跳与配置获取**：周期性向 Server 的 `/api/v1/tunnel/heartbeat` 和 `/api/v1/tunnel/config/active` 接口发起同步，验证 Token 并检测配置版本。
- **文件渲染**：当检测到配置版本（或校验和 Checksum）变化时，会自动拉取该隧道的完整路由规则。如果绑定了多个中继 Relay，将为每个 Relay 分别在 `data_dir` 下渲染出 `frpc_{relayNodeID}.toml`。
- **配置变更重启**：当配置或校验和变化时，重新拉起对应的 `frpc` 子进程，以确保流量映射保持最新。
- **异常自恢复**：如果本地 `frpc` 隧道进程异常退出，主控程序会按指数退避（初始 1 秒，上限 60 秒）自动重启。

### 2. 查看日志与连接状态

```bash
# Docker 容器日志
docker logs -f openflared
```

若进程运行无误，您会在日志中看到类似如下输出：
```text
flared config loaded ...
detected frpc version v0.69.0
flared process started
applying new tunnel config {"version": "...", "checksum": "..."}
frpc process missing, starting {"relay_id": "..."}
```

### 3. 管理端确认

打开管理后台的 **「节点管理」**，进入对应 Tunnel 节点的详情页：
- 查看节点在线状态与 flared 运行状态（WebSocket 已连接 / 运行中 / 离线）。
- 查看当前应用版本与最近一次应用记录。
