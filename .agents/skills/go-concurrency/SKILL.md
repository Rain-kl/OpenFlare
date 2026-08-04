---
name: go-concurrency
description: Use when writing concurrent Go code — goroutines, channels, mutexes, or thread-safety guarantees. Also use when parallelizing work, fixing data races, or protecting shared state, even if the user doesn't explicitly mention concurrency primitives. Does not cover context.Context patterns (see go-context).
license: Apache-2.0
compatibility: Requires go.uber.org/atomic for atomic operation wrappers
metadata:
  sources: "Effective Go, Google Style Guide, Uber Style Guide"
---

# Go 并发

## Goroutine 生命周期

> **规范**：当你启动 goroutine 时，要明确它们何时或是否退出。

Goroutine 可能因阻塞在 channel 的发送/接收上而泄漏。GC **不会终止**被阻塞的 goroutine，即使没有其他 goroutine 持有对该 channel 的引用。即使不泄漏的在途 goroutine 也会导致 panic（在已关闭的 channel 上发送）、数据竞争、内存问题和资源泄漏。

### 核心规则

1. **每个 goroutine 都需要停止机制** —— 可预测的结束时间、取消信号，或两者兼有
2. **代码必须能够等待** goroutine 完成
3. **不在 `init()` 中启动 goroutine** —— 改为暴露生命周期方法（`Close`、`Stop`、`Shutdown`）
4. **保持同步作用域化** —— 限制在函数作用域内，将逻辑分解为同步函数

```go
// 好：使用 WaitGroup 明确生命周期
var wg sync.WaitGroup
for item := range queue {
    wg.Add(1)
    go func() { defer wg.Done(); process(ctx, item) }()
}
wg.Wait()
```

```go
// 不好：无法停止或等待
go func() { for { flush(); time.Sleep(delay) } }()
```

使用 [go.uber.org/goleak](https://pkg.go.dev/go.uber.org/goleak) **检测泄漏**。

> **原则**：永远不要在不知道 goroutine 将如何停止的情况下启动它。

> 在实现 stop/done channel 模式、goroutine 等待策略或
> 生命周期管理的 worker 时，阅读 [references/GOROUTINE-PATTERNS.md](references/GOROUTINE-PATTERNS.md)。

---

## 通过通信共享

> "不要通过共享内存来通信；而是通过通信来共享内存。"

这是 Go 并发设计的基础原则。使用 **channel** 进行所有权转移和协调 —— 当一个 goroutine 生产值，另一个消费它时使用。当多个 goroutine 访问共享状态且 channel 会增加不必要的复杂性时，使用 **互斥锁**。

**默认使用 channel。** 当问题本质上是保护共享数据结构（例如缓存或计数器）而非在 goroutine 之间传递数据时，退回到 `sync.Mutex` / `sync.RWMutex`。

---

## 同步函数

> **规范**：优先使用同步函数而非异步函数。

| 优势 | 原因 |
|---|---|
| 局部化 goroutine | 生命周期更容易推理 |
| 避免泄漏和竞争 | 更容易防止资源泄漏和数据竞争 |
| 更容易测试 | 直接检查输入/输出，无需轮询 |
| 调用方灵活性 | 调用方在需要时添加并发 |

> **建议**：在调用方移除不必要的并发是相当困难的（有时是不可能的）。让调用方在需要时添加并发。

> 在编写同步优先的 API（调用方可以将其包装在 goroutine 中）时，
> 阅读 [references/GOROUTINE-PATTERNS.md](references/GOROUTINE-PATTERNS.md)。

---

## 零值互斥锁

`sync.Mutex` 和 `sync.RWMutex` 的零值是有效的 —— 几乎不需要互斥锁的指针。

```go
// 好：零值有效           // 不好：不必要的指针
var mu sync.Mutex          mu := new(sync.Mutex)
```

**不要嵌入互斥锁** —— 使用命名的 `mu` 字段，使 `Lock`/`Unlock` 保持为实现细节，而非导出的 API。

> 在实现互斥锁保护的 struct 或决定如何组织互斥锁字段时，
> 阅读 [references/SYNC-PRIMITIVES.md](references/SYNC-PRIMITIVES.md)。

---

## Channel 方向

> **规范**：尽可能指定 channel 方向。

方向可以防止错误（编译器会捕获对仅接收 channel 的关闭操作），传达所有权，并且具有自文档化效果。

```go
func produce(out chan<- int) { /* 仅发送 */ }
func consume(in <-chan int)  { /* 仅接收 */ }
func transform(in <-chan int, out chan<- int) { /* 双向 */ }
```

### Channel 大小：一或零

Channel 的大小应为 **零**（无缓冲）或 **一**。其他任何大小都需要给出理由：

- 大小是如何确定的
- 什么机制防止 channel 在负载下填满
- 当写入者阻塞时会发生什么

```go
c := make(chan int)     // 无缓冲 —— 好
c := make(chan int, 1)  // 大小为 1 —— 好
c := make(chan int, 64) // 任意大小 —— 需要给出理由
```

> 在审查详细的 channel 方向示例及易出错模式时，
> 阅读 [references/SYNC-PRIMITIVES.md](references/SYNC-PRIMITIVES.md)。

---

## 原子操作

使用 `atomic.Bool`、`atomic.Int64` 等（Go 1.19 起标准库 `sync/atomic` 提供，或 [go.uber.org/atomic](https://pkg.go.dev/go.uber.org/atomic)）进行类型安全的原子操作。原始的 `int32`/`int64` 字段容易在某些代码路径上忘记原子访问。

```go
// 好：类型安全              // 不好：容易忘记
var running atomic.Bool       var running int32 // 原子操作
running.Store(true)           atomic.StoreInt32(&running, 1)
running.Load()                running == 1 // 竞争！
```

> 在 sync/atomic 和 go.uber.org/atomic 之间选择，或在 struct 中实现原子
> 状态标志时，阅读 [references/SYNC-PRIMITIVES.md](references/SYNC-PRIMITIVES.md)。

---

## 并发文档

> **建议**：当线程安全性从操作类型不明显时，添加文档说明。

Go 用户假设只读操作可以安全地并发使用，而修改操作则不行。在以下情况添加并发文档：

1. **读取与修改不明确** —— 例如，会修改 LRU 状态的 `Lookup`
2. **API 提供同步** —— 例如，线程安全的客户端
3. **接口有并发要求** —— 在类型定义中添加文档

---

## Context 使用

> 有关 context.Context 的指导（参数位置、struct 存储、自定义
> 类型、派生模式），请参阅专门的
> [go-context](../go-context/SKILL.md) skill。

---

## 使用 Channel 的缓冲池

使用有缓冲 channel 作为空闲列表来复用已分配的缓冲区。这种"泄漏缓冲"模式使用带 `default` 的 `select` 进行非阻塞操作。

> 在实现带可复用缓冲区的 worker pool 或在基于 channel 的池和
> `sync.Pool` 之间选择时，阅读 [references/BUFFER-POOLING.md](references/BUFFER-POOLING.md)。

---

## 高级模式

> 在实现使用 channel 的 channel 进行请求-响应多路复用，或
> 跨核心的 CPU 密集型并行计算时，阅读 [references/ADVANCED-PATTERNS.md](references/ADVANCED-PATTERNS.md)。

---

## 相关 Skill

- **Context 传播**：在通过 goroutine 传递取消、截止时间或请求作用域值时，请参阅 [go-context](../go-context/SKILL.md)
- **错误处理**：在从 goroutine 传播错误或使用 errgroup 时，请参阅 [go-error-handling](../go-error-handling/SKILL.md)
- **防御性加固**：在 API 边界保护共享状态或使用 defer 清理时，请参阅 [go-defensive](../go-defensive/SKILL.md)
- **接口设计**：在为包含 sync 原语的类型选择接收器类型时，请参阅 [go-interfaces](../go-interfaces/SKILL.md)

### 外部资源

- [永远不要在不知道 goroutine 将如何停止的情况下启动它](https://dave.cheney.net/2016/12/22/never-start-a-goroutine-without-knowing-how-it-will-stop)
  —— Dave Cheney
- [重新思考经典并发模式](https://www.youtube.com/watch?v=5zXAHh5tJqQ) —— Bryan Mills
  （GopherCon 2018）
- [Go 程序何时结束](https://changelog.com/gotime/165) —— Go Time 播客
- [go.uber.org/goleak](https://pkg.go.dev/go.uber.org/goleak) —— 用于测试的 Goroutine 泄漏检测器
- [go.uber.org/atomic](https://pkg.go.dev/go.uber.org/atomic) —— 类型安全的原子操作
