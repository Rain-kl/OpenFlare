# 同步原语模式

互斥锁和原子操作的详细模式 —— 涵盖互斥锁嵌入陷阱和类型安全的原子访问。

---

## 不要嵌入互斥锁

如果你通过指针使用结构体，互斥锁应该是非指针字段。不要在结构体中嵌入互斥锁，即使该结构体未被导出。

```go
// 不好：嵌入的互斥锁将 Lock/Unlock 暴露为 API 的一部分
type SMap struct {
    sync.Mutex // Lock() 和 Unlock() 成为 SMap 的方法
    data map[string]string
}

func (m *SMap) Get(k string) string {
    m.Lock()
    defer m.Unlock()
    return m.data[k]
}
```

```go
// 好：命名字段使互斥锁保持为实现细节
type SMap struct {
    mu   sync.Mutex
    data map[string]string
}

func (m *SMap) Get(k string) string {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.data[k]
}
```

在不好的示例中，`Lock` 和 `Unlock` 方法无意中成为了导出 API 的一部分。在好的示例中，互斥锁是对调用方隐藏的实现细节。

---

## 原子操作：完整示例

标准 `sync/atomic` 包操作原始类型（`int32`、`int64` 等），容易忘记一致地使用原子操作。

```go
// 不好：容易忘记原子操作
type foo struct {
    running int32 // 原子操作
}

func (f *foo) start() {
    if atomic.SwapInt32(&f.running, 1) == 1 {
        return // 已在运行
    }
    // 启动 Foo
}

func (f *foo) isRunning() bool {
    return f.running == 1 // 竞争！忘记使用 atomic.LoadInt32
}
```

```go
// 好：类型安全的原子操作
type foo struct {
    running atomic.Bool
}

func (f *foo) start() {
    if f.running.Swap(true) {
        return // 已在运行
    }
    // 启动 Foo
}

func (f *foo) isRunning() bool {
    return f.running.Load() // 不可能意外地非原子读取
}
```

`atomic.Bool`、`atomic.Int64` 等类型（Go 1.19 起在标准库 `sync/atomic` 中可用，或通过 [go.uber.org/atomic](https://pkg.go.dev/go.uber.org/atomic)）通过隐藏底层类型来增加类型安全性。

---

## Channel 方向示例

指定方向可以防止意外误用：

```go
// 好：指定方向 - 清晰的所有权
func sum(values <-chan int) int {
    total := 0
    for v := range values {
        total += v
    }
    return total
}
```

```go
// 不好：未指定方向 - 允许意外误用
func sum(values chan int) (out int) {
    for v := range values {
        out += v
    }
    close(values) // 漏洞！这能通过编译但不应该发生。
}
```
