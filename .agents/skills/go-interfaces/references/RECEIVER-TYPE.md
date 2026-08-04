# 接收者类型：指针 vs 值

> **建议**：Go Wiki CodeReviewComments

选择在方法上使用值接收者还是指针接收者可能很困难。**如果不确定，使用指针**，但有时值接收者也是合理的。

## 何时使用指针接收者

- **方法修改接收者**：接收者必须是指针
- **接收者包含 sync.Mutex 或类似类型**：必须使用指针以避免复制
- **大型结构体或数组**：指针接收者更高效。如果将所有元素作为参数传递感觉太大，那对值接收者来说也太大了
- **并发或被调方法可能修改**：如果更改必须对原始接收者可见，则必须使用指针
- **元素是指向可变内容的指针**：优先使用指针接收者使意图更清晰

## 何时使用值接收者

- **小型不变的结构体或基本类型**：值接收者以提高效率
- **Map、func 或 chan**：不要对它们使用指针
- **不重新切片/重新分配的切片**：如果方法不重新切片或重新分配切片，不要使用指针
- **没有可变字段的小型值类型**：像 `time.Time` 这样没有可变字段且没有指针的类型适合作为值接收者
- **简单基本类型**：`int`、`string` 等

```go
// 值接收者：小型、不可变类型
type Point struct {
    X, Y float64
}

func (p Point) Distance(q Point) float64 {
    return math.Hypot(q.X-p.X, q.Y-p.Y)
}

// 指针接收者：方法修改接收者
func (p *Point) ScaleBy(factor float64) {
    p.X *= factor
    p.Y *= factor
}

// 指针接收者：包含 sync.Mutex
type Counter struct {
    mu    sync.Mutex
    count int
}

func (c *Counter) Increment() {
    c.mu.Lock()
    c.count++
    c.mu.Unlock()
}
```

## 一致性规则

**不要混合接收者类型**。为类型上所有可用的方法统一选择指针或结构体类型。如果任何方法需要指针接收者，则所有方法都使用指针接收者。

```go
// 好：一致的指针接收者
type Buffer struct {
    data []byte
}

func (b *Buffer) Write(p []byte) (int, error) { /* ... */ }
func (b *Buffer) Read(p []byte) (int, error)  { /* ... */ }
func (b *Buffer) Len() int                     { return len(b.data) }

// 不好：混合接收者类型
func (b Buffer) Len() int                      { return len(b.data) }  // 不一致
```
