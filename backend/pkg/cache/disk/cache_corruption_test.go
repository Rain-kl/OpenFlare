// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package disk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 这些用例锁住「LRU 链表节点被污染时不得 panic」的行为：一旦 items 与 evictList
// 的不变量被破坏（例如后续改动误写节点），缓存必须降级为未命中/跳过，
// 而不是在读、写、删除与淘汰路径上崩掉整个进程。

// corruptEntry 写入一个键后把其链表节点值换成非法类型，返回缓存。
func corruptEntry(t *testing.T, key string) *Cache {
	t.Helper()

	c := New(t.TempDir())
	require.NoError(t, c.Set(key, []byte("payload"), time.Minute))

	elem, ok := c.items[key]
	require.True(t, ok, "entry must be tracked after Set")
	elem.Value = "not-a-cacheItem"
	return c
}

func TestGetToleratesCorruptEvictEntry(t *testing.T) {
	c := corruptEntry(t, "k")

	got, err := c.Get("k")
	require.ErrorIs(t, err, ErrCacheMiss)
	require.Nil(t, got)
}

func TestSetOverCorruptEvictEntryReportsError(t *testing.T) {
	c := corruptEntry(t, "k")

	err := c.Set("k", []byte("second"), time.Minute)
	require.Error(t, err, "Set must report the corrupted tracker entry instead of panicking")
	require.Contains(t, err.Error(), "invalid type")
}

func TestDeleteToleratesCorruptEvictEntry(t *testing.T) {
	c := corruptEntry(t, "k")

	require.NotPanics(t, func() { _ = c.Delete("k") })
	require.NotContains(t, c.items, "k")
}

func TestEvictToleratesCorruptEvictEntry(t *testing.T) {
	c := corruptEntry(t, "k")
	// 让任意写入都触发淘汰扫描：扫到被污染的节点必须跳过而非 panic。
	c.UpdatePolicy(0, 0, true)

	require.NotPanics(t, func() {
		for i := range 4 {
			_ = c.Set(string(rune('a'+i)), []byte("x"), time.Minute)
		}
	})
}
