---
name: go-defensive
description: Use when hardening Go code at API boundaries — copying slices/maps, verifying interface compliance, using defer for cleanup, time.Time/time.Duration, or avoiding mutable globals. Also use when reviewing for robustness concerns like missing cleanup or unsafe crypto usage, even if the user doesn't mention "defensive programming." Does not cover error handling strategy (see go-error-handling).
license: Apache-2.0
compatibility: Uses crypto/rand.Text (Go 1.24+) in examples
metadata:
  sources: "Effective Go, Uber Style Guide, Go Wiki CodeReviewComments"
---

# Go 防御性编程模式

## 防御性检查清单优先级

在加固 API 边界代码时，按以下顺序检查：

```
正在审查 API 边界？
├─ 1. 错误处理     → 返回错误；不要 panic（参见 go-error-handling）
├─ 2. 输入验证     → 复制从调用者接收的切片/map
├─ 3. 输出安全     → 在返回给调用者之前复制切片/map
├─ 4. 资源清理     → 使用 defer 进行 Close/Unlock/Cancel
├─ 5. 接口检查     → var _ Interface = (*Type)(nil) 编译时验证
├─ 6. 时间正确性   → 使用 time.Time 和 time.Duration，不要用 int/float
├─ 7. 枚举安全     → iota 从 1 开始，使零值无效
└─ 8. 加密安全     → 用 crypto/rand 生成密钥，绝不用 math/rand
```

---

## 快速参考

| 模式 | 规则 | 详情 |
|------|------|------|
| 边界复制 | 在接收和返回时复制切片/map | [BOUNDARY-COPYING.md](references/BOUNDARY-COPYING.md) |
| Defer 清理 | 在 `os.Open` 之后立即 `defer f.Close()` | 见下文 |
| 接口检查 | `var _ I = (*T)(nil)` | 参见 go-interfaces |
| 时间类型 | `time.Time` / `time.Duration`，绝不用原始 int | [TIME-ENUMS-TAGS.md](references/TIME-ENUMS-TAGS.md) |
| 枚举起始值 | `iota + 1` 使零值 = 无效 | 见下文 |
| 加密随机数 | 用 `crypto/rand` 生成密钥，绝不用 `math/rand` | 见下文 |
| Must 函数 | 仅在初始化时使用；失败时 panic | [MUST-FUNCTIONS.md](references/MUST-FUNCTIONS.md) |
| Panic/recover | 绝不跨包暴露 panic | [PANIC-RECOVER.md](references/PANIC-RECOVER.md) |
| 可变全局变量 | 用依赖注入替代 | 见下文 |

---

## 验证接口合规性

使用编译时检查来验证接口实现。完整模式请参见 **go-interfaces**：接口满足检查。

```go
var _ http.Handler = (*Handler)(nil)
```

## 在边界处复制切片和 Map

切片和 map 包含指向底层数据的指针。在 API 边界处复制，以防止意外修改。

```go
// 接收：复制传入的切片
d.trips = make([]Trip, len(trips))
copy(d.trips, trips)

// 返回：在返回之前复制 map
result := make(map[string]int, len(s.counters))
for k, v := range s.counters { result[k] = v }
```

> 在 API 边界处复制切片或 map，或决定何时需要防御性复制、何时可以跳过时，请阅读 [references/BOUNDARY-COPYING.md](references/BOUNDARY-COPYING.md)。

## 使用 Defer 清理资源

使用 `defer` 清理资源（文件、锁）。避免在多个返回路径中遗漏清理。

```go
p.Lock()
defer p.Unlock()

if p.count < 10 {
  return p.count
}
p.count++
return p.count
```

Defer 的开销可以忽略不计。在 `os.Open` 之后立即放置 `defer f.Close()` 以提高清晰度。延迟函数的参数在 `defer` 执行时求值，而非在函数运行时。多个 defer 按 LIFO 顺序执行。

## 结构体字段标签

> **建议**：始终为需要序列化或反序列化的结构体添加显式字段标签。

```go
type User struct {
    Name  string `json:"name"  yaml:"name"`
    Email string `json:"email" yaml:"email"`
}
```

字段标签是**序列化契约**——重命名结构体字段而不更新标签会悄然破坏线格式兼容性。对于任何跨越序列化边界的类型，应将标签视为公共 API 的一部分。

## 枚举从 1 开始

枚举从非零值开始，以区分未初始化的值和有效值。

```go
const (
  Add Operation = iota + 1  // Add=1，零值 = 未初始化
  Subtract
  Multiply
)
```

**例外**：当零值是合理的默认值时（例如 `LogToStdout = iota`）。

## 时间、结构体标签和嵌入

> 在使用 `time.Time`/`time.Duration` 代替原始 int、为序列化结构体添加字段标签，或决定是否在公共结构体中嵌入类型时，请阅读 [references/TIME-ENUMS-TAGS.md](references/TIME-ENUMS-TAGS.md)。

## 避免可变全局变量

通过注入依赖代替修改包级变量。这使代码可以在不需要全局 save/restore 的情况下进行测试。

```go
type signer struct {
  now func() time.Time  // 注入的；测试中用固定时间替换
}

func newSigner() *signer {
  return &signer{now: time.Now}
}
```

> 在决定全局变量是否合适、设计 New() + Default() 包状态模式，或用依赖注入替代可变全局变量时，请阅读 [references/GLOBAL-STATE.md](references/GLOBAL-STATE.md)。

## 加密随机数

不要使用 `math/rand` 或 `math/rand/v2` 生成密钥——这是一个**安全问题**。时间种子的生成器输出是可预测的。

```go
import "crypto/rand"

func Key() string { return rand.Text() }
```

对于文本输出，直接使用 `crypto/rand.Text`，或用 `encoding/hex` 或 `encoding/base64` 编码随机字节。

---

## Panic 与 Recover

仅在真正不可恢复的情况下使用 `panic`。库函数应避免 panic。

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

**关键规则：**
- 绝不跨包边界暴露 panic——始终转换为 error
- 如果库确实无法在 `init()` 中完成初始化，可以接受 panic
- 使用 recover 隔离服务器 goroutine 处理器中的 panic

> 在编写 HTTP 服务器中的 panic 恢复、在解析器中使用 panic 作为内部控制流机制，或在 log.Fatal 和 panic 之间做选择时，请阅读 [references/PANIC-RECOVER.md](references/PANIC-RECOVER.md)。

## Must 函数

`Must` 函数在出错时 panic——**仅**在程序初始化阶段使用，因为失败意味着程序无法运行。

```go
var validID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
var tmpl = template.Must(template.ParseFiles("index.html"))
```

> 在编写自定义 Must 函数、决定 Must 是否适用于特定调用点，或将可能失败的初始化包装在 panic 辅助函数中时，请阅读 [references/MUST-FUNCTIONS.md](references/MUST-FUNCTIONS.md)。

---

## 相关技能

- **错误处理**：在选择返回错误还是 panic，或在边界处包装错误时，参见 [go-error-handling](../go-error-handling/SKILL.md)
- **并发安全**：在使用互斥锁、原子操作或通道保护共享状态时，参见 [go-concurrency](../go-concurrency/SKILL.md)
- **接口检查**：在添加编译时接口满足检查（`var _ I = (*T)(nil)`）时，参见 [go-interfaces](../go-interfaces/SKILL.md)
- **数据结构复制**：在处理切片/map 内部结构或指针别名时，参见 [go-data-structures](../go-data-structures/SKILL.md)
