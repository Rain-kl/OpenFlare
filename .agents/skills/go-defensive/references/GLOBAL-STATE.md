# 全局状态模式

> **来源**：Google 风格指南, Effective Go

全局状态使程序更难以测试、推理和维护。依赖注入是首选替代方案，但某些全局状态在谨慎使用时是可以接受的。

## 何时可以接受全局状态

并非所有包级变量都有害。当全局状态是**真正进程级别的**且**不值得注入**时，它是合适的：

- **默认实例**——`http.DefaultClient`、`log.Default()`、`flag.CommandLine`
- **一次编译的值**——包级别的 `regexp.MustCompile(...)`
- **注册表**——`database/sql.Register`、`image.RegisterFormat`
- **单例基础设施**——进程级别的指标收集器或追踪导出器

## 全局变量的试金石测试

在添加包级变量之前，请问自己：

1. **它是否真正是进程级别的？** 如果两个 goroutine 或测试可能需要不同的值，它不应该是全局的
2. **它是否妨碍了测试？** 如果测试必须保存/恢复变量，或因此无法并行运行，应改为注入
3. **它可以是常量吗？** 如果值在初始化后永远不会改变，优先使用 `const` 或未导出的只初始化一次的 `var`
4. **它是否携带可变状态？** 可变全局变量是最危险的——仅在有完善文档、并发安全的单例情况下才可接受

## 包状态 API 模式：New() + Default()

标准库模式同时提供可定制的构造器和便捷的默认值。这使调用者可以在简单场景下使用默认值，在测试或特殊行为需求下注入自定义实例。

**好**
```go
package mylog

type Logger struct {
    prefix string
    out    io.Writer
}

func New(prefix string, out io.Writer) *Logger {
    return &Logger{prefix: prefix, out: out}
}

var defaultLogger = New("", os.Stderr)

func Default() *Logger { return defaultLogger }

func (l *Logger) Info(msg string) {
    fmt.Fprintf(l.out, "%s%s\n", l.prefix, msg)
}

// 包级便捷函数委托给默认实例。
func Info(msg string) { defaultLogger.Info(msg) }
```

```go
// 调用者在简单场景下使用默认值
mylog.Info("starting server")

// 测试或特殊代码创建自定义实例
logger := mylog.New("[test] ", &buf)
logger.Info("test message")
```

此模式的标准库示例：
- `log.New()` + `log.Default()` + `log.Println()`
- `http.NewServeMux()` + `http.DefaultServeMux`
- `flag.NewFlagSet()` + `flag.CommandLine`

## 依赖注入作为首选替代方案

当代码需要可配置行为时，通过构造器参数或结构体字段接受依赖，而非读取包级变量。

**不好**
```go
var db *sql.DB

func GetUser(id int) (*User, error) {
    return db.QueryRow("SELECT ...", id) // 依赖全局变量
}
```

**好**
```go
type UserStore struct {
    db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
    return &UserStore{db: db}
}

func (s *UserStore) GetUser(id int) (*User, error) {
    return s.db.QueryRow("SELECT ...", id)
}
```

注入的好处：
- 测试可以提供 mock 或内存实现
- 多个实例可以共存（例如，只读副本与主库）
- 依赖在构造器签名中是显式的

## 注入时间

一个常见场景：替换 `time.Now` 以实现确定性测试。

**不好**
```go
func IsExpired(expiry time.Time) bool {
    return time.Now().After(expiry) // 不可测试
}
```

**好**
```go
type Checker struct {
    now func() time.Time
}

func NewChecker() *Checker {
    return &Checker{now: time.Now}
}

func (c *Checker) IsExpired(expiry time.Time) bool {
    return c.now().After(expiry)
}
```

测试用固定函数替换 `now`：

```go
c := &Checker{now: func() time.Time {
    return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}}
```

## 总结

| 场景 | 方法 |
|------|------|
| 进程级单例（日志、指标） | 默认实例 + `New()` 构造器 |
| 一次编译的正则或模板 | 包级 `var` 配合 `MustCompile` |
| 注册表（数据库驱动、编解码器） | 包级 `Register()` 函数 |
| 可配置行为 | 通过构造器进行依赖注入 |
| 时间相关逻辑 | 注入 `func() time.Time` |
| 测试需要变化的任何东西 | 不要使用全局状态 |
