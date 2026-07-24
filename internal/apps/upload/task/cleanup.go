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
	Name:         "系统垃圾清理",
	Description:  "定期清理未使用上传文件、历史推送记录和过期任务执行日志",
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

	task.AppendLog(ctx, "开始扫描未使用上传文件，阈值: %s", oneHourAgo.Format(time.RFC3339))

	for {
		unusedUploads, err := repository.ListPendingUploadsOlderThan(ctx, lastID, oneHourAgo, batchSize)
		if err != nil {
			task.AppendLog(ctx, "查询未使用的上传文件失败: %v", err)
			return nil, fmt.Errorf(shared.ErrQueryUnusedUploadsFailed, err)
		}

		if len(unusedUploads) == 0 {
			break
		}

		task.AppendLog(ctx, "本批次找到 %d 个需要清理的上传文件", len(unusedUploads))

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
				task.AppendLog(ctx, "清理上传文件失败 [ID:%d]: %v", u.ID, err)
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

	task.AppendLog(ctx, "开始清理历史推送审计日志，只保留最近7天数据...")
	cutoff := time.Now().AddDate(0, 0, -7)
	pushHistoryCount, err := repository.CountPushHistoriesCreatedBefore(ctx, cutoff)
	switch {
	case err != nil:
		task.AppendLog(ctx, "统计待清理的历史推送记录失败: %v", err)
	case pushHistoryCount == 0:
		task.AppendLog(ctx, "没有需要清理的历史推送记录 (截止时间: %s)", cutoff.Format("2006-01-02 15:04:05"))
	default:
		if _, delErr := repository.DeletePushHistoriesCreatedBefore(ctx, cutoff); delErr != nil {
			task.AppendLog(ctx, "删除历史推送记录失败: %v", delErr)
		} else {
			task.AppendLog(ctx, "成功删除 %d 条历史推送记录 (截止时间: %s)", pushHistoryCount, cutoff.Format("2006-01-02 15:04:05"))
		}
	}

	task.AppendLog(ctx, "开始清理任务执行日志：高频任务保留最近3天，低频任务保留最近30天...")
	taskLogStats, err := repository.CleanupTaskExecutionLogs(ctx, time.Now())
	if err != nil {
		task.AppendLog(ctx, "清理任务执行日志失败: %v", err)
		logger.ErrorF(ctx, "清理任务执行日志失败: %v", err)
	} else {
		task.AppendLog(ctx, "成功清理任务执行日志 %d 条（高频 %d 条，低频 %d 条）",
			taskLogStats.HighFrequencyDeleted+taskLogStats.LowFrequencyDeleted,
			taskLogStats.HighFrequencyDeleted,
			taskLogStats.LowFrequencyDeleted,
		)
	}

	msg := fmt.Sprintf("系统清理完成。成功清理未使用的上传文件 %d/%d 个；清理历史推送审计日志 %d 条；清理任务执行日志 %d 条。",
		totalDeleted,
		totalProcessed,
		pushHistoryCount,
		taskLogStats.HighFrequencyDeleted+taskLogStats.LowFrequencyDeleted,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}
