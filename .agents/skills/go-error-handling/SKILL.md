---
name: go-error-handling
description: Use when writing Go code that returns, wraps, or handles errors — choosing between sentinel errors, custom types, and fmt.Errorf (%w vs %v), structuring error flow, or deciding whether to log or return. Also use when propagating errors across package boundaries or using errors.Is/As, even if the user doesn't ask about error strategy. Does not cover panic/recover patterns (see go-defensive).
license: Apache-2.0
compatibility: Requires Go 1.13+ for errors.Is/errors.As and fmt.Errorf %w wrapping. Structured logging examples use slog (Go 1.21+).
metadata:
  sources: "Google Style Guide, Uber Style Guide"
allowed-tools: Bash(bash:*)
---

# Go 错误处理

## 可用脚本

- **`scripts/check-errors.sh`** — 检测错误处理反模式：对 `err.Error()` 进行字符串比较、没有上下文的裸 `return err`、以及日志并返回违规。运行 `bash scripts/check-errors.sh --help` 查看选项。

在 Go 中，[错误是值](https://go.dev/blog/errors-are-values) — 它们由代码创建，也由代码消费。

## 选择错误策略

1. 系统边界（RPC、IPC、存储）？→ 使用 `%v` 包装以避免泄露内部细节
2. 调用者需要匹配特定条件？→ 哨兵或类型化错误，使用 `%w` 包装
3. 调用者只需要调试上下文？→ `fmt.Errorf("...: %w", err)`
4. 叶子函数，无需包装？→ 直接返回错误

**默认**：使用 `%w` 包装，并将其放在格式字符串的末尾。

---

## 核心规则

### 永不返回具体错误类型

**永不从导出函数返回具体错误类型** — 具体的 `nil` 指针可能变成非 nil 接口：

```go
// 不好：具体类型可能导致微妙的 bug
func Bad() *os.PathError { /*...*/ }

// 好：始终返回 error 接口
func Good() error { /*...*/ }
```

### 错误字符串

错误字符串**不应**大写，也**不应**以标点符号结尾。例外：导出名称、专有名词或缩写。

```go
// 不好
err := fmt.Errorf("Something bad happened.")

// 好
err := fmt.Errorf("something bad happened")
```

对于显示的消息（日志、测试失败、API 响应），大写是适当的。

### 出错时的返回值

当函数返回错误时，调用者必须将所有非错误返回值视为未指定，除非有明确文档说明。

**提示**：接受 `context.Context` 的函数通常应返回 `error`，以便调用者判断上下文是否被取消。

---

## 处理错误

遇到错误时，做出**深思熟虑的选择** — 不要用 `_` 丢弃：

1. **立即处理** — 解决错误并继续
2. **返回给调用者** — 可选择用上下文包装
3. **在特殊情况下** — `log.Fatal` 或 `panic`

有意忽略时：添加注释说明原因。

```go
n, _ := b.Write(p) // 永不返回非 nil 错误
```

对于相关的并发操作，使用 [`errgroup`](https://pkg.go.dev/golang.org/x/sync/errgroup)：

```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return task1(ctx) })
g.Go(func() error { return task2(ctx) })
if err := g.Wait(); err != nil { return err }
```

### 避免带内错误

不要返回 `-1`、`nil` 或空字符串来表示错误。使用多返回值：

```go
// 不好：带内错误值
func Lookup(key string) int  // 缺失时返回 -1

// 好：显式的 error 或 ok 值
func Lookup(key string) (string, bool)
```

这可以防止调用者写出 `Parse(Lookup(key))` — 它会导致编译时错误，因为 `Lookup(key)` 有 2 个输出。

---

## 错误流程

在正常代码之前处理错误。提前返回使正常路径保持无缩进：

```go
// 好：错误优先，正常代码无缩进
if err != nil {
    return err
}
// 正常代码
```

**错误只处理一次** — 记录日志或返回，不要两者都做：

```
遇到错误？
├─ 调用者可以采取行动？→ 返回（通过 %w 附带上下文）
├─ 在调用链顶部？→ 记录日志并处理
└─ 都不是？→ 以适当级别记录日志，继续执行
```

> 在组织复杂的错误流程、决定记录日志还是返回、实现一次处理模式、或选择结构化日志级别时，请阅读 [references/ERROR-FLOW.md](references/ERROR-FLOW.md)。

---

## 错误类型

> **建议**：推荐的最佳实践。

| 调用者需要匹配？ | 消息类型 | 使用方式 |
|-----------------|---------|---------|
| 否 | 静态 | `errors.New("message")` |
| 否 | 动态 | `fmt.Errorf("msg: %v", val)` |
| 是 | 静态 | `var ErrFoo = errors.New("...")` |
| 是 | 动态 | 自定义 `error` 类型 |

**默认**：使用 `fmt.Errorf("...: %w", err)` 包装。升级为哨兵以使用 `errors.Is()`，升级为自定义类型以使用 `errors.As()`。

> 在定义哨兵错误、创建自定义错误类型、或为包 API 选择错误策略时，请阅读 [references/ERROR-TYPES.md](references/ERROR-TYPES.md)。

---

## 错误包装

> **建议**：推荐的最佳实践。

- **使用 `%v`**：在系统边界、用于日志记录、隐藏内部细节
- **使用 `%w`**：保留错误链以供 `errors.Is`/`errors.As` 使用

**关键规则**：将 `%w` 放在末尾。添加调用者没有的上下文。如果注释没有增加信息，直接返回 `err`。

> 在决定使用 %v 还是 %w、跨包边界包装错误、或添加上下文信息时，请阅读 [references/WRAPPING.md](references/WRAPPING.md)。

> **验证**：实现错误处理后，运行 `bash scripts/check-errors.sh` 检测常见的反模式。然后运行 `go vet ./...` 捕获其他问题。

---

## 相关技能

- **错误命名**：在命名哨兵错误（`ErrFoo`）或自定义错误类型时，参见 [go-naming](../go-naming/SKILL.md)
- **测试错误**：在使用 `errors.Is`/`errors.As` 测试错误语义或编写错误检查辅助函数时，参见 [go-testing](../go-testing/SKILL.md)
- **Panic 处理**：在决定 panic 还是返回错误、或编写 recover 守卫时，参见 [go-defensive](../go-defensive/SKILL.md)
- **守卫子句**：在组织提前返回的错误流程或减少嵌套时，参见 [go-control-flow](../go-control-flow/SKILL.md)
- **日志决策**：在选择日志级别、配置结构化日志、或决定日志消息中包含什么上下文时，参见 [go-logging](../go-logging/SKILL.md)
