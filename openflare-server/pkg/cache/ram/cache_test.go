// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ram

import "testing"

func TestCacheSetGetInvalidate(t *testing.T) {
	cache := MustNew[string, int](Options{MaximumSize: 8})

	cache.Set("count", 3)

	got, ok := cache.GetIfPresent("count")
	if !ok {
		t.Fatal("GetIfPresent(count) ok = false, want true")
	}
	if got != 3 {
		t.Fatalf("GetIfPresent(count) = %d, want %d", got, 3)
	}

	cache.Invalidate("count")
	if _, ok := cache.GetIfPresent("count"); ok {
		t.Fatal("GetIfPresent(count) after Invalidate ok = true, want false")
	}
}

func TestCacheInvalidateAll(t *testing.T) {
	cache := MustNew[string, string](Options{MaximumSize: 8})

	cache.Set("a", "1")
	cache.Set("b", "2")

	cache.InvalidateAll()

	if cache.EstimatedSize() != 0 {
		t.Fatalf("EstimatedSize() after InvalidateAll = %d, want 0", cache.EstimatedSize())
	}
}
