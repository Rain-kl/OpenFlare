# 部署 Pingora Edge

在控制台生成发现令牌，或在「节点」创建边缘节点并复制含专属令牌的安装命令。已登记的节点可以继续使用原专属令牌和节点记录。每个节点需要访问控制端 HTTPS 地址，并为访客开放相应的监听端口。

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \\
  --server-url https://openflare.example.com \\
  --token-type node \\
  --token '<节点专属令牌>'
```

使用发现令牌时将 `--token-type` 设置为 `discovery`。Linux 安装包支持 amd64 与 arm64，安装器使用支持当前配置协议的 `edge-v*` 版本。安装器默认创建 systemd 服务，监听 TCP 80 和 443，等待生效的站点快照加载后报告就绪。

使用 Docker 时，在节点详情复制「Docker 容器部署」命令。镜像渠道、数据卷与更新步骤见 [Edge Docker 部署](./openflare-edge.md#docker-部署)。

| 参数 | 说明 |
| --- | --- |
| `--server-url` | 控制端 HTTPS 地址 |
| `--token`、`--token-file` | 令牌或保存令牌的文件 |
| `--token-type` | `discovery` 或 `node` |
| `--version` | 指定 Edge 发行标签 |
| `--http-listen`、`--tls-listen` | 自定义监听地址 |
| `--install-dir`、`--state-dir` | 安装路径和状态目录 |

```bash
systemctl status openflare-edge
journalctl -u openflare-edge -f
```

确认节点在线后，可在 [性能策略](../guide/performance-policies.md) 中按服务器规格分配策略。需要移除节点服务时，在该节点运行发行包附带的 `scripts/uninstall-openflare-edge.sh`。
