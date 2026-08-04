---
name: "cache-framework"
description: "Wavelet 项目专用：当新增或修改业务缓存（RAM/Redis/DB 三层读路径）、缓存失效、多节点 pub/sub 同步、或评估高频读是否应接入缓存时必须使用。本技能说明系统标准缓存框架、参考实现、禁止写法与分布式一致性要求。"
---

# 系统三层缓存框架

开始前阅读根目录 `AGENTS.md`（含 **Skill 关联索引**）。Wavelet 标准读路径为 **本地 RAM → Redis → PostgreSQL**（由快到慢），不是 DB 优先。

详细性能背景见 `docs/PERFORMANCE.md`。

## 关联 Skill

| 关联 | 何时一并阅读 |
| :--- | :--- |
| [database-migration](../database-migration/SKILL.md) | 缓存对象对应新表/列/索引，或 seed 变更 |
| [new-setting](../new-setting/SKILL.md) | 系统配置类缓存（`GetSystemConfigByKey`、`ListSystemConfigsByKeys`） |
| [file-upload](../file-upload/SKILL.md) | 上传元数据 `upload:meta:{id}`、ingest/remove/cleanup 失效钩子 |
| [clickhouse-batchwriter](../clickhouse-batchwriter/SKILL.md) | 分析写入走 batchwriter，**不要**用本技能模式缓存 CH flush 队列 |
| [new-api](../new-api/SKILL.md) | 在 Handler 层接入 `GetXxxCached` 或评估高频读 |
| [new-async-task](../new-async-task/SKILL.md) | Worker/定时任务变更数据后必须 `Invalidate*`（如 `system:cleanup`） |

## 标准模式（金标准）

参考：`internal/repository/system_config_cache.go` + `GetSystemConfigByKey` / `ListSystemConfigsByKeys`。

| 层级 | 技术 | 职责 |
| :--- | :--- | :--- |
| L1 本地 | `pkg/cache/ram`（Otter v2） | 进程内热数据，最低延迟 |
| L2 共享 | Redis `db.GetJSON` / `SetJSON` / `HSetJSON` + `db.PrefixedKey` | 跨节点共享，带 TTL 或写穿 |
| L3 权威 | PostgreSQL via `db.DB(ctx)` | 唯一数据源 |

### 读路径模板

```go
func GetThingCached(ctx context.Context, key string) (Thing, error) {
    ensureThingCacheListener() // 订阅 pub/sub，仅 sync.Once

    if v, ok := thingRAM.GetIfPresent(key); ok {
        return cloneThing(v), nil
    }
    if db.Redis != nil {
        var v Thing
        if err := db.GetJSON(ctx, redisKey(key), &v); err == nil {
            thingRAM.Set(key, cloneThing(v))
            return v, nil
        }
    }
    v, err := loadThingFromDB(ctx, key)
    if err != nil {
        return Thing{}, err
    }
    populateThingCache(ctx, v) // 回写 RAM + Redis
    return v, nil
}
```

### 写穿（populate）

DB miss 或业务创建成功后，**必须**回写上层：

```go
func populateThingCache(ctx context.Context, v Thing) {
    thingRAM.Set(v.Key, cloneThing(v))
    if db.Redis != nil {
        _ = db.SetJSON(ctx, redisKey(v.Key), v, cacheTTL)
    }
}
```

### 失效（Invalidate）— 分布式必做三步

数据变更（Admin 更新、软删除、状态迁移）时：

1. **本机 RAM** — `thingRAM.Invalidate(key)` 或 `InvalidateAll()`
2. **Redis** — `Del` / `HDel` 对应 key
3. **pub/sub 广播** — 通知**其他节点**清除 RAM（Redis 已由写节点清掉）

```go
func InvalidateThingCache(ctx context.Context, key string) error {
    ensureThingCacheListener()
    thingRAM.Invalidate(key)
    if db.Redis != nil {
        if err := db.Redis.Del(ctx, db.PrefixedKey(redisKey(key))).Err(); err != nil {
            return err
        }
        publishThingRAMInvalidation(ctx, key) // 只广播 RAM 失效
    }
    return nil
}
```

### pub/sub 监听模板

```go
const thingInvalidationChannel = "domain:thing_invalidation"

func startThingCacheInvalidationListener() {
    if db.Redis == nil {
        return
    }
    go func() {
        pubsub := db.Redis.Subscribe(context.Background(), thingInvalidationChannel)
        defer func() { _ = pubsub.Close() }()
        for msg := range pubsub.Channel() {
            // 解析 payload，Invalidate RAM；勿重复 Del Redis
            thingRAM.Invalidate(parsedKey)
        }
    }()
}
```

- 使用 `sync.Once` 启动监听；**`ensureListener` 必须在 `db.Redis == nil` 时直接 return，不可消费 Once**（否则测试或 Redis 晚初始化时监听器永不启动）。
- 测试可提供 `StopThingCacheListener` + 重置 `Once`（参考 `StopUploadMetaCacheListener`、`StopAuthSourceCacheListener`）。
- 其他节点收到消息后**只清 RAM**，不再删 Redis。

## 现有实现速查

| 域 | 文件 | L1 | L2 | pub/sub |
| :--- | :--- | :--- | :--- | :--- |
| 系统配置 | `repository/system_config_cache.go` | `pkg/cache/store` | ❌ 无 Redis 缓存 | `system:config_broadcast` (别名 `system:config_invalidation`) ✅ |
| CAPTCHA 运行时 | `apps/cap/runtime_settings.go` | atomic.Pointer | （借配置 Redis） | 订阅 `system:config_invalidation` ✅ |
| 上传元数据 | `apps/upload/cache/meta_cache.go` | Otter | Redis JSON | `upload:meta_invalidation` ✅ |
| 上传访问白名单 | `apps/upload/cache/access_cache.go` | 进程内 TTL | （借配置读路径） | `upload:file_access_invalidation` ✅ |
| Auth Source | `repository/auth_source_cache.go` | Otter | Redis JSON | `oauth:auth_source_invalidation` ✅ |
| OAuth 用户/Token | `apps/oauth/cache.go` | 自研 map | Redis JSON | ❌ 无 pub/sub（历史债） |
| 推送渠道 | `repository/push_channel.go` | 无 | Redis JSON | ❌ 仅 Redis Del |
| Storage 驱动 | `internal/infra/objectstore/storage.go` | RWMutex 快照 | — | `storage:config_invalidation` ✅ |

## 新增缓存工作流

1. **判定是否需要缓存**：高频读、低变更、可容忍短暂 TTL；写路径必须能统一失效。
2. **选型 L1**：优先 `pkg/cache/ram.MustNew`；**禁止**自研 `map+mutex+TTL`，除非有充分理由并文档说明。
3. **选型 L2**：小对象 `SetJSON`；配置类多条目用 Redis Hash（`HSetJSON`）。
4. **定义 Redis key**：小写蛇形，带业务前缀（`upload:meta:{id}`）；统一 `db.PrefixedKey`。
5. **实现 Invalidate + pub/sub**：凡多实例部署可读的 RAM 缓存**必须**有失效广播。
6. **挂载变更钩子**：在所有 DB 变更入口调用 Invalidate（含 Worker/定时任务，不只 HTTP Handler）。
7. **测试**：
   - RAM hit / Redis hit / DB fallback
   - Invalidate 清 L1+L2
   - pub/sub 触发他机 RAM 失效（可用 miniredis Publish 模拟）
   - `Reset*RAMCacheForTest` 仅清本机 RAM
8. 运行 `go test` 相关包 + `make code-check`。

## 变更钩子清单（上传元数据示例）

| 入口 | 动作 |
| :--- | :--- |
| `ingest.persistUploadRecord` 创建成功 | `SetUploadMetaCache` |
| `ingest.Remove` / `RemoveOwned` | `InvalidateUploadMetaCache` |
| `task/cleanup.go` 软删除 pending 文件 | `InvalidateUploadMetaCache` |
| 直接 `repository.SoftDeleteUpload` | **禁止** — 必须走 `upload.Remove` |

## 禁止写法

```go
// ❌ 自研 L1，与 pkg/cache/ram 重复
var mu sync.RWMutex
var items = map[uint64]entry{}

// ❌ 只清本机 RAM + Redis，无 pub/sub（多节点 RAM 脏读）
func Invalidate(ctx context.Context, id uint64) {
    localDelete(id)
    redis.Del(...)
}

// ❌ DB 变更后忘记 Worker 路径
// cleanup 任务删了 upload 行，但未 InvalidateUploadMetaCache

// ❌ 在 Handler 里直接查 DB，绕过已有 GetXxxCached

// ❌ Redis key 不用 PrefixedKey（多环境共 Redis 时冲突）

// ❌ 在 init() 里启动 pub/sub 监听 — 与 bootstrap 规范冲突；用 sync.Once 懒启动
```

## 特殊场景

### 敏感字段（ClientSecret）

模型 `json:"-"` 时，Redis DTO 用独立 `*RedisRecord` struct 显式序列化字段（见 `auth_source_cache.go`）。

### 批量读配置

批量接口必须与单 key 一致走 Redis（`ListSystemConfigsByKeys` 在 RAM miss 后逐 key `HGetJSON`，再 DB `IN`）。

### 仅进程内、短 TTL、配置衍生

可用进程内快照 + 订阅上游 pub/sub（`access_cache.go`、`cap/runtime_settings.go`），不必强行 Redis L2。

### OAuth 用户/Token

沿用 `oauth/cache.go`；新增逻辑调用 `SetCachedUser` / `SetCachedToken` 预热，变更调用 `InvalidateCachedUser` / `InvalidateCachedToken`。

## 验证清单

```bash
go test ./internal/repository/... ./internal/apps/upload/cache/...
make code-check
```

- [ ] L1 使用 `pkg/cache/ram`（或已文档化的例外）
- [ ] 读路径：RAM → Redis → DB
- [ ] 写穿 populate 在 DB load / 创建成功后
- [ ] Invalidate：RAM + Redis + Publish
- [ ] `ensureListener` + pub/sub 清他机 RAM
- [ ] 所有变更入口（含 Worker）已挂钩
- [ ] 测试含 Invalidate 与 pub/sub

## 相关文件

- L1 引擎：`pkg/cache/ram/cache.go`
- DB/Redis 助手：`internal/infra/persistence/redis.go`（`GetJSON`, `SetJSON`, `HGetJSON`, `PrefixedKey`）
- 金标准：`internal/repository/system_config_cache.go`
- 上传元数据：`internal/apps/upload/cache/meta_cache.go`
- Auth Source：`internal/repository/auth_source_cache.go`
- 性能文档：`docs/PERFORMANCE.md`