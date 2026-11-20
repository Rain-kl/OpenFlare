# 部署 Relay（Tunnel 中继）

TunnelRelay 节点为 OpenFlare 内网穿透体系提供公网中继端点。节点运行 `openflare-relay` 守护控制器与 `frps`（Fast Reverse Proxy Server）中继进程，负责维护公网隧道连接、接收由 Pingora 边缘网关转发的流量，并将其分发给内网中的 OpenFlared 客户端。

---

## 前置条件

在部署 TunnelRelay 节点前，需确认以下事项：

1. 节点注册：在 OpenFlare 管理后台「节点管理」中创建类型为 `tunnel_relay` 的节点，获取专属 `agent_token`（或使用系统全局 `discovery_token`）。
2. 网络端口规划：
   - `7000` (TCP)：frpc 客户端连接端口（`bindPort`），需允许公网客户端入方向访问。
   - `8080` (TCP)：HTTP 虚拟主机端口（`vhostHTTPPort`），供 Pingora 边缘网关向 frps 转发流量（通常仅需本地回环或节点间内网连通）。
   - `17500` (TCP，可选)：frps 内嵌 Web UI 管理端口（需在管理后台开启）。
3. 软件依赖（仅主机裸机部署需要）：系统需包含 `frps` 二进制文件，或在配置中指定其绝对路径。

---

## 配置参数与环境变量

`openflare-relay` 启动时默认读取当前工作目录下的 `relay.json`，支持通过系统环境变量覆盖全部配置项：

| JSON 字段 | 环境变量 | 说明 | 默认值 |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server 控制端接口地址 | 必填 |
| `agent_token` | `OPENFLARE_AGENT_TOKEN` | 节点专属 Token | 与下者二选一 |
| `discovery_token` | `OPENFLARE_DISCOVERY_TOKEN` | 自动发现注册 Token | 与上者二选一 |
| `node_name` | `OPENFLARE_NODE_NAME` | 节点显示名称 | 默认使用系统主机名 |
| `node_ip` | `OPENFLARE_NODE_IP` | 节点出口/监听 IP | 自动检测公网出口 IP |
| `frps_path` | `OPENFLARE_FRPS_PATH` | frps 二进制执行文件路径 | `"frps"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | 运行时数据与生成的 `frps.toml` 存放目录 | `"./data"` |
| `state_path` | - | 本地状态记录文件路径 | `"{data_dir}/relay-state.json"` |
| `heartbeat_interval`| - | 向 Server 上报心跳周期（毫秒或 Go Duration） | `10000` (10s) |
| `request_timeout` | - | 请求 Server 接口的超时时间（毫秒或 Go Duration） | `10000` (10s) |

---

## Docker 容器部署

官方 Docker 镜像内置了 `openflare-relay` 控制器及 `frps` 二进制运行时：

```bash
docker pull ghcr.io/rain-kl/openflare-relay:latest
docker rm -f openflare-relay 2>/dev/null || true

docker run -d --name openflare-relay --restart unless-stopped \
  -p 7000:7000 \
  -p 17500:17500 \
  -e OPENFLARE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_AGENT_TOKEN=YOUR_AGENT_TOKEN \
  -v openflare-relay-data:/app/data \
  ghcr.io/rain-kl/openflare-relay:latest
```

配置说明：
- `-p 7000:7000` 映射 frpc 客户端连接中继的核心端口。若在控制端设置了自定义 `relay_bind_port`，需同步调整宿主机端口映射。
- 内嵌 frps Web 监控面板：若在控制端系统设置开启了中继监控面板（`relay_frps_web_ui_enabled: true`），需将 Web 端口（默认 `17500`）映射至宿主机。Web 界面登录用户名为 `admin`，密码为当前中继节点的 `agent_token`。

---

## 运行与验证

### 自动化工作流

Relay 启动后按以下步骤自动化运行：
1. 向 Server 发送 HTTP 心跳完成节点注册与状态上线。
2. 从 Server 获取中继基础配置（包含 `bindPort`、`vhostHTTPPort` 及动态生成的隧道通信凭证 `auth_token`）。
3. 在本地数据目录渲染生成 `data/frps.toml` 配置文件。
4. 启动并托管子进程 `frps -c data/frps.toml`。
5. 若 `frps` 意外退出，Relay 以指数退避策略（初始 1 秒，最大 60 秒）自动重启子进程。

### 检查运行日志

```bash
# 查看容器运行日志
docker logs -f openflare-relay
```

### 控制台状态确认

登录 OpenFlare 管理后台，进入「节点管理」页面：
- 确认该 TunnelRelay 节点状态标记为在线。
- 确认节点类型为中继节点，且 frps 运行状态显示为正常 (Healthy)。
