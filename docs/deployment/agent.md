# 边缘节点快速接入

本节提供 Pingora Edge 边缘节点的快速接入指南。关于 Edge 发行包体系、Linux systemd 低权限加固规范、双进程监督模型及平滑更新机制，请参考 [OpenFlare Edge 部署指南](./openflare-edge.md)。

## 接入前准备

每个边缘节点需满足以下网络与访问条件：
- 控制端出网访问：节点须能够通过 HTTPS 访问 OpenFlare Server 控制端地址。
- 访客入网端口：节点需开放公网入方向 TCP 端口（默认 HTTP `80` 与 HTTPS `443`）。
- 节点认证令牌：
  - 专属令牌 (Node Token)：在管理控制台「节点管理」中创建节点，在节点详情中获取预生成的专属令牌与专属安装命令（推荐生产使用，绑定特定节点配置）。
  - 发现令牌 (Discovery Token)：在控制台系统设置中生成全局发现令牌，节点首次启动时向控制端动态登记为新节点。

## 快速安装命令

### Linux 主机快速安装

```bash
# 下载安装脚本并执行（使用节点专属令牌）
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://openflare.example.com \
  --token-type node \
  --token '<节点专属令牌>'
```

若使用发现令牌进行快速自动注册，设置 `--token-type discovery`：

```bash
sudo bash install-openflare-edge.sh \
  --server-url https://openflare.example.com \
  --token-type discovery \
  --token '<全局发现令牌>'
```

若在本地仓库源码目录下，可直接运行：

```bash
sudo bash scripts/install-openflare-edge.sh \
  --server-url https://openflare.example.com \
  --token-type node \
  --token '<节点专属令牌>'
```

### 安装脚本常用参数

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `--server-url URL` | OpenFlare Server 控制端接口地址（必填） | 无 |
| `--token TOKEN` | 节点认证令牌（与 `--token-file` 二选一） | 无 |
| `--token-file PATH` | 从指定文件路径读取令牌 | 无 |
| `--token-type TYPE` | 令牌类型：`node`（专属）或 `discovery`（发现） | `discovery` |
| `--version TAG` | 指定安装的 Edge 发布标签（例如 `edge-v0.1.3`） | 最新兼容正式版 |
| `--http-listen ADDR` | HTTP 监听地址 | `0.0.0.0:80` |
| `--tls-listen ADDR` | HTTPS/TLS 监听地址（传入 `none` 可禁用 TLS） | `0.0.0.0:443` |
| `--install-dir DIR` | 程序安装目录 | `/opt/openflare-edge` |
| `--state-dir DIR` | 状态与快照存储目录 | `/var/lib/openflare-edge` |
| `--no-service` | 仅解压与初始化文件，不创建/启动 systemd 服务 | `false` |

### Docker 容器快速启动

在控制台节点详情复制 Docker 运行命令，或直接运行：

```bash
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://openflare.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<节点专属令牌>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

## 运行状态验证

在节点服务器上检查 systemd 服务状态与输出日志：

```bash
# 检查服务运行状态
sudo systemctl status openflare-edge

# 实时查看节点运行日志
sudo journalctl -u openflare-edge -f
```

控制台验证：
1. 登录 OpenFlare 管理后台，进入「节点管理」。
2. 确认该边缘节点状态显示为在线，且已成功上报已加载的配置版本与 Pingora 运行时状态。
3. 进入 [性能策略](../guide/performance-policies.md) 页面，根据服务器硬件配置为该节点绑定针对性的性能策略。

卸载节点服务请运行 `scripts/uninstall-openflare-edge.sh`。详细升级与维护步骤参见 [升级与维护指南](./upgrade.md)。
