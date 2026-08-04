# 基准测试方法

## 编写基准测试

Go 基准测试使用 `testing.B` 类型，位于 `_test.go` 文件中。
基准测试函数名必须以 `Benchmark` 开头。

```go
func BenchmarkStrconv(b *testing.B) {
    for i := 0; i < b.N; i++ {
        s := strconv.Itoa(rand.Int())
        _ = s
    }
}

func BenchmarkFmtSprint(b *testing.B) {
    for i := 0; i < b.N; i++ {
        s := fmt.Sprint(rand.Int())
        _ = s
    }
}
```

关键规则：
- 使用 `b.N` 作为循环边界——框架会调整它以获得稳定的计时
- 将结果赋值给变量（或 `_`），防止编译器优化掉调用
- 在不需要测量的昂贵设置之后使用 `b.ResetTimer()`
- 使用 `b.ReportAllocs()` 或 `-benchmem` 标志跟踪分配情况

### 子基准测试

```go
func BenchmarkConvert(b *testing.B) {
    for _, size := range []int{10, 100, 1000} {
        b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
            data := make([]byte, size)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                _ = string(data)
            }
        })
    }
}
```

---

## 运行基准测试

```bash
# 运行包中的所有基准测试
go test -bench=. ./...

# 运行特定基准测试并显示内存统计
go test -bench=BenchmarkStrconv -benchmem ./...

# 多次运行以获得统计显著性
go test -bench=. -benchmem -count=10 ./...
```

`-benchmem` 标志报告每次操作的分配次数。`-count` 标志将每个基准测试运行 N 次以获得统计显著性。

---

## 解读结果

```
BenchmarkStrconv-8     18705042    64.2 ns/op    16 B/op    1 allocs/op
BenchmarkFmtSprint-8    8249536   143.0 ns/op    16 B/op    2 allocs/op
```

| 字段 | 含义 |
|------|------|
| `-8` | GOMAXPROCS |
| `18705042` | 迭代次数 |
| `64.2 ns/op` | 每次操作时间 |
| `16 B/op` | 每次操作分配的字节数 |
| `1 allocs/op` | 每次操作的堆分配次数 |

---

## 使用 benchstat 进行比较

`benchstat` 对基准测试结果进行统计比较。安装它并将基准测试输出保存到文件：

```bash
# 安装 benchstat
go install golang.org/x/perf/cmd/benchstat@latest

# 运行基准测试并保存结果
go test -bench=. -benchmem -count=10 ./... > old.txt

# 进行修改后再次运行
go test -bench=. -benchmem -count=10 ./... > new.txt

# 比较结果
benchstat old.txt new.txt
```

### 解读 benchstat 输出

```
name          old time/op    new time/op    delta
Strconv-8     64.2ns ± 2%    61.8ns ± 1%   -3.74%  (p=0.001 n=10+10)
```

- **delta**：变化百分比（负数 = 更快）
- **p-value**：统计显著性（p < 0.05 为显著）
- **n**：使用的有效样本数量

提示：
- 始终使用 `-count=10` 或更高以获得可靠结果
- 小的 p 值确认变化是真实的，而非噪声
- 如果 benchstat 显示 `~`（波浪号），则差异不具有统计显著性

---

## 来自性能模式的基准测试示例

### strconv vs fmt

| 方式 | 速度 | 分配次数 |
|------|------|---------|
| `fmt.Sprint` | 143 ns/op | 2 allocs/op |
| `strconv.Itoa` | 64.2 ns/op | 1 allocs/op |

### 重复字节转换

```go
func BenchmarkRepeatedConversion(b *testing.B) {
    var buf bytes.Buffer
    for i := 0; i < b.N; i++ {
        buf.Write([]byte("Hello world"))
    }
}

func BenchmarkSingleConversion(b *testing.B) {
    var buf bytes.Buffer
    data := []byte("Hello world")
    for i := 0; i < b.N; i++ {
        buf.Write(data)
    }
}
```

| 方式 | 速度 |
|------|------|
| 重复转换 | 22.2 ns/op |
| 单次转换 | 3.25 ns/op |

### Slice 容量

```go
func BenchmarkNoCapacity(b *testing.B) {
    for n := 0; n < b.N; n++ {
        data := make([]int, 0)
        for k := 0; k < 1000; k++ {
            data = append(data, k)
        }
    }
}

func BenchmarkWithCapacity(b *testing.B) {
    for n := 0; n < b.N; n++ {
        data := make([]int, 0, 1000)
        for k := 0; k < 1000; k++ {
            data = append(data, k)
        }
    }
}
```

| 方式 | 时间（1 亿次迭代） |
|------|------------------------|
| 无容量 | 2.48s |
| 指定容量 | 0.21s |

---

## 使用 pprof 进行性能分析

使用 `pprof` 在优化前识别瓶颈。基准测试衡量改进效果；pprof 找到需要改进的地方。

### CPU 性能分析

```bash
# 从基准测试生成 CPU 分析文件
go test -bench=BenchmarkHotPath -cpuprofile=cpu.prof ./...

# 使用 pprof 分析
go tool pprof cpu.prof
```

常用 pprof 命令：

```
(pprof) top10          # 按 CPU 时间排列的前 10 个函数
(pprof) list funcName  # 某个函数的带注释源码
(pprof) web            # 浏览器中的交互式图表
```

### 内存性能分析

```bash
# 生成内存分析文件
go test -bench=BenchmarkHotPath -memprofile=mem.prof ./...

# 分析分配情况
go tool pprof -alloc_space mem.prof
```

### 运行中服务的 HTTP 性能分析

```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    // ... 应用程序代码 ...
}
```

通过 `http://localhost:6060/debug/pprof/` 访问性能分析数据。

### 性能分析工作流

1. 对疑似热点路径进行**基准测试**
2. 使用 pprof **分析**以确认时间花在了哪里
3. 使用本技能中的模式进行**优化**
4. **重新基准测试**以用 benchstat 验证改进
5. **重新分析**以检查是否出现新的瓶颈

---

## 常见错误

### 忽略 b.N

测试框架会调整 `b.N` 以获得稳定的计时。使用固定迭代次数会产生无意义的结果：

```go
// 不好：忽略 b.N —— 基准测试框架无法校准
func BenchmarkFixed(b *testing.B) {
    for i := 0; i < 1000; i++ {
        doWork()
    }
}

// 好：使用 b.N 作为循环边界
func BenchmarkCorrect(b *testing.B) {
    for i := 0; i < b.N; i++ {
        doWork()
    }
}
```

### 未防止编译器优化消除

如果函数调用的结果未被使用，编译器可能会完全优化掉该调用。将结果赋值给包级变量：

```go
// 不好：编译器可能会优化掉调用
func BenchmarkElided(b *testing.B) {
    for i := 0; i < b.N; i++ {
        expensiveFunc()
    }
}

// 好：赋值给包级变量以防止优化消除
var benchResult int

func BenchmarkKept(b *testing.B) {
    var r int
    for i := 0; i < b.N; i++ {
        r = expensiveFunc()
    }
    benchResult = r
}
```
