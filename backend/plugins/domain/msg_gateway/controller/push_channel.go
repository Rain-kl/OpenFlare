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

	"github.com/gin-gonic/gin"
)

// ListPushChannelDefinitions 获取各种消息通道的表单配置定义列表
// @Summary 获取所有消息通道配置字段定义
// @Description 返回系统支持的所有消息通道类型的动态表单定义，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "通道配置定义列表"
// @Router /api/v1/admin/push/channels/definitions [get]
func ListPushChannelDefinitions(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(do.ListPushDefinitions()))
}

// ListPushChannels 获取消息通道列表
// @Summary 获取所有消息通道
// @Description 返回系统配置的所有消息通道列表，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]entity.PushChannel} "消息通道列表"
// @Router /api/v1/admin/push/channels [get]
func ListPushChannels(c *gin.Context) {
	channels, err := service.ListPushChannels(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(channels))
}

// parsePushChannelID reads the path identifier of a push channel.
func parsePushChannelID(c *gin.Context) (uint64, bool) {
	return parseUint64Param(c, "id", consts.ErrInvalidChannelID)
}

// handlePushChannelNotFoundError maps a missing channel row to 404, others to fallback.
func handlePushChannelNotFoundError(c *gin.Context, err error, fallback func(c *gin.Context, msg string)) {
	if errors.Is(err, consts.ErrRecordNotFound) || errors.Is(err, consts.ErrChannelNotFound) || err.Error() == consts.ErrChannelNotFoundText {
		response.AbortNotFound(c, consts.ErrChannelNotFoundText)
		return
	}
	fallback(c, err.Error())
}

// CreatePushChannel 创建消息通道
// @Summary 创建消息通道
// @Description 新建一个消息通道配置，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body do.CreatePushChannelRequest true "创建参数"
// @Success 200 {object} response.Any{data=entity.PushChannel} "创建成功"
// @Router /api/v1/admin/push/channels [post]
func CreatePushChannel(c *gin.Context) {
	handleJSONRequest(c, service.CreatePushChannel)
}

// UpdatePushChannel 更新消息通道
// @Summary 更新消息通道
// @Description 修改消息通道配置，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "通道ID"
// @Param request body do.UpdatePushChannelRequest true "更新参数"
// @Success 200 {object} response.Any{data=entity.PushChannel} "更新成功"
// @Router /api/v1/admin/push/channels/{id} [put]
func UpdatePushChannel(c *gin.Context) {
	handleEntityUpdate(c, parsePushChannelID, service.UpdatePushChannel, func(c *gin.Context, err error) {
		handlePushChannelNotFoundError(c, err, response.AbortInternal)
	})
}

// DeletePushChannel 删除消息通道
// @Summary 删除消息通道
// @Description 根据ID删除消息通道，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "通道ID"
// @Success 200 {object} response.Any "删除成功"
// @Router /api/v1/admin/push/channels/{id} [delete]
func DeletePushChannel(c *gin.Context) {
	id, ok := parsePushChannelID(c)
	if !ok {
		return
	}

	if err := service.DeletePushChannel(c.Request.Context(), id); err != nil {
		handlePushChannelNotFoundError(c, err, response.AbortInternal)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// TestPushChannel 测试通道连通性
// @Summary 测试通道连通性
// @Description 触发一次临时的或现有的通道连通性推送测试，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body do.TestPushChannelRequest true "测试参数"
// @Success 200 {object} response.Any "测试触发成功"
// @Router /api/v1/admin/push/channels/test [post]
func TestPushChannel(c *gin.Context) {
	var req do.TestPushChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	payload, err := service.PreparePushChannelTest(c.Request.Context(), req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if err := service.EnqueuePushTask(c.Request.Context(), payload); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
