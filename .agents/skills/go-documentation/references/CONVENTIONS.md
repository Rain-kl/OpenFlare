# 文档约定参考

## 参数和配置

> **建议**：记录容易出错或非显而易见的参数，而非所有参数。

```go
// 不好：重复了显而易见的信息
// Sprintf formats according to a format specifier and returns the resulting string.
//
// format is the format, and data is the interpolation data.
func Sprintf(format string, data ...any) string

// 好：记录了非显而易见的行为
// Sprintf formats according to a format specifier and returns the resulting string.
//
// The provided data is used to interpolate the format string. If the data does
// not match the expected format verbs or the amount of data does not satisfy
// the format specification, the function will inline warnings about formatting
// errors into the output string.
func Sprintf(format string, data ...any) string
```

---

## 上下文

> **建议**：不要重复隐含的上下文行为；记录例外情况。

上下文取消被隐含地认为会中断函数并返回 `ctx.Err()`。不要记录这一点。

```go
// 不好：重复了隐含的行为
// Run executes the worker's run loop.
//
// The method will process work until the context is cancelled.
func (Worker) Run(ctx context.Context) error

// 好：只记录关键信息
// Run executes the worker's run loop.
func (Worker) Run(ctx context.Context) error
```

**当行为不同时记录：**

```go
// 好：非标准的取消行为
// Run executes the worker's run loop.
//
// If the context is cancelled, Run returns a nil error.
func (Worker) Run(ctx context.Context) error

// 好：特殊的上下文要求
// NewReceiver starts receiving messages sent to the specified queue.
// The context should not have a deadline.
func NewReceiver(ctx context.Context) *Receiver
```

---

## 并发

> **建议**：记录非显而易见的线程安全特性。

只读操作被认为是安全的；修改操作被认为是不安全的。不要重复说明这一点。

**何时记录：**

```go
// 不明确的操作（看似只读但内部有修改）
// Lookup returns the data associated with the key from the cache.
//
// This operation is not safe for concurrent use.
func (*Cache) Lookup(key string) (data []byte, ok bool)

// API 提供同步机制
// NewFortuneTellerClient returns an *rpc.Client for the FortuneTeller service.
// It is safe for simultaneous use by multiple goroutines.
func NewFortuneTellerClient(cc *rpc.ClientConn) *FortuneTellerClient

// 接口有并发要求
// A Watcher reports the health of some entity (usually a backend service).
//
// Watcher methods are safe for simultaneous use by multiple goroutines.
type Watcher interface {
    Watch(changed chan<- bool) (unwatch func())
    Health() error
}
```

---

## 清理

> **建议**：始终记录显式清理要求。

```go
// 好：
// NewTicker returns a new Ticker containing a channel that will send the
// current time on the channel after each tick.
//
// Call Stop to release the Ticker's associated resources when done.
func NewTicker(d Duration) *Ticker

// 好：展示如何清理
// Get issues a GET to the specified URL.
//
// When err is nil, resp always contains a non-nil resp.Body.
// Caller should close resp.Body when done reading from it.
//
//    resp, err := http.Get("http://example.com/")
//    if err != nil {
//        // handle error
//    }
//    defer resp.Body.Close()
//    body, err := io.ReadAll(resp.Body)
func (c *Client) Get(url string) (resp *Response, err error)
```

---

## 错误

> **建议**：记录重要的错误哨兵值和类型。

```go
// 好：记录哨兵值
// Read reads up to len(b) bytes from the File and stores them in b.
//
// At end of file, Read returns 0, io.EOF.
func (*File) Read(b []byte) (n int, err error)

// 好：记录错误类型（包含指针接收者）
// Chdir changes the current working directory to the named directory.
//
// If there is an error, it will be of type *PathError.
func Chdir(dir string) error
```

注意使用 `*PathError`（而非 `PathError`）可以确保 `errors.Is` 和 `errors.As` 的正确使用。

对于包级别的错误约定，在包注释中记录。

---

## 命名返回参数

> **建议**：在类型本身不够清晰时用于文档说明。

```go
// 好：多个同类型参数
func (n *Node) Children() (left, right *Node, err error)

// 好：面向操作的名称阐明了用法
// The caller must arrange for the returned cancel function to be called.
func WithTimeout(parent Context, d time.Duration) (ctx Context, cancel func())

// 不好：类型已经很清晰，命名没有增加信息
func (n *Node) Parent1() (node *Node)
func (n *Node) Parent2() (node *Node, err error)

// 好：类型已足够
func (n *Node) Parent1() *Node
func (n *Node) Parent2() (*Node, error)
```

不要仅为启用裸返回而命名返回值。清晰性 > 简洁性。

---

## 弃用通知

> **建议**：使用 `// Deprecated:` 注释标记符号为已弃用。

`Deprecated:` 段落必须出现在文档注释中紧接在符号之前。应说明使用什么替代。

**标准格式：**

```
// Deprecated: Use NewThing instead.
```

Godoc 会以特殊的视觉样式渲染 `Deprecated:` 注释，使其容易被发现。

**函数弃用：**

```go
// EstimateSize returns an approximate byte count.
//
// Deprecated: Use [Size] instead, which returns an exact count.
func EstimateSize(r io.Reader) (int64, error)
```

**类型弃用：**

```go
// LegacyClient talks to the v1 API.
//
// Deprecated: Use [Client] instead, which supports v2.
type LegacyClient struct{ /* ... */ }
```

**包弃用** — 在包文档注释中添加 `Deprecated:`：

```go
// Package old provides the original implementation.
//
// Deprecated: Use package example/new instead.
package old
```

始终建议具体的替代方案，让调用者知道迁移目标。

---

## 注释语句 — 详细说明

> **规范**：文档注释必须是完整的句子。

- 首字母大写，以标点符号结尾
- 例外：如果含义清晰，可以以小写标识符开头
- 结构体字段的行尾注释可以是短语：

```go
// 好：
// A Server handles serving quotes from Shakespeare.
type Server struct {
    // BaseDir points to the base directory for Shakespeare's works.
    //
    // Expected structure:
    //   {BaseDir}/manifest.json
    //   {BaseDir}/{name}/{name}-part{number}.txt
    BaseDir string

    WelcomeMessage  string // 用户登录时显示
    ProtocolVersion string // 与传入请求进行校验
    PageLength      int    // 每页行数（可选；默认值：20）
}
```
