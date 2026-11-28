# 常见故障排查

按症状定位控制面、Edge 节点、证书、缓存、观测与隧道接入问题。

## 快速定位表

| 现象 | 首选排查点 |
| :--- | :--- |
| 节点始终离线 | Server 公网地址可达性、Token 状态、防火墙出站端口与 `/var/lib/openflare-edge/identity.json` |
| 配置保存后 Edge 未更新 | 控制面日志、WebSocket 推送连通性，以及轮询同步周期 |
| 访问返回 `502` / `504` | Edge 到源站的连通性、`Host` 头、协议匹配（HTTP/HTTPS）与超时设置 |
| TLS 握手或浏览器证书警告 | 站点覆盖证书是否已绑定、ACME 验证路径可达性及源站验证模式 |
| 静态或半静态资源不命中缓存 | 源站 `Cache-Control: no-store` / `Set-Cookie` 响应头，及节点缓存规则 |
| 访问分析页面无数据 | Server 与 Edge 时间同步、节点异步日志发送与日志库写入状态 |
| 重定向循环（Too many redirects） | 源站与 Edge 的 HTTPS 强制跳转冲突 |

## 节点注册失败与离线

1. 在节点主机测试到控制面地址的 HTTPS 连通性：`curl -I https://your-openflare-server.com/api/v1/health`。
2. 核对 `/var/lib/openflare-edge/bootstrap-token` 与状态文件（`identity.json`、`ready.json`），必要时在 **节点管理** 检查是否被禁用或删除，并按 [OpenFlare Edge 部署](../deployment/openflare-edge.md) 检查服务日志。
3. 若使用 Discovery Token 批量接入，确认节点网络出口与服务日志存在正常心跳。

## 配置未同步到 Edge 节点

- 控制面默认通过 `/api/v1/agent/ws` 推送变更；若反向代理位于控制面之前，须放行 WebSocket `Upgrade` / `Connection` 请求头。
- 在 **节点管理** 对比节点 `current_version` 与目标版本，或使用 CLI `openflare node health` 检查 `config_sync` 是否为 `synced`。

## 访问源站报 `502` / `504` 或重定向循环

- **协议与证书**：若源站使用自签 HTTPS 证书，关闭源站证书严格校验或配置可信 CA 链；确保负载均衡/Cloudflare 与 OpenFlare 之间未同时开启相互冲突的“强制 HTTPS 跳转”。
- **Host 与连接池**：源站校验域名时保持原 `Host` 透传；若偶发上游断连超时，可在 [性能策略配置](./performance-policies.md) 调高读取超时或调整连接池复用。

## 边缘缓存命中率异常

- **完全未命中**：确认已开启路由 **边缘缓存**；若源站返回 `Cache-Control: private`、`no-store` 或携带 `Set-Cookie`，Edge 将默认跳过缓存。
- **频繁失效或串页**：检查是否配置了错误的 **强制刷新** 周期、忽略查询串开关，或未关闭对敏感动态接口的路径匹配。

## 访问分析页面无数据

- 检查控制面与 Edge 节点系统时间是否经 NTP 对齐；Edge 会将未实时送达的访问日志先缓冲在本地 spool，待链路恢复后重传（机制详见 [OpenFlare Edge 架构设计](../design/pingora-edge-design.md#53-访问日志缓冲与磁盘-spool)）。
- 确认控制台访问日志查询后端（ClickHouse / PostgreSQL / SQLite）可用且无写入报错。

## 管理员忘记密码与登录异常

- **重置管理员密码**（在 Server 主机或容器内执行交互式命令）：

```sh
./bin/openflare-server reset-passwd
# Docker Compose 环境：
docker compose exec openflare openflare-server reset-passwd
```

- **Auth / 验证码异常**：核对 **系统配置** > **安全设置** 中 Captcha 站点密钥、域名白名单，以及多实例间 `session_secret` 是否一致。
- **附件/文件上传失败**：检查存储后端配额、目录写权限及反向代理 `client_max_body_size` 限制。
