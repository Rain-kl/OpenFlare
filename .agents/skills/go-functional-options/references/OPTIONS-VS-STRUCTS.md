# 函数式选项 vs 配置结构体

> **来源**：Google 风格指南, Uber 风格指南

函数式选项和配置结构体解决相同的问题 — 构造函数的可选配置 — 但它们有不同的权衡。根据 API 受众、可扩展性需求和复杂性预算来选择。

## 决策框架

```
需要可选配置？
├─ 内部或仅测试 API？
│   └─ 配置结构体（更简单，更少样板代码）
├─ 具有 3 个以上选项的公共 API？
│   └─ 函数式选项（可扩展，向后兼容）
├─ 选项需要校验或有相互依赖？
│   └─ 函数式选项（在 apply 或构造函数中校验）
├─ 所有选项通常一起指定？
│   └─ 配置结构体（一个字面量，无需 With* 仪式）
└─ 选项可能随时间增长？
    └─ 函数式选项（添加 With* 不会破坏调用者）
```

## 配置结构体模式

配置结构体将可选参数分组为传递给构造函数的单个结构体。零值作为默认值，或提供 `DefaultConfig()`。

**好**
```go
type Config struct {
    Timeout  time.Duration // 零 = 无超时
    MaxRetry int           // 零 = 无重试
    Logger   *log.Logger   // nil = 丢弃
}

func NewClient(addr string, cfg Config) *Client {
    if cfg.Logger == nil {
        cfg.Logger = log.New(io.Discard, "", 0)
    }
    return &Client{addr: addr, cfg: cfg}
}
```

```go
c := NewClient("localhost:8080", Config{
    Timeout:  5 * time.Second,
    MaxRetry: 3,
})
```

**不好** — 在公共 API 中依赖未导出的配置字段：
```go
type config struct {  // 未导出：调用者无法构造
    timeout time.Duration
}

func NewClient(addr string, cfg config) *Client { ... }
```

### 当零值不适用时

如果零是一个有效的非默认值（例如，超时为 0 表示"无超时"，但期望的默认值是 30s），使用指针字段或哨兵值：

```go
type Config struct {
    Timeout *time.Duration // nil = 使用默认值（30s），零 = 无超时
}
```

## 比较

| 方面 | 函数式选项 | 配置结构体 |
|------|-----------|-----------|
| **样板代码** | 高（每个选项需要类型 + apply + With*） | 低（一个结构体） |
| **可扩展性** | 添加 `With*` — 无破坏性变更 | 添加字段 — 无破坏性变更 |
| **向后兼容** | 对公共 API 极好 | 好（新字段获得零值） |
| **默认值** | 内置于构造函数 | 零值或 `DefaultConfig()` |
| **校验** | 在 `apply` 或构造函数循环中 | 在接收到结构体后的构造函数中 |
| **可发现性** | `With*` 函数出现在 godoc 中 | 所有字段在一个结构体中可见 |
| **可测试性** | 比较选项或测试构造函数输出 | 比较结构体字面量 |
| **调用者体验** | 只指定与默认值不同的部分 | 必须构造结构体字面量 |
| **零值歧义** | 无 — 未设置的选项不应用 | 可能需要指针字段 |

## 何时优先使用配置结构体

- **内部 API** — 更少的仪式，在调用处更易读
- **少量选项（1-3 个）** — 函数式选项的开销不值得
- **所有选项通常一起设置** — 可变参数风格没有好处
- **不需要校验** — 简单的字段赋值即可
- **选项是数据而非行为** — 结构体字段自然映射

```go
srv := NewServer(Config{
    Port:    8080,
    TLSCert: "/path/to/cert.pem",
    TLSKey:  "/path/to/key.pem",
})
```

## 何时优先使用函数式选项

- **公共/库 API** — 调用者不应跟踪内部配置的演变
- **3 个以上选项**，每个都是可选的
- **复杂默认值** — 默认值计算依赖于其他选项
- **按选项校验** — 在 apply 时拒绝无效值
- **选项可能增长** — 新的 `With*` 函数是纯粹增量的

```go
srv := NewServer(
    WithPort(8080),
    WithTLS("/path/to/cert.pem", "/path/to/key.pem"),
    WithLogger(logger),
)
```

## 混合方法

对于同时需要便利性和可扩展性的 API，接受配置结构体用于常见设置，函数式选项用于高级覆盖：

```go
func NewServer(cfg Config, opts ...Option) *Server {
    s := &Server{cfg: cfg}
    for _, o := range opts {
        o.apply(&s.cfg)
    }
    return s
}
```

谨慎使用 — 它增加了复杂性。每个 API 优先使用一种方法。
