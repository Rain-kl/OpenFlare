---
name: go-functional-options
description: Use when designing a Go constructor or factory function with optional configuration — especially with 3+ optional parameters or extensible APIs. Also use when building a New* function that takes many settings, even if they don't mention "functional options" by name. Does not cover general function design (see go-functions).
license: Apache-2.0
metadata:
  sources: "Uber Style Guide"
---

# 函数式选项模式

函数式选项是一种模式，你声明一个不透明的 `Option` 类型，在内部结构体中记录信息。构造函数接受可变数量的这些选项并将其应用于配置结果。

## 何时使用

在以下情况使用函数式选项：

- 构造函数或公共 API 上有 **3 个以上可选参数**
- **可扩展 API**，可能随时间增加新选项
- **良好的调用者体验**很重要（无需传递默认值）

## 模式

### 核心组件

1. **未导出的 `options` 结构体** - 保存所有配置
2. **导出的 `Option` 接口** - 带有未导出的 `apply` 方法
3. **Option 类型** - 实现接口
4. **`With*` 构造函数** - 创建选项

### Option 接口

```go
type Option interface {
    apply(*options)
}
```

未导出的 `apply` 方法确保只能使用来自本包的选项。

## 完整实现

```go
package db

import "go.uber.org/zap"

// options 保存打开连接的所有配置。
type options struct {
    cache  bool
    logger *zap.Logger
}

// Option 配置我们如何打开连接。
type Option interface {
    apply(*options)
}

// cacheOption 为缓存设置实现 Option（简单类型别名）。
type cacheOption bool

func (c cacheOption) apply(opts *options) {
    opts.cache = bool(c)
}

// WithCache 启用或禁用缓存。
func WithCache(c bool) Option {
    return cacheOption(c)
}

// loggerOption 为日志设置实现 Option（用于指针的结构体）。
type loggerOption struct {
    Log *zap.Logger
}

func (l loggerOption) apply(opts *options) {
    opts.logger = l.Log
}

// WithLogger 设置连接的日志记录器。
func WithLogger(log *zap.Logger) Option {
    return loggerOption{Log: log}
}

// Open 创建一个连接。
func Open(addr string, opts ...Option) (*Connection, error) {
    // 从默认值开始
    options := options{
        cache:  defaultCache,
        logger: zap.NewNop(),
    }

    // 应用所有提供的选项
    for _, o := range opts {
        o.apply(&options)
    }

    // 使用 options.cache 和 options.logger...
    return &Connection{}, nil
}
```

## 使用示例

### 不使用函数式选项（不好）

```go
// 调用者必须始终提供所有参数，即使是默认值
db.Open(addr, db.DefaultCache, zap.NewNop())
db.Open(addr, db.DefaultCache, log)
db.Open(addr, false /* cache */, zap.NewNop())
db.Open(addr, false /* cache */, log)
```

### 使用函数式选项（好）

```go
// 只在需要时提供选项
db.Open(addr)
db.Open(addr, db.WithLogger(log))
db.Open(addr, db.WithCache(false))
db.Open(
    addr,
    db.WithCache(false),
    db.WithLogger(log),
)
```

## 比较：函数式选项 vs 配置结构体

| 方面 | 函数式选项 | 配置结构体 |
|------|-----------|-----------|
| **可扩展性** | 添加新的 `With*` 函数 | 添加新字段（可能破坏兼容性） |
| **默认值** | 内置于构造函数 | 零值或单独的默认值 |
| **调用者体验** | 只指定不同的部分 | 必须构造整个结构体 |
| **可测试性** | 选项可比较 | 结构体比较 |
| **复杂性** | 更多样板代码 | 更简单的设置 |

**优先使用配置结构体的场景**：少于 3 个选项、选项很少变化、所有选项通常一起指定、或仅用于内部 API。

> 在决定使用函数式选项还是配置结构体、设计具有适当默认值的配置结构体 API、或评估复杂构造函数的混合方法时，请阅读 [references/OPTIONS-VS-STRUCTS.md](references/OPTIONS-VS-STRUCTS.md)。

## 为什么不使用闭包？

另一种实现使用闭包：

```go
// 闭包方法（不推荐）
type Option func(*options)

func WithCache(c bool) Option {
    return func(o *options) { o.cache = c }
}
```

优先使用接口方法，因为：

1. **可测试性** - 选项可以在测试和 mock 中进行比较
2. **可调试性** - 选项可以实现 `fmt.Stringer`
3. **灵活性** - 选项可以实现额外的接口
4. **可见性** - 选项类型在文档中可见

## 快速参考

```go
// 1. 带有默认值的未导出 options 结构体
type options struct {
    field1 Type1
    field2 Type2
}

// 2. 导出的 Option 接口，未导出的方法
type Option interface {
    apply(*options)
}

// 3. Option 类型 + apply + With* 构造函数
type field1Option Type1

func (o field1Option) apply(opts *options) { opts.field1 = Type1(o) }
func WithField1(v Type1) Option            { return field1Option(v) }

// 4. 构造函数在默认值之上应用选项
func New(required string, opts ...Option) (*Thing, error) {
    o := options{field1: defaultField1, field2: defaultField2}
    for _, opt := range opts {
        opt.apply(&o)
    }
    // ...
}
```

### 检查清单

- [ ] `options` 结构体未导出
- [ ] `Option` 接口有未导出的 `apply` 方法
- [ ] 每个选项有 `With*` 构造函数
- [ ] 默认值在应用选项之前设置
- [ ] 必需参数与 `...Option` 分开

## 相关技能

- **接口设计**：在设计 `Option` 接口或选择接口与闭包方法时，参见 [go-interfaces](../go-interfaces/SKILL.md)
- **命名约定**：在命名 `With*` 构造函数、选项类型或未导出的 options 结构体时，参见 [go-naming](../go-naming/SKILL.md)
- **函数设计**：在组织文件中的构造函数或格式化可变参数签名时，参见 [go-functions](../go-functions/SKILL.md)
- **文档**：在记录 `Option` 类型、`With*` 函数或构造函数行为时，参见 [go-documentation](../go-documentation/SKILL.md)

### 外部资源

- [Self-referential functions and the design of options](https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html) - Rob Pike
- [Functional options for friendly APIs](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis) - Dave Cheney
