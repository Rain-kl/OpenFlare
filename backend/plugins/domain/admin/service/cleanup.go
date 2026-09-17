// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/model"
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// SystemCleanupTask 系统定期垃圾清理任务标识
	SystemCleanupTask = "system:cleanup"
	// TaskTypeSystemCleanup 系统定期垃圾清理管理类型
	TaskTypeSystemCleanup = "system_cleanup"
	taskQueueDefault      = "default"
)

// SystemCleanupMeta describes the system-wide cleanup task metadata.
var SystemCleanupMeta = contracts.TaskMetaDTO{
	Type:         TaskTypeSystemCleanup,
	AsynqTask:    SystemCleanupTask,
	Name:         "系统垃圾清理",
	DisplayName:  "系统垃圾清理",
	Description:  "定期清理过期任务执行记录，并通过领域事件广播触发各业务域自治清理（临时文件、历史推送等）",
	Category:     "maintenance",
	SupportsTime: false,
	MaxRetry:     3,
	Queue:        taskQueueDefault,
	Retryable:    true,
}

// SystemCleanupHandler handles the system-wide garbage cleanup task.
type SystemCleanupHandler struct{}

// Execute executes system cleanup: clears old task executions and emits EventTopicSystemCleanup.
func (h *SystemCleanupHandler) Execute(ctx context.Context, _ []byte) (*contracts.TaskResultDTO, error) {
	db := GetDB(ctx)
	if db == nil {
		return nil, errors.New("database service not available")
	}

	// 1. 清理自身域（admin 域）的过期任务执行记录（7 天前）
	var deletedExecutions int64
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)
	res := db.Where("created_at < ?", sevenDaysAgo).Delete(&model.TaskExecution{})
	if err := res.Error; err != nil {
		logger.WarnF(ctx, "清理过期任务执行日志失败: %v", err)
	} else {
		deletedExecutions = res.RowsAffected
		logger.InfoF(ctx, "已清理 7 天前任务执行日志，共 %d 条", deletedExecutions)
	}

	// 2. 广播 EventTopicSystemCleanup 领域事件，由各业务域插件（upload, msg_gateway, user 等）自治执行各自的清理逻辑
	nowStr := time.Now().Format(time.RFC3339)
	if err := EmitEvent(ctx, contracts.EventTopicSystemCleanup, contracts.SystemCleanupEvent{
		TriggeredAt: nowStr,
	}); err != nil {
		logger.WarnF(ctx, "广播系统清理领域事件失败: %v", err)
	}

	msg := fmt.Sprintf("系统垃圾清理完成，已清理过期任务执行日志 %d 条，并已广播领域清理事件", deletedExecutions)
	logger.InfoF(ctx, "%s", msg)
	return &contracts.TaskResultDTO{Message: msg}, nil
}
