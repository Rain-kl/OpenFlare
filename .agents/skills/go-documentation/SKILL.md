---
name: go-documentation
description: 在编写或审查 Go 包、类型、函数或方法的文档时使用。在创建新的导出类型、函数或包时也应主动使用，即使用户没有明确询问文档问题。不涵盖未导出符号的代码注释（参见 go-style-core）。
license: Apache-2.0
metadata:
  sources: "Google 风格指南"
allowed-tools: Bash(bash:*)
---

# Go 文档

## 可用脚本

- **`scripts/check-docs.sh`** — 报告缺少文档注释的导出函数、类型、方法、常量和包。运行 `bash scripts/check-docs.sh --help` 查看选项。

> 在为新包或导出类型编写文档注释并需要所有文档约定的完整参考时，请参阅 `assets/doc-template.go`。

---

## 文档注释

> **规范**：所有顶层导出名称必须有文档注释。

### 基本规则

1. 以被描述对象的名称开头
2. 冠词（"a"、"an"、"the"）可以放在名称前面
3. 使用完整句子（首字母大写，带标点符号）

```go
// A Request represents a request to run a command.
type Request struct { ...

// Encode writes the JSON encoding of req to w.
func Encode(w io.Writer, req *Request) { ...
```

行为不明显的未导出类型/函数也应有文档注释。

> **验证**：添加文档注释后，运行 `bash scripts/check-docs.sh` 验证是否有导出符号缺少文档。修复所有缺失后再继续。

---

## 注释语句

> **规范**：文档注释必须是完整的句子。

- 首字母大写，以标点符号结尾
- 例外：如果含义清晰，可以以小写标识符开头
- 结构体字段的行尾注释可以是短语

---

## 注释行长度

> **建议**：目标约 80 列，但不设硬性限制。

根据标点符号换行。不要拆分长 URL。

---

## 结构体文档

使用段落注释对字段分组。标记可选字段及默认值：

```go
type Options struct {
    // 通用设置：
    Name  string
    Group *FooGroup

    // 自定义设置：
    LargeGroupThreshold int // 可选；默认值：10
}
```

---

## 包注释

> **规范**：每个包必须有且仅有一个包注释。

```go
// Package math provides basic constants and mathematical functions.
package math
```

- 对于 `main` 包，使用二进制名称：`// The seed_generator command ...`
- 对于较长的包注释，使用 `doc.go` 文件

> 在编写包级文档、main 包注释、doc.go 文件或可运行示例时，请阅读 [references/EXAMPLES.md](references/EXAMPLES.md)。

---

## 文档编写要点

> **建议**：记录非显而易见的行为，显而易见的行为无需记录。

| 主题 | 何时记录... | 何时跳过... |
|------|------------|------------|
| 参数 | 非显而易见的行为、边界情况 | 只是重复类型签名 |
| 上下文 | 行为与标准取消不同 | 标准 `ctx.Err()` 返回 |
| 并发 | 线程安全性不明确（例如，看似读取但内部修改） | 只读安全、修改不安全 |
| 清理 | 始终记录资源释放要求 | — |
| 错误 | 哨兵值、错误类型（使用 `*PathError`） | — |
| 命名返回值 | 多个同类型参数、面向操作命名 | 类型本身已足够清晰 |

关键原则：

- 上下文取消返回 `ctx.Err()` 是隐含的 — 不要重复说明
- 只读操作默认线程安全；修改操作默认不安全 — 不要重复说明
- 始终记录清理要求（例如，`Call Stop to release resources`）
- 在错误类型文档中使用指针（`*PathError`），以确保 `errors.Is`/`errors.As` 正确使用
- 不要仅为启用裸返回而命名返回值 — 清晰性 > 简洁性

> 在记录参数行为、上下文取消、并发安全性、清理要求、错误返回或函数文档注释中的命名返回参数时，请阅读 [references/CONVENTIONS.md](references/CONVENTIONS.md)。

---

## 可运行示例

> **建议**：在测试文件（`*_test.go`）中提供可运行示例。

```go
func ExampleConfig_WriteTo() {
    cfg := &Config{Name: "example"}
    cfg.WriteTo(os.Stdout)
    // Output:
    // {"name": "example"}
}
```

示例会出现在 Godoc 中，附加到对应的文档元素上。

> 在编写可运行 Example 函数、选择示例命名约定（Example vs ExampleType_Method）或添加包级 doc.go 文件时，请阅读 [references/EXAMPLES.md](references/EXAMPLES.md)。

---

## Godoc 格式化

> 在格式化 godoc 标题、链接、列表或代码块，使用信号增强来标记弃用通知，或在本地预览文档输出时，请阅读 [references/FORMATTING.md](references/FORMATTING.md)。

---

## 快速参考

| 主题 | 关键规则 |
|------|---------|
| 文档注释 | 以名称开头，使用完整句子 |
| 行长度 | 约 80 字符，优先考虑可读性 |
| 包注释 | 每个包一个，放在 `package` 声明之前 |
| 参数 | 仅记录非显而易见的行为 |
| 上下文 | 记录与隐含行为不同的例外情况 |
| 并发 | 记录线程安全性不明确的情况 |
| 清理 | 始终记录资源释放要求 |
| 错误 | 记录哨兵值和类型（注意指针） |
| 示例 | 在测试文件中使用可运行示例 |
| 格式化 | 空行分隔段落，缩进表示代码 |

---

## 相关技能

- **命名约定**：在为文档注释描述的标识符选择名称时，参见 [go-naming](../go-naming/SKILL.md)
- **测试示例**：在编写出现在 godoc 中的可运行 `Example` 测试函数时，参见 [go-testing](../go-testing/SKILL.md)
- **Lint 强制执行**：在使用 revive 或其他 linter 强制执行文档注释存在性时，参见 [go-linting](../go-linting/SKILL.md)
- **风格原则**：在平衡文档详细程度与清晰简洁时，参见 [go-style-core](../go-style-core/SKILL.md)
