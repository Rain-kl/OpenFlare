# 内网穿透隧道

通过 **中继池 → 公共中继 (`openflare-relay`) → 隧道与内网连接器 (`openflared`)** 将无公网 IP 的内网 L7 Web 应用或 L4 TCP/UDP 服务安全暴露至公网与边缘节点。

## 1. 配置中继池

进入 **隧道管理** > **中继池**，检查默认池或新增中继池：

1. 选择 **Preferred Transport**（默认 `auto`，按 `noise` → `websocket` → `tcp` 顺序回退）。
2. 设置 **HA Relay Count**（默认 `2`，生产建议跨可用区部署至少两台公共中继）。
3. 配置 **Allowed / Blocked Target CIDRs**（默认封禁 `169.254.169.254/32` 云元数据网段；Allowed 留空表示允许除 Blocked 外的所有目标）。

## 2. 部署公共中继

1. 进入 **隧道管理** > **中继节点** 点击 **添加中继** 或复制自动发现注册命令，可设置 **Zone Tag** 与同机共置的 **Co-located Edge Node ID**。
2. 按 [部署 Relay](../deployment/relay.md) 在公网主机启动 `openflare-relay`，确认状态变为 **在线**。

## 3. 创建隧道并启动连接器

1. 进入 **隧道管理** > **隧道列表** 点击 **创建隧道**，关联中继池、传输偏好及隧道级 **Allowed / Blocked Target CIDRs**。
2. 保存后复制 `oftok1_` 引导令牌，按 [部署 OpenFlared](../deployment/openflared.md) 在内网主机启动 `openflared`。
3. 在隧道详情确认连接器状态为 **在线** 且已建立活跃中继附着（Attachments）。

## 4. 发布服务

- **L7 HTTP / WebSocket 站点**：在 **代理路由** 中将回源模式设为 **内网穿透隧道**，选择隧道并填写内网目标 URL（如 `http://127.0.0.1:8080`；自签 HTTPS 可启用 `insecure_skip_verify`）。
- **L4 TCP / UDP 服务**：在隧道详情点击 **添加 L4 服务**，选择协议、中继、公网监听端口（避开 `7443`、`7444`、`8443` 与已占用端口）及私网目标地址（如 `127.0.0.1:22`），保存后通过 `<中继公网 IP>:<监听端口>` 访问。

## 运维与排错

| 操作 / 现象 | 说明与处理方式 |
| :--- | :--- |
| Drain（排空）中继 | 在中继详情点击 **Drain** 停止分配新连接器并促使现存 `openflared` 平滑迁移后再下线维护 |
| 多连接器双活 | 在多台内网主机使用同一隧道令牌运行多个 `openflared` 实例，故障时自动接管 |
| 隔离与轮换令牌 | 在隧道或中继详情点击 **Quarantine** 立即吊销凭证阻断流量；点击 **Release Quarantine** 签发新令牌 |
| 连接器始终离线 | 排查至 Server URL 及中继 `7443`、`7444`、`8443` 的出站连通性，确认池内存在非 Drain/非隔离的在线中继；轮换后需更新 `/var/lib/openflared/bootstrap-token` 并重启 |
| 访问返回 `403` / `502` | `403` 检查目标 IP 及域名解析是否命中 Allowed/Blocked CIDR 限制；`502` 检查 `openflared` 网络命名空间能否连通目标端口或需开启 `insecure_skip_verify` |
| L4 服务保存或监听失败 | 确认端口未与 `7443`、`7444`、`8443` 或其他 L4 服务冲突，并在主机与云防火墙放行对应端口 |

协议细节参见 [内网穿透架构设计](../design/tunnel-design.md)。
