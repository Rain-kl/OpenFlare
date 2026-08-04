# Must 函数

> **来源**：Uber 风格指南, Go 标准库约定

`Must` 函数包装一个可能失败的函数，在出错时 panic。**仅**在程序初始化阶段使用，因为失败意味着程序无法运行。

## 标准库示例

```go
// regexp.MustCompile 在模式无效时 panic
var validID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// template.Must 在模板解析失败时 panic
var tmpl = template.Must(template.ParseFiles("index.html"))
```

这些是安全的，因为它们在包初始化时运行——如果失败，程序无法正确运行。

## 何时使用 Must

```
这是在程序初始化期间调用的吗（包级 var、init、main 设置）？
├─ 是 → 失败是否不可恢复（配置、正则、模板）？
│        ├─ 是 → 使用 Must 是合适的
│        └─ 否 → 改为返回 error
└─ 否 → 绝不使用 Must——返回 error
```

### 适当的使用场景

- **包级 `var`**：编译正则表达式、解析模板、加载必需的配置
- **`init()` 或 `main()` 早期**：设置程序运行所必需的资源
- **测试辅助函数**：测试中优先使用 `t.Fatal`，但 Must 在测试 fixture 中是可以接受的

### 绝不使用 Must 的场景

- 运行时请求处理
- 用户提供的输入
- 可能合理失败的网络或文件操作
- 程序启动后调用的任何代码

## 编写 Must 函数

遵循命名约定 `MustX`，其中 `X` 是可能失败的函数名：

```go
func MustParseConfig(path string) *Config {
    cfg, err := ParseConfig(path)
    if err != nil {
        panic(fmt.Sprintf("parsing config %s: %v", path, err))
    }
    return cfg
}
```

### 指南

- **命名**：`Must` 前缀 + 可能失败的函数名（例如 `MustParse`、`MustNew`、`MustCompile`）
- **Panic 消息**：包含输入和错误信息以便调试
- **文档**：始终记录函数在出错时会 panic

```go
// MustParseConfig 解析路径处的配置文件。
// 如果文件无法读取或包含无效配置，则会 panic。
func MustParseConfig(path string) *Config { ... }
```

### 泛型 Must 辅助函数

对于一次性使用，泛型 Must 辅助函数可以避免样板代码：

```go
func Must[T any](v T, err error) T {
    if err != nil {
        panic(err)
    }
    return v
}

// 在包级别使用
var cfg = Must(ParseConfig("app.yaml"))
```

## 与 Panic/Recover 的关系

Must 函数是对 `panic` 的受控使用。它们应该：

- 仅在初始化期间运行（因此不需要 recover）
- 产生清晰、可操作的 panic 消息
- 绝不在可以返回 error 的场景中使用

完整的 panic/recover 模式请参见 [PANIC-RECOVER.md](PANIC-RECOVER.md)。
