package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	taskExecutionLogRedisKeyPrefix = "task:execution:log:"
	taskExecutionLogExpiration     = 24 * time.Hour
	taskExecutionLogMaxLines       = 1000
)

// CreateTaskExecution 创建任务执行记录
func CreateTaskExecution(ctx context.Context, execution *model.TaskExecution) error {
	execution.ID = idgen.NextUint64ID()
	return db.DB(ctx).Create(execution).Error
}

// UpdateTaskExecution 更新任务执行记录，忽略由 Redis 缓冲和归档流程管理的 log 字段。
func UpdateTaskExecution(ctx context.Context, execution *model.TaskExecution) error {
	return db.DB(ctx).Omit("log").Save(execution).Error
}

// GetTaskExecutionByTaskID 根据 TaskID 获取执行记录
func GetTaskExecutionByTaskID(ctx context.Context, taskID string) (*model.TaskExecution, error) {
	var execution model.TaskExecution
	if err := db.DB(ctx).Where("task_id = ?", taskID).First(&execution).Error; err != nil {
		return nil, err
	}
	if err := loadTaskExecutionLog(ctx, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetTaskExecutionByID 根据 ID 获取执行记录
func GetTaskExecutionByID(ctx context.Context, id uint64) (*model.TaskExecution, error) {
	var execution model.TaskExecution
	if err := db.DB(ctx).Where("id = ?", id).First(&execution).Error; err != nil {
		return nil, err
	}
	if err := loadTaskExecutionLog(ctx, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetLatestTaskExecutionByTaskType returns the most recent execution for a task type.
// ok is false when no row exists.
func GetLatestTaskExecutionByTaskType(ctx context.Context, taskType string) (*model.TaskExecution, bool, error) {
	var execution model.TaskExecution
	err := db.DB(ctx).
		Where("task_type = ?", taskType).
		Order("id DESC").
		First(&execution).Error
	if err == nil {
		if loadErr := loadTaskExecutionLog(ctx, &execution); loadErr != nil {
			return nil, false, loadErr
		}
		return &execution, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, err
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

	result := db.DB(ctx).Model(&model.TaskExecution{}).
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

// ListTaskExecutions 分页查询任务执行记录
func ListTaskExecutions(ctx context.Context, req model.ListTaskExecutionsRequest) ([]model.TaskExecution, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	query := db.DB(ctx).Model(&model.TaskExecution{})

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

	var executions []model.TaskExecution
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&executions).Error; err != nil {
		return nil, 0, err
	}
	if err := loadTaskExecutionLogs(ctx, executions); err != nil {
		return nil, 0, err
	}

	return executions, total, nil
}

// MarkFailedTaskExecutionsSucceededTx marks failed executions of a task type as succeeded within a transaction.
func MarkFailedTaskExecutionsSucceededTx(
	tx *gorm.DB,
	taskType string,
	result string,
	finishedAt time.Time,
) error {
	return tx.Model(&model.TaskExecution{}).
		Where("task_type = ? AND status = ?", taskType, model.TaskExecutionStatusFailed).
		Updates(map[string]any{
			"status":      model.TaskExecutionStatusSucceeded,
			"result":      result,
			"finished_at": finishedAt,
		}).Error
}

// CleanupTaskExecutionLogs removes finished task execution logs according to frequency-based retention.
func CleanupTaskExecutionLogs(ctx context.Context, now time.Time) (model.TaskExecutionCleanupStats, error) {
	const (
		frequencyWindowDays    = 30
		highFrequencyThreshold = frequencyWindowDays
	)

	frequencyWindowStart := now.AddDate(0, 0, -frequencyWindowDays)
	highFrequencyCutoff := now.AddDate(0, 0, -3)
	lowFrequencyCutoff := now.AddDate(0, 0, -30)
	terminalStatuses := []model.TaskExecutionStatus{model.TaskExecutionStatusSucceeded, model.TaskExecutionStatusFailed}

	var highFrequencyTaskTypes []string
	if err := db.DB(ctx).
		Model(&model.TaskExecution{}).
		Select("task_type").
		Where("created_at >= ?", frequencyWindowStart).
		Group("task_type").
		Having("COUNT(*) > ?", highFrequencyThreshold).
		Pluck("task_type", &highFrequencyTaskTypes).Error; err != nil {
		return model.TaskExecutionCleanupStats{}, fmt.Errorf("query high-frequency task types: %w", err)
	}

	var highFrequencyDeleted int64
	if len(highFrequencyTaskTypes) > 0 {
		highFrequencyResult := db.DB(ctx).
			Where("status IN ?", terminalStatuses).
			Where("created_at < ?", highFrequencyCutoff).
			Where("task_type IN ?", highFrequencyTaskTypes).
			Delete(&model.TaskExecution{})
		if highFrequencyResult.Error != nil {
			return model.TaskExecutionCleanupStats{}, fmt.Errorf("delete high-frequency task execution logs: %w", highFrequencyResult.Error)
		}
		highFrequencyDeleted = highFrequencyResult.RowsAffected
	}

	lowFrequencyQuery := db.DB(ctx).
		Where("status IN ?", terminalStatuses).
		Where("created_at < ?", lowFrequencyCutoff)
	if len(highFrequencyTaskTypes) > 0 {
		lowFrequencyQuery = lowFrequencyQuery.Where("task_type NOT IN ?", highFrequencyTaskTypes)
	}
	lowFrequencyResult := lowFrequencyQuery.Delete(&model.TaskExecution{})
	if lowFrequencyResult.Error != nil {
		return model.TaskExecutionCleanupStats{}, fmt.Errorf("delete low-frequency task execution logs: %w", lowFrequencyResult.Error)
	}

	return model.TaskExecutionCleanupStats{
		HighFrequencyDeleted: highFrequencyDeleted,
		LowFrequencyDeleted:  lowFrequencyResult.RowsAffected,
	}, nil
}

func taskExecutionLogRedisKey(taskID string) string {
	return db.PrefixedKey(taskExecutionLogRedisKeyPrefix + taskID)
}

func loadTaskExecutionLog(ctx context.Context, execution *model.TaskExecution) error {
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

func loadTaskExecutionLogs(ctx context.Context, executions []model.TaskExecution) error {
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
