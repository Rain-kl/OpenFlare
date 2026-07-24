// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package batchwriter provides a reusable buffered batch writer for high-throughput
// append-only sinks such as ClickHouse. Each business domain should own an independent
// Writer instance with its own queue, flush callback, and tuning parameters.
package batchwriter

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// FlushFunc persists a batch of queued items. It is invoked from the worker goroutine.
type FlushFunc[T any] func(ctx context.Context, items []T) error

// FlushErrorHandler is called when FlushFunc returns an error after optional retries.
// The batch is discarded after the handler returns; the worker continues processing.
// Handlers receive the failed items so callers can release dedup keys or re-queue.
type FlushErrorHandler[T any] func(ctx context.Context, items []T, err error)

// Stats is a point-in-time snapshot of Writer queue and failure counters.
type Stats struct {
	Name        string `json:"name"`
	Depth       int    `json:"depth"`
	Cap         int    `json:"cap"`
	Drops       int64  `json:"drops"`
	FlushErrors int64  `json:"flush_errors"`
	Running     bool   `json:"running"`
}

// Writer buffers items and flushes them by size or interval.
type Writer[T any] struct {
	cfg   Config
	flush FlushFunc[T]

	onFlushError FlushErrorHandler[T]
	onDrop       func(T)

	startOnce sync.Once
	stopOnce  sync.Once

	mu        sync.RWMutex
	ch        chan T
	workerCtx context.Context
	done      chan struct{}

	drops       atomic.Int64
	flushErrors atomic.Int64
}

// Option configures optional Writer callbacks.
type Option[T any] func(*Writer[T])

// WithFlushErrorHandler registers a callback for flush failures.
func WithFlushErrorHandler[T any](handler FlushErrorHandler[T]) Option[T] {
	return func(w *Writer[T]) {
		w.onFlushError = handler
	}
}

// WithDropHandler registers a callback when TryEnqueue cannot accept an item.
func WithDropHandler[T any](handler func(T)) Option[T] {
	return func(w *Writer[T]) {
		w.onDrop = handler
	}
}

// New creates a Writer. Call Start before enqueueing items.
func New[T any](cfg Config, flush FlushFunc[T], opts ...Option[T]) (*Writer[T], error) {
	if flush == nil {
		return nil, errNilFlushFunc
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	w := &Writer[T]{
		cfg:   cfg,
		flush: flush,
		done:  make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w, nil
}

// Start launches the background worker. It is safe to call at most once.
func (w *Writer[T]) Start(parent context.Context) {
	w.startOnce.Do(func() {
		w.mu.Lock()
		defer w.mu.Unlock()

		w.ch = make(chan T, w.cfg.QueueSize)
		w.workerCtx = context.WithoutCancel(parent)
		go w.run()
	})
}

// Stop closes the queue and waits until the worker drains pending items and exits.
func (w *Writer[T]) Stop(ctx context.Context) error {
	w.mu.RLock()
	ch := w.ch
	done := w.done
	w.mu.RUnlock()

	if ch == nil {
		return nil
	}

	w.stopOnce.Do(func() {
		close(ch)
	})

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Running reports whether Start has been called and Stop has not completed.
func (w *Writer[T]) Running() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.ch == nil {
		return false
	}
	select {
	case <-w.done:
		return false
	default:
		return true
	}
}

// TryEnqueue adds one item without blocking. It returns false when the writer is not
// running or the queue is full.
func (w *Writer[T]) TryEnqueue(item T) bool {
	w.mu.RLock()
	ch := w.ch
	w.mu.RUnlock()
	if ch == nil {
		w.notifyDrop(item)
		return false
	}

	select {
	case ch <- item:
		return true
	default:
		w.notifyDrop(item)
		return false
	}
}

// IsFull reports whether the queue has no remaining capacity.
func (w *Writer[T]) IsFull() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.ch == nil {
		return false
	}
	return len(w.ch) >= cap(w.ch)
}

// Len returns the current queue depth.
func (w *Writer[T]) Len() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.ch == nil {
		return 0
	}
	return len(w.ch)
}

// Cap returns the queue capacity.
func (w *Writer[T]) Cap() int {
	return w.cfg.QueueSize
}

// Stats returns a point-in-time snapshot of queue depth and failure counters.
func (w *Writer[T]) Stats() Stats {
	return Stats{
		Name:        w.cfg.Name,
		Depth:       w.Len(),
		Cap:         w.Cap(),
		Drops:       w.drops.Load(),
		FlushErrors: w.flushErrors.Load(),
		Running:     w.Running(),
	}
}

func (w *Writer[T]) run() {
	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]T, 0, w.cfg.MaxBatchSize)
	var batchStartedAt time.Time
	flush := func() {
		if len(batch) == 0 {
			return
		}
		items := append([]T(nil), batch...)
		if err := w.flush(w.workerCtx, items); err != nil {
			w.flushErrors.Add(1)
			if w.onFlushError != nil {
				w.onFlushError(w.workerCtx, items, err)
			}
		}
		batch = batch[:0]
		batchStartedAt = time.Time{}
	}

	defer func() {
		flush()
		close(w.done)
	}()

	for {
		select {
		case item, ok := <-w.ch:
			if !ok {
				return
			}
			if len(batch) == 0 {
				batchStartedAt = time.Now()
			}
			batch = append(batch, item)
			if len(batch) >= w.cfg.MaxBatchSize {
				flush()
			}
		case <-ticker.C:
			if w.shouldFlushOnInterval(len(batch), batchStartedAt, time.Now()) {
				flush()
			}
		}
	}
}

func (w *Writer[T]) shouldFlushOnInterval(batchLen int, batchStartedAt time.Time, now time.Time) bool {
	if batchLen == 0 {
		return false
	}
	if w.cfg.MinBatchSize == 0 || batchLen >= w.cfg.MinBatchSize {
		return true
	}
	if w.cfg.MaxFlushWait <= 0 || batchStartedAt.IsZero() {
		return false
	}
	return !now.Before(batchStartedAt.Add(w.cfg.MaxFlushWait))
}

func (w *Writer[T]) notifyDrop(item T) {
	w.drops.Add(1)
	if w.onDrop == nil {
		return
	}
	w.onDrop(item)
}
