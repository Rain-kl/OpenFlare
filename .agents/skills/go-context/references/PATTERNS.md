# Context 模式

派生、检查和传播 `context.Context` 的常见模式。

---

## Context 不可变性

Context 是不可变的。将同一个 `ctx` 传递给共享相同截止时间、取消信号、凭据和父级追踪的多个调用是安全的：

```go
// 安全：同一个 context 传递给顺序调用
func ProcessBatch(ctx context.Context, items []Item) error {
    for _, item := range items {
        if err := process(ctx, item); err != nil {
            return err
        }
    }
    return nil
}

// 安全：同一个 context 传递给并发调用
func ProcessConcurrently(ctx context.Context, a, b *Data) error {
    g, ctx := errgroup.WithContext(ctx)
    g.Go(func() error { return processA(ctx, a) })
    g.Go(func() error { return processB(ctx, b) })
    return g.Wait()
}
```

---

## 何时使用 context.Background()

仅在**从不特定于请求**的函数中使用 `context.Background()`：

```go
func main() {
    ctx := context.Background()
    if err := run(ctx); err != nil {
        log.Fatal(err)
    }
}

func startBackgroundWorker() {
    ctx := context.Background()
    go worker(ctx)
}
```

**默认传递 Context**，即使你认为不需要。只有在有充分理由说明传递 context 是错误做法时，才直接使用 `context.Background()`：

```go
func LoadConfig(ctx context.Context) (*Config, error) {
    // 即使现在不使用 ctx，接受它可以在未来添加功能时
    // 不需要修改 API
}
```

---

## 派生 Context

```go
// 添加超时 — 持续时间结束后触发取消
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

// 添加取消 — 调用者控制何时取消
ctx, cancel := context.WithCancel(ctx)
defer cancel()

// 添加截止时间 — 在指定的墙钟时间触发取消
ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Hour))
defer cancel()

// 添加值（谨慎使用 — 仅用于请求作用域数据）
ctx = context.WithValue(ctx, requestIDKey, reqID)
```

创建派生 context 后，**始终立即 `defer cancel()`**。这确保即使函数提前返回，资源也会被释放。

### 嵌套派生

派生的 context 形成树状结构。取消父级会取消所有子级：

```go
func handleRequest(ctx context.Context) error {
    // 整个请求的父级超时
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // 数据库调用的更短超时
    dbCtx, dbCancel := context.WithTimeout(ctx, 5*time.Second)
    defer dbCancel()

    data, err := queryDB(dbCtx)
    if err != nil {
        return err
    }

    // 父级 context 的剩余时间适用于此处
    return sendResponse(ctx, data)
}
```

---

## 检查取消

### 在长时间运行的循环中

```go
func LongRunningOperation(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // 执行工作
        }
    }
}
```

### 在高开销操作之前

在开始无法中断的工作之前检查取消：

```go
func ProcessItems(ctx context.Context, items []Item) error {
    for _, item := range items {
        if ctx.Err() != nil {
            return ctx.Err()
        }
        if err := expensiveProcess(item); err != nil {
            return err
        }
    }
    return nil
}
```

### 区分取消原因

```go
if err := ctx.Err(); err != nil {
    switch {
    case errors.Is(err, context.Canceled):
        // 调用者显式取消（例如客户端断开连接）
    case errors.Is(err, context.DeadlineExceeded):
        // 超时或截止时间已过
    }
}
```

---

## 在 HTTP 处理器中遵守取消

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    result, err := slowOperation(ctx)
    if err != nil {
        if errors.Is(err, context.Canceled) {
            // 客户端已断开连接 — 无需写入
            return
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(result)
}
```

`r.Context()` 在以下情况被取消：
- 客户端关闭连接
- `http.Server` 的 `ReadTimeout` 或 `WriteTimeout` 触发
- `ServeHTTP` 方法返回

---

## Context 值的最佳实践

### 使用未导出的键类型

```go
type contextKey struct{}

var userIDKey contextKey

func WithUserID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, userIDKey, id)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
    id, ok := ctx.Value(userIDKey).(string)
    return id, ok
}
```

使用未导出的结构体类型作为键可以防止与其他包的键发生冲突 — 即使它们使用相同的 string 或 int 值。

### 提供访问器函数

始终将 `context.WithValue` 和 `ctx.Value` 包装在类型化的辅助函数中（如上所示），而不是暴露键。这提供了类型安全性和一个可以修改实现的单一位置。

---

## 快速参考

| 模式 | 指导 |
|------|------|
| 参数位置 | 始终第一个：`func F(ctx context.Context, ...)` |
| 结构体存储 | 不要存储在结构体中；传递给方法 |
| 自定义类型 | 不要创建；使用 `context.Context` 接口 |
| 应用数据 | 优先选择 参数 > 接收者 > 全局变量 > context 值 |
| 请求作用域数据 | 适用于 context 值 |
| 共享 context | 安全 — context 是不可变的 |
| `context.Background()` | 仅用于非请求特定的代码 |
| 默认行为 | 即使认为不需要也要传递 context |
| `defer cancel()` | 在 `WithTimeout`/`WithCancel`/`WithDeadline` 之后始终立即 defer |
| 值键 | 使用未导出的结构体类型，提供访问器函数 |
| 取消检查 | 在高开销操作前使用 `ctx.Err()`；在循环中使用 `select` 监听 `ctx.Done()` |
