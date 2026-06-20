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
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	observabilityQueueSize    = 5_000
	observabilityMaxBatchSize = 200
	observabilityFlushEvery   = 2 * time.Second

	nodeAccessLogQueueSize    = 10_000
	nodeAccessLogMaxBatchSize = 1_000
	nodeAccessLogFlushEvery   = time.Second
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
)

// Init starts OpenFlare ClickHouse batch writers. Safe to call multiple times.
func Init(ctx context.Context) {
	if !config.Config.ClickHouse.Enabled {
		return
	}

	initOnce.Do(func() {
		metricSnapshotDedup = newDedupSet()
		requestReportDedup = newDedupSet()

		metricSnapshotWriter = mustNewObservabilityWriter("metric_snapshots", analyticsrepo.BatchInsertNodeMetricSnapshots)
		requestReportWriter = mustNewObservabilityWriter("request_reports", analyticsrepo.BatchInsertNodeRequestReports)
		openrestyWriter = mustNewObservabilityWriter("openresty_obs", analyticsrepo.BatchInsertNodeObsOpenresty)
		frpsWriter = mustNewObservabilityWriter("frps_obs", analyticsrepo.BatchInsertNodeObsFrps)
		frpcWriter = mustNewObservabilityWriter("frpc_obs", analyticsrepo.BatchInsertNodeObsFrpc)
		nodeAccessLogWriter = mustNewNodeAccessLogWriter()

		metricSnapshotWriter.Start(ctx)
		requestReportWriter.Start(ctx)
		openrestyWriter.Start(ctx)
		frpsWriter.Start(ctx)
		frpcWriter.Start(ctx)
		nodeAccessLogWriter.Start(ctx)

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

// QueueMetricSnapshot enqueues a metric snapshot for asynchronous flush.
func QueueMetricSnapshot(snapshot analyticsmodel.NodeMetricSnapshot) {
	if metricSnapshotWriter == nil {
		return
	}
	key := fmt.Sprintf("%s|%d", snapshot.NodeID, snapshot.CapturedAt.UTC().UnixNano())
	if !metricSnapshotDedup.markIfNew(key) {
		return
	}
	metricSnapshotWriter.TryEnqueue(snapshot)
}

// QueueRequestReport enqueues a request report for asynchronous flush.
func QueueRequestReport(report analyticsmodel.NodeRequestReport) {
	if requestReportWriter == nil {
		return
	}
	key := fmt.Sprintf(
		"%s|%d|%d",
		report.NodeID,
		report.WindowStartedAt.UTC().UnixNano(),
		report.WindowEndedAt.UTC().UnixNano(),
	)
	if !requestReportDedup.markIfNew(key) {
		return
	}
	requestReportWriter.TryEnqueue(report)
}

// QueueOpenrestyObservation enqueues an OpenResty observation for asynchronous flush.
func QueueOpenrestyObservation(observation analyticsmodel.NodeObsOpenresty) {
	if openrestyWriter == nil {
		return
	}
	openrestyWriter.TryEnqueue(observation)
}

// QueueFrpsObservation enqueues an FRPS observation for asynchronous flush.
func QueueFrpsObservation(observation analyticsmodel.NodeObsFrps) {
	if frpsWriter == nil {
		return
	}
	frpsWriter.TryEnqueue(observation)
}

// QueueFrpcObservation enqueues an FRPC observation for asynchronous flush.
func QueueFrpcObservation(observation analyticsmodel.NodeObsFrpc) {
	if frpcWriter == nil {
		return
	}
	frpcWriter.TryEnqueue(observation)
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

func mustNewObservabilityWriter[T any](name string, flush batchwriter.FlushFunc[T]) *batchwriter.Writer[T] {
	cfg := batchwriter.Config{
		Name:          name,
		QueueSize:     observabilityQueueSize,
		MaxBatchSize:  observabilityMaxBatchSize,
		FlushInterval: observabilityFlushEvery,
	}
	writer, err := batchwriter.New(
		cfg,
		flush,
		withObservabilityDropHandler[T](name),
		batchwriter.WithFlushErrorHandler[T](func(ctx context.Context, batchSize int, err error) {
			logger.ErrorF(ctx, "[OpenFlare] flush %s failed (batch=%d): %v", name, batchSize, err)
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
		FlushInterval: nodeAccessLogFlushEvery,
	}
	writer, err := batchwriter.New[analyticsmodel.NodeAccessLog](cfg, analyticsrepo.BatchInsertNodeAccessLogs,
		batchwriter.WithDropHandler[analyticsmodel.NodeAccessLog](func(item analyticsmodel.NodeAccessLog) {
			logger.WarnF(context.Background(), "[OpenFlare] node access log queue full, dropping log for node %s path %s", item.NodeID, item.Path)
		}),
		batchwriter.WithFlushErrorHandler[analyticsmodel.NodeAccessLog](func(ctx context.Context, batchSize int, err error) {
			logger.ErrorF(ctx, "[OpenFlare] flush node access logs failed (batch=%d): %v", batchSize, err)
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

type batchStopper interface {
	Stop(ctx context.Context) error
}

func running() bool {
	return metricSnapshotWriter != nil && metricSnapshotWriter.Running()
}