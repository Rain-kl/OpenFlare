# Printf、Stringer 与自定义格式化

Go 的 `fmt` 打印动词、`Stringer` 和 `GoStringer` 接口、自定义 `Format()` 方法以及常见陷阱的深度参考。

---

## Printf 动词

### 通用动词

| 动词 | 用途 |
|------|------|
| `%v` | 默认格式（结构体字段、切片元素） |
| `%+v` | 带字段名的结构体：`{Name:alice Age:30}` |
| `%#v` | Go 语法表示：`main.User{Name:"alice", Age:30}` |
| `%T` | 值的类型：`main.User` |
| `%%` | 字面百分号 |

### 字符串与字节动词

| 动词 | 用途 |
|------|------|
| `%s` | 纯字符串或字节切片 |
| `%q` | 带 Go 语法转义的引号字符串：`"hello\n"` |
| `%x` | 十六进制编码，小写：`68656c6c6f` |
| `%X` | 十六进制编码，大写：`68656C6C6F` |

### 整数动词

| 动词 | 用途 |
|------|------|
| `%d` | 十进制整数 |
| `%b` | 二进制 |
| `%o` | 八进制 |
| `%O` | 带 `0o` 前缀的八进制 |
| `%x` | 十六进制，小写 |
| `%X` | 十六进制，大写 |

### 浮点数动词

| 动词 | 用途 |
|------|------|
| `%f` | 小数点，无指数：`123.456` |
| `%e` | 科学计数法：`1.23456e+02` |
| `%g` | 紧凑格式：大指数用 `%e`，否则用 `%f` |

### 宽度与精度

```go
fmt.Sprintf("%10d", 42)     // "        42"  （宽度 10，右对齐）
fmt.Sprintf("%-10d", 42)    // "42        "  （宽度 10，左对齐）
fmt.Sprintf("%.2f", 3.14159) // "3.14"       （2 位小数）
fmt.Sprintf("%010d", 42)    // "0000000042"  （零填充）
```

---

## 使用 `%q` 输出字符串

`%q` 动词在双引号内打印字符串，使空字符串和控制字符可见：

```go
fmt.Printf("value %q looks like English text", someText)

// 不好：手动添加引号
fmt.Printf("value \"%s\" looks like English text", someText)
```

在面向人类的输出中，如果值可能为空或包含控制字符，优先使用 `%q`。

---

## Printf 之外的格式字符串

在 `Printf` 风格调用之外声明格式字符串时，使用 `const`。这样 `go vet` 可以进行静态分析：

```go
// 不好：变量格式字符串——go vet 无法检查
msg := "unexpected values %v, %v\n"
fmt.Printf(msg, 1, 2)

// 好：常量格式字符串——go vet 可以验证
const msg = "unexpected values %v, %v\n"
fmt.Printf(msg, 1, 2)
```

---

## Printf 风格函数的命名

接受格式字符串的函数应以 `f` 结尾。这样 `go vet` 可以自动检查格式字符串：

```go
func Wrapf(err error, format string, args ...any) error
```

如果使用非标准名称，需要告知 `go vet`：

```bash
go vet -printfuncs=wrapf,statusf
```

---

## `fmt.Stringer` 接口

实现 `fmt.Stringer` 来控制类型在 `%v` 和 `%s` 下的显示方式：

```go
type fmt.Stringer interface {
    String() string
}
```

```go
type Point struct{ X, Y int }

func (p Point) String() string {
    return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}

// fmt.Println(Point{1, 2})           → "(1, 2)"
// fmt.Sprintf("point: %v", p)        → "point: (1, 2)"
// fmt.Sprintf("point: %s", p)        → "point: (1, 2)"
```

### 何时实现 Stringer

- 类型将出现在日志消息或面向用户的输出中
- 默认的 `%v` 输出（仅字段值）不够有意义
- 需要一种区别于序列化的、对人类友好的表示

---

## `fmt.GoStringer` 接口

实现 `fmt.GoStringer` 来控制 `%#v` 输出。这对于默认 Go 语法表示具有误导性或过于冗长的类型很有用：

```go
type fmt.GoStringer interface {
    GoString() string
}
```

```go
type Color struct{ R, G, B uint8 }

func (c Color) GoString() string {
    return fmt.Sprintf("Color(%#02x, %#02x, %#02x)", c.R, c.G, c.B)
}

// fmt.Sprintf("%#v", Color{255, 128, 0})
// → "Color(0xff, 0x80, 0x00)"   而非  "main.Color{R:0xff, G:0x80, B:0x00}"
```

`GoString()` 的输出应该是有效的 Go 语法或接近有效语法——它用于调试，而非面向用户的显示。

---

## 使用 `fmt.Formatter` 自定义格式化

要完全控制所有格式动词，实现 `fmt.Formatter`：

```go
type fmt.Formatter interface {
    Format(f fmt.State, verb rune)
}
```

```go
type Point struct{ X, Y int }

func (p Point) Format(f fmt.State, verb rune) {
    switch verb {
    case 'v':
        if f.Flag('#') {
            // %#v——Go 语法表示
            fmt.Fprintf(f, "Point{X: %d, Y: %d}", p.X, p.Y)
            return
        }
        if f.Flag('+') {
            // %+v——带字段名的详细格式
            fmt.Fprintf(f, "X:%d Y:%d", p.X, p.Y)
            return
        }
        // %v——默认
        fmt.Fprintf(f, "(%d, %d)", p.X, p.Y)
    case 's':
        fmt.Fprintf(f, "(%d, %d)", p.X, p.Y)
    case 'q':
        fmt.Fprintf(f, "%q", p.String())
    default:
        fmt.Fprintf(f, "%%!%c(Point=%d,%d)", verb, p.X, p.Y)
    }
}
```

### `fmt.State` 方法

| 方法 | 返回值 |
|------|--------|
| `Flag(c int) bool` | 标志（`+`、`-`、`#`、`0`、` `）是否设置 |
| `Width() (int, bool)` | 宽度值以及是否指定了宽度 |
| `Precision() (int, bool)` | 精度值以及是否指定了精度 |
| `Write(b []byte) (int, error)` | 写入输出字节 |

仅在 `String()` 不够用时才实现 `fmt.Formatter`——很少需要这样做。常见原因：需要为 `%v`、`%+v`、`%#v` 提供不同输出，或者需要遵循宽度/精度标志。

---

## 无限递归陷阱

**在 `String()` 方法内部对接收者使用 `%s` 或 `%v` 调用 `fmt.Sprintf` 会导致无限递归：**

```go
type MyString string

// BUG：无限递归——Sprintf 调用 String()，String() 又调用 Sprintf...
func (m MyString) String() string {
    return fmt.Sprintf("MyString: %s", m)  // 崩溃：栈溢出
}
```

修复方法——将接收者转换为其底层类型以打破方法集：

```go
func (m MyString) String() string {
    return fmt.Sprintf("MyString: %s", string(m))  // 安全：string 没有 String()
}
```

此陷阱还适用于：
- 底层类型为 string、[]byte 或另一个 Stringer 的类型
- 任何使用 `%s` 或 `%v` 格式化 `self` 的 `String()` 方法
- 使用 `%#v` 格式化 `self` 的 `GoString()` 方法

```go
type IPAddr [4]byte

// BUG：%v 调用 String()，无限递归
func (ip IPAddr) String() string {
    return fmt.Sprintf("%v.%v.%v.%v", ip[0], ip[1], ip[2], ip[3])
    // 这里安全——ip[0] 是 byte（uint8），没有 String() 方法。
    // 但如果 ip 是一个包装了 Stringer 的命名类型，就会递归。
}
```

**经验法则**：在 `String()` 内部，永远不要将接收者（或重新转换为自身类型的接收者）传递给 `%s` 或 `%v` 动词。先转换为底层原始类型。

---

## 快速参考

| 主题 | 规则 |
|------|------|
| `%q` | 用于人类可读的字符串输出 |
| `%+v` | 带字段名的结构体 |
| `%#v` | Go 语法表示；通过 `GoStringer` 自定义 |
| 格式字符串存储 | 在 Printf 调用之外声明为 `const` |
| Printf 函数名 | 以 `f` 结尾以支持 `go vet` |
| `Stringer` | 实现 `String() string` 用于 `%v`/`%s` 输出 |
| `GoStringer` | 实现 `GoString() string` 用于 `%#v` 输出 |
| `Formatter` | 实现 `Format(fmt.State, rune)` 以完全控制动词 |
| 递归陷阱 | 永远不要在 `String()` 内部使用 `Sprintf("%s", receiver)`；转换为底层类型 |
