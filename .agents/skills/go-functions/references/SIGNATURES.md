# 函数签名

格式化 Go 函数签名、避免裸参数以及保持调用点可读性的详细规则。

---

## 单行 vs 多行

当签名能轻松放在一行时保持单行。当必须换行时，将**所有参数放在各自的行上**并加尾随逗号：

**不好**——部分换行使对齐变得脆弱：

```go
func (r *SomeType) SomeLongFunctionName(foo1, foo2, foo3 string,
    foo4, foo5, foo6 int) {
    foo7 := bar(foo1)
}
```

**好**——完全换行，尾随逗号：

```go
func (r *SomeType) SomeLongFunctionName(
    foo1, foo2, foo3 string,
    foo4, foo5, foo6 int,
) {
    foo7 := bar(foo1)
}
```

### 返回值

当返回值也需要换行时，遵循相同的模式：

```go
func (r *SomeType) LongName(
    foo1, foo2, foo3 string,
    foo4, foo5, foo6 int,
) (
    *Result,
    error,
) {
    // ...
}
```

对于更简单的情况，命名返回值可以与参数右括号在同一行：

```go
func (r *SomeType) LongName(
    foo1, foo2, foo3 string,
) (result *Result, err error) {
    // ...
}
```

---

## 缩短调用点

提取局部变量，而不是将函数调用拆分到多行：

```go
// 不好：过长的内联调用
result := foo.Call(
    somePackage.ComplexFunction(arg1, arg2),
    anotherPackage.Transform(data),
    defaultOptions,
)

// 好：提取局部变量以提高清晰度
transformed := anotherPackage.Transform(data)
computed := somePackage.ComplexFunction(arg1, arg2)
result := foo.Call(computed, transformed, defaultOptions)
```

这提高了可读性，并使中间值可用于调试。

---

## 避免裸参数

函数调用中的裸参数会降低可读性。为含义不明确的参数添加 C 风格注释：

```go
// 不好：这些布尔值是什么意思？
printInfo("foo", true, true)

// 好：内联注释说明了意图
printInfo("foo", true /* isLocal */, true /* done */)
```

更好的做法是用自定义类型替换裸 `bool` 参数：

```go
type Region int

const (
    UnknownRegion Region = iota
    Local
)

type Status int

const (
    Pending Status = iota
    Done
)

func printInfo(name string, region Region, status Status)
```

### 何时使用每种方法

| 方法 | 时机 |
|------|------|
| C 风格注释 | 快速修复；调用点少；无法修改的第三方 API |
| 自定义类型 | 多个调用点；公开 API；多个 bool/int 参数 |
| 函数选项 | 3 个以上可选参数；参见 [go-functional-options](../../go-functional-options/SKILL.md) |

---

## 分组相关参数

当函数接受多个相同类型的参数时，将它们分组：

```go
// 可接受：将同类型参数分组
func Copy(dst, src string) error

// 可接受：尽管类型相同，但含义不同时分开声明
func Move(source string, destination string) error
```

当参数名称能清楚表明角色时使用分组；当不能清楚表明时使用分开声明。

---

## 方法接收者的位置

接收者放在函数名之前，格式类似于参数：

```go
// 短接收者——放在同一行
func (s *Server) Start(ctx context.Context) error { ... }

// 长接收者类型——如果整行过长则考虑换行
func (h *ComplicatedHandler) ServeHTTP(
    w http.ResponseWriter,
    r *http.Request,
) { ... }
```

参见 [go-naming](../../go-naming/SKILL.md) 了解接收者命名约定（简短的一到两个字母缩写）。

---

## 快速参考

| 主题 | 规则 |
|------|------|
| 单行 | 能放下时保持一行 |
| 多行 | 所有参数各占一行，尾随逗号 |
| 返回值换行 | 与参数相同的模式 |
| 调用点 | 提取局部变量而不是拆分调用 |
| 裸 bool | 添加 `/* name */` 注释或使用自定义类型 |
| 分组参数 | 当名称能清楚表明角色时将同类型分组 |
| 接收者 | 在函数名之前；简短缩写 |
