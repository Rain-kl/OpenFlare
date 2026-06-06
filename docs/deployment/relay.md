# 部署 Relay (Tunnel 中继)

你会学到：TunnelRelay 节点的职责、`openflare-relay` 的配置项与环境变量、使用 Docker 运行 Relay 的方法，以及如何通过源码手动构建并部署 Relay。

在 OpenFlare 的内网穿透体系中，**TunnelRelay 节点** 扮演着关键的角色。它与普通的边缘节点（Edge Node）不同，除了运行传统的 Agent（托管 OpenResty 进行 HTTPS/WAF 处理）外，还同机运行了 **Relay (frps 隧道管理器)** 服务，负责监听内网客户端（OpenFlared）的隧道连接并进行流量中继。

---

## 前置条件

在部署 TunnelRelay 节点之前，请确保：

1. **已注册为 TunnelRelay 类型节点**：在 OpenFlare 管理端「节点管理」中，添加一个类型为 `tunnel_relay` 的节点，并获取其专属的 `agent_token` 或使用全局 `discovery_token`。
2. **网络端口**：
   - 必须确保 `bindPort`（frpc 连接端口，默认 `7000`）可被公网/内网客户端访问。
   - 必须确保 `vhostHTTPPort`（HTTP Vhost 端口，默认 `8080`）处于空闲状态，Agent 将在此端口上与 frps 进行流量传递。
3. **软件依赖**（仅限宿主机直接部署）：
   - 本地需有可执行的 `frps` 二进制文件（建议版本为 `v0.61.0+` 或最新稳定版 `v0.69.0`），或通过参数显式指定路径。

---

## 配置文件与环境变量

`openflare-relay` 启动时默认会读取当前目录下的 `relay.json`。同时也完全支持通过环境变量进行覆盖。

### 配置字段详情

| JSON 字段 | 环境变量 | 说明 | 默认值 |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server 接口服务地址 | **无（必填）** |
| `agent_token` | `OPENFLARE_AGENT_TOKEN` | 节点专属 Token | 与下者二选一 |
| `discovery_token` | `OPENFLARE_DISCOVERY_TOKEN` | 自动注册 Token | 与上者二选一 |
| `node_name` | `OPENFLARE_NODE_NAME` | 节点标识名称 | 默认获取本机主机名 |
| `node_ip` | `OPENFLARE_NODE_IP` | 节点出口/监听 IP | 自动检测真实出口 IP |
| `frps_path` | `OPENFLARE_FRPS_PATH` | frps 可执行二进制文件路径 | `"frps"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | 本地数据与生成的 `frps.toml` 存放目录 | `"./data"` |
| `state_path` | - | 本地状态 JSON 记录文件路径 | `"{data_dir}/relay-state.json"` |
| `heartbeat_interval`| - | 心跳周期（支持毫秒数或 Go Duration 字符串） | `10000` (10s) |
| `request_timeout` | - | 接口请求超时时长 | `10000` (10s) |

---

## Docker 运行（推荐）

Docker 运行是 TunnelRelay 节点最便捷的部署方案。官方镜像内置了 `openflare-relay` 控制器与 `frps v0.69.0` 运行时，开箱即用。

```bash
docker pull ghcr.io/rain-kl/openflare-relay:latest
docker rm -f openflare-relay 2>/dev/null || true

docker run -d --name openflare-relay --restart unless-stopped \
  -p 7000:7000 \
  -e OPENFLARE_SERVER_URL=http://your-server:3000 \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  -v openflare-relay-data:/var/lib/openflare-relay \
  ghcr.io/rain-kl/openflare-relay:latest
```

> [!TIP]
> 这里的 `-p 7000:7000` 映射的是 `frpc` 客户端连接中继的端口。如果管理端配置了自定义的 `relay_bind_port`，请对应修改宿主机端口映射。

---

## 宿主机手动运行

如果您倾向于在物理机或虚拟机上直接运行：

### 1. 编译二进制

```bash
cd openflare-relay
go build -o openflare-relay ./cmd/relay
```

### 2. 准备 `relay.json`

在程序同级目录下创建 `relay.json` 配置文件：

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "your-relay-node-agent-token",
  "frps_path": "/usr/local/bin/frps",
  "data_dir": "./data",
  "heartbeat_interval": "10s",
  "request_timeout": "10s"
}
```

### 3. 运行服务

```bash
export LOG_LEVEL='info'
./openflare-relay -config ./relay.json
```

---

## 启动与验证

### 1. 查看进程日志

```bash
# Docker 容器日志
docker logs -f openflare-relay
```

如果是在 Linux 上通过 Systemd 托管的，可执行：
```bash
journalctl -u openflare-relay -f
```

### 2. 验证运行状态

启动成功后，Relay 将进行以下工作：
- 向控制面发送 HTTP 心跳以注册/上线。
- 从控制面获取最新的 frps 基础配置（包括 `bindPort`、`vhostHTTPPort` 与自动生成的隧道认证凭证 `auth_token`）。
- 在本地自动渲染出 `data/frps.toml` 配置文件。
- 自动拉起子进程 `frps -c data/frps.toml`。
- 如果进程意外崩溃，Relay 将在 2 秒后自动拉起它。

### 3. 管理端确认

登录管理后台，导航至 **「节点管理」**，确认：
- 该 TunnelRelay 节点状态标记为 **「在线」**。
- 节点类型正确标记为 **中继节点** 且 frps 运行状态为 **正常 (Healthy)**。
