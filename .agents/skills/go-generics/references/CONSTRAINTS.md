# Go 泛型中的类型约束

> **来源**：Google Go 风格指南、Go 语言规范

约束定义了类型参数支持的操作。选择满足函数需求的最窄约束——不要更多。

---

## 内置约束

> **规范**：在自行编写约束之前，优先使用标准约束。

| 约束 | 含义 |
|------|------|
| `any` | `interface{}` 的别名；对类型没有要求 |
| `comparable` | 支持 `==` 和 `!=`；映射键所必需 |
| `cmp.Ordered` | 支持 `<`、`<=`、`>=`、`>`（Go 1.21+，替代 `constraints.Ordered`） |

在新代码中优先使用 `cmp.Ordered`（来自 `cmp` 包），而不是已弃用的 `golang.org/x/exp/constraints.Ordered`。

---

## `~` 运算符（底层类型）

> **建议**：当你想接受基于原始类型构建的命名类型时使用 `~`。

`~T` 语法匹配任何**底层类型**为 `T` 的类型。没有 `~` 时，只有精确的类型匹配。

```go
type Celsius float64

type ExactFloat interface{ float64 }   // 拒绝 Celsius
type AnyFloat64 interface{ ~float64 }  // 接受 Celsius
```

当调用者可能基于基础类型定义命名类型时使用 `~`。仅在需要限制为精确的内置类型时才省略 `~`。

---

## 组合与编写约束

> **建议**：仅在没有标准约束适用时才定义自定义约束。

使用 `|` 组合类型并嵌入约束来组合它们：

```go
type Numeric interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~float32 | ~float64
}

type Addable interface {
    Numeric | ~string  // 数字和字符串拼接
}
```

约束可以同时要求方法和类型元素：

```go
type Stringer interface {
    comparable
    String() string
}
```

满足 `Stringer` 的类型必须是可比较的 **并且** 具有 `String()` 方法。

---

## 避免过度约束

> **规范**：使用支持所执行操作的最小约束。

**不好**
```go
// 只使用了 == 但限制为 int 和 string
func Contains[T interface{ ~int | ~string }](s []T, v T) bool { ... }
```

**好**
```go
// comparable 是 == 的最小约束
func Contains[T comparable](s []T, v T) bool { ... }
```

过度约束限制了复用，并迫使调用者绕过实现中根本不需要的限制。

## 类型推断

> **建议**：当类型明确时让编译器推断类型参数。

编译器从函数参数推断类型参数：

```go
result := slices.Contains[string](names, "alice")  // 显式——不必要
result := slices.Contains(names, "alice")           // 推断——推荐
```

仅在以下情况下才显式提供类型参数：没有可用于推断的函数参数、推断的类型不正确（例如无类型常量提升为错误的类型），或者将类型显式展示出来有助于可读性。

---

## 常见陷阱

### 接口已足够时不要使用泛型

> **规范**：来自 Google 风格指南——当类型共享一个有用的统一接口时，优先使用接口。

**不好**
```go
// T 仅用于满足 io.Reader——直接使用接口即可
func Process[T io.Reader](r T) error { ... }
```

**好**
```go
func Process(r io.Reader) error { ... }
```

如果约束是单个已有接口，直接接受该接口。

### 不要泛型地包装标准库类型

> **建议**：单次使用的泛型只是多余的间接层。

**不好**
```go
type Set[T comparable] struct{ m map[T]struct{} }  // 永远只是 Set[string]
```

**好**
```go
seen := map[string]struct{}{}  // 对于单次实例化直接使用 map
```

泛型在消除**多个调用点**之间的重复时才能证明其复杂度的合理性。如果只使用一种类型，从具体类型开始。

### 方法集与类型约束

你只能调用约束允许的操作：

**不好**
```go
func Stringify[T any](v T) string {
    return v.String()  // 编译错误：any 没有 String()
}
```

**好**
```go
func Stringify[T fmt.Stringer](v T) string {
    return v.String()
}
```

---

## 快速参考

| 主题 | 指导 |
|------|------|
| 默认约束 | `any`——不需要对 T 进行任何操作时使用 |
| 相等性检查 | `comparable`——`==`、`!=` 和映射键所必需 |
| 排序 | `cmp.Ordered`（Go 1.21+）用于 `<`、`>` 比较 |
| 命名类型 | 使用 `~T` 接受底层类型为 T 的类型 |
| 联合类型 | 使用 `\|` 组合——例如 `~int \| ~float64` |
| 自定义约束 | 定义为包含类型元素和/或方法的接口 |
| 类型推断 | 当编译器可以推断时省略类型参数 |
| 最小约束 | 使用函数实际需要的最窄约束 |
