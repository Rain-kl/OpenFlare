# 时间、结构体标签和嵌入模式

## 使用 time.Time 和 time.Duration

始终使用 `time` 包。避免使用原始 `int` 表示时间值。

### 时间点

**不好**
```go
func isActive(now, start, stop int) bool {
  return start <= now && now < stop
}
```

**好**
```go
func isActive(now, start, stop time.Time) bool {
  return (start.Before(now) || start.Equal(now)) && now.Before(stop)
}
```

### 时长

**不好**
```go
func poll(delay int) {
  time.Sleep(time.Duration(delay) * time.Millisecond)
}
poll(10)  // 秒？毫秒？
```

**好**
```go
func poll(delay time.Duration) {
  time.Sleep(delay)
}
poll(10 * time.Second)
```

### JSON 字段

当无法使用 `time.Duration` 时，在字段名中包含单位：

**不好**
```go
type Config struct {
  Interval int `json:"interval"`
}
```

**好**
```go
type Config struct {
  IntervalMillis int `json:"intervalMillis"`
}
```

## 避免在公共结构体中嵌入类型

嵌入类型会泄露实现细节并阻碍类型演进。

**不好**
```go
type ConcreteList struct {
  *AbstractList
}
```

**好**
```go
type ConcreteList struct {
  list *AbstractList
}

func (l *ConcreteList) Add(e Entity) {
  l.list.Add(e)
}

func (l *ConcreteList) Remove(e Entity) {
  l.list.Remove(e)
}
```

嵌入的问题：
- 向嵌入接口添加方法是破坏性变更
- 从嵌入结构体移除方法是破坏性变更
- 替换嵌入类型是破坏性变更

## 在序列化结构体中使用字段标签

始终为 JSON、YAML 等使用显式字段标签。

**不好**
```go
type Stock struct {
  Price int
  Name  string
}
```

**好**
```go
type Stock struct {
  Price int    `json:"price"`
  Name  string `json:"name"`
  // 可以安全地将 Name 重命名为 Symbol
}
```

标签使序列化契约显式化，并可以安全地进行重构。
