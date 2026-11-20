# 部署 OpenFlared 客户端

OpenFlared 是部署于内网私有环境（局域网、私有云或无公网 IP 的服务器）的隧道客户端程序。OpenFlared 通过 `X-Tunnel-Token` 与控制面（OpenFlare Server）保持通信，动态拉取隧道规则并在本地自动启动和托管一个或多个 `frpc`（Fast Reverse Proxy Client）子进程，将本地私有服务的 HTTP/TCP 流量经公网中继节点向外暴露。

---

## 前置条件

1. 获取 Tunnel Token：在 OpenFlare 管理后台「节点管理」中创建类型为 Tunnel 的节点，在节点详情中复制该节点的接入凭证 `tunnel_token`。
2. 网络出网权限：内网服务器无需配置任何公网入站端口映射，但必须具备访问以下地址的出网连通性：
   - OpenFlare Server 控制端接口地址
   - 绑定的 TunnelRelay 节点的公网中继端口（默认 `7000`）
3. 软件依赖（仅主机裸机部署需要）：系统需包含 `frpc` 二进制文件，或在配置中指定其绝对路径。

---

## 配置参数与环境变量

`openflared` 启动时默认读取当前工作目录下的 `flared.json`，支持通过系统环境变量覆盖全部配置项：

| JSON 字段 | 环境变量 | 说明 | 默认值 |
| --- | --- | --- | --- |
| `server_url` | `OPENFLARE_SERVER_URL` | OpenFlare Server 控制端接口地址 | 必填 |
| `tunnel_token` | `OPENFLARE_TUNNEL_TOKEN` | 隧道客户端专属接入 Token | 必填 |
| `frpc_path` | `OPENFLARE_FRPC_PATH` | frpc 可执行二进制路径 | `"frpc"` |
| `data_dir` | `OPENFLARE_DATA_DIR` | 运行时数据与生成的 `frpc_{relayNodeID}.toml` 目录 | `"./data"` |
| `state_path` | - | 本地状态记录文件路径（保存最后应用的配置版本）| `"{data_dir}/flared-state.json"` |
| `heartbeat_interval`| - | 节点状态与心跳上报周期（毫秒或 Go Duration） | `10000` (10s) |
| `sync_interval` | - | 隧道路由规则拉取同步周期（毫秒或 Go Duration） | `30000` (30s) |
| `request_timeout` | - | 请求 Server 接口的超时时间（毫秒或 Go Duration） | `10000` (10s) |

---

## Docker 容器部署

官方 Docker 镜像内置了客户端控制器及 `frpc` 运行时：

```bash
docker pull ghcr.io/rain-kl/openflared:latest
docker rm -f openflared 2>/dev/null || true

docker run -d --name openflared --restart unless-stopped \
  -e OPENFLARE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_TUNNEL_TOKEN=YOUR_TUNNEL_TOKEN \
  -v openflared-data:/app/data \
  ghcr.io/rain-kl/openflared:latest
```

如需穿透访问宿主机上的本地服务（例如宿主机的 `http://127.0.0.1:8000`），在 Linux 下可使用 `--network host` 运行容器；或在默认桥接网络下使用 `host.docker.internal:8000` 作为后端地址。

---

## 运行与验证

### 自动同步流程

OpenFlared 运行后执行以下自动化操作：
- 心跳与状态同步：定期请求 Server 的 `/api/v1/tunnel/heartbeat` 与 `/api/v1/tunnel/config/active` 接口，校验 Token 有效性并对比配置版本。
- 动态配置渲染：当检测到路由规则版本或 Checksum 发生变更时，自动拉取最新的路由映射清单。若该隧道关联了多个中继 Relay，将为每个 Relay 分别在 `data_dir` 下渲染生成独立的 `frpc_{relayNodeID}.toml`。
- 进程重载与热启：配置变更后，重启对应的 `frpc` 子进程以使新路由生效。
- 故障自恢复：若本地 `frpc` 子进程异常中断，控制器按照指数退避策略（初始 1 秒，最大 60 秒）自动尝试拉起。

### 查看日志与连接状态

```bash
# 查看容器运行日志
docker logs -f openflared
```

运行正常的日志输出示例：

```text
flared config loaded ...
detected frpc version v0.69.0
flared process started
applying new tunnel config {"version": "...", "checksum": "..."}
frpc process missing, starting {"relay_id": "..."}
```

### 控制台状态确认

登录 OpenFlare 管理后台，进入「节点管理」并查看对应 Tunnel 节点详情：
- 确认节点在线状态标记为在线。
- 确认已同步的配置版本号及最近一次更新时间。
