# 高级并发模式

来自 Effective Go 的高级并发模式详细参考。这些模式适用于特定场景 —— 在需要请求/响应多路复用或 CPU 密集型并行化时使用。

---

## Channel 的 Channel

> **来源**：Effective Go

Channel 是一等公民值，可以像其他值一样被分配和传递。一个强大的模式是在请求结构体中嵌入 **回复 channel**，让每个客户端提供自己的应答路径：

```go
type Request struct {
    args       []int
    f          func([]int) int
    resultChan chan int
}
```

客户端发送一个包含函数、参数和接收结果 channel 的请求：

```go
request := &Request{[]int{3, 4, 5}, sum, make(chan int)}
clientRequests <- request
fmt.Printf("answer: %d\n", <-request.resultChan)
```

服务端处理器从队列中读取请求，并将结果发送回每个请求的回复 channel：

```go
func handle(queue chan *Request) {
    for req := range queue {
        req.resultChan <- req.f(req.args)
    }
}
```

这个模式构成了限速、并行、非阻塞 RPC 系统的基础，无需任何互斥锁。

---

## CPU 密集型并行化

> **来源**：Effective Go（现代化版本）

当计算可以分解为独立的部分时，使用 `sync.WaitGroup` 等待完成，将其并行化到多个 CPU 核心上：

```go
type Vector []float64

func (v Vector) DoSome(i, n int, u Vector) {
    for ; i < n; i++ {
        v[i] += u.Op(v[i])
    }
}

func (v Vector) DoAll(u Vector) {
    numCPU := runtime.NumCPU()
    var wg sync.WaitGroup
    wg.Add(numCPU)
    for i := 0; i < numCPU; i++ {
        go func(i int) {
            defer wg.Done()
            v.DoSome(i*len(v)/numCPU, (i+1)*len(v)/numCPU, u)
        }(i)
    }
    wg.Wait()
}
```

使用 `runtime.NumCPU()` 获取硬件核心数，或使用 `runtime.GOMAXPROCS(0)` 以遵循用户的资源配置。

> **重要**：不要混淆并发（将程序组织为独立执行的组件）和并行（在多个 CPU 上同时执行计算）。Go 是一门并发语言；并非所有并行化问题都适合它的模型。

---

## 常见错误

### 忘记通知完成

如果 goroutine 从未调用 `wg.Done()`（或从未在 done channel 上发送），等待的 goroutine 将永远阻塞：

```go
// 不好：缺少 wg.Done —— 死锁
var wg sync.WaitGroup
wg.Add(1)
go func() {
    doWork()
}()
wg.Wait()

// 好：始终 defer wg.Done
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    doWork()
}()
wg.Wait()
```

### 无限制的 goroutine 创建

为每个工作项无限制地启动 goroutine 可能会耗尽内存或压垮下游资源。使用信号量来限制并发数：

```go
// 不好：一次性创建 len(items) 个 goroutine
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func(it Item) {
        defer wg.Done()
        process(it)
    }(item)
}
wg.Wait()

// 好：信号量将并发限制为 maxWorkers
var wg sync.WaitGroup
sem := make(chan struct{}, maxWorkers)
for _, item := range items {
    wg.Add(1)
    sem <- struct{}{}
    go func(it Item) {
        defer wg.Done()
        defer func() { <-sem }()
        process(it)
    }(item)
}
wg.Wait()
```
