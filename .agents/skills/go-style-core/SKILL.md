---
name: go-style-core
description: Use when working with Go formatting, line length, nesting, naked returns, semicolons, or core style principles. Also use when a style question isn't covered by a more specific skill, even if the user doesn't reference a specific style rule. Does not cover domain-specific patterns like error handling, naming, or testing (see specialized skills). Acts as fallback when no more specific style skill applies.
license: Apache-2.0
metadata:
  sources: "Effective Go, Google Style Guide, Uber Style Guide, Go Wiki CodeReviewComments"
---

# Go 风格核心原则

## 风格原则（优先级顺序）

编写可读 Go 代码时，按以下重要性顺序应用这些原则：

### 优先级顺序

1. **清晰性** — 读者能否在没有额外上下文的情况下理解代码？
2. **简洁性** — 这是否是实现目标的最简单方式？
3. **精炼性** — 每一行是否都有其存在的价值？
4. **可维护性** — 后续修改是否容易？
5. **一致性** — 是否与周围代码和项目约定保持一致？

> 在解决清晰性、简洁性和精炼性之间的冲突时，或需要具体示例了解每个原则在实际 Go 代码中的应用时，请阅读 [references/PRINCIPLES.md](references/PRINCIPLES.md)。

---

## 格式化

运行 `gofmt` — 没有例外。**没有严格的行长度限制**，但 Uber 建议软限制为 99 个字符。按语义换行，而非按长度 — 选择重构而非仅仅换行。

> 在配置 gofmt、决定换行策略、应用 MixedCaps 规则或解决局部一致性问题时，请阅读 [references/FORMATTING.md](references/FORMATTING.md)。

---

## 减少嵌套

优先处理错误情况和特殊条件。提前返回或继续循环，使"正常路径"保持无缩进。

```go
// 不好：深度嵌套
for _, v := range data {
    if v.F1 == 1 {
        v = process(v)
        if err := v.Call(); err == nil {
            v.Send()
        } else {
            return err
        }
    } else {
        log.Printf("Invalid v: %v", v)
    }
}

// 好：扁平结构，提前返回
for _, v := range data {
    if v.F1 != 1 {
        log.Printf("Invalid v: %v", v)
        continue
    }

    v = process(v)
    if err := v.Call(); err != nil {
        return err
    }
    v.Send()
}
```

### 不必要的 Else

如果变量在 if 的两个分支中都被赋值，使用默认值 + 覆盖模式。

```go
// 不好：在两个分支中都赋值
var a int
if b {
    a = 100
} else {
    a = 10
}

// 好：默认值 + 覆盖
a := 10
if b {
    a = 100
}
```

---

## 裸返回

没有参数的 `return` 语句会返回命名返回值。这被称为"裸"返回。

```go
func split(sum int) (x, y int) {
    x = sum * 4 / 9
    y = sum - x
    return // 返回 x, y
}
```

### 裸返回的使用指南

- **在小型函数中可以使用**：裸返回在只有几行的函数中是没问题的
- **在中大型函数中要明确**：一旦函数增长到中等大小，为了清晰起见应明确指定返回值
- **不要仅为了裸返回而命名返回值**：文档的清晰性始终比节省一两行更重要

```go
// 好：小型函数，裸返回很清晰
func minMax(a, b int) (min, max int) {
    if a < b {
        min, max = a, b
    } else {
        min, max = b, a
    }
    return
}

// 好：较大的函数，显式返回
func processData(data []byte) (result []byte, err error) {
    result = make([]byte, 0, len(data))

    for _, b := range data {
        if b == 0 {
            return nil, errors.New("null byte in data")
        }
        result = append(result, transform(b))
    }

    return result, nil // 显式返回：在较长的函数中更清晰
}
```

关于命名返回参数的指导，请参阅 **go-documentation**。

---

## 分号

Go 的词法分析器会在任何最后一个 token 是标识符、字面量或以下关键字之一的行后自动插入分号：`break continue fallthrough return ++ -- ) }`。

这意味着 **左花括号必须与控制结构在同一行**：

```go
// 好：花括号在同一行
if i < f() {
    g()
}

// 不好：花括号在下一行 — 词法分析器会在 f() 后插入分号
if i < f()  // 错误！
{           // 错误！
    g()
}
```

在惯用 Go 中，显式分号仅出现在 `for` 循环子句中和用于分隔单行上的多个语句。

---

## 快速参考

| 原则 | 核心问题 |
|------|----------|
| 清晰性 | 读者能否理解代码的意图和原因？ |
| 简洁性 | 这是否是最简单的方法？ |
| 精炼性 | 信噪比是否高？ |
| 可维护性 | 后续能否安全地修改？ |
| 一致性 | 是否与周围代码保持一致？ |

## 相关 Skill

- **命名约定**：在应用 MixedCaps、选择标识符名称或解决命名争议时，请参阅 [go-naming](../go-naming/SKILL.md)
- **错误流程**：在构建错误优先的守卫子句或通过提前返回减少嵌套时，请参阅 [go-error-handling](../go-error-handling/SKILL.md)
- **文档**：在编写文档注释、命名返回参数或包级别文档时，请参阅 [go-documentation](../go-documentation/SKILL.md)
- **Linting 执行**：在使用 golangci-lint 自动化风格检查或配置 CI 时，请参阅 [go-linting](../go-linting/SKILL.md)
- **代码审查**：在系统性代码审查中应用风格原则时，请参阅 [go-code-review](../go-code-review/SKILL.md)
- **日志风格**：在审查日志实践、在 log 和 slog 之间选择或组织日志输出时，请参阅 [go-logging](../go-logging/SKILL.md)
