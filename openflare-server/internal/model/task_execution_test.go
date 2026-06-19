// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTaskExecutionTestEnvironment(t *testing.T) func() {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	err = sqliteDB.AutoMigrate(&TaskExecution{})
	require.NoError(t, err)

	miniRedis, err := miniredis.Run()
	require.NoError(t, err)
	redisClient := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	db.SetDB(sqliteDB)
	db.Redis = redisClient

	return func() {
		require.NoError(t, redisClient.Close())
		miniRedis.Close()
		db.SetDB(nil)
		db.Redis = nil
	}
}

func TestCreateTaskExecution(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	execution := &TaskExecution{
		TaskID:      "manual_cleanup_123",
		TaskType:    "system:cleanup",
		TaskName:    "清理未使用上传",
		Status:      TaskExecutionStatusPending,
		Retryable:   true,
		MaxRetry:    3,
		RetryCount:  0,
		Payload:     `{"test": true}`,
		TriggeredBy: "manual",
	}

	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)
	assert.NotZero(t, execution.ID, "ID should be generated")
	assert.NotZero(t, execution.CreatedAt, "CreatedAt should be set")
	assert.NotZero(t, execution.UpdatedAt, "UpdatedAt should be set")
}

func TestGetTaskExecutionByTaskID(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	// 创建记录
	execution := &TaskExecution{
		TaskID:      "test_task_id_001",
		TaskType:    "system:cleanup",
		TaskName:    "清理未使用上传",
		Status:      TaskExecutionStatusPending,
		Retryable:   true,
		MaxRetry:    3,
		TriggeredBy: "manual",
	}
	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 按 TaskID 查询
	found, err := GetTaskExecutionByTaskID(ctx, "test_task_id_001")
	require.NoError(t, err)
	assert.Equal(t, execution.ID, found.ID)
	assert.Equal(t, "test_task_id_001", found.TaskID)
	assert.Equal(t, TaskExecutionStatusPending, found.Status)
	assert.True(t, found.Retryable)
	assert.Equal(t, 3, found.MaxRetry)

	// 查询不存在的 TaskID
	_, err = GetTaskExecutionByTaskID(ctx, "nonexistent")
	assert.Error(t, err, "should return error for non-existent taskID")
}

func TestGetTaskExecutionByID(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	execution := &TaskExecution{
		TaskID:      "test_by_id_001",
		TaskType:    "system:cleanup",
		TaskName:    "清理未使用上传",
		Status:      TaskExecutionStatusPending,
		TriggeredBy: "system",
	}
	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 按主键查询
	found, err := GetTaskExecutionByID(ctx, execution.ID)
	require.NoError(t, err)
	assert.Equal(t, execution.TaskID, found.TaskID)
}

func TestUpdateTaskExecution(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	// 创建记录
	execution := &TaskExecution{
		TaskID:      "test_update_001",
		TaskType:    "system:cleanup",
		TaskName:    "清理未使用上传",
		Status:      TaskExecutionStatusPending,
		TriggeredBy: "manual",
	}
	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 更新状态为 running
	now := time.Now()
	execution.Status = TaskExecutionStatusRunning
	execution.StartedAt = &now
	err = UpdateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 验证更新
	found, err := GetTaskExecutionByTaskID(ctx, "test_update_001")
	require.NoError(t, err)
	assert.Equal(t, TaskExecutionStatusRunning, found.Status)
	assert.NotNil(t, found.StartedAt)

	// 更新为 succeeded
	finishTime := time.Now()
	execution.Status = TaskExecutionStatusSucceeded
	execution.FinishedAt = &finishTime
	execution.Duration = 1500
	execution.Result = "共清理 50 个文件"
	err = UpdateTaskExecution(ctx, execution)
	require.NoError(t, err)

	found, err = GetTaskExecutionByTaskID(ctx, "test_update_001")
	require.NoError(t, err)
	assert.Equal(t, TaskExecutionStatusSucceeded, found.Status)
	assert.Equal(t, int64(1500), found.Duration)
	assert.Equal(t, "共清理 50 个文件", found.Result)
}

func TestUpdateTaskExecutionFailed(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	execution := &TaskExecution{
		TaskID:      "test_fail_001",
		TaskType:    "system:cleanup",
		TaskName:    "清理未使用上传",
		Status:      TaskExecutionStatusPending,
		Retryable:   true,
		MaxRetry:    3,
		TriggeredBy: "manual",
	}
	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 标记为失败
	now := time.Now()
	execution.Status = TaskExecutionStatusFailed
	execution.StartedAt = &now
	execution.FinishedAt = &now
	execution.Duration = 200
	execution.ErrorMessage = "S3 连接超时"
	err = UpdateTaskExecution(ctx, execution)
	require.NoError(t, err)

	found, err := GetTaskExecutionByTaskID(ctx, "test_fail_001")
	require.NoError(t, err)
	assert.Equal(t, TaskExecutionStatusFailed, found.Status)
	assert.Equal(t, "S3 连接超时", found.ErrorMessage)
	assert.Equal(t, int64(200), found.Duration)
}

func TestUpdateTaskExecutionDoesNotPersistBufferedLog(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	execution := &TaskExecution{
		TaskID:      "test_omit_log_001",
		TaskType:    "system:cleanup",
		TaskName:    "清理未使用上传",
		Status:      TaskExecutionStatusPending,
		TriggeredBy: "manual",
	}
	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 运行中的日志仅缓存在 Redis。
	err = AppendTaskExecutionLog(ctx, "test_omit_log_001", "第一条执行日志")
	require.NoError(t, err)

	assert.Empty(t, execution.Log)

	execution.Status = TaskExecutionStatusSucceeded
	execution.Duration = 100
	err = UpdateTaskExecution(ctx, execution)
	require.NoError(t, err)

	var persisted TaskExecution
	err = db.DB(ctx).Where("task_id = ?", "test_omit_log_001").First(&persisted).Error
	require.NoError(t, err)
	assert.Equal(t, TaskExecutionStatusSucceeded, persisted.Status)
	assert.Empty(t, persisted.Log)

	found, err := GetTaskExecutionByTaskID(ctx, "test_omit_log_001")
	require.NoError(t, err)
	assert.Contains(t, found.Log, "第一条执行日志")
}

func TestAppendTaskExecutionLog(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	execution := &TaskExecution{
		TaskID:      "test_log_001",
		TaskType:    "system:cleanup",
		TaskName:    "清理未使用上传",
		Status:      TaskExecutionStatusPending,
		TriggeredBy: "manual",
	}
	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	// 追加多条日志
	err = AppendTaskExecutionLog(ctx, "test_log_001", "开始扫描未使用上传文件")
	require.NoError(t, err)

	err = AppendTaskExecutionLog(ctx, "test_log_001", "本批次找到 42 个待清理文件")
	require.NoError(t, err)

	err = AppendTaskExecutionLog(ctx, "test_log_001", "清理完成，共删除 42 个文件")
	require.NoError(t, err)

	// 读取时优先返回 Redis 中的在途日志。
	found, err := GetTaskExecutionByTaskID(ctx, "test_log_001")
	require.NoError(t, err)
	assert.Contains(t, found.Log, "开始扫描未使用上传文件")
	assert.Contains(t, found.Log, "本批次找到 42 个待清理文件")
	assert.Contains(t, found.Log, "清理完成，共删除 42 个文件")

	var persisted TaskExecution
	err = db.DB(ctx).Where("task_id = ?", "test_log_001").First(&persisted).Error
	require.NoError(t, err)
	assert.Empty(t, persisted.Log)

	err = FlushTaskExecutionLog(ctx, "test_log_001")
	require.NoError(t, err)

	err = db.DB(ctx).Where("task_id = ?", "test_log_001").First(&persisted).Error
	require.NoError(t, err)
	assert.Contains(t, persisted.Log, "开始扫描未使用上传文件")

	exists, err := db.Redis.Exists(ctx, taskExecutionLogRedisKey("test_log_001")).Result()
	require.NoError(t, err)
	assert.Zero(t, exists)
}

func TestAppendTaskExecutionLogLimitsLinesAndRefreshesTTL(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	const taskID = "limited_log_001"
	for i := 0; i < taskExecutionLogMaxLines+5; i++ {
		err := AppendTaskExecutionLog(ctx, taskID, fmt.Sprintf("日志-%04d", i))
		require.NoError(t, err)
	}

	key := taskExecutionLogRedisKey(taskID)
	logLines, err := db.Redis.LRange(ctx, key, 0, -1).Result()
	require.NoError(t, err)
	assert.Len(t, logLines, taskExecutionLogMaxLines)
	assert.Contains(t, logLines[0], "日志-0005")
	assert.Contains(t, logLines[len(logLines)-1], "日志-1004")

	ttl, err := db.Redis.TTL(ctx, key).Result()
	require.NoError(t, err)
	assert.Equal(t, taskExecutionLogExpiration, ttl)
}

func TestAppendTaskExecutionLogNonExistent(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	// Redis 缓冲不依赖数据库记录是否已经创建。
	err := AppendTaskExecutionLog(ctx, "nonexistent_task", "测试日志")
	assert.NoError(t, err)

	err = FlushTaskExecutionLog(ctx, "nonexistent_task")
	assert.Error(t, err)
}

func TestGetTaskExecutionLogPrefersRedis(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	execution := &TaskExecution{
		TaskID:      "redis_priority_001",
		TaskType:    "system:cleanup",
		TaskName:    "清理未使用上传",
		Status:      TaskExecutionStatusRunning,
		Log:         "数据库旧日志",
		TriggeredBy: "manual",
	}
	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)
	err = AppendTaskExecutionLog(ctx, execution.TaskID, "Redis 最新日志")
	require.NoError(t, err)

	found, err := GetTaskExecutionByID(ctx, execution.ID)
	require.NoError(t, err)
	assert.Contains(t, found.Log, "Redis 最新日志")
	assert.NotContains(t, found.Log, "数据库旧日志")
}

func TestListTaskExecutions(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	// 创建多条记录，包含不同状态和类型
	records := []*TaskExecution{
		{TaskID: "list_001", TaskType: "system:cleanup", TaskName: "系统垃圾清理", Status: TaskExecutionStatusSucceeded, TriggeredBy: "manual"},
		{TaskID: "list_002", TaskType: "system:cleanup", TaskName: "系统垃圾清理", Status: TaskExecutionStatusFailed, TriggeredBy: "system"},
		{TaskID: "list_003", TaskType: "other:task", TaskName: "其他任务", Status: TaskExecutionStatusPending, TriggeredBy: "manual"},
		{TaskID: "list_004", TaskType: "system:cleanup", TaskName: "系统垃圾清理", Status: TaskExecutionStatusRunning, TriggeredBy: "manual"},
		{TaskID: "list_005", TaskType: "other:task", TaskName: "其他任务", Status: TaskExecutionStatusSucceeded, TriggeredBy: "system"},
	}
	for _, r := range records {
		err := CreateTaskExecution(ctx, r)
		require.NoError(t, err)
	}
	err := AppendTaskExecutionLog(ctx, "list_004", "运行中的 Redis 日志")
	require.NoError(t, err)

	// 查询全部（分页）
	items, total, err := ListTaskExecutions(ctx, ListTaskExecutionsRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, items, 5)
	for _, item := range items {
		if item.TaskID == "list_004" {
			assert.Contains(t, item.Log, "运行中的 Redis 日志")
		}
	}

	// 按状态筛选：failed
	items, total, err = ListTaskExecutions(ctx, ListTaskExecutionsRequest{Status: "failed", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.Equal(t, "list_002", items[0].TaskID)

	// 按类型筛选
	_, total, err = ListTaskExecutions(ctx, ListTaskExecutionsRequest{TaskType: "other:task", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	// 分页测试
	items, total, err = ListTaskExecutions(ctx, ListTaskExecutionsRequest{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, items, 2)

	items2, total2, err := ListTaskExecutions(ctx, ListTaskExecutionsRequest{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total2)
	assert.Len(t, items2, 2)

	// 确保分页数据不重复
	assert.NotEqual(t, items[0].ID, items2[0].ID)

	// 状态 + 类型组合筛选
	items, total, err = ListTaskExecutions(ctx, ListTaskExecutionsRequest{Status: "succeeded", TaskType: "system:cleanup", Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "list_001", items[0].TaskID)
}

func TestListTaskExecutionsDefaultPaging(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	// 不传分页参数，应使用默认值 page=1, pageSize=20
	items, total, err := ListTaskExecutions(ctx, ListTaskExecutionsRequest{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, items, 0)
}

func TestCleanupTaskExecutionLogs(t *testing.T) {
	cleanup := setupTaskExecutionTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 31; i++ {
		createTaskExecutionForCleanup(t, ctx, fmt.Sprintf("high_recent_%02d", i), "high:task", TaskExecutionStatusSucceeded, now.Add(-2*time.Hour))
	}
	createTaskExecutionForCleanup(t, ctx, "high_old_4d", "high:task", TaskExecutionStatusSucceeded, now.AddDate(0, 0, -4))
	createTaskExecutionForCleanup(t, ctx, "high_old_40d", "high:task", TaskExecutionStatusFailed, now.AddDate(0, 0, -40))
	createTaskExecutionForCleanup(t, ctx, "high_running_old", "high:task", TaskExecutionStatusRunning, now.AddDate(0, 0, -10))
	createTaskExecutionForCleanup(t, ctx, "low_old_31d", "low:task", TaskExecutionStatusSucceeded, now.AddDate(0, 0, -31))
	createTaskExecutionForCleanup(t, ctx, "low_recent_29d", "low:task", TaskExecutionStatusSucceeded, now.AddDate(0, 0, -29))
	createTaskExecutionForCleanup(t, ctx, "low_pending_old", "low:task", TaskExecutionStatusPending, now.AddDate(0, 0, -45))

	stats, err := CleanupTaskExecutionLogs(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats.HighFrequencyDeleted)
	assert.Equal(t, int64(1), stats.LowFrequencyDeleted)

	for _, taskID := range []string{"high_old_4d", "high_old_40d", "low_old_31d"} {
		var count int64
		err := db.DB(ctx).Model(&TaskExecution{}).Where("task_id = ?", taskID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "CleanupTaskExecutionLogs(%s) should delete expired log", taskID)
	}
	for _, taskID := range []string{"high_recent_00", "high_running_old", "low_recent_29d", "low_pending_old"} {
		var count int64
		err := db.DB(ctx).Model(&TaskExecution{}).Where("task_id = ?", taskID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "CleanupTaskExecutionLogs(%s) should keep retained log", taskID)
	}
}

func TestTaskExecutionTableName(t *testing.T) {
	execution := TaskExecution{}
	assert.Equal(t, "w_task_executions", execution.TableName())
}

func createTaskExecutionForCleanup(t *testing.T, ctx context.Context, taskID string, taskType string, status TaskExecutionStatus, createdAt time.Time) {
	t.Helper()

	execution := &TaskExecution{
		TaskID:      taskID,
		TaskType:    taskType,
		TaskName:    taskType,
		Status:      status,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
		TriggeredBy: "system",
	}
	err := CreateTaskExecution(ctx, execution)
	require.NoError(t, err)
}
