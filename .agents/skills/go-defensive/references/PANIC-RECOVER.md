# Panic 与 Recover 模式

> **来源**：Effective Go

## Panic 指南

`panic` 创建一个运行时错误来停止程序。仅在真正不可恢复的情况下使用。

### 何时 Panic

真正的库函数应**避免 panic**。如果问题可以被掩盖或绕过，让程序继续运行，而不是让整个程序崩溃。

```go
// 可接受：真正不可能的情况
func CubeRoot(x float64) float64 {
    z := x/3
    for i := 0; i < 1e6; i++ {
        prevz := z
        z -= (z*z*z-x) / (3*z*z)
        if veryClose(z, prevz) {
            return z
        }
    }
    // 百万次迭代仍未收敛；出了问题。
    panic(fmt.Sprintf("CubeRoot(%g) did not converge", x))
}
```

### 初始化中的 Panic

例外：如果库在 `init()` 期间确实无法完成初始化，panic 可能是合理的：

```go
var user = os.Getenv("USER")

func init() {
    if user == "" {
        panic("no value for $USER")
    }
}
```

### 何时 Panic 是可接受的

除了初始化之外，panic 在以下窄泛场景中是可接受的：

1. **API 误用**——类似于核心语言对越界访问的 panic。`reflect` 包使用了这种方法。
2. **带有匹配 `recover` 的内部实现细节**在包边界处。Panic 简化了深层嵌套的控制流，而公共 API 仍然返回 error（下方的 Parse/parseInt 模式）。
3. **`panic("unreachable")`** 在 `log.Fatal` 之后，当编译器无法检测到不可达代码时。

#### Parse/parseInt 模式

在内部使用 panic 来回退复杂的递归，但始终在包边界处转换为 error：

```go
func parseInt(in string) int {
    n, err := strconv.Atoi(in)
    if err != nil {
        panic(&syntaxError{"not a valid integer"})
    }
    return n
}

func Parse(in string) (_ *Node, err error) {
    defer func() {
        if p := recover(); p != nil {
            sErr, ok := p.(*syntaxError)
            if !ok {
                panic(p)  // 不是我们的——重新 panic
            }
            err = fmt.Errorf("syntax error: %v", sErr.msg)
        }
    }()
    // ... 内部调用 parseInt
}
```

**关键**：类型检查 `p.(*syntaxError)` 确保只捕获*我们的* panic。意外的 panic（nil 指针等）正常传播。

---

## Recover 模式

`recover` 重新获得对正在 panic 的 goroutine 的控制。它只在延迟函数中有效。

### 基本恢复模式

```go
func safelyDo(work *Work) {
    defer func() {
        if err := recover(); err != nil {
            log.Println("work failed:", err)
        }
    }()
    do(work)
}
```

### 服务器 Goroutine 保护

在服务器中将 panic 隔离到各个 goroutine：

```go
func server(workChan <-chan *Work) {
    for work := range workChan {
        go safelyDo(work)  // 每个 worker 都受保护
    }
}
```

如果 `do(work)` panic，结果会被记录，goroutine 干净退出而不影响其他 goroutine。

### 包内部的 Panic/Recover

在内部使用 panic 但在 API 边界处转换为 error：

```go
// Error 是一个解析错误类型
type Error string
func (e Error) Error() string { return string(e) }

// 内部：使用 Error 类型 panic
func (regexp *Regexp) error(err string) {
    panic(Error(err))
}

// 外部 API：将 panic 转换为 error 返回
func Compile(str string) (regexp *Regexp, err error) {
    regexp = new(Regexp)
    defer func() {
        if e := recover(); e != nil {
            regexp = nil
            err = e.(Error)  // 如果不是我们的 Error 类型则重新 panic
        }
    }()
    return regexp.doParse(str), nil
}
```

**要点：**

- 延迟函数可以修改命名返回值
- 类型断言 `e.(Error)` 对意外错误类型重新 panic
- 绝不向客户端暴露 panic——始终在 API 边界处转换

---

## 快速参考

| 模式 | 描述 |
|------|------|
| 基本恢复 | `defer func() { if err := recover(); err != nil { ... } }()` |
| 服务器保护 | 将每个 goroutine 处理器包装在 safelyDo 中 |
| 包内部 | 内部 panic，在 API 边界处 recover 并返回 error |
| 类型安全恢复 | 使用类型断言对意外错误重新 panic |

## 何时使用

- **Panic**：仅用于真正不可恢复的情况或初始化失败
- **Recover**：服务器处理器、包内部错误简化
- **绝不**：跨包边界暴露 panic——始终转换为 error
