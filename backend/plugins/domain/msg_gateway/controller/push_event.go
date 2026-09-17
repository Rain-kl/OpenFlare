// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/service"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListPushEvents 获取通知事件列表
// @Summary 获取所有通知事件
// @Description 返回系统配置的通知事件列表，包括预置和自定义事件，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]entity.PushEvent} "通知事件列表"
// @Router /api/v1/admin/push/events [get]
func ListPushEvents(c *gin.Context) {
	ctx := c.Request.Context()
	events, err := service.ListPushEvents(ctx)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(events))
}

// ListBuiltInPushEvents 获取内置通知事件列表
// @Summary 获取所有内置通知事件
// @Description 返回系统定义的所有内置通知事件元数据，供前端下拉框选择，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "内置通知事件列表"
// @Router /api/v1/admin/push/events/builtin [get]
func ListBuiltInPushEvents(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(service.GetBuiltInEvents()))
}

// parsePushEventID reads the path identifier of a push event.
func parsePushEventID(c *gin.Context) (uint64, bool) {
	return parseUint64Param(c, "id", consts.ErrInvalidEventID)
}

// handlePushEventNotFoundError maps a missing event row to 404, others to fallback.
func handlePushEventNotFoundError(c *gin.Context, err error, fallback func(c *gin.Context, msg string)) {
	if errors.Is(err, consts.ErrRecordNotFound) || errors.Is(err, consts.ErrEventNotFound) || err.Error() == consts.ErrEventNotFound.Error() {
		response.AbortNotFound(c, consts.ErrEventNotFound.Error())
		return
	}
	fallback(c, err.Error())
}

// CreatePushEvent 创建通知事件
// @Summary 创建通知事件
// @Description 绑定系统内置事件或异步任务、推送渠道、接收目标并创建通知事件配置，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body do.CreatePushEventRequest true "创建参数"
// @Success 200 {object} response.Any{data=entity.PushEvent} "创建成功"
// @Router /api/v1/admin/push/events [post]
func CreatePushEvent(c *gin.Context) {
	handleJSONRequest(c, service.CreatePushEvent)
}

// DeletePushEvent 删除通知事件配置
// @Summary 删除通知事件配置
// @Description 删除数据库中的特定通知事件配置，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Param id path int true "事件 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Router /api/v1/admin/push/events/{id} [delete]
func DeletePushEvent(c *gin.Context) {
	id, ok := parsePushEventID(c)
	if !ok {
		return
	}

	if err := service.DeletePushEvent(c.Request.Context(), id); err != nil {
		handlePushEventNotFoundError(c, err, response.AbortInternal)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// UpdatePushEvent 更新通知事件
// @Summary 更新通知事件
// @Description 更新已有通知事件的推送渠道、接收目标和内容模板，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "事件 ID"
// @Param request body do.UpdatePushEventRequest true "更新参数"
// @Success 200 {object} response.Any{data=string} "修改成功"
// @Router /api/v1/admin/push/events/{id} [put]
func UpdatePushEvent(c *gin.Context) {
	id, ok := parsePushEventID(c)
	if !ok {
		return
	}

	var req do.UpdatePushEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := service.UpdatePushEvent(c.Request.Context(), id, req); err != nil {
		handlePushEventNotFoundError(c, err, response.AbortBadRequest)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// TogglePushEvent 快捷切换通知事件启用状态
// @Summary 快捷切换通知事件启用状态
// @Description 启用或禁用指定的通知事件
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Param id path int true "事件 ID"
// @Success 200 {object} response.Any{data=string} "切换成功"
// @Router /api/v1/admin/push/events/{id}/toggle [post]
func TogglePushEvent(c *gin.Context) {
	id, ok := parsePushEventID(c)
	if !ok {
		return
	}

	enabled, err := service.TogglePushEvent(c.Request.Context(), id)
	if err != nil {
		handlePushEventNotFoundError(c, err, response.AbortBadRequest)
		return
	}
	c.JSON(http.StatusOK, response.OK(enabled))
}

// ListPushHistories 分页获取通知推送历史
// @Summary 分页获取通知推送历史
// @Description 返回分页的通知历史日志数据，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Param page query int false "当前页码"
// @Param page_size query int false "分页大小"
// @Param event_key query string false "过滤事件名称"
// @Param status query string false "过滤发送状态"
// @Success 200 {object} response.Any "推送历史列表"
// @Router /api/v1/admin/push/histories [get]
func ListPushHistories(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	total, results, err := service.ListPushHistories(c.Request.Context(), do.PushHistoryListFilter{
		EventKey: c.Query("event_key"),
		Channel:  c.Query("channel"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(map[string]any{
		"total":   total,
		"results": results,
	}))
}

// TestPush 测试推送通道发送
// @Summary 测试推送通道发送
// @Description 接收临时通知渠道配置并在本地同步调用 Pusher.Send 发送测试消息
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body do.TestPushRequest true "测试请求体"
// @Success 200 {object} response.Any{data=string} "测试成功"
// @Router /api/v1/admin/push/test [post]
func TestPush(c *gin.Context) {
	var req do.TestPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := service.RunPushTest(c.Request.Context(), req.Config, req.Target); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
