# 在 API 边界处复制切片和 Map

> **来源**：Uber 风格指南

切片和 map 包含对其底层数据的引用。在 API 边界处复制它们，以防止调用者修改内部状态（反之亦然）。

## 接收切片和 Map

当函数存储调用者传入的切片或 map 时，始终进行防御性复制。调用者保留原始引用，可以在函数返回后修改它。

### 切片

**不好**
```go
func (d *Driver) SetTrips(trips []Trip) {
  d.trips = trips  // 调用者仍然可以修改 d.trips
}
```

**好**
```go
func (d *Driver) SetTrips(trips []Trip) {
  d.trips = make([]Trip, len(trips))
  copy(d.trips, trips)
}
```

### Map

**不好**
```go
func (s *Server) SetConfig(cfg map[string]string) {
  s.config = cfg  // 调用者仍然可以修改 s.config
}
```

**好**
```go
func (s *Server) SetConfig(cfg map[string]string) {
  s.config = make(map[string]string, len(cfg))
  for k, v := range cfg {
    s.config[k] = v
  }
}
```

## 返回切片和 Map

返回内部切片或 map 时，返回副本以防止调用者修改你的内部状态。

### 返回 Map

**不好**
```go
func (s *Stats) Snapshot() map[string]int {
  s.mu.Lock()
  defer s.mu.Unlock()
  return s.counters  // 暴露了内部状态！
}
```

**好**
```go
func (s *Stats) Snapshot() map[string]int {
  s.mu.Lock()
  defer s.mu.Unlock()
  result := make(map[string]int, len(s.counters))
  for k, v := range s.counters {
    result[k] = v
  }
  return result
}
```

### 返回切片

**不好**
```go
func (q *Queue) Items() []Item {
  return q.items  // 调用者可以追加、修改或重新切片
}
```

**好**
```go
func (q *Queue) Items() []Item {
  result := make([]Item, len(q.items))
  copy(result, q.items)
  return result
}
```

## 何时不需要复制

防御性复制有开销。在以下情况下可以跳过：

- 数据**按约定是不可变的**，并且有清晰的文档说明
- 切片/map 是**为调用者新创建的**（不在内部存储）
- 性能分析表明复制在热路径中是瓶颈

如有疑问，就复制。与共享引用导致的 bug 相比，开销通常可以忽略不计。
