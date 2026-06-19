// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package push defines push notification HTTP routes.
package push

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/push"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// UpdateEventRequest 更新事件请求参数
type UpdateEventRequest struct {
	Channels []string `json:"channels"`
	Targets  []string `json:"targets"`
	Template string   `json:"template" binding:"required"`
	Enabled  bool     `json:"enabled"`
}

// TestPushRequest 测试推送通道请求参数
type TestPushRequest struct {
	Config push.Config `json:"config" binding:"required"`
	Target string      `json:"target"`
}

// SyncEvents automatically registers/updates built-in events in the database.
func SyncEvents(ctx context.Context) error {
	return syncBuiltInEvents(ctx)
}

// ListEvents 获取通知事件列表
// @Summary 获取所有通知事件
// @Description 返回系统配置的通知事件列表，包括预置和自定义事件，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.PushEvent} "通知事件列表"
// @Router /api/v1/admin/push/events [get]
func ListEvents(c *gin.Context) {
	ctx := c.Request.Context()
	events, err := listPushEvents(ctx)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(events))
}

// CreateEventRequest 创建事件请求参数
type CreateEventRequest struct {
	EventKey string   `json:"event_key"`
	TaskType string   `json:"task_type"` // 关联的异步任务类型
	Channels []string `json:"channels"`
	Targets  []string `json:"targets"`
	Template string   `json:"template"`
	Enabled  bool     `json:"enabled"`
}

func findBuiltInEvent(key string) (EventMetadata, bool) {
	for _, meta := range BuiltInEvents {
		if meta.Key == key {
			return meta, true
		}
	}
	return EventMetadata{}, false
}

// ListBuiltInEvents 获取内置通知事件列表
// @Summary 获取所有内置通知事件
// @Description 返回系统定义的所有内置通知事件元数据，供前端下拉框选择，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]EventMetadata} "内置通知事件列表"
// @Router /api/v1/admin/push/events/builtin [get]
func ListBuiltInEvents(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(BuiltInEvents))
}

// CreateEvent 创建通知事件
// @Summary 创建通知事件
// @Description 绑定系统内置事件或异步任务、推送渠道、接收目标并创建通知事件配置，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body CreateEventRequest true "创建参数"
// @Success 200 {object} response.Any{data=model.PushEvent} "创建成功"
// @Router /api/v1/admin/push/events [post]
func CreateEvent(c *gin.Context) {
	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	event, err := createPushEvent(c.Request.Context(), req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(event))
}

// DeleteEvent 删除通知事件配置
// @Summary 删除通知事件配置
// @Description 删除数据库中的特定通知事件配置，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Param id path int true "事件 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Router /api/v1/admin/push/events/{id} [delete]
func DeleteEvent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid event id")
		return
	}

	if err := deletePushEvent(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "notification event not found")
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// UpdateEvent 更新通知事件
// @Summary 更新通知事件
// @Description 更新已有通知事件的推送渠道、接收目标和内容模板，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "事件 ID"
// @Param request body push.UpdateEventRequest true "更新参数"
// @Success 200 {object} response.Any{data=string} "修改成功"
// @Router /api/v1/admin/push/events/{id} [put]
func UpdateEvent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid event id")
		return
	}

	var req UpdateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := updatePushEvent(c.Request.Context(), id, req); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "notification event not found")
			return
		}
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ToggleEvent 快捷切换通知事件启用状态
// @Summary 快捷切换通知事件启用状态
// @Description 启用或禁用指定的通知事件
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Param id path int true "事件 ID"
// @Success 200 {object} response.Any{data=string} "切换成功"
// @Router /api/v1/admin/push/events/{id}/toggle [post]
func ToggleEvent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid event id")
		return
	}

	enabled, err := togglePushEvent(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "notification event not found")
			return
		}
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(enabled))
}

// pushHistoriesResponse 推送历史分页响应
//
//nolint:unused
type pushHistoriesResponse struct {
	Total   int64               `json:"total"`
	Results []model.PushHistory `json:"results"`
}

// ListHistories 分页获取通知推送历史
// @Summary 分页获取通知推送历史
// @Description 返回分页的通知历史日志数据，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Param page query int false "当前页码"
// @Param page_size query int false "分页大小"
// @Param event_key query string false "过滤事件名称"
// @Param status query string false "过滤发送状态"
// @Success 200 {object} response.Any{data=pushHistoriesResponse} "推送历史列表"
// @Router /api/v1/admin/push/histories [get]
func ListHistories(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	total, results, err := listPushHistories(c.Request.Context(), repository.PushHistoryListFilter{
		EventKey: c.Query("event_key"),
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
// @Param request body push.TestPushRequest true "测试请求体"
// @Success 200 {object} response.Any{data=string} "测试成功"
// @Router /api/v1/admin/push/test [post]
func TestPush(c *gin.Context) {
	var req TestPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	pusher, err := push.GetPusher(req.Config.Channel)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if err := pusher.ValidateConfig(req.Config); err != nil {
		response.AbortBadRequest(c, fmt.Sprintf("validation failed: %v", err))
		return
	}

	applySMTPFallbackToPushConfig(c.Request.Context(), &req.Config)

	testBody := map[string]any{
		keyTitle:   "测试通道推送",
		keyContent: "当您收到这条消息，说明当前渠道连通性测试通过。",
		keyLevel:   defaultLevelInfo,
	}
	if err := pusher.Send(c.Request.Context(), req.Config, req.Target, testBody, "", nil); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
