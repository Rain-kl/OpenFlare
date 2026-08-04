---
name: go-functions
description: Use when organizing functions within a Go file, formatting function signatures, designing return values, or following Printf-style naming conventions. Also use when a user is adding or refactoring any Go function, even if they don't mention function design or signature formatting. Does not cover functional options constructors (see go-functional-options).
license: Apache-2.0
metadata:
  sources: "Effective Go, Google Style Guide, Uber Style Guide"
---

# Go 函数设计

> **本技能不适用的场景**：对于函数选项构造函数（`WithTimeout`、`WithLogger`），参见 [go-functional-options](../go-functional-options/SKILL.md)。对于错误返回约定，参见 [go-error-handling](../go-error-handling/SKILL.md)。对于函数和方法的命名，参见 [go-naming](../go-naming/SKILL.md)。

---

## 函数分组与排序

按以下规则组织文件中的函数：

1. 函数按**大致调用顺序**排序
2. 函数**按接收者分组**
3. **导出**函数排在最前面，位于 `struct`/`const`/`var` 定义之后
4. `NewXxx`/`newXxx` 构造函数紧跟在类型定义之后
5. 普通工具函数排在文件末尾

```go
type something struct{ ... }

func newSomething() *something { return &something{} }

func (s *something) Cost() int { return calcCost(s.weights) }

func (s *something) Stop() { ... }

func calcCost(n []int) int { ... }
```

---

## 函数签名

> 在格式化多行签名、包装返回值、缩短调用点或用自定义类型替换裸 bool 参数时，阅读 [references/SIGNATURES.md](references/SIGNATURES.md)。

尽量将签名保持在一行内。当必须换行时，将**所有参数放在各自的行上**并加尾随逗号：

```go
func (r *SomeType) SomeLongFunctionName(
    foo1, foo2, foo3 string,
    foo4, foo5, foo6 int,
) {
    foo7 := bar(foo1)
}
```

为含义不明确的参数添加 `/* name */` 注释，或者更好的做法是用自定义类型替换裸 `bool` 参数。

---

## 接口指针

几乎不需要指向接口的指针。将接口作为值传递——底层数据仍然可以是指针。

```go
// 不好：接口指针
func process(r *io.Reader) { ... }

// 好：传递接口值
func process(r io.Reader) { ... }
```

---

## Printf 与 Stringer

> 在使用 %v/%s/%d 之外的 Printf 动词、实现 fmt.Stringer 或 fmt.GoStringer、编写自定义 Format() 方法或调试 String() 方法中的无限递归时，阅读 [references/PRINTF-STRINGER.md](references/PRINTF-STRINGER.md)。

### Printf 风格函数名

接受格式字符串的函数应以 `f` 结尾，以便 `go vet` 支持。在 `Printf` 调用之外使用格式字符串时，将其声明为 `const`。

在格式化日志或错误消息中的字符串时，优先使用 `%q` 而非手动加引号的 `%s`——它能安全地转义特殊字符并加上引号：

```go
return fmt.Errorf("unknown key %q", key) // 输出：unknown key "foo\nbar"
```

设计具有 3 个以上可选参数的构造函数时，参见 **go-functional-options**。

---

## 快速参考

| 主题 | 规则 |
|------|------|
| 文件排序 | 类型 -> 构造函数 -> 导出 -> 未导出 -> 工具函数 |
| 签名换行 | 所有参数各占一行，加尾随逗号 |
| 裸参数 | 添加 `/* name */` 注释或使用自定义类型 |
| 接口指针 | 几乎不需要；按值传递接口 |
| Printf 函数名 | 以 `f` 结尾以支持 `go vet` |

---

## 相关技能

- **错误返回**：在设计错误返回模式或在多返回值函数中包装错误时，参见 [go-error-handling](../go-error-handling/SKILL.md)
- **命名约定**：在为函数、方法命名或选择 getter/setter 模式时，参见 [go-naming](../go-naming/SKILL.md)
- **函数选项**：在设计具有 3 个以上可选参数的构造函数时，参见 [go-functional-options](../go-functional-options/SKILL.md)
- **格式化原则**：在决定行长度、裸返回或签名格式时，参见 [go-style-core](../go-style-core/SKILL.md)
