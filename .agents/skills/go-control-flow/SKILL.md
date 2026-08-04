---
name: go-control-flow
description: Use when writing conditionals, loops, or switch statements in Go — including if with initialization, early returns, for loop forms, range, switch, type switches, and blank identifier patterns. Also use when writing a simple if/else or for loop, even if the user doesn't mention guard clauses or variable scoping. Does not cover error flow patterns (see go-error-handling).
license: Apache-2.0
metadata:
  sources: "Effective Go, Google Style Guide"
---

# Go 控制流

> 在使用 switch 语句、类型 switch 或带标签的 break 时，阅读 [references/SWITCH-PATTERNS.md](references/SWITCH-PATTERNS.md)

> 在使用 `_`、空白标识符导入或编译时接口检查时，阅读 [references/BLANK-IDENTIFIER.md](references/BLANK-IDENTIFIER.md)

---

## 带初始化的 If

`if` 和 `switch` 接受可选的初始化语句。使用它将变量限定在条件块作用域内：

```go
if err := file.Chmod(0664); err != nil {
    log.Print(err)
    return err
}
```

如果需要在 `if` 之后超出几行的范围使用该变量，请单独声明并使用标准 `if`：

```go
x, err := f()
if err != nil {
    return err
}
// 大量使用 x 的代码
```

## 缩进错误流（守卫子句）

当 `if` 主体以 `break`、`continue`、`goto` 或 `return` 结尾时，省略不必要的 `else`。保持成功路径不缩进：

```go
f, err := os.Open(name)
if err != nil {
    return err
}
d, err := f.Stat()
if err != nil {
    f.Close()
    return err
}
codeUsing(f, d)
```

当 `if` 已经返回时，绝不要将正常流程埋在 `else` 中。

---

## 重新声明和重新赋值

`:=` 短声明允许在同一作用域中重新声明变量：

```go
f, err := os.Open(name)  // 声明 f 和 err
d, err := f.Stat()       // 声明 d，重新赋值 err
```

变量 `v` 即使已经声明过，也可以出现在 `:=` 声明中，前提是：

1. 声明在与现有 `v` **相同的作用域**中
2. 值**可赋值**给 `v`
3. 声明中至少创建了**一个其他新变量**

### 变量遮蔽

**警告**：如果 `v` 在外层作用域中声明，`:=` 会创建一个**新的**遮蔽变量 — 这是常见的 bug 来源：

```go
// Bug：if 块内的 ctx 遮蔽了外层的 ctx
if *shortenDeadlines {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
}
// 此处的 ctx 仍然是原始的 — 被遮蔽的 ctx 没有逃逸

// 修复：使用 = 而不是 :=
var cancel func()
ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
```

---

## For 循环

Go 的 `for` 是唯一的循环结构，统一了 `while`、`do-while` 和 C 风格的 `for`：

```go
// 仅条件（Go 的 "while"）
for x > 0 {
    x = process(x)
}

// 无限循环
for {
    if done() { break }
}

// C 风格的三组件形式
for i := 0; i < n; i++ { ... }
```

### Range

`range` 遍历切片、映射、字符串和通道：

```go
for i, v := range slice { ... }   // 索引 + 值
for k, v := range myMap { ... }   // 键 + 值（顺序不确定）
for i, r := range "héllo" { ... } // 字节索引 + rune（不是字节）
for v := range ch { ... }         // 接收直到通道关闭
```

**关键规则：**
- 对字符串 range 产生 **rune**，不是字节 — `i` 是字节偏移量
- 对映射 range 的顺序**不确定** — 不要依赖它
- 使用 `_` 丢弃索引或值：`for _, v := range slice`

### 并行赋值

Go 没有逗号运算符。使用并行赋值处理多个循环变量：

```go
for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
    a[i], a[j] = a[j], a[i]
}
```

`++` 和 `--` 是语句，不是表达式 — 它们不能出现在并行赋值中。

---

## Switch：带标签的 Break

`for` 循环内 `switch` 中的 `break` 只会中断 switch。使用带标签的 `break` 退出外层循环：

```go
Loop:
    for _, v := range items {
        switch v.Type {
        case "done":
            break Loop  // 中断 for 循环
        }
    }
```

关于类型 switch，参见 **go-interfaces**：类型 Switch。

---

## 空白标识符

**绝不要随意丢弃错误** — 空指针解引用 panic 可能随之而来。

在编译时验证接口实现：`var _ io.Writer = (*MyType)(nil)`。
参见 **go-interfaces** 中的接口满足检查模式。

---

## 快速参考

| 模式 | Go 惯用法 |
|------|-----------|
| If 初始化 | `if err := f(); err != nil { }` |
| 提前返回 | 当 if 主体返回时省略 `else` |
| 重新声明 | `:=` 在相同作用域 + 新变量时重新赋值 |
| 遮蔽陷阱 | `:=` 在内层作用域创建新变量 |
| 并行赋值 | `i, j = i+1, j-1` |
| 无表达式 switch | `switch { case cond: }` |
| 逗号 case | `case 'a', 'b', 'c':` |
| 无 fallthrough | 默认行为（需要时显式使用 `fallthrough`） |
| 从 switch 中跳出循环 | `break Label` |
| 丢弃值 | `_, err := f()` |
| 副作用导入 | `import _ "pkg"` |
| 接口检查 | `var _ Interface = (*Type)(nil)` |

---

## 相关技能

- **错误流程**：在构建守卫子句、提前返回或错误优先模式时，参见 [go-error-handling](../go-error-handling/SKILL.md)
- **类型 switch**：在使用类型 switch、comma-ok 惯用法或接口满足检查时，参见 [go-interfaces](../go-interfaces/SKILL.md)
- **减少嵌套**：在减少嵌套深度或解决格式问题时，参见 [go-style-core](../go-style-core/SKILL.md)
- **变量作用域**：在使用 if 初始化、`:=` 重新声明或减少变量作用域时，参见 [go-declarations](../go-declarations/SKILL.md)
