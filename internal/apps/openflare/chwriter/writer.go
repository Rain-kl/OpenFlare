// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package chwriter queues OpenFlare ClickHouse writes and flushes them through
// internal/db/batchwriter with per-table writer instances.
package chwriter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db/batchwriter"
	"github.com/Rain-kl/Wavelet/internal/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	// Observability traffic is sparse (heartbeat ~10s/node). Prefer larger batches to
	// cut ClickHouse parts/merges; MaxFlushWait bounds visibility lag for single-node labs.
	observabilityQueueSize    = 5_000
	observabilityMaxBatchSize = 500
	observabilityMinBatchSize = 20
	observabilityFlushEvery   = 10 * time.Second
	observabilityMaxFlushWait = 30 * time.Second

	nodeAccessLogQueueSize    = 10_000
	nodeAccessLogMaxBatchSize = 1_000
	nodeAccessLogMinBatchSize = 50
	nodeAccessLogFlushEvery   = 2 * time.Second
	nodeAccessLogMaxFlushWait = 5 * time.Second

	// flushAttempts is total tries (1 initial + short retries) before giving up a batch.
	flushAttempts     = 2
	flushRetryBackoff = 50 * time.Millisecond
)

var (
	initOnce sync.Once

	metricSnapshotWriter *batchwriter.Writer[analyticsmodel.NodeMetricSnapshot]
	requestReportWriter  *batchwriter.Writer[analyticsmodel.NodeRequestReport]
	openrestyWriter      *batchwriter.Writer[analyticsmodel.NodeObsOpenresty]
	frpsWriter           *batchwriter.Writer[analyticsmodel.NodeObsFrps]
	frpcWriter           *batchwriter.Writer[analyticsmodel.NodeObsFrpc]
	nodeAccessLogWriter  *batchwriter.Writer[analyticsmodel.NodeAccessLog]

	metricSnapshotDedup *dedupSet
	requestReportDedup  *dedupSet
	openrestyDedup      *dedupSet
	frpsDedup           *dedupSet
	frpcDedup           *dedupSet
)

// Init starts OpenFlare ClickHouse batch writers. Safe to call multiple times.
func Init(ctx context.Context) {
	if !config.Config.ClickHouse.Enabled {
		return
	}

	initOnce.Do(func() {
		metricSnapshotDedup = newDedupSet()
		requestReportDedup = newDedupSet()
		openrestyDedup = newDedupSet()
		frpsDedup = newDedupSet()
		frpcDedup = newDedupSet()

		metricSnapshotWriter = mustNewObservabilityWriter(
			"metric_snapshots",
			withFlushRetries(analyticsrepo.BatchInsertNodeMetricSnapshots),
			metricSnapshotDedup,
			metricSnapshotKey,
		)
		requestReportWriter = mustNewObservabilityWriter(
			"request_reports",
			withFlushRetries(analyticsrepo.BatchInsertNodeRequestReports),
			requestReportDedup,
			requestReportKey,
		)
		openrestyWriter = mustNewObservabilityWriter(
			"openresty_obs",
			withFlushRetries(analyticsrepo.BatchInsertNodeObsOpenresty),
			openrestyDedup,
			openrestyKey,
		)
		frpsWriter = mustNewObservabilityWriter(
			"frps_obs",
			withFlushRetries(analyticsrepo.BatchInsertNodeObsFrps),
			frpsDedup,
			frpsKey,
		)
		frpcWriter = mustNewObservabilityWriter(
			"frpc_obs",
			withFlushRetries(analyticsrepo.BatchInsertNodeObsFrpc),
			frpcDedup,
			frpcKey,
		)
		nodeAccessLogWriter = mustNewNodeAccessLogWriter()

		metricSnapshotWriter.Start(ctx)
		requestReportWriter.Start(ctx)
		openrestyWriter.Start(ctx)
		frpsWriter.Start(ctx)
		frpcWriter.Start(ctx)
		nodeAccessLogWriter.Start(ctx)

		wireModelInsertHooks()
		lifecycle.OnShutdown("openflare_chwriter", Stop)
	})
}

// Stop drains all OpenFlare ClickHouse writers.
func Stop(ctx context.Context) error {
	if !running() {
		return nil
	}

	var firstErr error
	for _, writer := range []batchStopper{
		metricSnapshotWriter,
		requestReportWriter,
		openrestyWriter,
		frpsWriter,
		frpcWriter,
		nodeAccessLogWriter,
	} {
		if writer == nil {
			continue
		}
		if err := writer.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// WriterStats returns queue depth and failure counters for all OpenFlare writers.
func WriterStats() []batchwriter.Stats {
	writers := []statsProvider{
		metricSnapshotWriter,
		requestReportWriter,
		openrestyWriter,
		frpsWriter,
		frpcWriter,
		nodeAccessLogWriter,
	}
	out := make([]batchwriter.Stats, 0, len(writers))
	for _, w := range writers {
		if w == nil {
			continue
		}
		out = append(out, w.Stats())
	}
	return out
}

// QueueMetricSnapshot enqueues a metric snapshot for asynchronous flush.
func QueueMetricSnapshot(snapshot analyticsmodel.NodeMetricSnapshot) {
	queueWithDedup(metricSnapshotWriter, metricSnapshotDedup, metricSnapshotKey(snapshot), snapshot)
}

// QueueRequestReport enqueues a request report for asynchronous flush.
func QueueRequestReport(report analyticsmodel.NodeRequestReport) {
	queueWithDedup(requestReportWriter, requestReportDedup, requestReportKey(report), report)
}

// QueueOpenrestyObservation enqueues an OpenResty observation for asynchronous flush.
func QueueOpenrestyObservation(observation analyticsmodel.NodeObsOpenresty) {
	queueWithDedup(openrestyWriter, openrestyDedup, openrestyKey(observation), observation)
}

// QueueFrpsObservation enqueues an FRPS observation for asynchronous flush.
func QueueFrpsObservation(observation analyticsmodel.NodeObsFrps) {
	queueWithDedup(frpsWriter, frpsDedup, frpsKey(observation), observation)
}

// QueueFrpcObservation enqueues an FRPC observation for asynchronous flush.
func QueueFrpcObservation(observation analyticsmodel.NodeObsFrpc) {
	queueWithDedup(frpcWriter, frpcDedup, frpcKey(observation), observation)
}

// QueueNodeAccessLogs enqueues node access logs for asynchronous flush.
func QueueNodeAccessLogs(logs []analyticsmodel.NodeAccessLog) {
	if nodeAccessLogWriter == nil || len(logs) == 0 {
		return
	}
	for _, logItem := range logs {
		nodeAccessLogWriter.TryEnqueue(logItem)
	}
}

func queueWithDedup[T any](writer *batchwriter.Writer[T], dedup *dedupSet, key string, item T) {
	if writer == nil {
		return
	}
	// Mark first so concurrent duplicates still collapse; release on enqueue failure
	// so a full queue does not permanently suppress the item.
	if !dedup.markIfNew(key) {
		return
	}
	if !writer.TryEnqueue(item) {
		dedup.unmark(key)
	}
}

func mustNewObservabilityWriter[T any](
	name string,
	flush batchwriter.FlushFunc[T],
	dedup *dedupSet,
	keyFn func(T) string,
) *batchwriter.Writer[T] {
	cfg := batchwriter.Config{
		Name:          name,
		QueueSize:     observabilityQueueSize,
		MaxBatchSize:  observabilityMaxBatchSize,
		MinBatchSize:  observabilityMinBatchSize,
		FlushInterval: observabilityFlushEvery,
		MaxFlushWait:  observabilityMaxFlushWait,
	}
	writer, err := batchwriter.New(
		cfg,
		flush,
		withObservabilityDropHandler[T](name),
		batchwriter.WithFlushErrorHandler[T](func(ctx context.Context, items []T, err error) {
			logger.ErrorF(ctx, "[OpenFlare] flush %s failed (batch=%d): %v", name, len(items), err)
			if dedup == nil || keyFn == nil {
				return
			}
			for _, item := range items {
				dedup.unmark(keyFn(item))
			}
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("openflare chwriter %s: %v", name, err))
	}
	return writer
}

func mustNewNodeAccessLogWriter() *batchwriter.Writer[analyticsmodel.NodeAccessLog] {
	cfg := batchwriter.Config{
		Name:          "node_access_logs",
		QueueSize:     nodeAccessLogQueueSize,
		MaxBatchSize:  nodeAccessLogMaxBatchSize,
		MinBatchSize:  nodeAccessLogMinBatchSize,
		FlushInterval: nodeAccessLogFlushEvery,
		MaxFlushWait:  nodeAccessLogMaxFlushWait,
	}
	writer, err := batchwriter.New[analyticsmodel.NodeAccessLog](
		cfg,
		withFlushRetries(analyticsrepo.BatchInsertNodeAccessLogs),
		batchwriter.WithDropHandler[analyticsmodel.NodeAccessLog](func(item analyticsmodel.NodeAccessLog) {
			logger.WarnF(context.Background(), "[OpenFlare] node access log queue full, dropping log for node %s path %s", item.NodeID, item.Path)
		}),
		batchwriter.WithFlushErrorHandler[analyticsmodel.NodeAccessLog](func(ctx context.Context, items []analyticsmodel.NodeAccessLog, err error) {
			logger.ErrorF(ctx, "[OpenFlare] flush node access logs failed (batch=%d): %v", len(items), err)
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("openflare chwriter node_access_logs: %v", err))
	}
	return writer
}

func withObservabilityDropHandler[T any](name string) batchwriter.Option[T] {
	return batchwriter.WithDropHandler(func(_ T) {
		logger.WarnF(context.Background(), "[OpenFlare] %s queue full, dropping observability item", name)
	})
}

// withFlushRetries wraps a flush function with a short retry to ride out brief CH blips.
func withFlushRetries[T any](flush batchwriter.FlushFunc[T]) batchwriter.FlushFunc[T] {
	return func(ctx context.Context, items []T) error {
		var err error
		for attempt := 1; attempt <= flushAttempts; attempt++ {
			err = flush(ctx, items)
			if err == nil {
				return nil
			}
			if attempt == flushAttempts {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(flushRetryBackoff * time.Duration(attempt)):
			}
		}
		return err
	}
}

func wireModelInsertHooks() {
	model.SetObservabilityInsertHooks(model.ObservabilityInsertHooks{
		QueueMetricSnapshot:       QueueMetricSnapshot,
		QueueRequestReport:        QueueRequestReport,
		QueueOpenrestyObservation: QueueOpenrestyObservation,
		QueueFrpsObservation:      QueueFrpsObservation,
		QueueFrpcObservation:      QueueFrpcObservation,
	})
	model.SetAccessLogInsertHooks(model.AccessLogInsertHooks{
		QueueNodeAccessLogs: QueueNodeAccessLogs,
	})
}

func metricSnapshotKey(snapshot analyticsmodel.NodeMetricSnapshot) string {
	return fmt.Sprintf("%s|%d", snapshot.NodeID, snapshot.CapturedAt.UTC().UnixNano())
}

func requestReportKey(report analyticsmodel.NodeRequestReport) string {
	return fmt.Sprintf(
		"%s|%d|%d",
		report.NodeID,
		report.WindowStartedAt.UTC().UnixNano(),
		report.WindowEndedAt.UTC().UnixNano(),
	)
}

func openrestyKey(observation analyticsmodel.NodeObsOpenresty) string {
	return fmt.Sprintf("%s|%d", observation.NodeID, observation.CapturedAt.UTC().UnixNano())
}

func frpsKey(observation analyticsmodel.NodeObsFrps) string {
	return fmt.Sprintf("%s|%d", observation.NodeID, observation.CapturedAt.UTC().UnixNano())
}

func frpcKey(observation analyticsmodel.NodeObsFrpc) string {
	return fmt.Sprintf("%s|%d", observation.NodeID, observation.CapturedAt.UTC().UnixNano())
}

type batchStopper interface {
	Stop(ctx context.Context) error
}

type statsProvider interface {
	Stats() batchwriter.Stats
}

func running() bool {
	return metricSnapshotWriter != nil && metricSnapshotWriter.Running()
}
