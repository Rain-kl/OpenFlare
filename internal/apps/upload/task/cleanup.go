// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package task provides upload-related async background task handlers.
package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/apps/upload/ingest"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	// SystemCleanupTask 系统定期垃圾清理任务标识
	SystemCleanupTask = "system:cleanup"
	// TaskTypeSystemCleanup 系统定期垃圾清理管理类型
	TaskTypeSystemCleanup = "system_cleanup"
)

// SystemCleanupMeta represents the task metadata.
var SystemCleanupMeta = task.TaskMeta{
	Type:         TaskTypeSystemCleanup,
	AsynqTask:    SystemCleanupTask,
	Name:         "System cleanup",
	Description:  "Periodically remove unused uploads, historical notifications and expired task execution logs",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// SystemCleanupHandler 系统定期垃圾清理异步任务处理器
type SystemCleanupHandler struct{}

// Execute 执行系统清理（包含文件清理、历史推送日志和任务执行日志清理）
func (h *SystemCleanupHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	if uploadstorage.ReadOnly(ctx) {
		return nil, errors.New(shared.ErrStorageReadOnly)
	}
	const batchSize = 100
	var lastID uint64
	var totalProcessed int
	var totalDeleted int

	oneHourAgo := time.Now().Add(-1 * time.Hour)

	task.AppendLog(ctx, "Scanning unused uploads, cutoff: %s", oneHourAgo.Format(time.RFC3339))

	for {
		unusedUploads, err := repository.ListPendingUploadsOlderThan(ctx, lastID, oneHourAgo, batchSize)
		if err != nil {
			task.AppendLog(ctx, "Failed to query unused uploads: %v", err)
			return nil, fmt.Errorf(shared.ErrQueryUnusedUploadsFailed, err)
		}

		if len(unusedUploads) == 0 {
			break
		}

		task.AppendLog(ctx, "Found %d uploads to clean up in this batch", len(unusedUploads))

		for _, u := range unusedUploads {
			totalProcessed++
			transitioned := false

			// Multi-step: row lock + ownership-safe soft-delete + stats delta stay orchestrated here.
			if err := repository.RunInTransaction(ctx, func(tx *gorm.DB) error {
				locked, err := repository.GetUploadByIDForUpdateTx(tx, u.ID)
				if err != nil {
					return err
				}
				if locked.Status != model.UploadStatusPending || !locked.CreatedAt.Before(oneHourAgo) {
					return nil
				}
				var removeErr error
				transitioned, removeErr = ingest.RemoveLockedTx(tx, &locked)
				return removeErr
			}); err != nil {
				task.AppendLog(ctx, "Failed to clean up upload [ID:%d]: %v", u.ID, err)
				lastID = u.ID
				continue
			}

			ingest.InvalidateUploadMetaCache(ctx, u.ID)
			if transitioned {
				totalDeleted++
			}
			lastID = u.ID
		}
	}

	task.AppendLog(ctx, "Cleaning up notification audit history; retaining the last 7 days...")
	cutoff := time.Now().AddDate(0, 0, -7)
	pushHistoryCount, err := repository.CountPushHistoriesCreatedBefore(ctx, cutoff)
	switch {
	case err != nil:
		task.AppendLog(ctx, "Failed to count notification history for cleanup: %v", err)
	case pushHistoryCount == 0:
		task.AppendLog(ctx, "No notification history requires cleanup (cutoff: %s)", cutoff.Format("2006-01-02 15:04:05"))
	default:
		if _, delErr := repository.DeletePushHistoriesCreatedBefore(ctx, cutoff); delErr != nil {
			task.AppendLog(ctx, "Failed to delete notification history: %v", delErr)
		} else {
			task.AppendLog(ctx, "Deleted %d notification history records (cutoff: %s)", pushHistoryCount, cutoff.Format("2006-01-02 15:04:05"))
		}
	}

	task.AppendLog(ctx, "Cleaning up task execution logs: retaining 3 days for high-frequency tasks and 30 days for low-frequency tasks...")
	taskLogStats, err := repository.CleanupTaskExecutionLogs(ctx, time.Now())
	if err != nil {
		task.AppendLog(ctx, "Failed to clean up task execution logs: %v", err)
		logger.ErrorF(ctx, "Failed to clean up task execution logs: %v", err)
	} else {
		task.AppendLog(ctx, "Cleaned up %d task execution logs (%d high-frequency, %d low-frequency)",
			taskLogStats.HighFrequencyDeleted+taskLogStats.LowFrequencyDeleted,
			taskLogStats.HighFrequencyDeleted,
			taskLogStats.LowFrequencyDeleted,
		)
	}

	task.AppendLog(ctx, "Cleaning up expired logs using log database retention and separate short metric retention...")
	summary, err := logstore.CleanupExpired(ctx)
	switch {
	case err != nil:
		logger.ErrorF(ctx, "Failed to clean up expired logs: %v", err)
		task.AppendLog(ctx, "Failed to clean up expired logs: %v", err)
	case summary.Deleted == 0:
		task.AppendLog(ctx, "No expired logs require cleanup (access log retention: %d days; metric retention: %d days)", summary.RetentionDays, summary.MetricRetentionDays)
	default:
		task.AppendLog(ctx, "Log cleanup completed: access log retention %d days, metric retention %d days, deleted %d records", summary.RetentionDays, summary.MetricRetentionDays, summary.Deleted)
	}

	msg := fmt.Sprintf("System cleanup completed. Removed %d/%d unused uploads, %d notification audit records and %d task execution logs.",
		totalDeleted,
		totalProcessed,
		pushHistoryCount,
		taskLogStats.HighFrequencyDeleted+taskLogStats.LowFrequencyDeleted,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}
