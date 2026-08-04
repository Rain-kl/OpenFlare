# 包注释和示例参考

## 包注释

> **规范**：每个包必须有且仅有一个包注释。

```go
// 好：
// Package math provides basic constants and mathematical functions.
//
// This package does not guarantee bit-identical results across architectures.
package math
```

### Main 包

使用二进制名称（与 BUILD 文件匹配）：

```go
// 好：
// The seed_generator command is a utility that generates a Finch seed file
// from a set of JSON study configs.
package main
```

有效格式：`Binary seed_generator`、`Command seed_generator`、`The seed_generator command`、`Seed_generator ...`

### doc.go

- 对于较长的包注释，使用仅包含包注释和 `package` 声明的 `doc.go` 文件
- 放在 import 之后的维护者注释不会出现在 Godoc 中
- 保持 doc.go 文件专注于面向用户的文档

```go
// Package complex provides advanced mathematical operations for
// complex number arithmetic, including polar form conversion,
// matrix operations, and numerical integration.
//
// Basic usage
//
// Create a complex number and perform operations:
//
//   z := complex.New(3, 4)
//   magnitude := z.Abs()    // 5.0
//   conjugate := z.Conj()   // (3, -4)
//
// Matrix operations
//
// The package supports complex-valued matrices:
//
//   m := complex.NewMatrix(2, 2)
//   m.Set(0, 0, complex.New(1, 0))
//   det := m.Det()
package complex
```

---

## 可运行示例

> **建议**：提供可运行示例来展示包的用法。

将示例放在测试文件（`*_test.go`）中：

```go
// 好：
func ExampleConfig_WriteTo() {
    cfg := &Config{
        Name: "example",
    }
    if err := cfg.WriteTo(os.Stdout); err != nil {
        log.Exitf("Failed to write config: %s", err)
    }
    // Output:
    // {
    //   "name": "example"
    // }
}
```

示例会出现在 Godoc 中，附加到对应的文档元素上。

### 命名约定

| 函数名称 | 文档对象 |
|----------|---------|
| `Example()` | 包级别示例 |
| `ExampleFoo()` | 函数 `Foo` |
| `ExampleBar_Baz()` | 方法 `Bar.Baz` |
| `ExampleFoo_suffix()` | `Foo` 示例的命名变体 |

### 技巧

- 使用 `// Output:` 注释使示例可通过 `go test` 进行测试和验证
- 保持示例专注于展示一个概念
- 使用真实但精简的数据
- 对于复杂的设置，使用 `testMain` 或辅助函数保持示例主体简洁
- 同一符号的多个示例使用小写 `_suffix`：

```go
func ExampleNewClient_withTimeout() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    client := NewClient(ctx)
    // ...
}
```
