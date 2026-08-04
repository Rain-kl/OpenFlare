---
name: go-data-structures
description: Use when working with Go slices, maps, or arrays — choosing between new and make, using append, declaring empty slices (nil vs literal for JSON), implementing sets with maps, and copying data at boundaries. Also use when building or manipulating collections, even if the user doesn't ask about allocation idioms. Does not cover concurrent data structure safety (see go-concurrency).
license: Apache-2.0
metadata:
  sources: "Effective Go, Google Style Guide, Uber Style Guide, Go Wiki CodeReviewComments"
---

# Go 数据结构

---

## 选择数据结构

```
你需要什么？
├─ 有序的元素集合
│  ├─ 编译时已知固定大小 → 数组 [N]T
│  └─ 动态大小 → 切片 []T
│     ├─ 知道大概的大小？→ make([]T, 0, capacity)
│     └─ 未知大小或需要 nil 安全的 JSON？→ var s []T (nil)
├─ 键值查找
│  └─ 映射 map[K]V
│     ├─ 知道大概的大小？→ make(map[K]V, capacity)
│     └─ 需要集合？→ map[T]struct{}（零大小值）
└─ 需要传递给函数？
   └─ 如果调用者可能会修改它，则在边界处复制
```

> **此技能不适用的场景**：对于数据结构的并发访问（互斥锁、原子操作），参见 [go-concurrency](../go-concurrency/SKILL.md)。对于 API 边界处的防御性复制，参见 [go-defensive](../go-defensive/SKILL.md)。对于为性能预分配容量，参见 [go-performance](../go-performance/SKILL.md)。

---

## 切片

### append 函数

**始终赋值结果** — 底层数组可能会改变：

```go
x := []int{1, 2, 3}
x = append(x, 4, 5, 6)

// 将切片追加到切片
x = append(x, y...)  // 注意 ...
```

### 二维切片

**独立的内部切片**（可以独立增长/缩小）：

```go
picture := make([][]uint8, YSize)
for i := range picture {
    picture[i] = make([]uint8, XSize)
}
```

**单次分配**（对于固定大小更高效）：

```go
picture := make([][]uint8, YSize)
pixels := make([]uint8, XSize*YSize)
for i := range picture {
    picture[i], pixels = pixels[:XSize], pixels[XSize:]
}
```

> 在调试意外的切片行为、跨 goroutine 共享切片或处理切片头时，阅读 [references/SLICES.md](references/SLICES.md)。

### 声明空切片

优先使用 nil 切片而非空字面量：

```go
// 好：nil 切片
var t []string

// 避免：非 nil 但零长度
t := []string{}
```

两者的 `len` 和 `cap` 都是零，但 nil 切片是首选风格。

**JSON 例外**：nil 切片编码为 `null`，而 `[]string{}` 编码为 `[]`。当需要 JSON 数组时使用非 nil。

在设计接口时，避免区分 nil 和非 nil 的零长度切片。

---

## 映射

### 实现集合

使用 `map[T]bool` — 惯用且阅读自然：

```go
attended := map[string]bool{"Ann": true, "Joe": true}
if attended[person] {  // 不在映射中则为 false
    fmt.Println(person, "was at the meeting")
}
```

---

## 复制

从另一个包复制结构体时要小心。如果类型的方法定义在指针类型（`*T`）上，复制值可能导致别名 bug。

**通用规则：** 如果类型 `T` 的方法与指针类型 `*T` 关联，则不要复制 `T` 的值。这适用于 `bytes.Buffer`、`sync.Mutex`、`sync.WaitGroup` 以及包含它们的类型。

```go
// 不好：复制互斥锁
var mu sync.Mutex
mu2 := mu  // 几乎总是 bug

// 好：通过指针传递
func increment(sc *SafeCounter) {
    sc.mu.Lock()
    sc.count++
    sc.mu.Unlock()
}
```

---

## 快速参考

| 主题 | 关键点 |
|------|--------|
| 切片 | 始终赋值 `append` 结果；`nil` 切片优于 `[]T{}` |
| 集合 | `map[T]bool` 是惯用写法 |
| 复制 | 如果方法在 `*T` 上则不要复制 `T`；注意别名问题 |

## 相关技能

- **防御性复制**：在 API 边界处复制切片或映射以防止修改时，参见 [go-defensive](../go-defensive/SKILL.md)
- **容量提示**：为已知工作负载预分配切片或映射容量时，参见 [go-performance](../go-performance/SKILL.md)
- **迭代模式**：在对切片、映射或通道使用 range 循环时，参见 [go-control-flow](../go-control-flow/SKILL.md)
- **声明风格**：在 `new`、`make`、`var` 和复合字面量之间选择时，参见 [go-declarations](../go-declarations/SKILL.md)
