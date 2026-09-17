// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package task provides upload-related async background task handlers.
package task

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	uploadcache "Wavelet/plugins/domain/upload/cache"

	uploadstats "Wavelet/plugins/domain/upload/stats"
	uploadstorage "Wavelet/plugins/domain/upload/storage"
)

const (
	// SystemCleanupTask 系统定期垃圾清理任务标识
	SystemCleanupTask = "system:cleanup"
	// TaskTypeSystemCleanup 系统定期垃圾清理管理类型
	TaskTypeSystemCleanup = "system_cleanup"
)

// SystemCleanupMeta represents the task metadata.
var SystemCleanupMeta = contracts.TaskMetaDTO{
	Type:         TaskTypeSystemCleanup,
	AsynqTask:    SystemCleanupTask,
	Name:         "清理未确认上传文件",
	DisplayName:  "清理未确认上传文件",
	Description:  "定期清理超过1小时未使用的上传临时文件与底层存储资源",
	Category:     "maintenance",
	SupportsTime: false,
	MaxRetry:     3,
	Queue:        taskQueueDefault,
	Retryable:    true,
}

// SystemCleanupHandler 未确认上传文件清理异步任务处理器
type SystemCleanupHandler struct{}

// Execute 执行系统清理（清理未使用上传文件）
func (h *SystemCleanupHandler) Execute(ctx context.Context, _ []byte) (*contracts.TaskResultDTO, error) {
	totalProcessed, totalDeleted, err := CleanupOrphanUploads(ctx)
	if err != nil {
		return nil, err
	}

	msg := fmt.Sprintf(
		"系统垃圾清理完成，处理未确认文件: %d 个，物理删除: %d 个",
		totalProcessed,
		totalDeleted,
	)
	logger.InfoF(ctx, "%s", msg)
	return &contracts.TaskResultDTO{Message: msg}, nil
}

// CleanupOrphanUploads 扫描并清理超过1小时未确认的 pending 状态上传文件及物理存储
func CleanupOrphanUploads(ctx context.Context) (int, int, error) {
	if uploadstorage.ReadOnly(ctx) {
		return 0, 0, errors.New(shared.ErrStorageReadOnly)
	}
	const batchSize = 100
	var lastID uint64
	var totalProcessed int
	var totalDeleted int

	oneHourAgo := time.Now().Add(-1 * time.Hour)

	logger.InfoF(ctx, "开始扫描未使用的待删除上传文件，阈值时间: %s", oneHourAgo.Format(time.RFC3339))

	db := shared.GetDB(ctx)
	if db == nil {
		return 0, 0, errors.New("database service not available")
	}

	storageSvc := shared.GetStorage(ctx)

	for {
		if err := ctx.Err(); err != nil {
			return totalProcessed, totalDeleted, fmt.Errorf("system cleanup canceled: %w", err)
		}

		var pendingUploads []models.Upload
		if err := db.
			Where("id > ? AND status = ? AND created_at < ?", lastID, models.UploadStatusPending, oneHourAgo).
			Order("id ASC").
			Limit(batchSize).
			Find(&pendingUploads).Error; err != nil {
			logger.ErrorF(ctx, "查询过期待使用上传文件失败: %v", err)
			return totalProcessed, totalDeleted, fmt.Errorf("failed to query pending uploads: %w", err)
		}

		if len(pendingUploads) == 0 {
			break
		}

		for i := range pendingUploads {
			if err := ctx.Err(); err != nil {
				return totalProcessed, totalDeleted, fmt.Errorf("system cleanup canceled: %w", err)
			}

			upload := &pendingUploads[i]
			totalProcessed++
			lastID = upload.ID

			if storageSvc != nil {
				if err := storageSvc.Delete(ctx, upload.FilePath); err != nil {
					logger.WarnF(ctx, "清理过期未确认上传底层文件失败 [ID:%d, Path:%s]: %v", upload.ID, upload.FilePath, err)
				}
			}

			statsSnapshot := *upload
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Delete(upload).Error; err != nil {
					return err
				}
				return uploadstats.ApplyUploadStatsDeltaTx(tx, &statsSnapshot, -1)
			}); err != nil {
				logger.ErrorF(ctx, "删除过期未确认上传记录失败 [ID:%d]: %v", upload.ID, err)
				continue
			}

			uploadcache.EvictUploadMeta(ctx, upload.ID)
			totalDeleted++
		}
	}

	return totalProcessed, totalDeleted, nil
}
