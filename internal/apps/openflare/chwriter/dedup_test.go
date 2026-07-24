// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package chwriter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/persistence/batchwriter"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func TestDedupSetMarkIfNew(t *testing.T) {
	t.Parallel()

	set := newDedupSet()
	if !set.markIfNew("node-a|1") {
		t.Fatal("markIfNew() = false, want true on first key")
	}
	if set.markIfNew("node-a|1") {
		t.Fatal("markIfNew() = true, want false on duplicate key")
	}
	if !set.markIfNew("node-b|1") {
		t.Fatal("markIfNew() = false, want true on different key")
	}
	if set.markIfNew("") {
		t.Fatal("markIfNew() = true, want false on empty key")
	}
}

func TestDedupSetUnmarkAllowsRetry(t *testing.T) {
	t.Parallel()

	set := newDedupSet()
	if !set.markIfNew("k") {
		t.Fatal("markIfNew() = false, want true")
	}
	set.unmark("k")
	if !set.markIfNew("k") {
		t.Fatal("markIfNew() after unmark = false, want true")
	}
}

func TestQueueWithDedupDoesNotMarkWhenEnqueueFails(t *testing.T) {
	t.Parallel()

	cfg := batchwriter.DefaultConfig()
	cfg.QueueSize = 1
	cfg.MaxBatchSize = 10
	cfg.FlushInterval = time.Hour

	// Block the worker so the queue stays full after one enqueue.
	block := make(chan struct{})
	writer, err := batchwriter.New[int](cfg, func(context.Context, []int) error {
		<-block
		return nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	writer.Start(context.Background())
	t.Cleanup(func() {
		close(block)
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = writer.Stop(stopCtx)
	})

	// Fill the channel buffer (and the worker's current receive slot may empty one).
	// Keep enqueueing until full so subsequent queueWithDedup fails.
	for i := 0; i < cfg.QueueSize+2; i++ {
		_ = writer.TryEnqueue(i)
		if writer.IsFull() {
			break
		}
	}
	if !writer.IsFull() {
		t.Fatal("writer not full after filling; cannot test enqueue failure path")
	}

	dedup := newDedupSet()
	queueWithDedup(writer, dedup, "dedup-key", 99)
	// Key must not remain marked after failed enqueue.
	if !dedup.markIfNew("dedup-key") {
		t.Fatal("dedup key still marked after failed enqueue; want unmark")
	}
}

func TestQueueWithDedupMarksOnlyOnSuccess(t *testing.T) {
	t.Parallel()

	cfg := batchwriter.DefaultConfig()
	cfg.MaxBatchSize = 100
	cfg.FlushInterval = time.Hour

	writer, err := batchwriter.New[int](cfg, func(context.Context, []int) error { return nil })
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = writer.Stop(stopCtx)
	})

	dedup := newDedupSet()
	queueWithDedup(writer, dedup, "ok-key", 1)
	if dedup.markIfNew("ok-key") {
		t.Fatal("markIfNew() = true after successful enqueue, want false (key marked)")
	}
}

func TestFlushErrorHandlerUnmarksKeys(t *testing.T) {
	t.Parallel()

	dedup := newDedupSet()
	flushErr := errors.New("ch down")

	var (
		mu       sync.Mutex
		errCount int
	)

	cfg := batchwriter.Config{
		Name:          "test_obs",
		QueueSize:     10,
		MaxBatchSize:  1,
		FlushInterval: time.Hour,
	}
	keyFn := func(s analyticsmodel.NodeMetricSnapshot) string {
		return metricSnapshotKey(s)
	}
	writer, err := batchwriter.New(
		cfg,
		func(context.Context, []analyticsmodel.NodeMetricSnapshot) error { return flushErr },
		batchwriter.WithFlushErrorHandler[analyticsmodel.NodeMetricSnapshot](func(_ context.Context, items []analyticsmodel.NodeMetricSnapshot, err error) {
			mu.Lock()
			errCount++
			mu.Unlock()
			for _, item := range items {
				dedup.unmark(keyFn(item))
			}
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	writer.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = writer.Stop(stopCtx)
	})

	item := analyticsmodel.NodeMetricSnapshot{
		NodeID:     "n1",
		CapturedAt: time.Unix(1, 0).UTC(),
	}
	key := keyFn(item)
	if !dedup.markIfNew(key) {
		t.Fatal("markIfNew failed")
	}
	if !writer.TryEnqueue(item) {
		t.Fatal("TryEnqueue failed")
	}

	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		ready := errCount >= 1
		mu.Unlock()
		if ready || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if !dedup.markIfNew(key) {
		t.Fatal("key still marked after flush error unmark; want available for retry")
	}
}
