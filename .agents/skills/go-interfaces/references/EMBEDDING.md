# Go 中的嵌入模式

> **来源**：Effective Go、Uber 风格指南

Go 使用嵌入来实现组合而非继承。嵌入将内部类型的方法提升到外部类型，自动满足接口。

## 接口嵌入

通过嵌入来组合接口：

```go
type ReadWriter interface {
    Reader
    Writer
}
```

`ReadWriter` 既能做 `Reader` 能做的事，*也能*做 `Writer` 能做的事。接口中只能嵌入接口。

## 结构体嵌入

嵌入将内部类型的方法提升到外部类型，无需显式转发。

```go
type ReadWriter struct {
    *Reader  // *bufio.Reader
    *Writer  // *bufio.Writer
}
```

通过嵌入，`bufio.ReadWriter` 自动满足 `io.Reader`、`io.Writer` 和 `io.ReadWriter`。

混合使用嵌入字段和命名字段：

```go
type Job struct {
    Command string
    *log.Logger
}

job.Println("starting now...")
job.Logger.SetPrefix("Job: ")
```

## 方法重写

在外部类型上定义方法以重写提升的方法：

```go
func (job *Job) Printf(format string, args ...any) {
    job.Logger.Printf("%q: %s", job.Command, fmt.Sprintf(format, args...))
}
```

外部方法优先——对 `job.Printf(...)` 的调用会调用外部方法，而嵌入方法仍可通过 `job.Logger.Printf(...)` 访问。

## 嵌入 vs 子类化

当调用嵌入方法时，接收者是**内部**类型，而非外部类型。嵌入类型不知道自己被嵌入——不存在类似于 `this` 或 `super` 的引用指向包含它的类型。

```go
type Base struct{}
func (b *Base) Name() string { return "Base" }

type Derived struct{ Base }

d := Derived{}
d.Name() // 返回 "Base"，而非 "Derived"
```

## 名称冲突解决

1. **外部隐藏内部**——外部类型上的字段或方法会遮蔽嵌入类型在同名位置提升的字段或方法
2. **同级冲突是错误**——如果两个同深度的嵌入类型提升了相同的名称，则为编译错误（除非该名称从未被访问）

```go
type A struct{}
func (A) Hello() string { return "A" }

type B struct{}
func (B) Hello() string { return "B" }

type C struct {
    A
    B
}

// c.Hello()  // 编译错误：选择器不明确
c.A.Hello()   // 可以：显式消歧
```

## 不要在公开结构体中嵌入

嵌入将内部类型的完整方法集暴露为你的公开 API 的一部分。这带来了维护负担：嵌入类型方法的更改会破坏 API 的兼容性保证。

**不好**
```go
type SMap struct {
    sync.Mutex  // Lock 和 Unlock 现在是 SMap API 的一部分
    data map[string]string
}
```

**好**
```go
type SMap struct {
    mu   sync.Mutex  // 未导出的字段——实现细节
    data map[string]string
}

func (m *SMap) Get(k string) string {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.data[k]
}
```

例外：在测试类型和 API 稳定性无关紧要的内部结构体中，嵌入是可以接受的。

## HandlerFunc 适配器模式

方法可以在任何命名类型上定义，不仅仅是结构体。`http.HandlerFunc` 模式将普通函数转换为接口实现：

```go
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, req *Request) {
    f(w, req)
}
```

任何具有正确签名的函数都可以成为 HTTP 处理器：

```go
http.Handle("/args", http.HandlerFunc(ArgServer))
```

这种适配器模式在需要让独立函数满足单方法接口时非常有用。
