// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"io"
	"sync"
)

// LogEntry 日志条目，对应 ring buffer 中的一行日志
type LogEntry struct {
	Index int    `json:"index"` // 全局递增序号
	Data  string `json:"data"`  // 一行日志原文（含换行符）
}

// LogRingBuffer 固定容量的环形缓冲区，存储最近的日志行
// 支持：追加日志、按 cursor 分页查询、订阅实时推送
type LogRingBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	cap     int
	head    int // 下一条写入的位置
	count   int // 当前条目数
	seq     int // 全局递增序号

	subscribers map[chan LogEntry]struct{}
	subMu       sync.RWMutex
}

// NewLogRingBuffer 创建指定容量的日志环形缓冲区
func NewLogRingBuffer(capacity int) *LogRingBuffer {
	return &LogRingBuffer{
		entries:     make([]LogEntry, capacity),
		cap:         capacity,
		subscribers: make(map[chan LogEntry]struct{}),
	}
}

// Write 实现 io.Writer 接口，供 zapcore.WriteSyncer 调用
// 按 '\n' 分割为独立行写入 ring buffer
func (r *LogRingBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	data := string(p)
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line := data[start:i]
			start = i + 1
			if len(line) > 0 {
				r.appendLine(line)
			}
		}
	}
	// 处理最后一行（没有换行符结尾的情况）
	if start < len(data) && len(data[start:]) > 0 {
		r.appendLine(data[start:])
	}

	return len(p), nil
}

// Sync 实现 zapcore.WriteSyncer 接口
func (r *LogRingBuffer) Sync() error {
	return nil
}

// appendLine 追加一行日志到 ring buffer 并通知订阅者
func (r *LogRingBuffer) appendLine(line string) {
	r.mu.Lock()
	entry := LogEntry{
		Index: r.seq,
		Data:  line,
	}
	r.entries[r.head] = entry
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
	r.seq++
	r.mu.Unlock()

	// 异步通知订阅者
	r.subMu.RLock()
	for ch := range r.subscribers {
		select {
		case ch <- entry:
		default:
			// 订阅者消费太慢，丢弃（避免阻塞日志写入）
		}
	}
	r.subMu.RUnlock()
}

// Query 查询历史日志
// cursor=0 表示查询最新日志，cursor>0 表示查询 index < cursor 的更早日志
// limit 为返回条数上限
// 返回日志条目（按 index 升序）和是否有更早的日志
func (r *LogRingBuffer) Query(cursor int, limit int) ([]LogEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.count == 0 {
		return nil, false
	}

	// 计算 ring buffer 中有效条目的范围
	// oldest index in ring: head - count (wrapping)
	oldestPos := (r.head - r.count + r.cap) % r.cap

	// 将 ring buffer 中的有效条目按顺序收集
	ordered := make([]LogEntry, 0, r.count)
	for i := 0; i < r.count; i++ {
		pos := (oldestPos + i) % r.cap
		ordered = append(ordered, r.entries[pos])
	}

	if cursor == 0 {
		// 查询最新日志：返回最后 limit 条
		if len(ordered) <= limit {
			return ordered, false
		}
		return ordered[len(ordered)-limit:], true
	}

	// 查询 index < cursor 的更早日志
	// 找到 index < cursor 的条目
	var cut int
	for cut = len(ordered); cut > 0; cut-- {
		if ordered[cut-1].Index < cursor {
			break
		}
	}

	if cut == 0 {
		return nil, false
	}

	// 返回 cut 之前的最后 limit 条
	start := cut - limit
	if start < 0 {
		start = 0
	}

	hasMore := start > 0
	return ordered[start:cut], hasMore
}

// subscribeChanSize 订阅者 channel 缓冲区大小
const subscribeChanSize = 64

// Subscribe 订阅实时日志推送
// 返回一个 channel，调用者应 defer Unsubscribe
func (r *LogRingBuffer) Subscribe() chan LogEntry {
	ch := make(chan LogEntry, subscribeChanSize)
	r.subMu.Lock()
	r.subscribers[ch] = struct{}{}
	r.subMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (r *LogRingBuffer) Unsubscribe(ch chan LogEntry) {
	r.subMu.Lock()
	delete(r.subscribers, ch)
	r.subMu.Unlock()
	close(ch)
}

// 确保 LogRingBuffer 实现 io.Writer 接口
var _ io.Writer = (*LogRingBuffer)(nil)
