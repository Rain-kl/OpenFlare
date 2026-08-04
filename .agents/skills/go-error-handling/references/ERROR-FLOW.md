# 错误流程模式

错误流程、一次处理原则和日志决策的详细模式。

## 缩进错误流程

在继续正常代码之前先处理错误。这通过使读者能够快速找到正常路径来提高可读性。

```go
// 好：错误处理优先，正常代码无缩进
if err != nil {
    // 错误处理
    return // 或 continue 等
}
// 正常代码
```

```go
// 不好：正常代码隐藏在 else 子句中
if err != nil {
    // 错误处理
} else {
    // 正常代码因缩进看起来不自然
}
```

### 避免对长期使用的变量使用 if 初始化语句

如果变量在多行中使用，将声明移出：

```go
// 好：声明与错误检查分开
x, err := f()
if err != nil {
    return err
}
// 大量使用 x 的代码
// 跨越多行
```

```go
// 不好：变量作用域限制在 else 块中，难以阅读
if x, err := f(); err != nil {
    return err
} else {
    // 大量使用 x 的代码
    // 跨越多行
}
```

---

## 错误只处理一次

当调用者收到错误时，应该**只处理一次**。选择一种响应方式：

1. **返回错误**（包装或原文）让调用者处理
2. **记录日志并优雅降级**（不返回错误）
3. **匹配并处理**特定错误情况，返回其他错误

**如果返回了错误，就不要自己记录日志** — 让调用者处理。对同一错误既记录日志又返回是最常见的"一次处理"违规，导致重复噪音，因为调用栈上层的调用者也会处理该错误。

```go
// 不好：既记录日志又返回 — 导致日志噪音
u, err := getUser(id)
if err != nil {
    log.Printf("Could not get user %q: %v", id, err)
    return err  // 调用者也会记录这个！
}

// 好：包装并返回 — 让调用者决定如何处理
u, err := getUser(id)
if err != nil {
    return fmt.Errorf("get user %q: %w", id, err)
}

// 好：记录日志并优雅降级（不返回错误）
if err := emitMetrics(); err != nil {
    // 写入指标失败不应影响应用程序
    log.Printf("Could not emit metrics: %v", err)
}
// 继续执行...

// 好：匹配特定错误，返回其他错误
tz, err := getUserTimeZone(id)
if err != nil {
    if errors.Is(err, ErrUserNotFound) {
        // 用户不存在，使用 UTC
        tz = time.UTC
    } else {
        return fmt.Errorf("get user %q: %w", id, err)
    }
}
```

---

## 记录日志 vs 返回错误

> 错误只处理一次 — 记录日志或返回，不要两者都做。

### 决策流程

```
遇到错误？
├─ 调用者可以采取行动？→ 返回错误（通过 %w 附带上下文）
├─ 在调用链顶部？→ 记录日志并处理（返回 HTTP 状态码、退出等）
└─ 都不是？→ 以适当级别记录日志并继续
```

### 不要既记录日志又返回

```go
// 不好：错误既被记录又被返回 — 在日志中出现两次
func process(ctx context.Context, id string) error {
    result, err := fetch(ctx, id)
    if err != nil {
        log.Printf("failed to fetch %s: %v", id, err)
        return fmt.Errorf("fetching %s: %w", id, err)
    }
    return handle(result)
}

// 好：带上下文返回 — 让调用者决定是否记录日志
func process(ctx context.Context, id string) error {
    result, err := fetch(ctx, id)
    if err != nil {
        return fmt.Errorf("fetching %s: %w", id, err)
    }
    return handle(result)
}
```

### 结构化日志

在生产代码中，优先使用结构化日志（Go 1.21+ 的 `slog`，或 `log/slog` 兼容库）而非 `log.Printf`：

```go
// 好：结构化字段可被机器解析
slog.Error("fetch failed", "id", id, "err", err)

// 避免：非结构化的字符串插值
log.Printf("fetch failed for %s: %v", id, err)
```

### 日志级别

| 级别 | 使用场景 |
|------|---------|
| Error | 需要关注的可操作故障 |
| Warn  | 不需要立即处理的降级行为 |
| Info  | 关键生命周期事件（启动、关闭、配置加载） |
| Debug | 开发期间有用的诊断细节 |
