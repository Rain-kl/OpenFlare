---
name: go-context
description: 在 Go 中使用 context.Context 时使用 — 包括函数签名中的位置、传播取消和截止时间、以及在 context 中存储值与使用参数的对比。也适用于取消长时间运行的操作、设置超时或传递请求作用域数据，即使未直接提及 context.Context。不涵盖 goroutine 生命周期或 sync 原语（参见 go-concurrency）。
license: Apache-2.0
compatibility: 需要 Go 1.7+（context 在 Go 1.7 中移入标准库）
metadata:
  sources: "Go Wiki CodeReviewComments"
---

# Go Context 用法

## Context 作为第一个参数

使用 Context 的函数应将其作为**第一个参数**：

```go
func F(ctx context.Context, /* 其他参数 */) error
func ProcessRequest(ctx context.Context, req *Request) (*Response, error)
```

这是 Go 中的一个强约定，使 context 的传递在代码库中可见且一致。

---

## 不要在结构体中存储 Context

不要在结构体类型中添加 Context 成员。相反，将 `ctx` 作为参数传递给每个需要它的方法：

```go
// 不好：Context 存储在结构体中
type Worker struct {
    ctx context.Context  // 不要这样做
}

// 好：Context 传递给方法
type Worker struct{ /* ... */ }

func (w *Worker) Process(ctx context.Context) error {
    // Context 显式传递 — 生命周期清晰
}
```

**例外**：签名必须匹配标准库或第三方库中接口的方法可能需要变通处理。

---

## 不要创建自定义 Context 类型

不要创建自定义的 Context 类型或在函数签名中使用 `context.Context` 以外的接口：

```go
// 不好：自定义 context 类型
type MyContext interface {
    context.Context
    GetUserID() string
}

// 好：使用标准 context.Context 并提取值
func Process(ctx context.Context) error {
    userID := GetUserID(ctx)
}
```

---

## 应用数据放在哪里

按以下优先级顺序考虑：

1. **函数参数** — 最明确且类型安全
2. **接收者** — 适用于属于该类型的数据
3. **全局变量** — 适用于真正的全局配置（谨慎使用）
4. **Context 值** — 仅用于请求作用域数据

Context 值适用于：
- 请求 ID 和追踪 ID
- 随请求流动的认证/授权信息
- 截止时间和取消信号

Context 值**不适用**于：
- 可选的函数参数
- 可以显式传递的数据
- 不随请求变化的配置

---

## 常见模式

> 在派生 context（WithTimeout、WithCancel、WithDeadline）、在循环或 HTTP 处理器中检查取消、使用带类型键的 context 值、或需要快速参考表时，阅读 [references/PATTERNS.md](references/PATTERNS.md)。

### 派生 Context

创建派生 context 后，始终立即 `defer cancel()`：

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```

### 检查取消

```go
select {
case <-ctx.Done():
    return ctx.Err()
default:
    // 执行工作
}
```

### Context 不可变性

Context 是不可变的 — 将同一个 `ctx` 传递给共享相同截止时间和取消信号的多个并发调用是安全的。

---

## 相关技能

- **Goroutine 协调**：在使用 context 进行 goroutine 取消、基于 select 的超时或 errgroup 时，参见 [go-concurrency](../go-concurrency/SKILL.md)
- **错误处理**：在决定如何包装或返回 `ctx.Err()` 取消错误时，参见 [go-error-handling](../go-error-handling/SKILL.md)
- **接口设计**：在设计接受 context 并结合接口的 API 时，参见 [go-interfaces](../go-interfaces/SKILL.md)
- **请求作用域日志**：在将 logger 注入 context 或将请求 ID 添加到结构化日志输出时，参见 [go-logging](../go-logging/SKILL.md)
