// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/redis/go-redis/v9"
)

// TaskExecutionStatus 任务执行状态
type TaskExecutionStatus string

// 任务执行状态
const (
	TaskExecutionStatusPending   TaskExecutionStatus = "pending"
	TaskExecutionStatusRunning   TaskExecutionStatus = "running"
	TaskExecutionStatusSucceeded TaskExecutionStatus = "succeeded"
	TaskExecutionStatusFailed    TaskExecutionStatus = "failed"

	taskExecutionLogRedisKeyPrefix = "task:execution:log:"
	taskExecutionLogExpiration     = 24 * time.Hour
	taskExecutionLogMaxLines       = 1000
)

// TaskExecution 任务执行记录
type TaskExecution struct {
	ID           uint64              `json:"id,string" gorm:"primaryKey"`
	TaskID       string              `json:"task_id" gorm:"size:128;uniqueIndex;not null"`
	TaskType     string              `json:"task_type" gorm:"size:64;index;not null"`
	TaskName     string              `json:"task_name" gorm:"size:128"`
	Status       TaskExecutionStatus `json:"status" gorm:"size:32;index;not null"`
	Retryable    bool                `json:"retryable" gorm:"not null;default:false"`
	MaxRetry     int                 `json:"max_retry" gorm:"not null;default:0"`
	RetryCount   int                 `json:"retry_count" gorm:"not null;default:0"`
	Log          string              `json:"log" gorm:"type:text"`
	ErrorMessage string              `json:"error_message" gorm:"type:text"`
	Result       string              `json:"result" gorm:"type:text"`
	StartedAt    *time.Time          `json:"started_at" gorm:"index"`
	FinishedAt   *time.Time          `json:"finished_at"`
	Duration     int64               `json:"duration" gorm:"comment:耗时毫秒"`
	Payload      string              `json:"payload" gorm:"type:text"`
	TriggeredBy  string              `json:"triggered_by" gorm:"size:32;not null;default:system"`
	CreatedAt    time.Time           `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    time.Time           `json:"updated_at" gorm:"autoUpdateTime"`
}

// TaskExecutionCleanupStats describes task execution log cleanup results.
type TaskExecutionCleanupStats struct {
	HighFrequencyDeleted int64
	LowFrequencyDeleted  int64
}

// TableName 表名
func (TaskExecution) TableName() string {
	return "w_task_executions"
}

// CreateTaskExecution 创建任务执行记录
func CreateTaskExecution(ctx context.Context, execution *TaskExecution) error {
	execution.ID = idgen.NextUint64ID()
	return db.DB(ctx).Create(execution).Error
}

// UpdateTaskExecution 更新任务执行记录，忽略由 Redis 缓冲和归档流程管理的 log 字段。
func UpdateTaskExecution(ctx context.Context, execution *TaskExecution) error {
	return db.DB(ctx).Omit("log").Save(execution).Error
}

// GetTaskExecutionByTaskID 根据 TaskID 获取执行记录
func GetTaskExecutionByTaskID(ctx context.Context, taskID string) (*TaskExecution, error) {
	var execution TaskExecution
	if err := db.DB(ctx).Where("task_id = ?", taskID).First(&execution).Error; err != nil {
		return nil, err
	}
	if err := loadTaskExecutionLog(ctx, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetTaskExecutionByID 根据 ID 获取执行记录
func GetTaskExecutionByID(ctx context.Context, id uint64) (*TaskExecution, error) {
	var execution TaskExecution
	if err := db.DB(ctx).Where("id = ?", id).First(&execution).Error; err != nil {
		return nil, err
	}
	if err := loadTaskExecutionLog(ctx, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// AppendTaskExecutionLog 将日志追加到 Redis 缓冲，任务完成后再持久化到数据库。
func AppendTaskExecutionLog(ctx context.Context, taskID string, logLine string) error {
	if db.Redis == nil {
		return errors.New("redis client is not initialized")
	}

	now := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s\n", now, logLine)
	key := taskExecutionLogRedisKey(taskID)

	_, err := db.Redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.RPush(ctx, key, line)
		pipe.LTrim(ctx, key, -taskExecutionLogMaxLines, -1)
		pipe.Expire(ctx, key, taskExecutionLogExpiration)
		return nil
	})
	if err != nil {
		return fmt.Errorf("append task execution log to redis: %w", err)
	}
	return nil
}

// FlushTaskExecutionLog 将 Redis 中的完整任务日志写入数据库，并在成功后清理缓存。
func FlushTaskExecutionLog(ctx context.Context, taskID string) error {
	if db.Redis == nil {
		return errors.New("redis client is not initialized")
	}

	key := taskExecutionLogRedisKey(taskID)
	logLines, err := db.Redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("get task execution log from redis: %w", err)
	}
	if len(logLines) == 0 {
		return nil
	}
	logText := strings.Join(logLines, "")

	result := db.DB(ctx).Model(&TaskExecution{}).
		Where("task_id = ?", taskID).
		Update("log", logText)
	if result.Error != nil {
		return fmt.Errorf("persist task execution log: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("persist task execution log: task %q not found", taskID)
	}

	if err := db.Redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete persisted task execution log from redis: %w", err)
	}
	return nil
}

// ListTaskExecutionsRequest 查询任务执行记录列表请求
type ListTaskExecutionsRequest struct {
	Status   string `form:"status"`
	TaskType string `form:"task_type"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ListTaskExecutions 分页查询任务执行记录
func ListTaskExecutions(ctx context.Context, req ListTaskExecutionsRequest) ([]TaskExecution, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	query := db.DB(ctx).Model(&TaskExecution{})

	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.TaskType != "" {
		query = query.Where("task_type = ?", req.TaskType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var executions []TaskExecution
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&executions).Error; err != nil {
		return nil, 0, err
	}
	if err := loadTaskExecutionLogs(ctx, executions); err != nil {
		return nil, 0, err
	}

	return executions, total, nil
}

// CleanupTaskExecutionLogs removes finished task execution logs according to frequency-based retention.
func CleanupTaskExecutionLogs(ctx context.Context, now time.Time) (TaskExecutionCleanupStats, error) {
	const (
		frequencyWindowDays    = 30
		highFrequencyThreshold = frequencyWindowDays
	)

	frequencyWindowStart := now.AddDate(0, 0, -frequencyWindowDays)
	highFrequencyCutoff := now.AddDate(0, 0, -3)
	lowFrequencyCutoff := now.AddDate(0, 0, -30)
	terminalStatuses := []TaskExecutionStatus{TaskExecutionStatusSucceeded, TaskExecutionStatusFailed}

	var highFrequencyTaskTypes []string
	if err := db.DB(ctx).
		Model(&TaskExecution{}).
		Select("task_type").
		Where("created_at >= ?", frequencyWindowStart).
		Group("task_type").
		Having("COUNT(*) > ?", highFrequencyThreshold).
		Pluck("task_type", &highFrequencyTaskTypes).Error; err != nil {
		return TaskExecutionCleanupStats{}, fmt.Errorf("query high-frequency task types: %w", err)
	}

	var highFrequencyDeleted int64
	if len(highFrequencyTaskTypes) > 0 {
		highFrequencyResult := db.DB(ctx).
			Where("status IN ?", terminalStatuses).
			Where("created_at < ?", highFrequencyCutoff).
			Where("task_type IN ?", highFrequencyTaskTypes).
			Delete(&TaskExecution{})
		if highFrequencyResult.Error != nil {
			return TaskExecutionCleanupStats{}, fmt.Errorf("delete high-frequency task execution logs: %w", highFrequencyResult.Error)
		}
		highFrequencyDeleted = highFrequencyResult.RowsAffected
	}

	lowFrequencyQuery := db.DB(ctx).
		Where("status IN ?", terminalStatuses).
		Where("created_at < ?", lowFrequencyCutoff)
	if len(highFrequencyTaskTypes) > 0 {
		lowFrequencyQuery = lowFrequencyQuery.Where("task_type NOT IN ?", highFrequencyTaskTypes)
	}
	lowFrequencyResult := lowFrequencyQuery.Delete(&TaskExecution{})
	if lowFrequencyResult.Error != nil {
		return TaskExecutionCleanupStats{}, fmt.Errorf("delete low-frequency task execution logs: %w", lowFrequencyResult.Error)
	}

	return TaskExecutionCleanupStats{
		HighFrequencyDeleted: highFrequencyDeleted,
		LowFrequencyDeleted:  lowFrequencyResult.RowsAffected,
	}, nil
}

func taskExecutionLogRedisKey(taskID string) string {
	return db.PrefixedKey(taskExecutionLogRedisKeyPrefix + taskID)
}

func loadTaskExecutionLog(ctx context.Context, execution *TaskExecution) error {
	if db.Redis == nil {
		return nil
	}

	logLines, err := db.Redis.LRange(ctx, taskExecutionLogRedisKey(execution.TaskID), 0, -1).Result()
	if err != nil {
		return fmt.Errorf("get task execution log from redis: %w", err)
	}
	if len(logLines) == 0 {
		return nil
	}

	execution.Log = strings.Join(logLines, "")
	return nil
}

func loadTaskExecutionLogs(ctx context.Context, executions []TaskExecution) error {
	if db.Redis == nil || len(executions) == 0 {
		return nil
	}

	commands := make([]*redis.StringSliceCmd, len(executions))
	_, err := db.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i := range executions {
			commands[i] = pipe.LRange(ctx, taskExecutionLogRedisKey(executions[i].TaskID), 0, -1)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("get task execution logs from redis: %w", err)
	}

	for i := range executions {
		logLines := commands[i].Val()
		if len(logLines) > 0 {
			executions[i].Log = strings.Join(logLines, "")
		}
	}
	return nil
}
