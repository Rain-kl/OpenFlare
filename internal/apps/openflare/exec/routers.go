// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return httpx.AbortNotFoundIfMissing(c, err, errExecTaskNotFound)
}

// ExecuteHandler dispatches a remote command to one or more edge nodes.
// @Summary 下发远程执行命令
// @Description 向选中的在线边缘节点下发 Shell 命令并返回任务 ID 与节点初始状态，需要管理员权限
// @Tags openflare-exec
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body exec.RunInput true "执行参数"
// @Success 200 {object} response.Any{data=exec.RunResponse} "任务已创建并下发"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "启动配置未启用远程执行"
// @Failure 404 {object} response.Any "无权限"
// @Router /api/v1/d/remote-exec [post]
func ExecuteHandler(c *gin.Context) {
	var input RunInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	userID := uint(oauth.GetUserIDFromContext(c))
	result, err := RunCommand(c.Request.Context(), userID, input)
	if errors.Is(err, ErrWebShellDisabled) {
		response.AbortForbidden(c, err.Error())
		return
	}
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// ListTasksHandler returns recent remote execution tasks and their node results.
// @Summary 获取远程执行历史列表
// @Description 返回最近的远程执行任务及各节点执行结果，需要管理员权限
// @Tags openflare-exec
// @Produce json
// @Security SessionCookie
// @Param limit query int false "返回数量上限（默认 50，最大 200）"
// @Success 200 {object} response.Any{data=[]exec.TaskView} "远程执行任务列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Router /api/v1/d/remote-exec/tasks [get]
func ListTasksHandler(c *gin.Context) {
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.AbortBadRequest(c, "invalid parameters")
			return
		}
		limit = parsed
	}
	tasks, err := ListTasks(c.Request.Context(), limit)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(tasks))
}

// GetTaskHandler returns one remote execution task and its node results.
// @Summary 获取远程执行任务详情
// @Description 按 task_id 返回远程执行任务及各节点的最新输出与退出码，需要管理员权限
// @Tags openflare-exec
// @Produce json
// @Security SessionCookie
// @Param task_id path string true "任务 ID"
// @Success 200 {object} response.Any{data=exec.TaskView} "远程执行任务详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或任务不存在"
// @Router /api/v1/d/remote-exec/tasks/{task_id} [get]
func GetTaskHandler(c *gin.Context) {
	taskID := c.Param("task_id")
	task, err := GetTask(c.Request.Context(), taskID)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(task))
}

// DeleteTaskHandler removes a remote execution task and its results.
// @Summary 删除远程执行任务
// @Description 按 task_id 删除远程执行记录及其各节点输出，需要管理员权限
// @Tags openflare-exec
// @Produce json
// @Security SessionCookie
// @Param task_id path string true "任务 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或任务不存在"
// @Router /api/v1/d/remote-exec/tasks/{task_id}/delete [post]
func DeleteTaskHandler(c *gin.Context) {
	taskID := c.Param("task_id")
	if err := DeleteTask(c.Request.Context(), taskID); handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
