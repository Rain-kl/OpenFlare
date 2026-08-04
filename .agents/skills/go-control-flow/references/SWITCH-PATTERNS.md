# Switch 模式

Go `switch` 语句的详细模式，包括无表达式 switch、逗号 case、break 行为和带标签的 break。

---

## 无自动 Fallthrough

Go `switch` 的 case 默认**不会** fall through（与 C/Java 不同）。每个 case 主体隐式地 break。仅在明确需要时使用 `fallthrough` — 这在惯用 Go 中很少见。

```go
switch n {
case 1:
    fmt.Println("one")
    // 无 fallthrough — 下一个 case 不会执行
case 2:
    fmt.Println("two")
}
```

---

## 无表达式 Switch

没有表达式的 `switch` 对 `true` 进行 switch。在将单个变量与多个条件进行比较时，用它来替代 if-else-if 链：

```go
func unhex(c byte) byte {
    switch {
    case '0' <= c && c <= '9':
        return c - '0'
    case 'a' <= c && c <= 'f':
        return c - 'a' + 10
    case 'A' <= c && c <= 'F':
        return c - 'A' + 10
    }
    return 0
}
```

---

## 逗号分隔的 Case

多个值可以使用逗号共享一个 case 主体 — 不需要 `fallthrough`：

```go
func shouldEscape(c byte) bool {
    switch c {
    case ' ', '?', '&', '=', '#', '+', '%':
        return true
    }
    return false
}
```

---

## 带标签的 Break

`switch` 中的 `break` 仅终止 switch，**不会**终止外层的 `for` 循环。使用标签来跳出循环：

```go
Loop:
    for n := 0; n < len(src); n += size {
        switch {
        case src[n] < sizeOne:
            break        // 仅中断 switch
        case src[n] < sizeTwo:
            if n+1 >= len(src) {
                break Loop   // 跳出 for 循环
            }
        }
    }
```

另一个常见模式 — 从 switch 内部中断 range 循环：

```go
Loop:
    for _, v := range items {
        switch v.Type {
        case "done":
            break Loop  // 中断 for 循环
        case "skip":
            break  // 仅中断 switch
        }
    }
```

**经验法则**：当 `for` 循环内有 `switch` 且需要从 case 中退出循环时，始终使用带标签的 break。

---

## 类型 Switch

关于类型 switch（`switch v := x.(type)`），参见 [go-interfaces](../../go-interfaces/SKILL.md)：类型 Switch。

---

## 快速参考

| 模式 | 语法 |
|------|------|
| 无表达式 switch | `switch { case cond: }` |
| 逗号 case | `case 'a', 'b', 'c':` |
| 无 fallthrough | 默认行为；需要时使用 `fallthrough` 关键字 |
| 仅中断 switch | case 内使用 `break` |
| 中断外层循环 | 使用带标签的 `for` 和 `break Label` |
