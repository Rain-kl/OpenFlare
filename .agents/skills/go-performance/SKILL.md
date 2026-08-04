---
name: go-performance
description: Use when optimizing Go code, investigating slow performance, or writing performance-critical sections. Also use when a user mentions slow Go code, string concatenation in loops, or asks about benchmarking, even if the user doesn't explicitly mention performance patterns. Does not cover concurrent performance patterns (see go-concurrency).
license: Apache-2.0
metadata:
  sources: "Uber Style Guide, Google Style Guide, Go Wiki CodeReviewComments"
allowed-tools: Bash(bash:*)
---

# Go 性能模式

## 可用脚本

- **`scripts/bench-compare.sh`** — 运行 Go 基准测试 N 次，并可选通过 benchstat 进行基线比较。支持保存结果以供未来比较。运行 `bash scripts/bench-compare.sh --help` 查看选项。

性能特定的指南仅适用于**热点路径**。不要过早优化——将这些模式集中在最重要的地方。

---

## 优先使用 strconv 而非 fmt

在基本类型和字符串之间转换时，`strconv` 比 `fmt` 更快：

```go
s := strconv.Itoa(rand.Int()) // 比 fmt.Sprint() 快约 2 倍
```

| 方式 | 速度 | 分配次数 |
|------|------|---------|
| `fmt.Sprint` | 143 ns/op | 2 allocs/op |
| `strconv.Itoa` | 64.2 ns/op | 1 allocs/op |

> 在 strconv 和 fmt 之间选择类型转换方式时，或需要完整的转换对照表时，阅读 [references/STRING-OPTIMIZATION.md](references/STRING-OPTIMIZATION.md)。

---

## 避免重复的字符串到字节转换

将固定字符串在循环外转换为 `[]byte` 一次：

```go
data := []byte("Hello world")
for i := 0; i < b.N; i++ {
    w.Write(data) // 比每次迭代 []byte("...") 快约 7 倍
}
```

> 在优化热点循环中的重复字节转换时，阅读 [references/STRING-OPTIMIZATION.md](references/STRING-OPTIMIZATION.md)。

---

## 优先指定容器容量

尽可能指定容器容量，以便预先分配内存。这可以最大程度减少后续添加元素时因复制和调整大小而产生的分配。

### Map 容量提示

使用 `make()` 初始化 map 时提供容量提示：

```go
m := make(map[string]os.DirEntry, len(files))
```

**注意**：与 slice 不同，map 的容量提示不保证完整的预分配——它只是近似计算所需的哈希桶数量。

### Slice 容量

使用 `make()` 初始化 slice 时提供容量提示，特别是在追加时：

```go
data := make([]int, 0, size)
```

与 map 不同，slice 容量**不是提示**——编译器会精确分配那么多内存。后续的 `append()` 操作在达到容量之前不会产生任何分配。

| 方式 | 时间（1 亿次迭代） |
|------|------------------------|
| 无容量 | 2.48s |
| 指定容量 | 0.21s |

指定容量的版本**快约 12 倍**，因为追加期间零重新分配。

---

## 传值

不要仅为了节省几个字节就将指针作为函数参数传递。如果函数在整个函数体中仅通过 `*x` 引用其参数 `x`，则该参数不应该是`指针。

```go
func process(s string) { // 不是 *string —— string 是小的固定大小头部
    fmt.Println(s)
}
```

**常见的按值传递类型**：`string`、`io.Reader`、小结构体。

**例外**：
- 复制代价高的大结构体
- 未来可能增长的小结构体

---

## 字符串拼接

根据复杂度选择正确的策略：

| 方法 | 最佳用途 |
|------|---------|
| `+` | 少量字符串，简单拼接 |
| `fmt.Sprintf` | 混合类型的格式化输出 |
| `strings.Builder` | 循环/逐段构建 |
| `strings.Join` | 连接 slice |
| 反引号字面量 | 常量多行文本 |

> 在选择字符串拼接策略、在循环中使用 strings.Builder 或在 fmt.Sprintf 和手动拼接之间做决定时，阅读 [references/STRING-OPTIMIZATION.md](references/STRING-OPTIMIZATION.md)。

---

## 基准测试和性能分析

在优化前后始终要进行测量。使用 Go 内置的基准测试框架和性能分析工具。

```bash
go test -bench=. -benchmem -count=10 ./...
```

> 在编写基准测试、使用 benchstat 比较结果、使用 pprof 进行性能分析或解读基准测试输出时，阅读 [references/BENCHMARKS.md](references/BENCHMARKS.md)。

> **验证**：在应用优化后，运行 `bash scripts/bench-compare.sh` 测量实际影响。只保留有可衡量改进的优化。

---

## 快速参考

| 模式 | 不好 | 好 | 改进 |
|------|-----|------|-------------|
| 整数转字符串 | `fmt.Sprint(n)` | `strconv.Itoa(n)` | 快约 2 倍 |
| 重复 `[]byte` | 循环中 `[]byte("str")` | 在循环外转换一次 | 快约 7 倍 |
| Map 初始化 | `make(map[K]V)` | `make(map[K]V, size)` | 更少分配 |
| Slice 初始化 | `make([]T, 0)` | `make([]T, 0, cap)` | 快约 12 倍 |
| 小型固定大小参数 | `*string`、`*io.Reader` | `string`、`io.Reader` | 无间接引用 |
| 简单字符串连接 | `s1 + " " + s2` | （已经很好） | 对少量字符串使用 `+` |
| 循环构建字符串 | 重复 `+=` | `strings.Builder` | O(n) vs O(n²) |

---

## 相关技能

- **数据结构**：在 slice、map 和数组之间选择或理解分配语义时，参见 [go-data-structures](../go-data-structures/SKILL.md)
- **声明模式**：在使用 `make` 配合容量提示或初始化 map 和 slice 时，参见 [go-declarations](../go-declarations/SKILL.md)
- **并发**：在跨 goroutine 并行化工作或使用 sync.Pool 复用缓冲区时，参见 [go-concurrency](../go-concurrency/SKILL.md)
- **风格原则**：在判断优化是否值得牺牲可读性时，参见 [go-style-core](../go-style-core/SKILL.md)
