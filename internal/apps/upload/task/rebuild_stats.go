// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"fmt"

	uploadstats "github.com/Rain-kl/Wavelet/internal/apps/upload/stats"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	// RebuildUploadStatsTask is the Asynq task name for rebuilding upload stats.
	RebuildUploadStatsTask = "upload:rebuild_stats"
	// TaskTypeRebuildUploadStats is the admin-dispatchable task type.
	TaskTypeRebuildUploadStats = "rebuild_upload_stats"
)

// RebuildUploadStatsMeta describes the upload stats rebuild task.
var RebuildUploadStatsMeta = task.TaskMeta{
	Type:         TaskTypeRebuildUploadStats,
	AsynqTask:    RebuildUploadStatsTask,
	Name:         "Rebuild file storage statistics",
	Description:  "Rebuild w_upload_stats from active w_uploads records, including totals, types, categories and trends",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// RebuildUploadStatsHandler rebuilds incremental upload stats from active upload records.
type RebuildUploadStatsHandler struct{}

// Execute scans active uploads and rebuilds all upload stat dimensions.
func (h *RebuildUploadStatsHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	activeCount, err := repository.CountActiveUploads(ctx)
	if err != nil {
		task.AppendLog(ctx, "Failed to count active uploads: %v", err)
		return nil, fmt.Errorf("count active uploads: %w", err)
	}

	task.AppendLog(ctx, "Rebuilding file storage statistics; active records: %d", activeCount)

	if err := uploadstats.RebuildUploadStats(ctx); err != nil {
		task.AppendLog(ctx, "Failed to rebuild file storage statistics: %v", err)
		return nil, fmt.Errorf("rebuild upload stats: %w", err)
	}

	totalStat, err := repository.GetTotalUploadStat(ctx)
	if err != nil {
		task.AppendLog(ctx, "Failed to read aggregate statistics: %v", err)
		return nil, fmt.Errorf("load total upload stats: %w", err)
	}

	msg := fmt.Sprintf(
		"File storage statistics rebuilt: %d active records, %d files, %d bytes total",
		activeCount,
		totalStat.FileCount,
		totalStat.FileSize,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}
