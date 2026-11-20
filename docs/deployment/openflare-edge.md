# OpenFlare Edge 部署

OpenFlare Edge 是独立的 Pingora 边缘节点二进制。节点启动后直接连接主控，完成注册、心跳、配置同步、Pages 资源同步和流量代理。它与 Go Agent / OpenResty 部署线并行发布，同一节点只需要启动一个二进制，不需要额外安装 OpenResty 或暴露 Agent 到 OpenResty 的本地 HTTP 端口。

## 发布包

OpenFlare Edge 使用独立的 `edge-v*` 版本标签，与主控和 Go Agent 的 `v*` 发布隔离。发布工作流同时构建 Linux `amd64` 和 `arm64`：

- `openflare-edge_edge-v0.1.1_linux_amd64.tar.gz`
- `openflare-edge_edge-v0.1.1_linux_arm64.tar.gz`

归档内包含 `openflare-edge` 二进制、`VERSION`、`SHA256SUMS`、许可证、安装、卸载和 GeoIP 下载脚本。CI 会校验 `edge-v` 标签必须与 Rust 包版本一致，避免下载页与 `--version` 展示不同版本。

## 安装

使用共享配置契约 v3 的控制端要求 Edge `edge-v0.1.1` 或更新版本。安装器会在修改本地服务前拒绝旧发行包。

节点详情的「边缘节点部署」提供 Linux 本地部署命令，自动填入控制端 URL 和该节点的 Agent Token。Docker 命令用于部署 OpenResty Agent。迁移现有 OpenResty 节点时，先发布 Pingora 配置并停止旧服务。新版 Edge 使用原节点的 Agent Token 注册时，控制端自动切换该节点的引擎，保留节点 ID 和业务绑定。

在 Linux 节点执行：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type node \
  --token <agent-token> \
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

安装或升级时都会尝试刷新 GeoIP 数据库。下载失败会中止安装，避免节点以缺失的地区数据运行。可通过读取 `current` 相对符号链接确认当前版本。

指定版本安装：

```bash
sudo bash install-openflare-edge.sh --version edge-v0.1.1 ...
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

## 升级与回滚

日常仅更新二进制程序时，可直接运行无参数更新脚本，自动识别现有安装目录与状态目录，仅替换程序并重启服务，不改动现有 `openflare-edge.service` 配置、令牌、快照及 GeoIP 数据库：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash
```

如需同时修改控制端地址、令牌或监听端口，沿用安装命令 `install-openflare-edge.sh`。脚本校验并安装新二进制，更新 `current` 后重启 systemd 服务。确认服务处于运行状态、主进程指向新二进制且就绪记录与当前进程和版本一致后，脚本只保留当前发行版和上一个通过同样验证的发行版。首次安装通过验证后只保留当前版。重复安装同一版本时继续保留已有的回滚版本。

Edge 在监听端口就绪且已加载一份有效配置后通知 systemd。首次部署会等待从控制端同步配置。已有部署可以从磁盘恢复上次成功应用的配置。启动超时后安装器输出服务日志并回滚，运行中的服务正常停止时仍按原有方式处理在途连接。

新版本启动验证失败时，脚本恢复切换前的版本和服务配置，并重新启动原先运行的服务。`--no-service` 模式保留已下载的发行版，待后续由脚本验证服务后再清理。自动清理的范围为 `releases/` 下的发行版目录。

手动回滚：

```bash
sudo bash install-openflare-edge.sh --version <retained-edge-version> ...
```

卸载：

```bash
sudo bash uninstall-openflare-edge.sh
```

需要同时删除节点身份、配置快照、Pages 与 GeoIP 数据时增加 `--remove-state`。

## 与 OpenResty 线并行

OpenFlare Edge 的发布组、版本号和安装目录与 Go Agent / OpenResty 主线完全隔离。`pingora_enabled` 控制 Pingora 发布组，其余 OpenResty 节点继续使用原发布组。同一台节点迁移时，应先停止旧服务并释放 80/443，再安装 OpenFlare Edge。
