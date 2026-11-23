# 边缘节点 TLS 私钥全内存态托管、下发审计与紧急隔离（方案 C）

## 背景

此前 Pingora 配置契约 v3 将全量证书私钥（`key_pem` / `*.key`）嵌入 `of_config_versions.support_files_json` 与 `sites.v1`，边缘节点将其持久化到本地 `snapshot.json`。单台边缘节点主机或备份盘一旦泄露，所有域名 TLS 私钥将随之暴露，且控制端缺少「某个节点究竟接触过哪些证书密钥」的精确审计与应急止血链路。

## 决策

1. **契约升级与私钥剥离（Contract v4）**：`edgeconfig.ContractVersion` 升级为 `4` 并强制要求 `tls-memory-keys-v1`。`config_version` 与 `edgeconfig` 快照只保留归一化公钥链 `cert_pem` 与 `cert_sha256`（`assets/certificates/*.crt`），彻底移除 `key_pem` 与 `*.key`；历史 `of_config_versions` 中的私钥记录由 `202611230002` goose 迁移自动清零。
2. **边缘节点全内存密钥环与离线容灾边界**：`openflare-edge` 在进程内维护 `CertificateKeyRing`（通过 `zeroize::Zeroizing` 清理响应缓冲区，校验证书公钥与摘要一致后写入，快照切换时调用 `retain_for` 剔除过期私钥）。控制面不可达时，已运行节点不受影响；若在控制面离线期间冷重启，代理可从无私钥的 `snapshot.json` 立即恢复 HTTP 路由与策略，HTTPS 握手拒绝直到控制面恢复可达并通过 `POST /api/v1/agent/tls/keys` 自动补全内存密钥，无需重启进程。
3. **Fail-closed 下发审计与紧急隔离**：新增 `of_node_cert_deliveries`（`repository.UpsertNodeCertDeliveries`），`POST /api/v1/agent/tls/keys` 先写库成功再下发私钥，响应头强制 `Cache-Control: private, no-store, ...`。节点触发紧急隔离时：轮换 `access_token`（脱敏不回显）、通过 `listener.NodeTokenRevoked` 在 `bootstrap` 立即失效本机与 Redis pub/sub 多副本鉴权缓存、向 WebSocket 下发 `quarantined` 帧（并由心跳响应头 `X-OpenFlare-Quarantined: true` 与 `X-OpenFlare-Auth-Error: invalid_token` 兜底）清零边缘节点内存私钥、自动切换 Cloudflare 备用解析，并按需派发携带 `previous_cert_pem` 的 `SSLSingleRenewTask` 以签发新密钥后按 `CRLReasonKeyCompromise`（RFC 5280 reason 1）吊销旧 ACME 证书（避免 Asynq 重试重复重签）。
4. **主机与进程加固**：Linux 上启动时设置 `PR_SET_DUMPABLE=0` 与 `RLIMIT_CORE=0`；`install-openflare-edge.sh` 设置状态目录权限 `0750` 与 systemd `LimitCORE=0` / `UMask=0077`，私钥写入临时文件按 `0600` 创建持久化目录，降低转储或旁路读取风险。

## 已知边界

- 方案 C 防护落盘与离线文件泄露；若攻击者已获取边缘主机 `root` 权限并对运行中进程内存直接读取，仍可在内存中截获当前活跃站点私钥，需后续按节点维度独立短有效期 ACME 证书（方案 B）进一步缩小单节点信任边界。
- 手动上传型证书在节点隔离后无法自动向外部 CA 吊销重签，需在控制台隔离报告或证书审计列表中由管理员手动替换并重新下发。
