---
title: CLI 与 MCP 运维
description: 使用 OpenFlare CLI 或 MCP 为 AI 授权日常网站运营与故障排查。
---

# CLI 与 MCP 运维

## 开通连接

在控制台打开「系统设置 → MCP」，填入客户端可访问的 HTTP 或 HTTPS 地址并保存。公网部署建议使用 HTTPS。该标签页显示 MCP 地址、实例 ID 和两种连接方式。清空地址并保存可暂停本实例的 CLI 和 MCP 入口。部分客户端要求 MCP 地址使用 HTTPS。

- **OAuth**：复制连接说明，或在客户端添加 Streamable HTTP 连接并选择 OAuth。客户端须支持动态客户端注册和 S256 PKCE。首次连接时，在浏览器使用管理员账户登录，勾选需要授予的权限并确认。
- **Access Token**：为支持自定义请求头的客户端勾选权限并生成 Access Token。令牌只显示一次，有效期为 30 天。将它填入客户端的 `Authorization: Bearer TOKEN` 请求头。

连接后可调用 `openflare_connection_info` 核对实例 ID 和已授权权限。OAuth 访问令牌有效期为 15 分钟，客户端可自动刷新，授权最长持续 30 天。「撤销全部授权」会使当前账户授予 CLI 和 MCP 客户端的授权与 Access Token 立即失效。

若页面提示会话密钥不足 32 字节，请检查 `app.session_secret` 或 `APP_SESSION_SECRET`。新实例须使用强随机密钥；已保存证书私钥、DNS 凭据等加密数据的实例须先迁移凭据，再更换密钥。

反向代理需将 `/mcp`、`/.well-known/`、`/oauth/automation/`、`/automation/` 和现有 API 前缀转发至同一控制端，并保留原始 `Host` 请求头。多实例部署时，各控制端应共用数据库及会话密钥。

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
