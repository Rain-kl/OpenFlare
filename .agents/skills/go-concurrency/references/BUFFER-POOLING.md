# 使用 Channel 的缓冲池

使用有缓冲 channel 作为空闲列表来复用已分配的缓冲区，避免重复分配。这种"泄漏缓冲"模式使用带 `default` 的 `select` 进行非阻塞操作。

> **来源**：Effective Go

```go
var freeList = make(chan *Buffer, 100) // 有缓冲 channel 作为空闲列表

// 客户端：从空闲列表获取缓冲区或分配新的
func getBuffer() *Buffer {
    select {
    case b := <-freeList:
        return b // 复用已有缓冲区
    default:
        return new(Buffer) // 空闲列表为空；分配新缓冲区
    }
}

// 服务端：如有空间则将缓冲区归还空闲列表，否则丢弃
func putBuffer(b *Buffer) {
    b.Reset() // 为重用做准备
    select {
    case freeList <- b:
        // 缓冲区已归还空闲列表
    default:
        // 空闲列表已满；丢弃缓冲区（GC 会回收）
    }
}
```

## 工作原理

1. **非阻塞接收**：客户端尝试从 `freeList` 获取缓冲区。如果为空，`default` 分支运行并分配新缓冲区。
2. **非阻塞发送**：服务端尝试归还缓冲区。如果 `freeList` 已满，`default` 分支运行，缓冲区被丢弃等待垃圾回收。
3. **有限内存**：channel 容量（100）限制了池中缓冲区的数量，防止无限增长。

当分配成本较高且缓冲区复用有益，但你不希望在池空或池满时出现阻塞行为时，这种模式非常有用。

## 何时使用

- 高频分配相似大小的对象
- 分配开销影响性能的代码路径
- 需要限制内存使用量的场景

## 生产环境替代方案

对于生产代码，考虑使用 `sync.Pool`，它提供类似功能并与垃圾收集器有更好的集成：

```go
var bufferPool = sync.Pool{
    New: func() any {
        return new(Buffer)
    },
}

func getBuffer() *Buffer {
    return bufferPool.Get().(*Buffer)
}

func putBuffer(b *Buffer) {
    b.Reset()
    bufferPool.Put(b)
}
```

`sync.Pool` 的优势：
- 垃圾收集期间自动清理
- 无需管理池大小
- 天生线程安全
- 高并发下性能更好

基于 channel 的方式对于理解 Go 的并发原语以及需要更多控制池行为的场景仍然很有价值。
