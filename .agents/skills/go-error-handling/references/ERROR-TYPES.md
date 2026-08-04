# 错误类型参考

本参考涵盖结构化错误类型、哨兵错误，以及如何为你的用例选择正确的错误类型。

---

## 错误结构

> 错误类型决策表在父技能中（SKILL.md § 错误类型）。
> 本参考涵盖：扩展的代码示例、哨兵错误、使用 `errors.Is`/`errors.As` 进行错误检查，以及结构化错误类型。

**关键考虑因素**：

- 调用者是否需要使用 `errors.Is` 或 `errors.As` 来匹配错误？
- 错误消息是静态的还是需要运行时值？
- 导出的错误变量/类型将成为公共 API 的一部分

```go
// 无需匹配，静态消息
func Open() error {
    return errors.New("could not open")
}

// 需要匹配，静态消息 - 导出哨兵
var ErrCouldNotOpen = errors.New("could not open")

func Open() error {
    return ErrCouldNotOpen
}

// 需要匹配，动态消息 - 使用自定义类型
type NotFoundError struct {
    File string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("file %q not found", e.File)
}

func Open(file string) error {
    return &NotFoundError{File: file}
}
```

---

## 哨兵错误

最简单的结构化错误是无参数化的全局值：

```go
// 好：用于程序化检查的哨兵错误
var (
    // ErrDuplicate 在该动物已被见过时发生。
    ErrDuplicate = errors.New("duplicate")

    // ErrMarsupial 因为我们不支持有袋类动物。
    ErrMarsupial = errors.New("marsupials are not supported")
)

func process(animal Animal) error {
    switch {
    case seen[animal]:
        return ErrDuplicate
    case marsupial(animal):
        return ErrMarsupial
    }
    seen[animal] = true
    return nil
}
```

---

## 检查错误

对于直接比较（当错误未被包装时）：

```go
// 好：与哨兵直接比较
switch err := process(an); err {
case ErrDuplicate:
    return fmt.Errorf("feed %q: %v", an, err)
case ErrMarsupial:
    alternate := an.BackupAnimal()
    return handlePet(alternate)
}
```

当错误可能被包装时，使用 `errors.Is`：

```go
// 好：适用于被包装的错误
switch err := process(an); {
case errors.Is(err, ErrDuplicate):
    return fmt.Errorf("feed %q: %v", an, err)
case errors.Is(err, ErrMarsupial):
    // 尝试恢复...
}
```

**绝不**基于字符串内容匹配错误：

```go
// 不好：脆弱的字符串匹配
if regexp.MatchString(`duplicate`, err.Error()) {...}
if regexp.MatchString(`marsupial`, err.Error()) {...}
```

---

## 结构化错误类型

对于需要额外程序化信息的错误，使用结构体类型：

```go
// 好：具有可访问字段的结构化错误
type PathError struct {
    Op   string
    Path string
    Err  error
}

func (e *PathError) Error() string {
    return e.Op + " " + e.Path + ": " + e.Err.Error()
}

func (e *PathError) Unwrap() error { return e.Err }
```

调用者可以使用 `errors.As` 提取结构化错误：

```go
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    fmt.Println("Failed path:", pathErr.Path)
}
```

---

## 快速参考

| 场景 | 错误类型 |
|------|---------|
| 无需匹配，静态消息 | `errors.New("message")` |
| 无需匹配，动态消息 | `fmt.Errorf("msg: %v", val)` |
| 需要匹配，静态消息 | `var ErrFoo = errors.New(...)` |
| 需要匹配，动态消息 | 自定义结构体类型 |
| 检查哨兵错误 | `errors.Is(err, ErrFoo)` |
| 提取结构化错误 | `errors.As(err, &target)` |
