// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogRingBuffer_WriteAndQuery(t *testing.T) {
	rb := NewLogRingBuffer(5)

	// Write some logs
	_, _ = rb.Write([]byte("line1\nline2\nline3\n"))

	entries, hasMore := rb.Query(0, 10)
	assert.False(t, hasMore)
	assert.Equal(t, 3, len(entries))
	assert.Equal(t, "line1", entries[0].Data)
	assert.Equal(t, "line2", entries[1].Data)
	assert.Equal(t, "line3", entries[2].Data)
	assert.Equal(t, 0, entries[0].Index)
	assert.Equal(t, 1, entries[1].Index)
	assert.Equal(t, 2, entries[2].Index)
}

func TestLogRingBuffer_CapacityOverflow(t *testing.T) {
	rb := NewLogRingBuffer(3)

	_, _ = rb.Write([]byte("a\nb\nc\nd\ne\n"))

	entries, hasMore := rb.Query(0, 10)
	assert.False(t, hasMore)
	assert.Equal(t, 3, len(entries))
	assert.Equal(t, "c", entries[0].Data)
	assert.Equal(t, "d", entries[1].Data)
	assert.Equal(t, "e", entries[2].Data)
}

func TestLogRingBuffer_QueryLatest(t *testing.T) {
	rb := NewLogRingBuffer(10)

	_, _ = rb.Write([]byte("a\nb\nc\nd\ne\n"))

	// Query latest 2
	entries, hasMore := rb.Query(0, 2)
	assert.True(t, hasMore)
	assert.Equal(t, 2, len(entries))
	assert.Equal(t, "d", entries[0].Data)
	assert.Equal(t, "e", entries[1].Data)
}

func TestLogRingBuffer_QueryByCursor(t *testing.T) {
	rb := NewLogRingBuffer(10)

	_, _ = rb.Write([]byte("a\nb\nc\nd\ne\n"))

	// First get all to find indices
	all, _ := rb.Query(0, 10)
	assert.Equal(t, 5, len(all))

	// Query entries before index 3
	entries, hasMore := rb.Query(3, 10)
	assert.False(t, hasMore)
	assert.Equal(t, 3, len(entries))
	assert.Equal(t, "a", entries[0].Data)
	assert.Equal(t, "b", entries[1].Data)
	assert.Equal(t, "c", entries[2].Data)
}

func TestLogRingBuffer_QueryByCursorWithLimit(t *testing.T) {
	rb := NewLogRingBuffer(10)

	_, _ = rb.Write([]byte("a\nb\nc\nd\ne\n"))

	// Query 2 entries before index 4
	entries, hasMore := rb.Query(4, 2)
	assert.True(t, hasMore)
	assert.Equal(t, 2, len(entries))
	assert.Equal(t, "c", entries[0].Data)
	assert.Equal(t, "d", entries[1].Data)
}

func TestLogRingBuffer_QueryEmpty(t *testing.T) {
	rb := NewLogRingBuffer(5)

	entries, hasMore := rb.Query(0, 10)
	assert.False(t, hasMore)
	assert.Nil(t, entries)
}

func TestLogRingBuffer_QueryNonExistentCursor(t *testing.T) {
	rb := NewLogRingBuffer(5)
	_, _ = rb.Write([]byte("a\nb\n"))

	entries, hasMore := rb.Query(999, 10)
	assert.False(t, hasMore)
	assert.Equal(t, 2, len(entries))
	assert.Equal(t, "a", entries[0].Data)
	assert.Equal(t, "b", entries[1].Data)
}

func TestLogRingBuffer_Subscribe(t *testing.T) {
	rb := NewLogRingBuffer(5)

	ch := rb.Subscribe()
	defer rb.Unsubscribe(ch)

	_, _ = rb.Write([]byte("hello\n"))

	entry := <-ch
	assert.Equal(t, "hello", entry.Data)
	assert.Equal(t, 0, entry.Index)
}

func TestLogRingBuffer_SubscribeMultiple(t *testing.T) {
	rb := NewLogRingBuffer(5)

	ch1 := rb.Subscribe()
	defer rb.Unsubscribe(ch1)
	ch2 := rb.Subscribe()
	defer rb.Unsubscribe(ch2)

	_, _ = rb.Write([]byte("msg\n"))

	e1 := <-ch1
	e2 := <-ch2
	assert.Equal(t, "msg", e1.Data)
	assert.Equal(t, "msg", e2.Data)
}

func TestLogRingBuffer_WriteNoNewline(t *testing.T) {
	rb := NewLogRingBuffer(5)

	_, _ = rb.Write([]byte("partial"))

	entries, _ := rb.Query(0, 10)
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "partial", entries[0].Data)
}

func TestLogRingBuffer_WriteEmpty(t *testing.T) {
	rb := NewLogRingBuffer(5)

	n, err := rb.Write([]byte(""))
	assert.Equal(t, 0, n)
	assert.NoError(t, err)

	entries, _ := rb.Query(0, 10)
	assert.Nil(t, entries)
}

func TestLogRingBuffer_QueryAfterOverflow(t *testing.T) {
	rb := NewLogRingBuffer(3)

	_, _ = rb.Write([]byte("1\n2\n3\n4\n5\n6\n7\n"))

	entries, hasMore := rb.Query(0, 10)
	assert.False(t, hasMore)
	assert.Equal(t, 3, len(entries))
	assert.Equal(t, "5", entries[0].Data)
	assert.Equal(t, "6", entries[1].Data)
	assert.Equal(t, "7", entries[2].Data)

	// Query by cursor - index 4 is "5", so cursor=4 should return index < 4
	older, hasMore2 := rb.Query(4, 10)
	assert.False(t, hasMore2)
	assert.Nil(t, older)
}

func TestLogRingBuffer_NextCursor(t *testing.T) {
	rb := NewLogRingBuffer(10)

	_, _ = rb.Write([]byte("a\nb\nc\nd\ne\n"))

	// Query latest 2, should return next_cursor pointing to first returned entry
	entries, _ := rb.Query(0, 2)
	assert.Equal(t, 2, len(entries))
	// entries[0].Index = 3 ("d"), entries[1].Index = 4 ("e")
	assert.Equal(t, 3, entries[0].Index)

	// Now use that index as cursor to get older entries
	older, hasMore := rb.Query(entries[0].Index, 10)
	assert.False(t, hasMore)
	assert.Equal(t, 3, len(older))
	assert.Equal(t, "a", older[0].Data)
	assert.Equal(t, "b", older[1].Data)
	assert.Equal(t, "c", older[2].Data)
}
