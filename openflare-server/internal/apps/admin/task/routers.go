// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/internal/task/scheduler"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// ListTaskTypes 获取支持的任务类型列表
// @Summary 获取支持的任务类型
// @Description 返回系统支持的所有可调度任务类型列表，包括任务名称、描述、是否支持时间范围等元数据，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]task.TaskMeta} "任务类型列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/tasks/types [get]
func ListTaskTypes(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(task.GetDispatchableTasks()))
}

// DispatchTaskRequest 下发任务请求
type DispatchTaskRequest struct {
	TaskType  string     `json:"task_type" binding:"required"`
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	UserID    *uint64    `json:"user_id"`
	Payload   string     `json:"payload"`
}

// DispatchTask 下发任务
// @Summary 下发异步任务
// @Description 手动触发指定类型的异步任务，支持指定时间范围和用户，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body DispatchTaskRequest true "任务请求参数"
// @Success 200 {object} response.Any{data=string} "任务已入队"
// @Failure 400 {object} response.Any "任务类型不存在或参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "任务入队失败"
// @Router /api/v1/admin/tasks/dispatch [post]
func DispatchTask(c *gin.Context) {
	var req DispatchTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	meta := task.GetTaskMeta(req.TaskType)
	if meta == nil {
		response.AbortBadRequest(c, InvalidTaskType)
		return
	}

	var payloadBytes []byte
	if strings.TrimSpace(req.Payload) != "" {
		payloadBytes = []byte(req.Payload)
	}

	validated, err := task.ValidateAndNormalizePayload(meta.AsynqTask, payloadBytes)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	taskID, err := task.DispatchTask(c.Request.Context(), req.TaskType, validated, "manual")
	if err != nil {
		response.AbortInternal(c, fmt.Sprintf("%s: %v", TaskDispatchFailed, err))
		return
	}

	c.JSON(http.StatusOK, response.OK(taskID))
}

// ListTaskExecutions 查询任务执行记录列表
// @Summary 查询任务执行记录
// @Description 分页查询任务执行记录，支持按状态和任务类型筛选，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param status query string false "状态筛选 (pending/running/succeeded/failed)"
// @Param task_type query string false "任务类型筛选"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.Any{data=object} "任务执行记录列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/tasks/executions [get]
func ListTaskExecutions(c *gin.Context) {
	var req model.ListTaskExecutionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if req.TaskType != "" {
		if meta := task.GetTaskMeta(req.TaskType); meta != nil {
			req.TaskType = meta.AsynqTask
		}
	}

	executions, total, err := model.ListTaskExecutions(c.Request.Context(), req)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(gin.H{
		"items":     executions,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	}))
}

// GetTaskExecution 查询单条任务执行详情
// @Summary 查询任务执行详情
// @Description 根据 ID 查询任务执行记录详情，包含完整执行日志，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param id path int true "任务执行记录 ID"
// @Success 200 {object} response.Any{data=model.TaskExecution} "任务执行详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "记录不存在"
// @Router /api/v1/admin/tasks/executions/{id} [get]
func GetTaskExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, admin.InvalidTaskExecutionID)
		return
	}

	execution, err := model.GetTaskExecutionByID(c.Request.Context(), id)
	if err != nil {
		response.AbortNotFound(c, TaskNotFound)
		return
	}

	c.JSON(http.StatusOK, response.OK(execution))
}

// RetryTask 重试失败的任务
// @Summary 重试失败任务
// @Description 重新下发一条失败的任务，创建新的执行记录，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param id path int true "任务执行记录 ID"
// @Success 200 {object} response.Any{data=string} "新任务的 TaskID"
// @Failure 400 {object} response.Any "任务不支持重试或参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "记录不存在"
// @Failure 500 {object} response.Any "重试失败"
// @Router /api/v1/admin/tasks/executions/{id}/retry [post]
func RetryTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, admin.InvalidTaskExecutionID)
		return
	}

	newTaskID, err := task.RetryTask(c.Request.Context(), id)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "不存在"):
			response.AbortNotFound(c, errMsg)
		case strings.Contains(errMsg, "只有失败的任务") || strings.Contains(errMsg, "不支持重试") || strings.Contains(errMsg, "已达到最大重试"):
			response.AbortBadRequest(c, errMsg)
		default:
			response.AbortInternal(c, fmt.Sprintf("%s: %v", TaskRetryFailed, err))
		}
		return
	}

	c.JSON(http.StatusOK, response.OK(newTaskID))
}

// ListSchedules 获取定时任务列表
// @Summary 获取定时任务列表
// @Description 返回系统所有的定时任务配置列表，包括名称、关联的异步任务类型、Cron 表达式和启用状态，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.Schedule} "定时任务列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/tasks/schedules [get]
func ListSchedules(c *gin.Context) {
	schedules, err := model.ListSchedules(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(schedules))
}

// CreateScheduleRequest 创建定时任务请求
type CreateScheduleRequest struct {
	Name     string `json:"name" binding:"required"`
	TaskType string `json:"task_type" binding:"required"`
	Cron     string `json:"cron" binding:"required"`
	Payload  string `json:"payload"`
	IsActive *bool  `json:"is_active" binding:"required"`
}

// CreateSchedule 创建定时任务
// @Summary 创建定时任务
// @Description 新增一个动态定时任务配置，关联已有的异步任务，配置 Cron 表达式和执行参数，并触发调度器热加载，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body CreateScheduleRequest true "创建定时任务请求参数"
// @Success 200 {object} response.Any{data=model.Schedule} "创建成功的定时任务信息"
// @Failure 400 {object} response.Any "Cron 表达式无效、异步任务类型不存在或参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "保存定时任务失败"
// @Router /api/v1/admin/tasks/schedules [post]
func CreateSchedule(c *gin.Context) {
	var req CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	// 校验 Cron 表达式
	if _, err := cron.ParseStandard(req.Cron); err != nil {
		response.AbortBadRequest(c, InvalidCronExpression)
		return
	}

	// 校验关联的异步任务类型
	meta := task.GetTaskMeta(req.TaskType)
	if meta == nil {
		response.AbortBadRequest(c, InvalidTaskType)
		return
	}

	// 校验并规范化 Payload
	var payloadBytes []byte
	if strings.TrimSpace(req.Payload) != "" {
		payloadBytes = []byte(req.Payload)
	}
	validated, err := task.ValidateAndNormalizePayload(meta.AsynqTask, payloadBytes)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	schedule := &model.Schedule{
		Name:     req.Name,
		TaskType: req.TaskType,
		Cron:     req.Cron,
		Payload:  string(validated),
		IsActive: *req.IsActive,
	}

	if err := model.CreateSchedule(c.Request.Context(), schedule); err != nil {
		response.AbortInternal(c, fmt.Sprintf("%s: %v", ScheduleSaveFailed, err))
		return
	}

	// 触发调度服务重载
	if err := scheduler.ReloadScheduler(); err != nil {
		logger.ErrorF(c.Request.Context(), "[TaskAdmin] 重载调度器失败: %v", err)
	}

	c.JSON(http.StatusOK, response.OK(schedule))
}

// UpdateScheduleRequest 修改定时任务请求
type UpdateScheduleRequest struct {
	Name     string `json:"name" binding:"required"`
	TaskType string `json:"task_type" binding:"required"`
	Cron     string `json:"cron" binding:"required"`
	Payload  string `json:"payload"`
	IsActive *bool  `json:"is_active" binding:"required"`
}

// UpdateSchedule 修改定时任务
// @Summary 修改定时任务
// @Description 修改一个定时任务的配置（名称、Cron 表达式、异步任务参数和是否启用等），并触发调度器热加载，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "定时任务 ID"
// @Param request body UpdateScheduleRequest true "修改定时任务请求参数"
// @Success 200 {object} response.Any{data=model.Schedule} "修改后的定时任务信息"
// @Failure 400 {object} response.Any "Cron 表达式无效、参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "定时任务不存在"
// @Failure 500 {object} response.Any "修改定时任务失败"
// @Router /api/v1/admin/tasks/schedules/{id} [put]
func UpdateSchedule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "无效的定时任务ID")
		return
	}

	var req UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	// 检查定时任务是否存在
	schedule, err := model.GetScheduleByID(c.Request.Context(), id)
	if err != nil {
		response.AbortNotFound(c, ScheduleNotFound)
		return
	}

	// 校验 Cron 表达式
	if _, err := cron.ParseStandard(req.Cron); err != nil {
		response.AbortBadRequest(c, InvalidCronExpression)
		return
	}

	// 校验关联的异步任务类型
	meta := task.GetTaskMeta(req.TaskType)
	if meta == nil {
		response.AbortBadRequest(c, InvalidTaskType)
		return
	}

	// 校验并规范化 Payload
	var payloadBytes []byte
	if strings.TrimSpace(req.Payload) != "" {
		payloadBytes = []byte(req.Payload)
	}
	validated, err := task.ValidateAndNormalizePayload(meta.AsynqTask, payloadBytes)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	schedule.Name = req.Name
	schedule.TaskType = req.TaskType
	schedule.Cron = req.Cron
	schedule.Payload = string(validated)
	schedule.IsActive = *req.IsActive

	if err := model.UpdateSchedule(c.Request.Context(), schedule); err != nil {
		response.AbortInternal(c, fmt.Sprintf("%s: %v", ScheduleSaveFailed, err))
		return
	}

	// 触发调度服务重载
	if err := scheduler.ReloadScheduler(); err != nil {
		logger.ErrorF(c.Request.Context(), "[TaskAdmin] 重载调度器失败: %v", err)
	}

	c.JSON(http.StatusOK, response.OK(schedule))
}

// DeleteSchedule 删除定时任务
// @Summary 删除定时任务
// @Description 删除指定的定时任务配置，并触发调度器热加载，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param id path int true "定时任务 ID"
// @Success 200 {object} response.Any{data=string} "删除结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "删除定时任务失败"
// @Router /api/v1/admin/tasks/schedules/{id} [delete]
func DeleteSchedule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "无效的定时任务ID")
		return
	}

	if err := model.DeleteSchedule(c.Request.Context(), id); err != nil {
		response.AbortInternal(c, fmt.Sprintf("%s: %v", ScheduleDeleteFailed, err))
		return
	}

	// 触发调度服务重载
	if err := scheduler.ReloadScheduler(); err != nil {
		logger.ErrorF(c.Request.Context(), "[TaskAdmin] 重载调度器失败: %v", err)
	}

	c.JSON(http.StatusOK, response.OKNil())
}
