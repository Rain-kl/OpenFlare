// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	uploadtask "github.com/Rain-kl/Wavelet/internal/apps/upload/task"
	"github.com/Rain-kl/Wavelet/internal/apps/user"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/platform/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
)

const (
	testInternalOnlyTaskType  = "test_internal_only_admin"
	testInternalOnlyAsynqTask = "test:internal_only_admin"
)

func registerInternalOnlyTaskMeta() {
	task.RegisterTaskMeta(task.TaskMeta{
		Type:         testInternalOnlyTaskType,
		AsynqTask:    testInternalOnlyAsynqTask,
		Name:         "内部测试任务",
		InternalOnly: true,
	})
}

func setupTaskTestEnvironment(t *testing.T) func() {
	_, mr, cleanup := testhelper.SetupTestEnvironment(t)
	bootstrap.RegisterTasks()
	task.AsynqClient = asynq.NewClient(asynq.RedisClientOpt{
		Addr: mr.Addr(),
	})
	return func() {
		if task.AsynqClient != nil {
			_ = task.AsynqClient.Close()
			task.AsynqClient = nil
		}
		cleanup()
	}
}

func setupTestRouter(authUser *model.User) *gin.Engine {
	r := testhelper.NewTestGinEngine()
	adminGroup := r.Group("/api/v1/admin")

	// Mock authentication middleware
	adminGroup.Use(func(c *gin.Context) {
		if authUser != nil {
			oauth.SetToContext(c, oauth.UserObjKey, authUser)
		}
		c.Next()
	})

	adminGroup.GET("/tasks/types", ListTaskTypes)
	adminGroup.POST("/tasks/dispatch", DispatchTask)
	adminGroup.GET("/tasks/executions", ListTaskExecutions)
	adminGroup.GET("/tasks/executions/:id", GetTaskExecution)
	adminGroup.POST("/tasks/executions/:id/retry", RetryTask)
	adminGroup.GET("/tasks/schedules", ListSchedules)
	adminGroup.POST("/tasks/schedules", CreateSchedule)
	adminGroup.PUT("/tasks/schedules/:id", UpdateSchedule)
	adminGroup.DELETE("/tasks/schedules/:id", DeleteSchedule)
	return r
}

func TestListTaskTypes(t *testing.T) {
	cleanup := setupTaskTestEnvironment(t)
	defer cleanup()
	registerInternalOnlyTaskMeta()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	req, _ := http.NewRequest("GET", "/api/v1/admin/tasks/types", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	var resp response.Any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	dataBytes, _ := json.Marshal(resp.Data)
	var taskMetas []task.TaskMeta
	_ = json.Unmarshal(dataBytes, &taskMetas)

	if len(taskMetas) == 0 {
		t.Error("expected at least one dispatchable task type")
	}

	foundCleanup := false
	foundWarmImageCache := false
	for _, m := range taskMetas {
		if m.Type == testInternalOnlyTaskType {
			t.Errorf("internal-only task type %s must not be listed", testInternalOnlyTaskType)
		}
		if m.Type == uploadtask.TaskTypeSystemCleanup {
			foundCleanup = true
		}
		if m.Type == uploadtask.TaskTypeWarmImageCache {
			foundWarmImageCache = true
		}
	}
	if !foundCleanup {
		t.Errorf("expected task type %s to be listed", uploadtask.TaskTypeSystemCleanup)
	}
	if !foundWarmImageCache {
		t.Errorf("expected task type %s to be listed", uploadtask.TaskTypeWarmImageCache)
	}
}

func TestInternalOnlyTaskAdminBoundaries(t *testing.T) {
	cleanup := setupTaskTestEnvironment(t)
	defer cleanup()
	registerInternalOnlyTaskMeta()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)
	ctx := context.Background()

	t.Run("list hides internal-only schedule", func(t *testing.T) {
		internalSchedule := &model.Schedule{
			Name:     "隐藏的系统内部排程",
			TaskType: testInternalOnlyTaskType,
			Cron:     "*/5 * * * *",
			Payload:  "{}",
			IsActive: true,
		}
		publicSchedule := &model.Schedule{
			Name:     "可见的公开排程",
			TaskType: uploadtask.TaskTypeSystemCleanup,
			Cron:     "0 * * * *",
			Payload:  "{}",
			IsActive: true,
		}
		require.NoError(t, repository.CreateSchedule(ctx, internalSchedule))
		require.NoError(t, repository.CreateSchedule(ctx, publicSchedule))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tasks/schedules", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		data, err := json.Marshal(resp.Data)
		require.NoError(t, err)
		var schedules []model.Schedule
		require.NoError(t, json.Unmarshal(data, &schedules))
		assert.NotContains(t, scheduleIDs(schedules), internalSchedule.ID)
		assert.Contains(t, scheduleIDs(schedules), publicSchedule.ID)
	})

	t.Run("dispatch rejects internal-only task", func(t *testing.T) {
		body, err := json.Marshal(DispatchTaskRequest{TaskType: testInternalOnlyTaskType})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tasks/dispatch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp response.Any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, InvalidTaskType, resp.ErrorMsg)
	})

	t.Run("create schedule rejects internal-only task", func(t *testing.T) {
		isActive := true
		body, err := json.Marshal(CreateScheduleRequest{
			Name:     "内部任务排程",
			TaskType: testInternalOnlyTaskType,
			Cron:     "0 * * * *",
			IsActive: &isActive,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tasks/schedules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp response.Any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, InvalidTaskType, resp.ErrorMsg)
	})

	t.Run("update cannot convert existing internal schedule to public task", func(t *testing.T) {
		schedule := &model.Schedule{
			Name:     "系统内部排程",
			TaskType: testInternalOnlyTaskType,
			Cron:     "0 * * * *",
			IsActive: true,
		}
		require.NoError(t, repository.CreateSchedule(ctx, schedule))
		isActive := false
		body, err := json.Marshal(UpdateScheduleRequest{
			Name:     "尝试修改内部排程",
			TaskType: uploadtask.TaskTypeSystemCleanup,
			Cron:     "5 * * * *",
			IsActive: &isActive,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(
			http.MethodPut,
			fmt.Sprintf("/api/v1/admin/tasks/schedules/%d", schedule.ID),
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		unchanged, err := repository.GetScheduleByID(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, "系统内部排程", unchanged.Name)
		assert.Equal(t, testInternalOnlyTaskType, unchanged.TaskType)
		assert.True(t, unchanged.IsActive)
	})

	t.Run("update public schedule rejects internal-only target task", func(t *testing.T) {
		schedule := &model.Schedule{
			Name:     "公开排程",
			TaskType: uploadtask.TaskTypeSystemCleanup,
			Cron:     "0 * * * *",
			IsActive: true,
		}
		require.NoError(t, repository.CreateSchedule(ctx, schedule))
		isActive := true
		body, err := json.Marshal(UpdateScheduleRequest{
			Name:     "尝试切入内部任务",
			TaskType: testInternalOnlyTaskType,
			Cron:     "10 * * * *",
			IsActive: &isActive,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(
			http.MethodPut,
			fmt.Sprintf("/api/v1/admin/tasks/schedules/%d", schedule.ID),
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		unchanged, err := repository.GetScheduleByID(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, "公开排程", unchanged.Name)
		assert.Equal(t, uploadtask.TaskTypeSystemCleanup, unchanged.TaskType)
	})

	t.Run("delete rejects internal-only schedule", func(t *testing.T) {
		schedule := &model.Schedule{
			Name:     "不可删除的系统内部排程",
			TaskType: testInternalOnlyTaskType,
			Cron:     "*/5 * * * *",
			IsActive: true,
		}
		require.NoError(t, repository.CreateSchedule(ctx, schedule))
		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/admin/tasks/schedules/%d", schedule.ID),
			nil,
		)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp response.Any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, InvalidTaskType, resp.ErrorMsg)
		preserved, err := repository.GetScheduleByID(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, testInternalOnlyTaskType, preserved.TaskType)
	})

	t.Run("delete missing schedule returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/tasks/schedules/999999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var resp response.Any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, ScheduleNotFound, resp.ErrorMsg)
	})

	t.Run("delete public schedule remains allowed", func(t *testing.T) {
		schedule := &model.Schedule{
			Name:     "可删除的公开排程",
			TaskType: uploadtask.TaskTypeSystemCleanup,
			Cron:     "0 * * * *",
			IsActive: false,
		}
		require.NoError(t, repository.CreateSchedule(ctx, schedule))
		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/admin/tasks/schedules/%d", schedule.ID),
			nil,
		)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		_, err := repository.GetScheduleByID(ctx, schedule.ID)
		assert.Error(t, err)
	})
}

func scheduleIDs(schedules []model.Schedule) []uint64 {
	ids := make([]uint64, 0, len(schedules))
	for _, schedule := range schedules {
		ids = append(ids, schedule.ID)
	}
	return ids
}

func TestDispatchTask(t *testing.T) {
	cleanup := setupTaskTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)

	t.Run("dispatch valid task successfully", func(t *testing.T) {
		payload := DispatchTaskRequest{
			TaskType: uploadtask.TaskTypeSystemCleanup,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/tasks/dispatch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Body: %s", w.Body.String())

		var resp response.Any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Empty(t, resp.ErrorMsg)
		assert.NotNil(t, resp.Data)

		// 返回的 data 应该是 taskID
		taskID, ok := resp.Data.(string)
		assert.True(t, ok)
		assert.NotEmpty(t, taskID)
	})

	t.Run("dispatch send_email task successfully with valid payload", func(t *testing.T) {
		payload := DispatchTaskRequest{
			TaskType: user.TaskTypeSendEmail,
			Payload:  `{"to":"receiver@example.com","subject":"Test Subject","body":"Test Body"}`,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/tasks/dispatch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Body: %s", w.Body.String())

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Empty(t, resp.ErrorMsg)
		assert.NotNil(t, resp.Data)
	})

	t.Run("dispatch send_email task failure with invalid payload json", func(t *testing.T) {
		payload := DispatchTaskRequest{
			TaskType: user.TaskTypeSendEmail,
			Payload:  `{"to":`,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/tasks/dispatch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp.ErrorMsg, "无效的 JSON 格式")
	})

	t.Run("dispatch send_email task failure with missing fields", func(t *testing.T) {
		payload := DispatchTaskRequest{
			TaskType: user.TaskTypeSendEmail,
			Payload:  `{"to":"","subject":"Test","body":"Test"}`,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/tasks/dispatch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Contains(t, resp.ErrorMsg, "不能为空")
	})

	t.Run("dispatch invalid task type failure", func(t *testing.T) {
		payload := DispatchTaskRequest{
			TaskType: "invalid_task_type",
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/admin/tasks/dispatch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, InvalidTaskType, resp.ErrorMsg)
	})

	t.Run("dispatch with empty body failure", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/admin/tasks/dispatch", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestListTaskExecutions(t *testing.T) {
	cleanup := setupTaskTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)
	ctx := context.Background()

	// 准备测试数据
	now := time.Now()
	records := []*model.TaskExecution{
		{TaskID: "exec_001", TaskType: "system:cleanup", TaskName: "系统垃圾清理", Status: model.TaskExecutionStatusSucceeded, TriggeredBy: "manual", Duration: 1500, Result: "清理完成", StartedAt: &now, FinishedAt: &now},
		{TaskID: "exec_002", TaskType: "system:cleanup", TaskName: "系统垃圾清理", Status: model.TaskExecutionStatusFailed, TriggeredBy: "system", ErrorMessage: "连接超时", StartedAt: &now, FinishedAt: &now},
		{TaskID: "exec_003", TaskType: "system:cleanup", TaskName: "系统垃圾清理", Status: model.TaskExecutionStatusPending, TriggeredBy: "manual", Retryable: true, MaxRetry: 3},
	}
	for _, r := range records {
		err := repository.CreateTaskExecution(ctx, r)
		require.NoError(t, err)
	}

	t.Run("list all executions", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/tasks/executions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var data map[string]interface{}
		json.Unmarshal(dataBytes, &data)

		assert.Equal(t, float64(3), data["total"])
	})

	t.Run("filter by status", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/tasks/executions?status=failed", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var data map[string]interface{}
		json.Unmarshal(dataBytes, &data)

		assert.Equal(t, float64(1), data["total"])
	})

	t.Run("filter by task_type (asynq task name)", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/tasks/executions?task_type=system:cleanup", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var data map[string]interface{}
		json.Unmarshal(dataBytes, &data)

		assert.Equal(t, float64(3), data["total"])
	})

	t.Run("filter by task_type (management task type)", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/tasks/executions?task_type=system_cleanup", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var data map[string]interface{}
		json.Unmarshal(dataBytes, &data)

		assert.Equal(t, float64(3), data["total"])
	})

	t.Run("pagination", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/tasks/executions?page=1&page_size=2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var data map[string]interface{}
		json.Unmarshal(dataBytes, &data)

		assert.Equal(t, float64(3), data["total"])
	})
}

func TestGetTaskExecution(t *testing.T) {
	cleanup := setupTaskTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)
	ctx := context.Background()

	// 创建测试记录
	execution := &model.TaskExecution{
		TaskID:      "detail_001",
		TaskType:    "system:cleanup",
		TaskName:    "系统垃圾清理",
		Status:      model.TaskExecutionStatusSucceeded,
		Log:         "[10:00:01] 开始扫描\n[10:00:02] 找到 50 个文件\n[10:00:03] 清理完成",
		Result:      "共清理 50 个文件",
		Duration:    2000,
		Retryable:   true,
		MaxRetry:    3,
		TriggeredBy: "manual",
	}
	err := repository.CreateTaskExecution(ctx, execution)
	require.NoError(t, err)

	t.Run("get existing execution", func(t *testing.T) {
		url := fmt.Sprintf("/api/v1/admin/tasks/executions/%d", execution.ID)
		req, _ := http.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)

		dataBytes, _ := json.Marshal(resp.Data)
		var detail model.TaskExecution
		json.Unmarshal(dataBytes, &detail)

		assert.Equal(t, "detail_001", detail.TaskID)
		assert.Equal(t, model.TaskExecutionStatusSucceeded, detail.Status)
		assert.Contains(t, detail.Log, "开始扫描")
		assert.Contains(t, detail.Log, "清理完成")
		assert.Equal(t, int64(2000), detail.Duration)
	})

	t.Run("get non-existent execution", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/tasks/executions/99999999", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid ID format", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/tasks/executions/invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestRetryTask(t *testing.T) {
	cleanup := setupTaskTestEnvironment(t)
	defer cleanup()

	adminUser := &model.User{ID: 1001, Username: "admin", IsAdmin: true}
	router := setupTestRouter(adminUser)
	ctx := context.Background()

	t.Run("retry failed task successfully", func(t *testing.T) {
		now := time.Now()
		execution := &model.TaskExecution{
			TaskID:       "retry_api_001",
			TaskType:     "system:cleanup",
			TaskName:     "系统垃圾清理",
			Status:       model.TaskExecutionStatusFailed,
			ErrorMessage: "S3 连接超时",
			Retryable:    true,
			MaxRetry:     3,
			RetryCount:   0,
			TriggeredBy:  "manual",
			StartedAt:    &now,
			FinishedAt:   &now,
		}
		err := repository.CreateTaskExecution(ctx, execution)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/admin/tasks/executions/%d/retry", execution.ID)
		req, _ := http.NewRequest("POST", url, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp response.Any
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Empty(t, resp.ErrorMsg)
		assert.NotNil(t, resp.Data)

		// 验证新记录
		newTaskID, ok := resp.Data.(string)
		assert.True(t, ok)
		assert.NotEmpty(t, newTaskID)

		newExecution, err := repository.GetTaskExecutionByTaskID(ctx, newTaskID)
		require.NoError(t, err)
		assert.Equal(t, 1, newExecution.RetryCount)
		assert.Equal(t, "retry", newExecution.TriggeredBy)
	})

	t.Run("retry succeeded task fails", func(t *testing.T) {
		execution := &model.TaskExecution{
			TaskID:      "retry_succeeded_001",
			TaskType:    "system:cleanup",
			TaskName:    "系统垃圾清理",
			Status:      model.TaskExecutionStatusSucceeded,
			Retryable:   true,
			MaxRetry:    3,
			TriggeredBy: "manual",
		}
		err := repository.CreateTaskExecution(ctx, execution)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/admin/tasks/executions/%d/retry", execution.ID)
		req, _ := http.NewRequest("POST", url, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("retry non-retryable task fails", func(t *testing.T) {
		execution := &model.TaskExecution{
			TaskID:      "retry_not_allowed_001",
			TaskType:    "system:cleanup",
			TaskName:    "系统垃圾清理",
			Status:      model.TaskExecutionStatusFailed,
			Retryable:   false,
			TriggeredBy: "manual",
		}
		err := repository.CreateTaskExecution(ctx, execution)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/admin/tasks/executions/%d/retry", execution.ID)
		req, _ := http.NewRequest("POST", url, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("retry non-existent task", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/admin/tasks/executions/99999999/retry", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("retry with invalid ID", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/admin/tasks/executions/invalid/retry", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
