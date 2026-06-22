# of_options 迁移到 w_system_configs 方案

## 背景

当前 OpenFlare 使用 `of_options` 表存储系统配置，维护了独立的 OptionMap 内存缓存和热重载机制。为了统一配置管理框架，需要将所有配置迁移到标准的 `w_system_configs` 表，复用现有的 SystemConfig 读取 API 和 Redis 缓存机制。

## 配置项分类

### 已存在于 w_system_configs，无需迁移

以下配置在新系统中已存在，不从 of_options 迁移：

- `PasswordLoginEnabled` → `password_login_enabled` (已存在)
- `CapLoginEnabled` → `cap_login_enabled` (已存在)
- `PasswordRegisterEnabled` → `password_register_enabled` (已存在)
- `EmailVerificationEnabled` → 映射到 `email_login_verification_enabled` (已存在)
- `ServerAddress` → `server_address` (已存在)
- `SMTPServer` → `smtp_host` (已存在，字段名不同)
- `SMTPPort` → `smtp_port` (已存在)
- `SMTPAccount` → `smtp_username` (已存在，字段名不同)
- `SMTPToken` → `smtp_password` (已存在，字段名不同)

### 旧系统冗余配置，直接删除

以下配置是旧系统遗留，当前系统不使用，不迁移：

- `SystemName` - 前端不再使用系统名称配置
- `Footer` - 前端不再使用页脚 HTML
- `HomePageLink` - 前端不再使用首页链接
- `About` - 前端不再使用关于信息

### 需要迁移的配置（全部 type=business）

**所有迁移的配置都设为 business 类型**。system 类型仅用于框架级配置（如 upload_allowed_extensions、disk_cache_max_size_mb 等）。

#### Agent 相关配置 (business, visibility=0)

| 原 Key (PascalCase) | 新 Key (snake_case) | 默认值 | 说明 |
|---------------------|---------------------|--------|------|
| AgentDiscoveryToken | agent_discovery_token | "" | Agent 发现令牌（敏感） |
| AgentHeartbeatInterval | agent_heartbeat_interval | 10000 | Agent 心跳间隔（毫秒） |
| AgentWebsocketUpgradeEnabled | agent_websocket_upgrade_enabled | true | Agent WebSocket 升级开关 |
| NodeOfflineThreshold | node_offline_threshold | 120000 | 节点离线阈值（毫秒） |
| AgentUpdateRepo | agent_update_repo | Rain-kl/OpenFlare | Agent 更新仓库 |

#### 系统功能配置 (business, visibility=0)

| 原 Key (PascalCase) | 新 Key (snake_case) | 默认值 | 说明 |
|---------------------|---------------------|--------|------|
| GeoIPProvider | geoip_provider | ipinfo | GeoIP 服务商 |
| DatabaseAutoCleanupEnabled | database_auto_cleanup_enabled | false | 数据库自动清理开关 |
| DatabaseAutoCleanupRetentionDays | database_auto_cleanup_retention_days | 30 | 数据库保留天数 |

#### UptimeKuma 集成配置 (business, visibility=0)

| 原 Key (PascalCase) | 新 Key (snake_case) | 默认值 | 说明 |
|---------------------|---------------------|--------|------|
| UptimeKumaEnabled | uptime_kuma_enabled | false | UptimeKuma 集成开关 |
| UptimeKumaUrl | uptime_kuma_url | "" | UptimeKuma URL |
| UptimeKumaUsername | uptime_kuma_username | "" | UptimeKuma 用户名 |
| UptimeKumaPassword | uptime_kuma_password | "" | UptimeKuma 密码（敏感） |
| UptimeKumaMonitorScope | uptime_kuma_monitor_scope | all | UptimeKuma 监控范围 |
| UptimeKumaSelectedSites | uptime_kuma_selected_sites | "" | UptimeKuma 选定站点 |
| UptimeKumaSyncInterval | uptime_kuma_sync_interval | 5 | UptimeKuma 同步间隔（分钟） |
| UptimeKumaInterval | uptime_kuma_interval | 60 | UptimeKuma 监控间隔（秒） |
| UptimeKumaRetry | uptime_kuma_retry | 0 | UptimeKuma 重试次数 |
| UptimeKumaRetryInterval | uptime_kuma_retry_interval | 60 | UptimeKuma 重试间隔（秒） |
| UptimeKumaTimeout | uptime_kuma_timeout | 48 | UptimeKuma 超时（秒） |

### OpenResty 配置 (type=business)

OpenResty 反向代理和缓存配置，全部为 business 类型，visibility=0：

| 原 Key (PascalCase) | 新 Key (snake_case) | 默认值 |
|---------------------|---------------------|--------|
| OpenRestyDefaultServerReturnStatus | openresty_default_server_return_status | 421 |
| OpenRestyWorkerProcesses | openresty_worker_processes | auto |
| OpenRestyWorkerConnections | openresty_worker_connections | 4096 |
| OpenRestyWorkerRlimitNofile | openresty_worker_rlimit_nofile | 65535 |
| OpenRestyEventsUse | openresty_events_use | epoll |
| OpenRestyEventsMultiAcceptEnabled | openresty_events_multi_accept_enabled | true |
| OpenRestyKeepaliveTimeout | openresty_keepalive_timeout | 20 |
| OpenRestyKeepaliveRequests | openresty_keepalive_requests | 1000 |
| OpenRestyClientHeaderTimeout | openresty_client_header_timeout | 15 |
| OpenRestyClientBodyTimeout | openresty_client_body_timeout | 15 |
| OpenRestyClientMaxBodySize | openresty_client_max_body_size | 64m |
| OpenRestyLargeClientHeaderBuffers | openresty_large_client_header_buffers | 4 16k |
| OpenRestySendTimeout | openresty_send_timeout | 30 |
| OpenRestyResolvers | openresty_resolvers | "" |
| OpenRestyProxyConnectTimeout | openresty_proxy_connect_timeout | 3 |
| OpenRestyProxySendTimeout | openresty_proxy_send_timeout | 60 |
| OpenRestyProxyReadTimeout | openresty_proxy_read_timeout | 60 |
| OpenRestyWebsocketEnabled | openresty_websocket_enabled | true |
| OpenRestyHTTP3Enabled | openresty_http3_enabled | true |
| OpenRestyProxyRequestBufferingEnabled | openresty_proxy_request_buffering_enabled | false |
| OpenRestyProxyBufferingEnabled | openresty_proxy_buffering_enabled | true |
| OpenRestyProxyBuffers | openresty_proxy_buffers | 16 16k |
| OpenRestyProxyBufferSize | openresty_proxy_buffer_size | 8k |
| OpenRestyProxyBusyBuffersSize | openresty_proxy_busy_buffers_size | 64k |
| OpenRestyGzipEnabled | openresty_gzip_enabled | true |
| OpenRestyGzipMinLength | openresty_gzip_min_length | 1024 |
| OpenRestyGzipCompLevel | openresty_gzip_comp_level | 5 |
| OpenRestyCacheEnabled | openresty_cache_enabled | false |
| OpenRestyCachePath | openresty_cache_path | "" |
| OpenRestyCacheLevels | openresty_cache_levels | 1:2 |
| OpenRestyCacheInactive | openresty_cache_inactive | 30m |
| OpenRestyCacheMaxSize | openresty_cache_max_size | 1g |
| OpenRestyCacheKeyTemplate | openresty_cache_key_template | $scheme$host$request_uri |
| OpenRestyCacheLockEnabled | openresty_cache_lock_enabled | true |
| OpenRestyCacheLockTimeout | openresty_cache_lock_timeout | 5s |
| OpenRestyCacheUseStale | openresty_cache_use_stale | error timeout updating http_500... |
| OpenRestyMainConfigTemplate | openresty_main_config_template | (长模板) |

**总计**：约 58 个配置需要迁移，全部为 business 类型。

## 迁移策略

### 1. 保持向后兼容

- 在迁移期间同时支持旧 API (`/api/v1/openflare/options`) 和新 API (`/api/v1/admin/system-configs`)
- 旧 API 内部委派到 SystemConfig 读写，不再直接操作 of_options 表
- 保留 `/api/v1/openflare/status` 接口，但从 SystemConfig 读取数据

### 2. 数据迁移顺序

1. 创建新的 ConfigKey 常量（已完成）
2. 创建 goose 迁移脚本，将 of_options 真正需要的配置复制到 w_system_configs（已完成）
3. 重构代码使用 repository.GetSystemConfigByKey / GetBoolByKey / GetIntByKey
4. 标记 of_options 表为 deprecated（保留一段时间用于回滚）
5. 后续版本完全删除 of_options 相关代码

### 3. 配置类型说明

**所有从 of_options 迁移的配置都设为 business 类型**：
- `type='business'`：业务配置，影响业务规则和功能行为
- `type='system'`：框架配置，仅用于框架级设置（如 upload_allowed_extensions、disk_cache_max_size_mb）

**不迁移的配置**：
- 已存在于 w_system_configs 的配置（如 password_login_enabled、smtp_host）
- 旧系统冗余配置（SystemName、Footer、HomePageLink、About）

### 4. 包级变量处理

原 `openflare_option.go` 中的包级变量（如 `SystemName`、`PasswordLoginEnabled`）将被移除。所有读取改为：

```go
// 旧方式（包级变量）
systemName := model.SystemName
enabled := model.PasswordLoginEnabled

// 新方式（repository 读取）
systemName, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySystemName)
enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyPasswordLoginEnabled)
```

注意：对于已存在的配置，使用对应的新 key：
```go
// 已存在的配置使用现有 key
enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyPasswordLoginEnabled)  // 不是 ConfigKeySystemName
```

### 5. 热重载机制

- 移除 `InitOptionMap` 和 `OptionMapRWMutex`
- SystemConfig 已通过 Redis 缓存实现热重载，更新后自动失效

## 实现步骤

1. ✅ 分析配置项并设计迁移方案（本文档）
2. 在 `system_configs.go` 添加所有 ConfigKey 常量
3. 创建 goose 迁移脚本（PostgreSQL + SQLite）
4. 重构所有业务代码使用 SystemConfig API
5. 更新 option 模块 API 和测试
6. 清理 of_options 遗留代码并验证

## 风险与注意事项

1. **敏感配置处理**：包含 Token/Password/Secret 的配置不应暴露给前端（visibility=0）
2. **类型转换**：of_options 将数值存为字符串，需要在读取时正确转换
3. **默认值一致性**：确保 SQL 迁移中的默认值与代码中的默认值一致
4. **模板配置**：`OpenRestyMainConfigTemplate` 是长文本，需要正确处理
5. **测试覆盖**：所有使用 OptionMap 的测试需要更新为使用 SystemConfig

## 后续清理计划

迁移完成后，在下一个主版本（如 v2.0）中完全移除：

- `internal/model/openflare_option.go`
- `internal/model/openflare_option_apply.go`
- `of_options` 表及相关迁移文件
- `internal/apps/openflare/option` 模块（或重构为 SystemConfig 的代理）
