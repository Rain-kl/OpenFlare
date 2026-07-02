// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package batchwriter provides a reusable buffered batch writer for high-throughput
// append-only sinks such as ClickHouse. Each business domain should own an independent
// Writer instance with its own queue, flush callback, and tuning parameters.
package batchwriter

import (
	"context"
	"sync"
	"time"
)

// FlushFunc persists a batch of queued items. It is invoked from the worker goroutine.
type FlushFunc[T any] func(ctx context.Context, items []T) error

// FlushErrorHandler is called when FlushFunc returns an error. The batch is discarded
// after the handler returns; the worker continues processing.
type FlushErrorHandler func(ctx context.Context, batchSize int, err error)

// Writer buffers items and flushes them by size or interval.
type Writer[T any] struct {
	cfg   Config
	flush FlushFunc[T]

	onFlushError FlushErrorHandler
	onDrop       func(T)

	startOnce sync.Once
	stopOnce  sync.Once

	mu        sync.RWMutex
	ch        chan T
	workerCtx context.Context
	done      chan struct{}
}

// Option configures optional Writer callbacks.
type Option[T any] func(*Writer[T])

// WithFlushErrorHandler registers a callback for flush failures.
func WithFlushErrorHandler[T any](handler FlushErrorHandler) Option[T] {
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

func (w *Writer[T]) run() {
	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]T, 0, w.cfg.MaxBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		items := append([]T(nil), batch...)
		if err := w.flush(w.workerCtx, items); err != nil {
			if w.onFlushError != nil {
				w.onFlushError(w.workerCtx, len(items), err)
			}
		}
		batch = batch[:0]
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
			batch = append(batch, item)
			if len(batch) >= w.cfg.MaxBatchSize {
				flush()
			}
		case <-ticker.C:
			if len(batch) > 0 && (w.cfg.MinBatchSize == 0 || len(batch) >= w.cfg.MinBatchSize) {
				flush()
			}
		}
	}
}

func (w *Writer[T]) notifyDrop(item T) {
	if w.onDrop == nil {
		return
	}
	w.onDrop(item)
}