# 错误包装参考

本参考涵盖使用 `%v` vs `%w` 的错误包装、放置约定、向错误添加上下文以及日志最佳实践。

---

## 包装错误：%v vs %w

> **建议**：推荐的最佳实践。

`%v` 和 `%w` 的选择会显著影响错误的传播和检查方式。

### 使用 %v 进行简单注释

当你需要以下操作时使用 `%v`：

- 添加上下文但不保留错误链以供程序化检查
- 创建全新的、独立的错误（特别是在 RPC/IPC 等系统边界）
- 向人类记录或显示错误

```go
// 好：%v 在系统边界 — 隐藏内部细节
func (s *Server) SuggestFortune(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    if err != nil {
        return nil, fmt.Errorf("couldn't find fortune database: %v", err)
    }
}
```

### 使用 %w 保留错误链

当你需要调用者以编程方式检查底层错误时使用 `%w`：

```go
// 好：%w 保留错误链以供 errors.Is/errors.As 使用
func (s *Server) internalFunction(ctx context.Context) error {
    if err != nil {
        return fmt.Errorf("couldn't find remote file: %w", err)
    }
}

// 调用者现在可以检查：
if errors.Is(err, fs.ErrNotExist) {
    // 处理未找到的情况
}
```

### 何时使用哪种

**使用 %w 的场景**：
- 在添加上下文的同时保留原始错误以供程序化检查
- 你明确记录并测试了所暴露的底层错误

**使用 %v 的场景**：
- 在系统边界（RPC、IPC、存储）转换为规范错误空间
- 向人类记录日志或显示
- 创建隐藏实现细节的独立错误

---

## %w 的放置位置

> **建议**：推荐的最佳实践。

将 `%w` 放在错误字符串的**末尾**，使错误文本反映错误链结构：

```go
// 好：%w 在末尾 — 从最新到最旧打印
err1 := fmt.Errorf("err1")
err2 := fmt.Errorf("err2: %w", err1)
err3 := fmt.Errorf("err3: %w", err2)
fmt.Println(err3) // err3: err2: err1
```

```go
// 不好：%w 在开头 — 从最旧到最新打印（令人困惑）
err1 := fmt.Errorf("err1")
err2 := fmt.Errorf("%w: err2", err1)
err3 := fmt.Errorf("%w: err3", err2)
fmt.Println(err3) // err1: err2: err3
```

```go
// 不好：%w 在中间 — 不连贯的顺序
err1 := fmt.Errorf("err1")
err2 := fmt.Errorf("err2-1 %w err2-2", err1)
err3 := fmt.Errorf("err3-1 %w err3-2", err2)
fmt.Println(err3) // err3-1 err2-1 err1 err2-2 err3-2
```

**模式**：使用 `context message: %w` 的形式

---

## 向错误添加信息

> **建议**：推荐的最佳实践。

### 添加上下文，而非冗余

添加你拥有但调用者/被调用者可能没有的信息。避免重复底层错误已提供的信息：

```go
// 好：添加有意义的上下文
if err := os.Open("settings.txt"); err != nil {
    return fmt.Errorf("launch codes unavailable: %v", err)
}
// 输出：launch codes unavailable: open settings.txt: no such file or directory
```

```go
// 不好：重复了文件名
if err := os.Open("settings.txt"); err != nil {
    return fmt.Errorf("could not open settings.txt: %v", err)
}
// 输出：could not open settings.txt: open settings.txt: no such file or directory
```

### 不要无目的地注释

如果注释仅表示失败而没有添加信息，直接返回错误：

```go
// 不好：注释没有增加信息
return fmt.Errorf("failed: %v", err)

// 好：直接返回错误
return err
```

---

## 记录错误日志

> **建议**：推荐的最佳实践。

当需要记录错误时，使用 `log/slog`（Go 1.21+）配合结构化键值对和适当的日志级别：

- **`slog.Error`**：保留用于需要调查的可操作问题。
- **`slog.Warn`**：用于可能需要关注但不可立即操作的问题。
- **`slog.Debug`**：用于开发追踪 — 仅在 handler 级别设为 `LevelDebug` 时才输出。

```go
// 好：使用适当级别的结构化日志
for _, q := range queries {
    slog.Debug("handling query", "query", q)
    q.Run()
}

// 好：在级别检查后保护昂贵的格式化操作
if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
    slog.Debug("query plan", "explain", q.Explain())
}

// 不好：即使禁用了 debug 日志也会执行昂贵的调用
slog.Debug("query plan", "explain", q.Explain())
```

### 保护敏感信息

注意日志消息中的 PII（个人身份信息）。许多日志接收器不适合存放敏感用户数据。

---

## 快速参考

| 模式 | 指导 |
|------|------|
| `%v` | 在系统边界使用、用于日志记录、隐藏细节 |
| `%w` | 保留错误链以供程序化检查 |
| `%w` 放置 | 始终在末尾：`"context: %w"` |
| 添加上下文 | 添加新信息，不要重复现有信息 |
| 空注释 | 直接返回 `err` 而非 `fmt.Errorf("failed: %v", err)` |
| 日志 | 不要既记录日志又返回；使用适当的日志级别 |
