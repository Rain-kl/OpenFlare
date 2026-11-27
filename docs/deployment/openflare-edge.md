# OpenFlare Edge 部署

OpenFlare Edge 是 OpenFlare 自 v4.0.0 起采用的基于 OpenFlare Edge 构建的边缘网关。节点启动后连接控制面（OpenFlare Server），执行节点注册、心跳上报、配置同步、Pages 资源拉取与流量代理。原 OpenResty 架构节点请参考[升级与维护指南](./upgrade.md)执行迁移。

## 发布包与版本管理

OpenFlare Edge 使用独立的 `edge-v*` 版本标签进行发布，与 Server、Relay 和 OpenFlared 的发布解耦。Edge 构建工作流针对 Linux `amd64` 与 `arm64` 架构生成归档：

- `openflare-edge_edge-v0.1.3_linux_amd64.tar.gz`
- `openflare-edge_edge-v0.1.3_linux_arm64.tar.gz`

归档内包含 `openflare-edge` 二进制可执行文件、`VERSION`、`SHA256SUMS`、许可证文件，以及安装、更新、卸载与 GeoIP 下载脚本。

- 正式版触发：向仓库推送 `edge-v<主版本>.<次版本>.<修订号>` 格式标签触发发布构建。手动触发时，在 GitHub Actions 工作流 **Build OpenFlare Edge** 中输入目标 Edge 标签，工作流将针对该标签提交编译。
- 版本号注入：构建工作流通过环境变量 `OPENFLARE_EDGE_VERSION` 从 Git 标签注入版本号。`openflare-edge --version` 与节点上报版本均显示该版本（例如标签 `edge-v0.1.3` 对应程序版本 `0.1.3`），发版无需手动修改 `Cargo.toml` 中的版本字段；构建采用 `--locked` 保证依赖版本与 `Cargo.lock` 严格一致。
- Canary 预览版：向 `canary` 分支推送代码将触发 Edge 预览版构建。版本号采用下一修订号叠加工作流编号（例如基于 `edge-v0.1.3` 生成 `edge-v0.1.4-canary.42`）。控制台「检查预览版」可检索到该版本；默认安装与自动更新策略仅使用正式版。
- Release 归档索引：GitHub Releases 的 `Latest` 标签指向 Server 正式版；Edge 安装脚本、控制台更新检测及节点自动更新均独立筛选 `edge-v*` 标签列表。

Docker 容器镜像发布于 `ghcr.io/rain-kl/openflare-edge`，基于 Alpine 构建，包含静态链接 musl 的 Edge 二进制及系统 CA 证书，支持 Linux `amd64` 与 `arm64`。正式版发布同步更新 `:latest` 标签，Canary 预览版发布同步更新 `:canary` 标签。

## 节点安装

配置同步协议 v1 要求 OpenFlare Edge 版本为 `edge-v0.1.2` 或更高。安装器在修改本地服务前会校验发行包的协议兼容性。

在 OpenFlare 管理后台「节点管理」进入目标节点详情，点击「边缘节点部署」可查看预填控制端地址与节点令牌的安装命令。

### Linux 主机安装

使用安装脚本进行部署：

```bash
# 远程下载安装脚本并执行
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflare-edge.sh -o install-openflare-edge.sh
sudo bash install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type node \
  --token <节点专属令牌> \
  --http-listen 0.0.0.0:80 \
  --tls-listen 0.0.0.0:443
```

若在源码仓库或解压后的发布包目录下，可直接运行本地脚本：

```bash
sudo bash scripts/install-openflare-edge.sh \
  --server-url https://control.example.com \
  --token-type node \
  --token <节点专属令牌> \
  --http-listen 0.0.0.0:80 \
  --tls-listen 0.0.0.0:443
```

安装脚本的主要执行流程：
1. 识别当前系统架构（`amd64` 或 `arm64`），从 GitHub Releases 检索最新兼容正式版（或通过 `--version <tag>` 指定版本）。
2. 下载归档并校验 SHA-256 摘要，解压至目标目录。
3. 初始化默认路径与状态文件：
   - 运行程序：`/opt/openflare-edge/current`（指向当前版本的可执行二进制）
   - 节点身份凭据：`/var/lib/openflare-edge/identity.json`
   - 运行配置快照：`/var/lib/openflare-edge/snapshot.json`
   - Pages 静态资源：`/var/lib/openflare-edge/pages/`
   - GeoIP 数据库：`/var/lib/openflare-edge/GeoLite2-Country.mmdb` 与 `GeoLite2-City.mmdb`
   - 访问明细暂存队列：`/var/lib/openflare-edge/access-logs.jsonl`
4. 下载并校验 GeoIP 数据库（下载失败将终止安装流程）。
5. 创建 systemd 服务 `openflare-edge.service` 并启动。

验证版本与运行状态：

```bash
/opt/openflare-edge/current --version
sudo systemctl status openflare-edge
sudo journalctl -u openflare-edge -f
```

### Docker 容器部署

在管理后台节点详情复制 Docker 运行命令，或手动执行：

```bash
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  --cap-add=NET_RAW \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<节点专属令牌>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

容器运行说明：
- 容器内以非 root 用户（UID 10001 / GID 10001）运行，镜像内置 `cap_net_raw+p` 文件能力；配合 `--cap-add=NET_RAW` 后，仅独立探测子进程会在执行原生 ICMP Ping 与路由追踪探测时按需启用 `CAP_NET_RAW`，主代理线程保持最小权限。内部监听端口为 `8080`（HTTP）与 `8443`（HTTPS），通过宿主机端口映射对外暴露 `80` 与 `443`。
- 节点身份、配置快照与 Pages 资源持久化于数据卷 `openflare-edge-data`（挂载至 `/var/lib/openflare-edge`）。
- 可通过环境变量 `OPENFLARE_EDGE_TOKEN_FILE` 指定只读挂载的令牌文件路径（文件须允许 UID 10001 读取），替代明文传入 `OPENFLARE_EDGE_TOKEN`。
- 当容器加载有效配置并成功开启监听后，健康检查标记为 healthy。查看日志：`docker logs -f openflare-edge`。

## 权限与安全加固

Linux 安装脚本自动创建专用系统用户 `openflare-edge`，并以该用户运行 systemd 服务。服务仅申请低端口绑定特权 `CAP_NET_BIND_SERVICE`（允许非 root 进程监听低于 1024 的特权端口，如 80 和 443）与原始套接字特权 `CAP_NET_RAW`（供隔离探测子进程执行原生 ICMP Ping 与路由追踪），并通过以下安全策略限制权限：

- `NoNewPrivileges=yes`
- `PrivateTmp=yes`
- `ProtectSystem=strict`
- `ProtectHome=yes`
- `ProtectKernelTunables=yes`
- `ProtectKernelModules=yes`
- `ProtectControlGroups=yes`
- `RestrictSUIDSGID=yes`
- `LockPersonality=yes`
- `MemoryDenyWriteExecute=yes`
- `RestrictNamespaces=yes`
- `SystemCallFilter=@system-service`

状态目录 `/var/lib/openflare-edge` 是系统服务唯一具备写权限的目录，用于持久化令牌凭证、节点身份、配置快照、GeoIP 库、Pages 静态包与访问日志队列。

## 进程模型与容灾机制

OpenFlare Edge 二进制内置双进程架构：控制面监督进程与 OpenFlare Edge 数据面工作进程。

1. 控制面监督进程：负责维护长连接/心跳、拉取站点配置快照、同步 Pages 静态资源、上报运行指标，并监督数据面生命周期。
2. OpenFlare Edge 数据面工作进程：负责监听端口、处理 TLS 握手、执行 WAF 规则匹配、缓存加速及反向代理转发。

容灾与恢复机制：
- 进程级监控：控制面每秒检查数据面存活状态。数据面异常退出时，控制面立即重新拉起新的数据面进程；控制面进程异常退出时，由 systemd 自动重启（配置 `Restart=always`，间隔 5 秒）并重新接管数据面。
- 本地快照恢复：节点重启时，优先从本地 `/var/lib/openflare-edge/snapshot.json` 恢复最近一次有效配置，再向控制面请求最新配置。在控制面不可用或网络中断期间，数据面按本地快照持续提供代理服务。需要留意的是，为防止边缘磁盘泄密，本地 `snapshot.json` 与控制端配置快照（配置契约 v4 / `tls-memory-keys-v1`）均不再包含 TLS 私钥：正在运行的节点不依赖控制端即可继续处理 HTTPS；若节点在控制端不可达期间冷启动，HTTP 路由与静态策略可从本地快照立即恢复，HTTPS 私钥会在控制端恢复连通后通过 `POST /api/v1/agent/tls/keys` 自动按需拉取至内存并恢复 TLS 握手，无需重启进程。
- 全内存态 TLS 私钥与进程加固：边缘节点通过 `POST /api/v1/agent/tls/keys`（响应强制禁用缓存）获取活跃发布范围内的证书私钥并仅驻留于进程内存（`CertificateKeyRing`），快照切换或证书轮换后立即清除旧私钥并归零临时缓冲区；Linux 下启动时自动设置 `PR_SET_DUMPABLE=0` 与 `RLIMIT_CORE=0`，systemd 单元额外启用 `LimitCORE=0` 与 `UMask=0077` 且状态目录收紧至 `0750`，防止从核心转储或快照文件提取密钥。
- 证书下发审计与紧急隔离：控制面通过 `of_node_cert_deliveries` 记录每个节点在内存接收过的证书公钥指纹。发现单节点异常时，可在控制台触发「紧急隔离」，一键轮换并吊销节点接入令牌、失效鉴权缓存、通过 WebSocket 下发 `quarantined` 通知清零节点内存私钥、自动切换 Cloudflare 备用解析，并按需为受影响的 ACME 证书签发新密钥与通过 `CRLReasonKeyCompromise` 吊销旧证书；主机重新加固后可通过「解除隔离并重置令牌」恢复接入。
- 访问日志可靠上报：数据面通过文件锁向 `/var/lib/openflare-edge/access-logs.jsonl` 追加访问记录，控制面随心跳批量读取并向 Server 上报，确认成功后截断已上报数据。本地队列上限为 2048 条，防止异常断网导致磁盘与内存无限制增长。

## 升级与维护

### 二进制平滑升级

日常更新可直接使用更新脚本，该脚本自动检测已有的安装路径与状态目录，原子替换可执行二进制并在缺失时为既有 systemd 单元补充 `CAP_NET_RAW` 能力，同时保留其他配置、节点令牌、快照数据与 GeoIP 数据库：

```bash
# 更新到最新正式版
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash

# 切换到最近发布的预发布版本
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash -s -- canary
```

在本地仓库可执行 `sudo bash scripts/update-openflare-edge.sh canary`。省略 `canary` 即可切回最新正式版。若要更新到指定版本，使用 `--version TAG`。

更新过程特性：
- 校验下载归档的 SHA-256 摘要与配置协议兼容性。
- 原子替换 `/opt/openflare-edge/current` 二进制文件。
- 自 `edge-v0.1.2` 起，OpenFlare Edge 退出宽限期优化为 2 秒，平滑排空连接后重启。
- 更新脚本通过 systemd 校验启动结果。启动失败时，脚本输出服务日志并保留新版本二进制文件。

### Docker 容器升级

拉取最新镜像并重新创建容器：

```bash
docker pull ghcr.io/rain-kl/openflare-edge:latest
docker stop openflare-edge && docker rm openflare-edge
docker run -d --pull always --name openflare-edge --restart unless-stopped \
  --cap-add=NET_RAW \
  -p 80:8080 -p 443:8443 \
  -v openflare-edge-data:/var/lib/openflare-edge \
  -e OPENFLARE_EDGE_SERVER_URL=https://control.example.com \
  -e OPENFLARE_EDGE_TOKEN_TYPE=node \
  -e OPENFLARE_EDGE_TOKEN='<节点专属令牌>' \
  ghcr.io/rain-kl/openflare-edge:latest
```

保留原有数据卷 `openflare-edge-data`，新容器将直接复用节点身份与缓存快照。

### 节点卸载

停止并移除服务及安装文件：

```bash
# 远程下载卸载脚本执行
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflare-edge.sh | sudo bash -s --

# 或在本地仓库执行
sudo bash scripts/uninstall-openflare-edge.sh
```

如需彻底清除节点身份、配置快照、Pages 资源及 GeoIP 数据库，附加 `--remove-state` 参数：

```bash
sudo bash scripts/uninstall-openflare-edge.sh --remove-state
```
