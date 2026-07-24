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
	Name:         "重算文件存储统计",
	Description:  "根据当前 w_uploads 活跃记录全量重建 w_upload_stats（总量、类型、分类、趋势）",
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
		task.AppendLog(ctx, "统计活跃上传记录失败: %v", err)
		return nil, fmt.Errorf("count active uploads: %w", err)
	}

	task.AppendLog(ctx, "开始重算文件存储统计，活跃记录数: %d", activeCount)

	if err := uploadstats.RebuildUploadStats(ctx); err != nil {
		task.AppendLog(ctx, "重算文件存储统计失败: %v", err)
		return nil, fmt.Errorf("rebuild upload stats: %w", err)
	}

	totalStat, err := repository.GetTotalUploadStat(ctx)
	if err != nil {
		task.AppendLog(ctx, "读取总量统计失败: %v", err)
		return nil, fmt.Errorf("load total upload stats: %w", err)
	}

	msg := fmt.Sprintf(
		"文件存储统计重算完成，活跃记录 %d 条，统计文件数 %d，总大小 %d 字节",
		activeCount,
		totalStat.FileCount,
		totalStat.FileSize,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}
