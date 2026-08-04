---
name: go-interfaces
description: Use when defining or implementing Go interfaces, designing abstractions, creating mockable boundaries for testing, or composing types through embedding. Also use when deciding whether to accept an interface or return a concrete type, or using type assertions or type switches, even if the user doesn't explicitly mention interfaces. Does not cover generics-based polymorphism (see go-generics).
license: Apache-2.0
metadata:
  sources: "Effective Go, Google Style Guide, Uber Style Guide"
allowed-tools: Bash(bash:*)
---

# Go 接口与组合

## 可用脚本

- **`scripts/check-interface-compliance.sh`**——查找缺少编译时合规性检查（`var _ I = (*T)(nil)`）的导出接口。运行 `bash scripts/check-interface-compliance.sh --help` 查看选项。

---

## 接受接口，返回具体类型

接口属于**消费**值的包，而不是**实现**值的包。从构造函数返回具体类型（通常是指针或结构体），这样可以在不重构的情况下添加新方法。

```go
// 好：消费者定义自己需要的接口
package consumer

type Thinger interface { Thing() bool }

func Foo(t Thinger) string { ... }
```

```go
// 好：生产者返回具体类型
package producer

type Thinger struct{ ... }
func (t Thinger) Thing() bool { ... }
func NewThinger() Thinger { return Thinger{ ... } }
```

```go
// 不好：生产者定义并返回自己的接口
package producer

type Thinger interface { Thing() bool }
type defaultThinger struct{ ... }
func NewThinger() Thinger { return defaultThinger{ ... } }
```

**不要在接口被使用之前定义它。** 如果没有现实的使用示例，很难判断接口是否真的有必要。

---

## 通用性：隐藏实现，暴露接口

如果一个类型仅用于实现某个接口，且没有该接口之外的导出方法，则从构造函数返回接口以隐藏实现：

```go
func NewHash() hash.Hash32 {
    return &myHash{}  // 未导出的类型
}
```

好处：实现可以在不影响调用者的情况下更改，替换算法只需更改构造函数调用。

---

## 类型断言：Comma-Ok 模式

不进行检查的话，失败的断言会导致运行时 panic。始终使用 comma-ok 模式进行安全测试：

```go
str, ok := value.(string)
if ok {
    fmt.Printf("string value is: %q\n", str)
}
```

检查值是否实现了某个接口：

```go
if _, ok := val.(json.Marshaler); ok {
    fmt.Printf("value %v implements json.Marshaler\n", val)
}
```

---

## 类型切换

重用变量名是惯用做法（`t := t.(type)`）——变量在每个 case 分支中拥有正确的类型。当 case 列出多个类型（`case int, int64:`）时，变量拥有接口类型。

---

## 嵌入

避免在公开结构体中嵌入类型——内部类型的完整方法集将成为你公开 API 的一部分。改用未导出的字段。

> 在使用结构体嵌入进行组合、重写嵌入方法、解决名称冲突、应用 HandlerFunc 适配器模式或决定是否在公开 API 类型中使用嵌入时，阅读 [references/EMBEDDING.md](references/EMBEDDING.md)。

---

## 接口满足检查

使用空标识符赋值在编译时验证类型是否实现了接口：

```go
var _ json.Marshaler = (*RawMessage)(nil)
```

如果 `*RawMessage` 没有实现 `json.Marshaler`，这会导致编译错误。

在以下情况下使用此模式：
- 没有能自动验证接口的静态转换
- 类型必须满足接口才能正确运行（例如自定义 JSON 序列化）
- 接口更改应该导致编译失败，而不是静默降级

**不要**为每个接口都添加这些检查——仅在没有其他静态转换能捕获错误时才使用。

> **验证**：在定义接口或实现后，运行 `bash scripts/check-interface-compliance.sh` 验证所有具体类型都有编译时的 `var _ I = (*T)(nil)` 检查。

---

## 接收者类型

如果不确定，使用指针接收者。不要在单个类型上混合接收者类型——如果任何方法需要指针，则所有方法都使用指针。仅在小型不可变类型（`Point`、`time.Time`）或基本类型上使用值接收者。

> 在为新类型决定使用指针接收者还是值接收者时，特别是对于包含 sync 原语或大型结构体的类型，阅读 [references/RECEIVER-TYPE.md](references/RECEIVER-TYPE.md)。

---

## 快速参考

| 概念 | 模式 | 说明 |
|------|------|------|
| 消费者拥有接口 | 在使用处定义接口 | 不在实现包中 |
| 安全类型断言 | `v, ok := x.(Type)` | 返回零值 + false |
| 类型切换 | `switch v := x.(type)` | 变量在每个 case 中拥有正确类型 |
| 接口嵌入 | `type RW interface { Reader; Writer }` | 方法的并集 |
| 结构体嵌入 | `type S struct { *T }` | 提升 T 的方法 |
| 接口检查 | `var _ I = (*T)(nil)` | 编译时验证 |
| 通用性 | 从构造函数返回接口 | 隐藏实现 |

---

## 相关技能

- **接口命名**：在为接口命名（`-er` 后缀约定）或选择接收者名称时，参见 [go-naming](../go-naming/SKILL.md)
- **错误类型**：在实现 `error` 接口、自定义错误类型或 `errors.As` 匹配时，参见 [go-error-handling](../go-error-handling/SKILL.md)
- **泛型 vs 接口**：在决定是否需要泛型或接口是否已足够时，参见 [go-generics](../go-generics/SKILL.md)
- **函数选项**：在使用基于接口的 Option 模式实现灵活构造函数时，参见 [go-functional-options](../go-functional-options/SKILL.md)
- **编译时检查**：在 API 边界添加 `var _ I = (*T)(nil)` 满足检查时，参见 [go-defensive](../go-defensive/SKILL.md)
