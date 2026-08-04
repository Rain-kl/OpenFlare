# 字符串优化模式

## strconv vs fmt

在基本类型和字符串之间转换时，`strconv` 比 `fmt` 更快，因为 `fmt` 使用反射并处理任意类型。

**不好：**

```go
for i := 0; i < b.N; i++ {
    s := fmt.Sprint(rand.Int())
}
```

**好：**

```go
for i := 0; i < b.N; i++ {
    s := strconv.Itoa(rand.Int())
}
```

**基准测试比较：**

| 方式 | 速度 | 分配次数 |
|------|------|---------|
| `fmt.Sprint` | 143 ns/op | 2 allocs/op |
| `strconv.Itoa` | 64.2 ns/op | 1 allocs/op |

常用转换：

| 任务 | `fmt` | `strconv` |
|------|-------|-----------|
| Int → string | `fmt.Sprint(n)` | `strconv.Itoa(n)` |
| Int64 → string | `fmt.Sprint(n)` | `strconv.FormatInt(n, 10)` |
| Float → string | `fmt.Sprint(f)` | `strconv.FormatFloat(f, 'f', -1, 64)` |
| String → int | — | `strconv.Atoi(s)` |
| Bool → string | `fmt.Sprint(b)` | `strconv.FormatBool(b)` |

---

## 重复的字符串到字节转换

不要重复从固定字符串创建字节切片。应该只转换一次并保存结果。

**不好：**

```go
for i := 0; i < b.N; i++ {
    w.Write([]byte("Hello world"))
}
```

**好：**

```go
data := []byte("Hello world")
for i := 0; i < b.N; i++ {
    w.Write(data)
}
```

**基准测试比较：**

| 方式 | 速度 |
|------|------|
| 重复转换 | 22.2 ns/op |
| 单次转换 | 3.25 ns/op |

好的版本**快约 7 倍**，因为它避免了每次迭代都分配新的字节切片。

---

## 字符串拼接

根据复杂度选择正确的字符串构建策略。

### 简单场景使用 `+`

```go
key := "projectid: " + p
```

`+` 运算符对于少量、固定数量的字符串是高效的。编译器通常可以优化相邻的字符串字面量。

### 格式化使用 `fmt.Sprintf`

```go
// 好：清晰的格式化
str := fmt.Sprintf("%s [%s:%d]-> %s", src, qos, mtu, dst)

// 不好：使用 + 手动转换
str := src.String() + " [" + qos.String() + ":" + strconv.Itoa(mtu) + "]-> " + dst.String()
```

当写入 `io.Writer` 时，直接使用 `fmt.Fprintf` 而不是先用 `fmt.Sprintf` 构建临时字符串。

### 逐段构建使用 `strings.Builder`

`strings.Builder` 花费摊销线性时间，而重复使用 `+` 或
`fmt.Sprintf` 在构建大字符串时花费二次时间：

```go
b := new(strings.Builder)
for i, d := range digitsOfPi {
    fmt.Fprintf(b, "the %d digit of pi is: %d\n", i, d)
}
str := b.String()
```

### 常量多行字符串使用反引号

```go
// 好：原始字符串字面量
usage := `Usage:

custom_tool [args]`

// 不好：使用转义序列拼接
usage := "" +
    "Usage:\n" +
    "\n" +
    "custom_tool [args]"
```

### 策略总结

| 方法 | 最佳用途 | 性能 |
|------|---------|------|
| `+` | 少量字符串，简单拼接 | 小 n 时 O(n) |
| `fmt.Sprintf` | 格式化输出 | 较慢，但更清晰 |
| `strings.Builder` | 循环/逐段构建 | 摊销 O(n) |
| `strings.Join` | 连接 slice | O(n) |
| 反引号字面量 | 常量多行文本 | 零开销 |
