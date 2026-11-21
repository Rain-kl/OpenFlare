---
title: CLI 与 MCP 运维
description: 使用 OpenFlare CLI 或 MCP 为 AI 授权日常网站运营与故障排查。
---

# CLI 与 MCP 运维

## 开通连接

在控制台的「设置 → 业务设置 → CLI 与 MCP」填写客户端可访问的 HTTPS 地址（例如 `https://openflare.example.com`），保存后打开 MCP 连接说明。反向代理需将 `/mcp`、`/.well-known/`、`/oauth/automation/`、`/automation/` 和现有 API 前缀转发至同一控制端，并保留原始 `Host` 请求头。若使用多实例控制端，各实例应共用数据库及会话密钥。

选择「复制给 AI」，将生成的文字发送给 AI。客户端注册 Streamable HTTP 连接后，在浏览器登录并授权。完成后通过 `openflare_connection_info` 核对实例 ID 和实际权限。客户端需支持远程 MCP、OAuth S256 PKCE 和动态客户端注册；支持带端口的 HTTP 本机回调，或与注册地址完全一致的 HTTPS 回调。无需部署独立 MCP 服务，也不需要把令牌发送给 AI。

| 授权选项 | 可执行操作 |
| --- | --- |
| 只读排查 | 查询站点、域名、证书、节点、访问日志、指标和性能策略。 |
| 日常运营 | 另可创建 Zone 和站点、登记并追加绑定域名、替换直连上游、修改已有性能策略的参数。档位内操作无需逐条批准。 |

授权面向当前实例，不限定到个别站点；实际权限也受客户端请求范围限制。在连接说明页可撤销当前账户的全部自动化授权，清空 HTTPS 地址并保存可暂停实例的自动化入口。访问令牌有效期 15 分钟，客户端可自动刷新，最长授权期为 30 天。

## 使用 CLI

从 [OpenFlare Releases](https://github.com/Rain-kl/OpenFlare/releases) 下载对应系统的 `openflare_cli_*`，解压后将 `openflare`（Windows 为 `openflare.exe`）加入 `PATH`。也可在源码目录运行 `go build -o bin/openflare ./cmd/openflare`。在 Linux 上，CLI 需要可用的 Secret Service 凭据库；macOS 和 Windows 使用系统凭据库。

```sh
openflare auth login --server https://openflare.example.com
openflare connection info
openflare operations list
openflare site create --help
```

登录时指定服务器地址，后续读写命令均使用保存的默认连接。管理多个实例时，可用 `openflare auth login --profile staging --server https://staging.example.com` 保存额外连接，访问该实例的命令再加 `--profile staging`。

数组参数使用 JSON 字符串数组，数字 ID 也要加引号。浏览器侧是否启用 HTTPS 与回源 URL 的协议分别设置。

## 常用操作

| 任务 | CLI / MCP |
| --- | --- |
| 创建网站 | 使用 `zone list` 或 `zone create` 取得 `zone_id`。用 `domain create` 登记域名并取得 `domain_record_id`，再用 `site create` 指定这些记录、上游 URL 和浏览器侧 HTTPS 开关。使用 HTTPS 时，先通过 `certificate list` 找到覆盖对应域名的现有证书。 |
| 新增站点别名 | 用 `site list` 找到 `site_id`，用 `domain create` 或 `domain list` 取得域名记录 ID，再执行 `site bind-domain`。该操作保留原有域名。 |
| 更换站点上游 | 先查询 `site list`，再使用 `site set-upstreams` 提交目标站点的完整直连上游列表。省略 `origin_host` 保留现有 Host 覆盖值。 |
| 排查流量与节点 | 使用 `logs list` 按主机、节点、状态码和时间查询；`metrics traffic` 查询流量趋势；`node list`、`node get` 和 `node metrics` 查看节点状态、配置应用情况和资源指标。 |
| 调整性能策略参数 | 先用 `performance list` 查看现有策略、受影响节点和参数范围，再使用 `performance set` 修改一个参数。修改默认策略影响未显式分配策略的 Pingora 节点。 |

MCP 工具使用相同的对象和动作，例如 `openflare_site_set_upstreams` 对应 `openflare site set-upstreams`。建站和绑定不会修改公网 DNS 或签发证书。控制端保存后由现有配置同步链路分发；通过 `node get` 比较 `current_version` 与 `desired_site_version`，核对节点是否已应用控制端当前的站点快照。
