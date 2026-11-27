# 升级与维护指南

在执行版本升级或运维操作前，请先备份主数据库，并确认当前站点配置已成功发布至各边缘节点。

---

## 从 v3.6.x 升级至 v4.0.0

OpenFlare 自 v4.0.0 起采用基于 OpenFlare Edge 的边缘网关（OpenFlare Edge）全面替代原 OpenResty 架构。升级前请记录原节点的专属令牌，自定义的 OpenResty 性能参数需在控制台「性能策略」中重新配置。

### 迁移步骤

1. 升级控制面 Server：升级 Server 至 v4.0.0+，确认数据库迁移自动执行完毕，且控制端已成功生成 OpenFlare Edge 站点配置快照。
2. 逐台迁移边缘节点：
   - 停止旧版本 Agent 及 OpenResty 服务，释放 TCP `80` 与 `443` 端口：
     ```bash
     sudo systemctl stop openflare-agent openresty 2>/dev/null || true
     sudo systemctl disable openflare-agent openresty 2>/dev/null || true
     ```
   - 在控制台节点详情中复制边缘节点部署命令，或参考 [OpenFlare Edge 部署指南](./openflare-edge.md) 部署新网关。使用原节点的专属令牌可保留原节点 ID、统计数据及域名绑定关系。
3. 验证并灰度推进：
   - 确认新节点状态变为在线，且成功加载有效快照。
   - 验证经过该节点的 HTTP 与 HTTPS 请求转发正常，确认无误后推进下一台节点。

---

## 升级至配置契约 v4（TLS 私钥内存态托管）

本次安全升级将 OpenFlare Edge 配置契约由 v3 升级为 v4（新增必选能力标识 `tls-memory-keys-v1`）：

1. **必须同步升级 `openflare-edge`**：控制面已将 TLS 私钥（`key_pem` 与 `*.key`）完全移出配置快照与 `sites.v1`，v3 及更早版本的 `openflare-edge` 不支持从 `POST /api/v1/agent/tls/keys` 按需拉取私钥，升级前请确保控制端与所有边缘节点均升级到匹配版本。
2. **历史快照自动脱敏**：控制面升级迁移 `202611230002_scrub_config_version_private_keys.sql` 会自动清空 `of_config_versions.support_files_json` 中历史残留的 `PRIVATE KEY` 内容，并推进配置版本重编（`of_edge_snapshot_state.source_revision`）。
3. **离线冷重启行为变更**：由于边缘磁盘不再落盘私钥，冷重启时 HTTP 站点可直接基于本地 `snapshot.json` 启动，HTTPS 站点会在控制面恢复可达后由内存密钥环自动补齐，无需人工干预或重启服务。

## Server 控制面升级

### 1. 数据库备份

在拉取新版本前，务必备份主数据库：

```bash
# PostgreSQL 备份
docker compose exec postgres pg_dump -U openflare openflare > openflare_backup_$(date +%Y%m%d).sql

# SQLite 单机版备份
cp ./openflare-data/openflare.db ./openflare-data/openflare_backup_$(date +%Y%m%d).db
```

### 2. Docker Compose 容器升级

拉取最新发布镜像并重启服务：

```bash
docker compose pull
docker compose up -d
docker compose ps
docker compose logs -f openflare
```

启动过程中，Server 会通过内置 migrator 自动应用未执行的数据库迁移脚本（涵盖 PostgreSQL 与 SQLite 双数据库引擎），并在启动后根据数据库当前配置自动重新编译站点快照。

### 3. 源码部署升级

```bash
git fetch --tags
git checkout <target-version-tag>

# 重新编译前端静态资源
cd frontend && pnpm install && pnpm build:embed && cd ..

# 重启 Server
go run ./cmd/server all
```

---

## 边缘节点更新

### 方式 1：控制台远程更新

1. 进入管理后台「节点管理」，打开目标边缘节点详情。
2. 点击「更新边缘节点」，选择正式版或预览版目标版本，确认后提交更新任务。
3. 控制面将在下一次心跳周期（默认 10 秒内）将更新指令下发至节点。节点下载新版本、校验 SHA-256 摘要、排空现有连接并平滑重启。
4. 若节点当前处于离线状态，指令将保留至节点恢复心跳后执行。

> [!TIP]
> 自动更新机制：在节点的「编辑」对话框中开启「自动更新节点」，节点在就绪后每 6 小时自动检测 GitHub Releases 较新的正式版并静默升级。手动触发的任务优先级高于自动更新。

### 方式 2：主机脚本就地更新

在 Linux 节点上执行更新脚本，脚本将自动检测已有安装目录与状态目录，原子替换 `/opt/openflare-edge/current` 二进制并重启 systemd 服务：

```bash
# 远程下载执行
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash

# 指定特定版本更新
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash -s -- --version edge-v0.1.3
```

### 方式 3：Docker 容器节点更新

拉取最新边缘节点镜像并重建容器（数据卷保留节点身份与缓存）：

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

---

## Relay 与 Tunnel 客户端更新

在管理后台对应的节点详情页中，可查看当前运行版本并执行更新，或开启自动更新。对于基于 Docker 运行的 Relay 和 OpenFlared：

```bash
# 更新 Relay
docker pull ghcr.io/rain-kl/openflare-relay:latest
docker stop openflare-relay && docker rm openflare-relay
# 使用原参数重新运行 docker run ...

# 更新 OpenFlared
docker pull ghcr.io/rain-kl/openflared:latest
docker stop openflared && docker rm openflared
# 使用原参数重新运行 docker run ...
```

---

## 异常回滚与故障排查

若升级后服务未正常拉起：

1. 查看边缘节点日志：
   ```bash
   sudo journalctl -u openflare-edge -n 100 --no-pager
   ```
2. 容灾快照保留机制：OpenFlare Edge 在启动失败时不会覆盖本地 `/var/lib/openflare-edge/snapshot.json` 快照文件，确保在排查期间持续按有效快照提供服务。
3. 回滚 Edge 版本：通过脚本传入历史稳定版本标签执行降级：
   ```bash
   sudo bash scripts/install-openflare-edge.sh --version edge-v0.1.2 ...
   ```
