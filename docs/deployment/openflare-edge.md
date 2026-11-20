# OpenFlare Edge 部署

OpenFlare Edge 是 OpenFlare v4.0.0 起使用的 Pingora 边缘节点。节点启动后直接连接控制端，完成注册、心跳、配置同步、Pages 资源同步和流量代理。已有 OpenResty 节点请按[升级指南](./upgrade.md)迁移。

## 发布包

OpenFlare Edge 使用独立的 `edge-v*` 版本标签。Server、Relay 和隧道节点随 `v*` 标签发布。Edge 发布工作流同时构建 Linux `amd64` 和 `arm64`：

- `openflare-edge_edge-v0.1.2_linux_amd64.tar.gz`
- `openflare-edge_edge-v0.1.2_linux_arm64.tar.gz`

归档内包含 `openflare-edge` 二进制、`VERSION`、`SHA256SUMS`、许可证、安装、更新、卸载和 GeoIP 下载脚本。推送 `edge-v<主版本>.<次版本>.<修订号>` 标签会触发发布。手动重建时，在 **Build OpenFlare Edge** 中填写已存在的 Edge 标签，工作流将构建该标签对应的提交。归档名和 `VERSION` 保留完整标签；例如标签为 `edge-v0.1.3` 时，`openflare-edge --version` 和节点上报的版本均为 `0.1.3`。发版无需修改 `Cargo.toml` 或 `Cargo.lock`。

推送 `canary` 分支会发布 Edge 预览版。版本采用下一修订号加工作流运行编号，例如从 `edge-v0.1.6` 生成 `edge-v0.1.7-canary.42`。控制台的「检查预览版」可发现该版本，默认安装和自动更新继续使用正式版。

GitHub Releases 的 Latest 指向 Server 正式版，Relay 和隧道节点默认从该发行版获取更新。Edge 的安装脚本、控制台和节点自动更新从发行列表独立查找 `edge-v*`。

Docker 镜像以 `ghcr.io/rain-kl/openflare-edge` 发布，支持 Linux amd64 和 arm64。每次发布都有对应的版本标签，正式版推送更新 `:latest`，Canary 推送更新 `:canary`。

Actions 中的 **Test OpenFlare Edge** 独立运行单元测试、原生运行时测试和容器测试。**Build OpenFlare Edge** 完成双架构编译后，**Publish OpenFlare Edge Release** 与 **Build Image (openflare-edge)** 并行下载同一批二进制并分别发布。发布失败可在产物保留期内单独重跑对应工作流。首次启用时，将两个发布工作流合入默认分支后再推送 Canary 或版本标签。

## 安装

使用配置同步协议 v1 的控制端要求 Edge `edge-v0.1.2` 或更新版本。安装器会在修改本地服务前检查发行包的协议版本。

节点详情的「边缘节点部署」提供 Linux 和 Docker 两种命令，自动填入控制端 URL 和节点专属令牌。

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

默认安装目录为 `/opt/openflare-edge`，状态目录为 `/var/lib/openflare-edge`。安装脚本会自动识别 `amd64` / `arm64`，从 GitHub Releases 查询最新正式版，校验归档摘要后写入：

```text
/opt/openflare-edge/current
/var/lib/openflare-edge/identity.json
/var/lib/openflare-edge/snapshot.json
/var/lib/openflare-edge/pages/
/var/lib/openflare-edge/GeoLite2-Country.mmdb
/var/lib/openflare-edge/GeoLite2-City.mmdb
/var/lib/openflare-edge/access-logs.jsonl
```

安装脚本会刷新 GeoIP 数据库，下载失败时中止安装。可执行 `/opt/openflare-edge/current --version` 查看已安装版本。

指定版本安装：

```bash
sudo bash install-openflare-edge.sh --version edge-v0.1.2 ...
```

### Docker 部署

从节点详情复制 Docker 命令，可直接启动 Pingora 容器。Canary 控制台默认生成 `:canary` 命令，其他控制台默认使用 `:latest`。指定镜像版本可将标签改为完整的 Edge 发布标签。

```bash
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<节点专属令牌>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

容器以 UID 和 GID `10001` 运行。节点身份、配置快照和 Pages 资源保存在数据卷。可通过 `OPENFLARE_EDGE_TOKEN_FILE` 指定一个只读挂载的令牌文件，该文件须允许容器用户读取。通过发现令牌首次注册时，可省略 `OPENFLARE_EDGE_TOKEN_TYPE`。

容器加载配置并开始监听后标记为健康。可用 `docker logs -f openflare-edge` 查看注册和同步过程。更新时拉取新镜像、重建容器并复用数据卷。

GHCR 首次创建的镜像包默认是私有的。需要让节点匿名拉取时，在包设置中公开 `openflare-edge`。

从当前源码构建镜像可在仓库根目录运行 `docker build -f docker/Dockerfile.edge -t openflare-edge:dev .`。

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

## 升级

日常仅更新二进制程序时，可直接运行无参数更新脚本，自动识别现有安装目录与状态目录，仅替换程序并重启服务，不改动现有 `openflare-edge.service` 配置、令牌、快照及 GeoIP 数据库：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash
```

安装和更新脚本均在校验归档及配置同步协议后，原子替换 `current` 二进制。每个安装目录只存放当前程序。旧安装首次切换时，会将 `current` 符号链接替换为程序文件，并清理原先的版本目录和验证记录。需要修改控制端地址、令牌或监听端口时，使用安装脚本。

更新脚本在下载和校验完成后，依次停止服务、替换程序、启动服务，并打印每一步。安装或更新正在进行时，后续脚本会直接返回锁占用错误。旧版 Edge 可能需要等待 300 秒的 Pingora 退出宽限期。自 `v0.1.2` 起，Pingora 的退出宽限期为 2 秒。可运行 `sudo journalctl -fu openflare-edge` 查看服务日志。

Edge 加载有效配置并开始监听后通知 systemd，更新脚本根据 systemd 的启动结果报告成功或失败。启动失败时，脚本输出服务日志并返回错误，新程序保留在 `current`。修复启动问题后，可重新执行更新命令。`--no-service` 安装完成后需手动启动。

卸载：

```bash
sudo bash uninstall-openflare-edge.sh
```

需要同时删除节点身份、配置快照、Pages 与 GeoIP 数据时增加 `--remove-state`。
