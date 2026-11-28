// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ping

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return httpx.AbortNotFoundIfMissing(c, err, errPingTaskNotFound)
}

// ListTasksHandler returns all configured latency probe tasks.
// @Summary 获取延迟检测任务列表
// @Description 返回按权重排序的全部延迟检测任务，需要管理员权限
// @Tags openflare-ping
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.OpenFlarePingTask} "延迟检测任务列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Router /api/v1/d/ping-tasks [get]
func ListTasksHandler(c *gin.Context) {
	tasks, err := ListTasks(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(tasks))
}

// CreateTaskHandler creates a latency probe task.
// @Summary 创建延迟检测任务
// @Description 创建新的 ICMP/TCP/HTTP 延迟检测任务并触发调度重载，需要管理员权限
// @Tags openflare-ping
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body ping.TaskInput true "任务参数"
// @Success 200 {object} response.Any{data=model.OpenFlarePingTask} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Router /api/v1/d/ping-tasks [post]
func CreateTaskHandler(c *gin.Context) {
	var input TaskInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	task, err := CreateTask(c.Request.Context(), input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(task))
}

// UpdateTaskHandler updates one latency probe task.
// @Summary 更新延迟检测任务
// @Description 更新指定 ID 的延迟检测任务并重载调度，需要管理员权限
// @Tags openflare-ping
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "任务 ID"
// @Param body body ping.TaskInput true "任务参数"
// @Success 200 {object} response.Any{data=model.OpenFlarePingTask} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或任务不存在"
// @Router /api/v1/d/ping-tasks/{id}/update [post]
func UpdateTaskHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input TaskInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	task, err := UpdateTask(c.Request.Context(), id, input)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(task))
}

// BatchUpdateTasksHandler batch updates latency probe tasks.
// @Summary 批量更新延迟检测任务
// @Description 在单个事务内更新多个延迟检测任务（如按节点视图批量调整绑定关系）并重载调度，需要管理员权限
// @Tags openflare-ping
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body ping.BatchUpdateTasksInput true "批量更新参数"
// @Success 200 {object} response.Any{data=[]model.OpenFlarePingTask} "更新后的任务列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或任务不存在"
// @Router /api/v1/d/ping-tasks/batch-update [post]
func BatchUpdateTasksHandler(c *gin.Context) {
	var input BatchUpdateTasksInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	tasks, err := UpdateTasks(c.Request.Context(), input.Tasks)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(tasks))
}

// DeleteTaskHandler deletes a single latency probe task.
// @Summary 删除延迟检测任务
// @Description 删除指定 ID 的延迟检测任务及其历史采样记录，需要管理员权限
// @Tags openflare-ping
// @Produce json
// @Security SessionCookie
// @Param id path int true "任务 ID"
// @Success 200 {object} response.Any{data=ping.DeleteTasksResult} "删除结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Router /api/v1/d/ping-tasks/{id}/delete [post]
func DeleteTaskHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	deleted, err := DeleteTasks(c.Request.Context(), []uint{id})
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(DeleteTasksResult{Deleted: deleted}))
}

// BatchDeleteTasksHandler deletes multiple latency probe tasks.
// @Summary 批量删除延迟检测任务
// @Description 按 ID 列表批量删除延迟检测任务及其历史采样记录，需要管理员权限
// @Tags openflare-ping
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body ping.DeleteTasksInput true "任务 ID 列表"
// @Success 200 {object} response.Any{data=ping.DeleteTasksResult} "删除结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Router /api/v1/d/ping-tasks/delete [post]
func BatchDeleteTasksHandler(c *gin.Context) {
	var input DeleteTasksInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	deleted, err := DeleteTasks(c.Request.Context(), input.IDs)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(DeleteTasksResult{Deleted: deleted}))
}

// ReorderTasksHandler updates display weights for latency probe tasks.
// @Summary 调整延迟检测任务排序
// @Description 按任务 ID 到权重值的映射批量更新排序权重，需要管理员权限
// @Tags openflare-ping
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body map[string]int true "任务 ID 到排序权重的映射"
// @Success 200 {object} response.Any "排序更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或任务不存在"
// @Router /api/v1/d/ping-tasks/order [post]
func ReorderTasksHandler(c *gin.Context) {
	var raw map[string]int
	if !httpx.BindJSON(c, &raw) {
		return
	}
	weights := make(map[uint]int, len(raw))
	for rawID, weight := range raw {
		parsed, err := httpx.ParseUint(rawID)
		if err != nil || parsed == 0 {
			response.AbortBadRequest(c, errPingTaskWeightIDInvalid)
			return
		}
		weights[parsed] = weight
	}
	if err := ReorderTasks(c.Request.Context(), weights); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// GetNodePingRecordsHandler returns latency probe statistics and series for a node.
// @Summary 获取节点延迟检测记录与统计
// @Description 返回指定节点关联的延迟检测任务统计（最小/最大/均值/P50/P99/丢包率）及降采样时序记录，需要管理员权限
// @Tags openflare-ping
// @Produce json
// @Security SessionCookie
// @Param id path int true "节点 ID"
// @Param task_id query int false "按任务 ID 过滤"
// @Param hours query int false "查询时间范围（小时，默认 1，最大 720）"
// @Param max_points query int false "每任务最大返回采样点数（默认 300）"
// @Success 200 {object} response.Any{data=ping.NodePingRecordsView} "节点延迟统计与时序"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或节点不存在"
// @Router /api/v1/d/nodes/{id}/ping-records [get]
func GetNodePingRecordsHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var query NodePingRecordsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.AbortBadRequest(c, "invalid parameters")
		return
	}
	view, err := GetNodePingRecords(c.Request.Context(), id, query)
	if err != nil {
		httpx.AbortNotFoundIfMissing(c, err, errNodeNotFound)
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}
