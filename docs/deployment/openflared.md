# 部署 OpenFlared

`openflared` 是运行于内网主机的隧道连接器，内嵌 `rathole` 客户端并支持多副本冗余接入。调度策略与 CIDR SSRF 防护见 [内网穿透隧道设计](../design/tunnel-design.md)，L7/L4 发布步骤见 [内网穿透与隧道使用](../guide/tunnel-usage.md)。

## 接入条件

- **引导令牌**：在控制台 **内网穿透** > **逻辑隧道** 创建隧道后复制 `oftok1_` 引导令牌或一键安装命令。
- **出网连通**：内网主机无需公网入站端口，仅需出网访问 OpenFlare Server 以及中继池内 `openflare-relay` 的 `7443/tcp`（或 `7444/tcp`）与 `8443/tcp` 端口。

## Linux 主机部署

运行安装脚本部署静态二进制与 `openflared.service`：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-openflared.sh | sudo bash -s -- \
  --token "<OFTOK1_BOOTSTRAP_TOKEN>"
```

多副本部署可在不同主机重复执行该命令；同机部署多个实例时须指定不同的 `--state-dir` 与 `--connector-id`。常用安装参数（完整 CLI 与环境变量见 [配置项参考](../reference/configuration.md)）：

- `--server-url URL`：使用非 `oftok1_` 令牌时指定控制端地址。
- `--token-file PATH`：从文件读取令牌。
- `--connector-id ID`：显式指定持久化连接器 ID（默认自动生成 UUID）。
- `--version tunnel-vX.Y.Z` / `--install-dir` / `--state-dir` / `--no-service`：指定版本、目录或跳过 systemd 注册。

## Docker 容器部署

运行官方镜像 `ghcr.io/rain-kl/openflared`（非 root 用户 `10001`）：

```bash
docker run -d --name openflared --restart unless-stopped \
  -e OPENFLARE_TUNNEL_TOKEN="<OFTOK1_BOOTSTRAP_TOKEN>" \
  -v openflared-data:/var/lib/openflared \
  ghcr.io/rain-kl/openflared:latest
```

在 Linux 上转发宿主机 `127.0.0.1` 监听的服务时需添加 `--network host`；桥接模式下可直接指向同网络容器名或允许的局域网 IP。

## 验证与卸载

查看服务日志并在控制台 **内网穿透** > **隧道详情** 确认连接器在线且 **附着链路** 已连上池内中继：

```bash
sudo systemctl status openflared
sudo journalctl -u openflared -n 50 --no-pager
```

卸载 systemd 服务（追加 `--remove-state` 同时删除连接器 ID 与状态目录）：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/uninstall-openflared.sh | sudo bash
```

升级与回滚流程见 [升级与维护](./upgrade.md)。
