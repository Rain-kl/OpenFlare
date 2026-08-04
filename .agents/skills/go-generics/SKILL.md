---
name: go-generics
description: Use when deciding whether to use Go generics, writing generic functions or types, choosing constraints, or picking between type aliases and type definitions. Also use when a user is writing a utility function that could work with multiple types, even if they don't mention generics explicitly. Does not cover interface design without generics (see go-interfaces).
license: Apache-2.0
compatibility: Requires Go 1.18+ (generics were introduced in Go 1.18)
metadata:
  sources: "Google Style Guide"
---

# Go 泛型与类型参数

---

## 何时使用泛型

从具体类型开始。只在出现第二种类型时才进行泛化。

### 优先使用泛型的场景

- 多种类型共享相同的逻辑（排序、过滤、map/reduce）
- 否则需要依赖 `any` 和大量的类型切换
- 正在构建可复用的数据结构（并发安全的集合、有序映射）

### 避免使用泛型的场景

- 实践中只有一种类型被实例化
- 接口已经能清晰地表达共享行为
- 泛型代码比特定类型的替代方案更难阅读

> "写代码，不要设计类型。"—— Robert Griesemer 和 Ian Lance Taylor

### 决策流程

```
多种类型是否共享相同的逻辑？
├─ 否 → 使用具体类型
├─ 是 → 它们是否共享一个有用的接口？
│        ├─ 是 → 使用接口
│        └─ 否 → 使用泛型
```

**不好：**

```go
// 过早使用泛型：只会被 int 调用
func Sum[T constraints.Integer | constraints.Float](vals []T) T {
    var total T
    for _, v := range vals {
        total += v
    }
    return total
}
```

**好：**

```go
func SumInts(vals []int) int {
    var total int
    for _, v := range vals {
        total += v
    }
    return total
}
```

---

## 类型参数命名

| 名称 | 典型用途 |
|------|----------|
| `T` | 通用类型参数 |
| `K` | 映射键类型 |
| `V` | 映射值类型 |
| `E` | 元素/项目类型 |

对于复杂约束，可以使用简短的描述性名称：

```go
func Marshal[Opts encoding.MarshalOptions](v any, opts Opts) ([]byte, error)
```

---

## 类型别名 vs 类型定义

类型别名（`type Old = new.Name`）很少使用——仅用于包迁移或渐进式 API 重构。

---

## 约束组合

使用 `~`（底层类型）和 `|`（联合）组合约束：

```go
type Numeric interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~float32 | ~float64
}

func Sum[T Numeric](vals []T) T {
    var total T
    for _, v := range vals {
        total += v
    }
    return total
}
```

使用 `constraints` 包或 `cmp` 包（Go 1.21+）中的标准约束如 `cmp.Ordered`，而不是自己编写。

> 在编写自定义类型约束、使用 ~ 和 | 组合约束或调试类型推断问题时，阅读 [references/CONSTRAINTS.md](references/CONSTRAINTS.md)。

---

## 常见陷阱

### 不要包装标准库类型

```go
// 不好：泛型包装器增加了复杂度但没有价值
type Set[T comparable] struct {
    m map[T]struct{}
}

// 更好：当用法简单时直接使用 map[T]struct{}
seen := map[string]struct{}{}
```

泛型在消除**多个调用点**之间的重复时才能证明其复杂度的合理性。单次使用的泛型只是多余的间接层。

### 不要为接口满足而使用泛型

```go
// 不好：T 仅用于满足接口——直接使用接口即可
func Process[T io.Reader](r T) error { ... }

// 好：直接接受接口
func Process(r io.Reader) error { ... }
```

### 避免过度约束

```go
// 不好：约束比需要的更严格
func Contains[T interface{ ~int | ~string }](slice []T, target T) bool { ... }

// 好：comparable 就足够了
func Contains[T comparable](slice []T, target T) bool { ... }
```

---

## 快速参考

| 主题 | 指导 |
|------|------|
| 何时使用泛型 | 仅在多种类型共享相同逻辑且接口不够用时 |
| 起点 | 先写具体代码；之后再泛化 |
| 命名 | 单个大写字母（`T`、`K`、`V`、`E`） |
| 类型别名 | 相同类型，替代名称；仅用于迁移 |
| 约束组合 | 使用 `~` 表示底层类型，`|` 表示联合；优先使用 `cmp.Ordered` 而非自定义 |
| 常见陷阱 | 不要对单次使用的代码或接口已足够时使用泛型 |

---

## 相关技能

- **接口 vs 泛型**：在决定接口是否已经能表达共享行为而无需泛型时，参见 [go-interfaces](../go-interfaces/SKILL.md)
- **类型声明**：在定义新类型、类型别名或在类型定义和别名之间选择时，参见 [go-declarations](../go-declarations/SKILL.md)
- **文档化泛型 API**：在为泛型函数编写文档注释和可运行示例时，参见 [go-documentation](../go-documentation/SKILL.md)
- **命名类型参数**：在为类型参数或约束接口选择名称时，参见 [go-naming](../go-naming/SKILL.md)
