# OpenFlare Edge 部署

OpenFlare Edge 是独立的 Pingora 边缘节点二进制。节点启动后直接连接主控，完成注册、心跳、配置同步、Pages 资源同步和流量代理。它与 Go Agent / OpenResty 部署线并行发布，同一节点只需要启动一个二进制，不需要额外安装 OpenResty 或暴露 Agent 到 OpenResty 的本地 HTTP 端口。

## 发布包

OpenFlare Edge 使用独立的 `edge-v*` 版本标签，与主控和 Go Agent 的 `v*` 发布隔离。发布工作流同时构建 Linux `amd64` 和 `arm64`：

- `openflare-edge_edge-v0.1.0_linux_amd64.tar.gz`
- `openflare-edge_edge-v0.1.0_linux_arm64.tar.gz`

归档内包含 `openflare-edge` 二进制、`VERSION`、`SHA256SUMS`、许可证、安装、卸载和 GeoIP 下载脚本。CI 会校验 `edge-v` 标签必须与 Rust 包版本一致，避免下载页与 `--version` 展示不同版本。

## 安装

在 Linux 节点执行：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token <discovery-or-node-token> \
  --http-listen 0.0.0.0:80 \
  --tls-listen 0.0.0.0:443
```

默认安装目录为 `/opt/openflare-edge`，状态目录为 `/var/lib/openflare-edge`。安装脚本会自动识别 `amd64` / `arm64`，从 GitHub Releases 查询最新的 `edge-v*` 版本，校验归档摘要后写入：

```text
/opt/openflare-edge/releases/<edge-version>/openflare-edge
/opt/openflare-edge/current -> releases/<edge-version>/openflare-edge
/var/lib/openflare-edge/identity.json
/var/lib/openflare-edge/snapshot.json
/var/lib/openflare-edge/pages/
/var/lib/openflare-edge/GeoLite2-Country.mmdb
/var/lib/openflare-edge/GeoLite2-City.mmdb
/var/lib/openflare-edge/access-logs.jsonl
```

安装或升级时都会尝试刷新 GeoIP 数据库。下载失败会中止安装，避免节点以缺失的地区数据运行。历史版本默认保留 3 个，可通过读取 `current` 相对符号链接确认当前版本。

指定版本安装：

```bash
sudo bash install-openflare-edge.sh --version edge-v0.1.0 ...
```

## 低权限运行

安装脚本创建 `openflare-edge` 系统用户，并以该用户运行 systemd 服务。服务仅授予绑定 80/443 所需的 `CAP_NET_BIND_SERVICE`，同时启用以下限制：

- `NoNewPrivileges`
- `PrivateTmp`
- `ProtectSystem=strict`
- `ProtectHome`
- `ProtectKernelTunables`
- `ProtectKernelModules`
- `ProtectControlGroups`
- `RestrictSUIDSGID`
- `LockPersonality`
- `MemoryDenyWriteExecute`
- `RestrictNamespaces`
- `SystemCallFilter=@system-service`

状态目录是唯一可写路径。令牌、节点身份、配置快照、GeoIP 数据库、Pages 包、访问明细队列和升级用 PID/Unix Socket 都保存在该目录。

## 进程模型与恢复

节点进程包含控制面循环和 Pingora 数据面守护进程。控制面负责注册、心跳、配置拉取、Pages 同步、结果上报，并监督数据面生命周期；数据面负责监听、TLS、代理与安全策略。二者使用同一个状态目录和二进制。

systemd 对控制面设置 `Restart=always` 与 `RestartSec=5`。控制面每秒检查数据面进程，数据面异常退出时立即拉起新数据面；控制面自身异常退出时由 systemd 拉起并重建监督关系。重启后先恢复最后一次有效配置快照，再连接主控刷新配置。主控不可用时，数据面仍按本地快照继续服务。

访问明细由数据面写入带文件锁的 `access-logs.jsonl`，控制面按心跳批量领取并上报。上报成功后删除已确认部分，队列最多保留 2048 条，避免异常重启或升级窗口造成内存增长或明细丢失。

## 平滑升级与回滚

升级仍使用同一个安装命令。安装脚本先校验并写入新版本，再执行 `systemctl reload`。systemd 向控制面发送 `SIGQUIT`，控制面先启动新版本数据面；新数据面按 Pingora 的升级协议接收 80/443 监听 FD，完成配置和运行时初始化后，控制面再向旧数据面发送 `SIGQUIT`，旧数据面继续处理存量连接并在排空后退出。

新版本无法完成接管时，systemd reload 返回失败，安装脚本立即将 `current` 切回上一版本并重启服务。历史版本保留在 `releases/` 中，因此回滚只需要本地文件，不依赖主控或下载源。

手动回滚：

```bash
sudo bash install-openflare-edge.sh --version edge-v0.1.0 ...
```

卸载：

```bash
sudo bash uninstall-openflare-edge.sh
```

需要同时删除节点身份、配置快照、Pages 与 GeoIP 数据时增加 `--remove-state`。

## 与 OpenResty 线并行

OpenFlare Edge 的发布组、版本号和安装目录与 Go Agent / OpenResty 主线完全隔离。`pingora_enabled` 只影响 Pingora 发布组，不改变现有 OpenResty 节点的目标版本。同一台节点切换引擎时，应先停止旧服务并避免两个引擎同时监听 80/443；建议在灰度节点上先部署 OpenFlare Edge，再下线旧 OpenResty 部署。
