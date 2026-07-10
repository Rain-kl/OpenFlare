// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package batchwriter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestNewRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := New[int](Config{}, func(context.Context, []int) error { return nil })
	if err == nil {
		t.Fatal("New() = nil, want validation error")
	}
}

func TestNewRejectsNilFlushFunc(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	_, err := New[int](cfg, nil)
	if !errors.Is(err, errNilFlushFunc) {
		t.Fatalf("New() error = %v, want %v", err, errNilFlushFunc)
	}
}

func TestWriterFlushesOnMaxBatchSize(t *testing.T) {
	t.Parallel()

	var (
		mu     sync.Mutex
		batches [][]int
	)
	cfg := DefaultConfig()
	cfg.MaxBatchSize = 3
	cfg.FlushInterval = time.Hour

	writer, err := New[int](cfg, func(_ context.Context, items []int) error {
		mu.Lock()
		defer mu.Unlock()
		batches = append(batches, append([]int(nil), items...))
		return nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := writer.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	for i := range 3 {
		if !writer.TryEnqueue(i + 1) {
			t.Fatalf("TryEnqueue(%d) = false, want true", i+1)
		}
	}

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		ready := len(batches) == 1
		mu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	got := batches
	mu.Unlock()

	want := [][]int{{1, 2, 3}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("flush batches mismatch (-want +got):\n%s", diff)
	}
}

func TestWriterFlushesOnInterval(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		batch []int
	)
	cfg := DefaultConfig()
	cfg.MaxBatchSize = 100
	cfg.MinBatchSize = 0
	cfg.FlushInterval = 20 * time.Millisecond

	writer, err := New[int](cfg, func(_ context.Context, items []int) error {
		mu.Lock()
		defer mu.Unlock()
		batch = append([]int(nil), items...)
		return nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := writer.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	if !writer.TryEnqueue(42) {
		t.Fatal("TryEnqueue() = false, want true")
	}

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		ready := len(batch) == 1
		mu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	got := batch
	mu.Unlock()

	want := []int{42}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("interval flush mismatch (-want +got):\n%s", diff)
	}
}

func TestWriterTryEnqueueDropsWhenFull(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.QueueSize = 1
	cfg.MaxBatchSize = 10
	cfg.FlushInterval = time.Hour

	var dropped int
	writer, err := New[int](cfg, func(context.Context, []int) error { return nil }, WithDropHandler[int](func(int) {
		dropped++
	}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = writer.Stop(stopCtx)
	})

	if !writer.TryEnqueue(1) {
		t.Fatal("TryEnqueue(1) = false, want true")
	}
	if writer.TryEnqueue(2) {
		t.Fatal("TryEnqueue(2) = true, want false")
	}
	if !writer.IsFull() {
		t.Fatal("IsFull() = false, want true")
	}
	if dropped != 1 {
		t.Fatalf("dropped = %d, want 1", dropped)
	}
}

func TestWriterStopDrainsQueuedItems(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxBatchSize = 10
	cfg.FlushInterval = time.Hour

	var flushed []int
	writer, err := New[int](cfg, func(_ context.Context, items []int) error {
		flushed = append(flushed, items...)
		return nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	writer.Start(context.Background())
	for i := range 2 {
		if !writer.TryEnqueue(i + 1) {
			t.Fatalf("TryEnqueue(%d) = false, want true", i+1)
		}
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := writer.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	want := []int{1, 2}
	if diff := cmp.Diff(want, flushed); diff != "" {
		t.Fatalf("Stop() drain mismatch (-want +got):\n%s", diff)
	}
	if writer.Running() {
		t.Fatal("Running() = true after Stop(), want false")
	}
}

func TestWriterSkipsIntervalFlushBelowMinBatchSize(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		batch []int
	)
	cfg := DefaultConfig()
	cfg.MaxBatchSize = 100
	cfg.MinBatchSize = 5
	cfg.FlushInterval = 20 * time.Millisecond

	writer, err := New[int](cfg, func(_ context.Context, items []int) error {
		mu.Lock()
		defer mu.Unlock()
		batch = append([]int(nil), items...)
		return nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := writer.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	for i := range 3 {
		if !writer.TryEnqueue(i + 1) {
			t.Fatalf("TryEnqueue(%d) = false, want true", i+1)
		}
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	got := batch
	mu.Unlock()

	if len(got) != 0 {
		t.Fatalf("interval flush with below-min batch = %v, want no flush", got)
	}
}

func TestWriterFlushesOnIntervalWhenMinBatchSizeReached(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		batch []int
	)
	cfg := DefaultConfig()
	cfg.MaxBatchSize = 100
	cfg.MinBatchSize = 3
	cfg.FlushInterval = 20 * time.Millisecond

	writer, err := New[int](cfg, func(_ context.Context, items []int) error {
		mu.Lock()
		defer mu.Unlock()
		batch = append([]int(nil), items...)
		return nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := writer.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	for i := range 3 {
		if !writer.TryEnqueue(i + 1) {
			t.Fatalf("TryEnqueue(%d) = false, want true", i+1)
		}
	}

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		ready := len(batch) == 3
		mu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	got := batch
	mu.Unlock()

	want := []int{1, 2, 3}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("interval flush at min batch size mismatch (-want +got):\n%s", diff)
	}
}

func TestWriterForcesFlushAfterMaxFlushWait(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		batch []int
	)
	cfg := DefaultConfig()
	cfg.MaxBatchSize = 100
	cfg.MinBatchSize = 50
	cfg.FlushInterval = 20 * time.Millisecond
	cfg.MaxFlushWait = 80 * time.Millisecond

	writer, err := New[int](cfg, func(_ context.Context, items []int) error {
		mu.Lock()
		defer mu.Unlock()
		batch = append([]int(nil), items...)
		return nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := writer.Stop(stopCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	if !writer.TryEnqueue(1) {
		t.Fatal("TryEnqueue(1) = false, want true")
	}

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		ready := len(batch) == 1
		mu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	got := batch
	mu.Unlock()
	if diff := cmp.Diff([]int{1}, got); diff != "" {
		t.Fatalf("max flush wait mismatch (-want +got):\n%s", diff)
	}
}

func TestWriterInvokesFlushErrorHandler(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxBatchSize = 1
	cfg.FlushInterval = time.Hour

	flushErr := errors.New("flush failed")
	var (
		mu        sync.Mutex
		errCount  int
		batchSize int
	)

	writer, err := New[int](cfg, func(context.Context, []int) error {
		return flushErr
	}, WithFlushErrorHandler[int](func(_ context.Context, size int, err error) {
		mu.Lock()
		defer mu.Unlock()
		errCount++
		batchSize = size
		if !errors.Is(err, flushErr) {
			t.Errorf("flush error = %v, want %v", err, flushErr)
		}
	}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = writer.Stop(stopCtx)
	})

	if !writer.TryEnqueue(7) {
		t.Fatal("TryEnqueue() = false, want true")
	}

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		ready := errCount == 1
		mu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	gotCount := errCount
	gotSize := batchSize
	mu.Unlock()

	if gotCount != 1 {
		t.Fatalf("flush error handler count = %d, want 1", gotCount)
	}
	if gotSize != 1 {
		t.Fatalf("flush error handler batch size = %d, want 1", gotSize)
	}
}