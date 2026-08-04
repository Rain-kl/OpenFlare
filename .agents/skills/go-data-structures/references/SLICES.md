# Go 切片内部原理

> **来源**：Effective Go

---

## 三项描述符

切片是一个运行时数据结构，包含三个组件：

- **指针**：第一个可访问元素的地址
- **长度**：元素数量（`len(s)`）
- **容量**：到底层数组末尾的最大元素数（`cap(s)`）

```go
arr := [5]int{10, 20, 30, 40, 50}
s := arr[1:4]  // s = [20, 30, 40]
// 指针：&arr[1]，长度：3，容量：4
```

`nil` 切片的三项均为零/nil。

---

## 切片引用底层数组

切片不存储数据 — 它们描述数组的一部分：

```go
data := [4]int{1, 2, 3, 4}
a := data[0:2]  // [1, 2]
b := data[1:3]  // [2, 3]

b[0] = 99
fmt.Println(a)    // [1, 99] - 两者都看到变化
fmt.Println(data) // [1, 99, 3, 4]
```

---

## 切片运算符

`s[lo:hi]` 创建从索引 `lo` 到 `hi-1` 的切片：

```go
s := []int{0, 1, 2, 3, 4, 5}
s[2:4]   // [2, 3]
s[:3]    // [0, 1, 2]
s[3:]    // [3, 4, 5]
```

三索引形式 `s[lo:hi:max]` 将容量限制为 `max-lo`。

---

## 为什么 append 必须返回切片

切片头是**按值传递**的。函数可以修改元素但无法改变调用者的切片头：

```go
func Append(slice, data []byte) []byte {
    l := len(slice)
    if l+len(data) > cap(slice) {
        newSlice := make([]byte, (l+len(data))*2)
        copy(newSlice, slice)
        slice = newSlice  // 只改变局部变量
    }
    slice = slice[0 : l+len(data)]
    copy(slice[l:], data)
    return slice  // 调用者必须接收新的切片头
}
```

当发生重新分配时，`slice` 指向新数组。调用者的原始引用仍指向旧数组 — 返回使调用者能够更新其引用。

---

## copy 函数

`copy(dst, src)` 复制元素并返回复制的数量：

```go
src := []int{1, 2, 3, 4, 5}
dst := make([]int, 3)
n := copy(dst, src)  // n=3, dst=[1,2,3]
```

正确处理重叠切片。复制 `min(len(dst), len(src))` 个元素 — 不会发生重新分配。

---

## 切片常见陷阱

### 1. 共享底层数组

```go
original := []int{1, 2, 3, 4, 5}
subset := original[1:3]
subset[0] = 99
fmt.Println(original)  // [1, 99, 3, 4, 5] - 被修改了！

// 修复：创建独立副本
subset := make([]int, 2)
copy(subset, original[1:3])
```

### 2. append 可能重新分配也可能不

```go
a := make([]int, 3, 5)  // len=3, cap=5
b := a[0:3]
a = append(a, 4)    // 在容量内 - 仍然共享
a = append(a, 5, 6) // 超出容量 - 现在独立
```

### 3. 大底层数组导致内存泄漏

```go
// 不好：小切片将整个文件保留在内存中
func getHeader(file []byte) []byte { return file[:100] }

// 好：复制以释放大数组
func getHeader(file []byte) []byte {
    header := make([]byte, 100)
    copy(header, file)
    return header
}
```

### 4. nil vs 空切片

```go
var nilSlice []int     // nil, len=0, cap=0
emptySlice := []int{}  // 非 nil, len=0, cap=0
// 两者在 len、cap、append、range 中表现相同
// 未初始化状态优先使用 nil
```

## 快速参考

| 操作 | 行为 |
|------|------|
| `s[lo:hi]` | 从 lo 到 hi-1 的切片 |
| `s[lo:hi:max]` | 容量限制为 max-lo 的切片 |
| `append(s, x...)` | 返回新切片；可能重新分配 |
| `copy(dst, src)` | 返回复制数量；不重新分配 |
