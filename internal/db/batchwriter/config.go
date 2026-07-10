// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package batchwriter

import (
	"fmt"
	"time"
)

const (
	defaultQueueSize     = 10_000
	defaultMaxBatchSize  = 1_000
	defaultMinBatchSize  = 50
	defaultFlushEvery    = time.Second
)

// Config controls queue capacity and flush thresholds for a Writer instance.
type Config struct {
	// Name identifies the writer in logs and diagnostics. Optional.
	Name string

	// QueueSize is the buffered channel capacity.
	QueueSize int

	// MaxBatchSize triggers a flush when the in-memory batch reaches this count.
	MaxBatchSize int

	// MinBatchSize is the minimum in-memory batch size for time-based flushes.
	// Zero disables the threshold and preserves legacy interval flush behavior.
	// When set, interval flushes below this size are skipped unless MaxFlushWait elapses.
	MinBatchSize int

	// FlushInterval is how often the worker checks whether a time-based flush should run.
	FlushInterval time.Duration

	// MaxFlushWait forces a flush of any non-empty batch once the oldest item has waited
	// this long, even if MinBatchSize has not been reached. Zero disables the force path.
	MaxFlushWait time.Duration
}

// DefaultConfig returns production-friendly defaults aligned with audit log batching.
func DefaultConfig() Config {
	return Config{
		QueueSize:     defaultQueueSize,
		MaxBatchSize:  defaultMaxBatchSize,
		MinBatchSize:  defaultMinBatchSize,
		FlushInterval: defaultFlushEvery,
	}
}

func (c Config) validate() error {
	if c.QueueSize <= 0 {
		return fmt.Errorf("batchwriter: queue size must be positive")
	}
	if c.MaxBatchSize <= 0 {
		return fmt.Errorf("batchwriter: max batch size must be positive")
	}
	if c.MinBatchSize < 0 {
		return fmt.Errorf("batchwriter: min batch size must be non-negative")
	}
	if c.FlushInterval <= 0 {
		return fmt.Errorf("batchwriter: flush interval must be positive")
	}
	if c.MaxFlushWait < 0 {
		return fmt.Errorf("batchwriter: max flush wait must be non-negative")
	}
	return nil
}