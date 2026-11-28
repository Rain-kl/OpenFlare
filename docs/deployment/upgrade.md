# 升级与维护

升级前先备份主关系数据库，并确认在线边缘节点已同步当前站点配置。

## 从 v3.6.x 迁移至 v4.0.0

v4.0.0 起使用基于 Pingora 的 `openflare-edge` 替代原 OpenResty 运行时；升级前记录原节点专属令牌，原 OpenResty 性能参数须在 **性能策略** 中重新配置。

1. 升级 OpenFlare Server 至 v4.0.0+，等待内置迁移自动完成并生成站点快照。
2. 逐台停用旧版服务并按 [部署 OpenFlare Edge](./openflare-edge.md) 使用原节点令牌安装新网关（可保留节点 ID、历史统计与域名绑定）：

   ```bash
   sudo systemctl stop openflare-agent openresty 2>/dev/null || true
   sudo systemctl disable openflare-agent openresty 2>/dev/null || true
   ```

3. 确认节点上线并正常响应 HTTP/HTTPS 后，再推进下一台节点。

## 升级至配置契约 v4（TLS 私钥内存态）

配置契约 v4（`tls-memory-keys-v1`）不再将 TLS 私钥写入快照文件：

1. 将 Server 与所有 `openflare-edge` 同步升级至支持 `POST /api/v1/agent/tls/keys` 的兼容版本。
2. 迁移 `202611230002_scrub_config_version_private_keys.sql` 会自动清除历史快照中残留的私钥字段并触发重编。
3. 离线冷重启期间，节点立即从本地快照恢复 HTTP 服务，待控制端恢复连通后自动拉取内存私钥恢复 HTTPS，无需重启。

## OpenFlare Server 升级

### 备份数据库

```bash
# PostgreSQL
docker compose exec postgres pg_dump -U openflare openflare > openflare_backup_$(date +%Y%m%d).sql

# SQLite
cp ./openflare-data/openflare.db ./openflare-data/openflare_backup_$(date +%Y%m%d).db
```

### 更新服务

```bash
# Docker Compose
docker compose pull && docker compose up -d

# 源码部署
git fetch --tags && git checkout <TARGET_VERSION_TAG>
make build-embedded && ./bin/openflare-server all
```

Server 启动时自动执行未运行的 goose 迁移并重编站点快照。

## OpenFlare Edge 更新

- **控制台远程更新**：在 **节点管理** 节点详情点击 **更新边缘节点** 选择稳定版或 Canary 标签，或开启 **自动更新节点**（每 6 小时自动检测稳定版）。
- **主机脚本更新**：自动保留令牌、快照与 GeoIP 库，并在缺失时补充 `CAP_NET_RAW` 能力：

  ```bash
  curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-edge.sh | sudo bash
  # 预览版附加 canary；指定版本附加 --version edge-vX.Y.Z
  ```

- **Docker 更新**：保留 `openflare-edge-data` 数据卷，拉取 `:latest`（或 `:canary`）并重建容器。

## OpenFlare DNS 更新

- **控制台远程更新**：已启用 `openflare-dns-update.path` 的 systemd 主机可在 **DNS 管理** > **SmartDNS** > **权威实例** 执行单台或批量更新（启动失败时自动回滚）。
- **主机脚本更新**：保留 `identity.json`、`last-snapshot.bin` 与 GeoIP 库：

  ```bash
  curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-dns.sh | sudo bash
  # 预览版附加 canary；指定版本附加 --version dns-vX.Y.Z
  ```

- **Docker 更新**：保留 `openflare_dns_data` 数据卷，拉取 `:latest`（或 `:canary`）并重建容器。

## OpenFlare Relay 与 OpenFlared 更新

- **控制台远程更新**：非 root 的 systemd 主机可在 **内网穿透** 中继或隧道详情页触发更新或开启每 6 小时的稳定版自动检查（双槽位 `next -> trial -> active` 试运行切换）。
- **主机脚本更新**：

  ```bash
  curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflare-relay.sh | sudo bash
  curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/update-openflared.sh | sudo bash
  # 预览版附加 canary；指定版本附加 --version tunnel-vX.Y.Z
  ```

- **Docker 更新**：保留原数据卷与环境变量，拉取最新镜像重建容器。

## 回滚与排障

升级后若服务未就绪，先检查日志：

```bash
sudo journalctl -u openflare-edge -n 100 --no-pager
sudo journalctl -u openflare-dns -n 100 --no-pager
sudo journalctl -u openflare-relay -n 100 --no-pager
sudo journalctl -u openflared -n 100 --no-pager
```

各组件在启动失败时均会保留既有健康配置或自动回退上一版本；如需手动回滚至历史标签，使用对应更新脚本传入 `--version <TAG>` 即可，无需重设令牌或状态目录。
