# Goroutine 生命周期模式

管理 goroutine 生命周期的详细模式 —— 确保每个 goroutine 都有清晰的启动/停止机制并防止资源泄漏。

---

## 使生命周期清晰

> WaitGroup 示例和作用域规则在父 skill（SKILL.md § Goroutine 生命周期，核心规则）中。本参考涵盖：stop/done channel 模式、等待策略、init() 生命周期示例和同步 API 设计。

---

## Stop/Done Channel 模式

每个 goroutine 必须有可预测的停止机制。使用 stop channel 通知关闭，使用 done channel 确认退出：

```go
var (
    stop = make(chan struct{}) // 通知 goroutine 停止
    done = make(chan struct{}) // 通知我们 goroutine 已退出
)
go func() {
    defer close(done)
    ticker := time.NewTicker(delay)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            flush()
        case <-stop:
            return
        }
    }
}()

// 关闭时：
close(stop)  // 通知 goroutine 停止
<-done       // 并等待它退出
```

在已关闭的 channel 上发送会 panic —— 始终使用 `close()` 来发信号，不要直接发送：

```go
ch := make(chan int)
close(ch)
ch <- 13 // panic: 在已关闭的 channel 上发送
```

---

## 等待 Goroutine

> 多 goroutine 的 `sync.WaitGroup` 模式在父 skill 中（SKILL.md § Goroutine 生命周期）。以下是单 goroutine 的 done-channel 替代方案。

为单个 goroutine 使用 done channel：

```go
done := make(chan struct{})
go func() {
    defer close(done)
    // 工作...
}()
<-done // 等待 goroutine 完成
```

---

## 不在 init() 中使用 Goroutine

> 核心规则在父 skill 中（SKILL.md § 核心规则，规则 3）。以下是展示生命周期管理的扩展示例。

```go
// 不好：创建了不可控的后台 goroutine
func init() {
    go doWork()
}
```

```go
// 好：显式的生命周期管理
type Worker struct {
    stop chan struct{}
    done chan struct{}
}

func NewWorker() *Worker {
    w := &Worker{
        stop: make(chan struct{}),
        done: make(chan struct{}),
    }
    go w.doWork()
    return w
}

func (w *Worker) Shutdown() {
    close(w.stop)
    <-w.done
}
```

---

## 优先使用同步函数

> 理由和优势表在父 skill 中（SKILL.md § 同步函数）。以下是具体的代码示例。

```go
// 好：同步函数 - 调用方控制并发
func ProcessItems(items []Item) ([]Result, error) {
    var results []Result
    for _, item := range items {
        result, err := processItem(item)
        if err != nil {
            return nil, err
        }
        results = append(results, result)
    }
    return results, nil
}

// 调用方可以在需要时添加并发：
go func() {
    results, err := ProcessItems(items)
    // 处理结果
}()
```
