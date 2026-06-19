// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	pkgpush "github.com/Rain-kl/Wavelet/pkg/push"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// ListChannelDefinitions 获取各种消息通道的表单配置定义列表
// @Summary 获取所有消息通道配置字段定义
// @Description 返回系统支持的所有消息通道类型（如飞书、邮件、自定义、Telegram）的动态表单定义，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]Definition} "通道配置定义列表"
// @Router /api/v1/admin/push/channels/definitions [get]
func ListChannelDefinitions(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(ListDefinitions()))
}

// ListChannels 获取消息通道列表
// @Summary 获取所有消息通道
// @Description 返回系统配置的所有消息通道列表，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.PushChannel} "消息通道列表"
// @Router /api/v1/admin/push/channels [get]
func ListChannels(c *gin.Context) {
	channels, err := listPushChannels(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(channels))
}

// CreateChannelRequest 创建通道参数
type CreateChannelRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Type        string `json:"type" binding:"required"`
	Token       string `json:"token"`
	URL         string `json:"url"`
	Other       string `json:"other"`
	Enabled     bool   `json:"enabled"`
}

// CreateChannel 创建消息通道
// @Summary 创建消息通道
// @Description 新建一个消息通道配置，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body CreateChannelRequest true "创建参数"
// @Success 200 {object} response.Any{data=model.PushChannel} "创建成功"
// @Router /api/v1/admin/push/channels [post]
func CreateChannel(c *gin.Context) {
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	channel, err := createPushChannel(c.Request.Context(), req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(channel))
}

// UpdateChannelRequest 修改通道参数
type UpdateChannelRequest struct {
	Description string `json:"description"`
	Type        string `json:"type" binding:"required"`
	Token       string `json:"token"`
	URL         string `json:"url"`
	Other       string `json:"other"`
	Enabled     bool   `json:"enabled"`
}

// UpdateChannel 更新消息通道
// @Summary 更新消息通道
// @Description 修改消息通道配置，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "通道ID"
// @Param request body UpdateChannelRequest true "更新参数"
// @Success 200 {object} response.Any{data=model.PushChannel} "更新成功"
// @Router /api/v1/admin/push/channels/{id} [put]
func UpdateChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid channel id")
		return
	}

	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	channel, err := updatePushChannel(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "channel not found")
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(channel))
}

// DeleteChannel 删除消息通道
// @Summary 删除消息通道
// @Description 根据ID删除消息通道，需要管理员权限
// @Tags admin-push
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "通道ID"
// @Success 200 {object} response.Any "删除成功"
// @Router /api/v1/admin/push/channels/{id} [delete]
func DeleteChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid channel id")
		return
	}

	if err := deletePushChannel(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "channel not found")
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// TestChannelRequest 测试通道连通性参数
type TestChannelRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Token  string `json:"token"`
	URL    string `json:"url"`
	Other  string `json:"other"`
	Target string `json:"target"`
}

// TestChannel 测试通道连通性
// @Summary 测试通道连通性
// @Description 触发一次临时的或现有的通道连通性推送测试，需要管理员权限
// @Tags admin-push
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body TestChannelRequest true "测试参数"
// @Success 200 {object} response.Any "测试触发成功"
// @Router /api/v1/admin/push/channels/test [post]
func TestChannel(c *gin.Context) {
	var req TestChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	url, token, other, channelType, err := loadChannelForTest(ctx, req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if channelType == channelEmail {
		url, token, other = resolveSMTPConfig(ctx, url, token, other)
	}

	tempChannel := model.PushChannel{
		Name:    "test_temp",
		URL:     url,
		Token:   token,
		Other:   other,
		Type:    channelType,
		Enabled: true,
	}
	if err := tempChannel.Validate(); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	url = tempChannel.URL

	var config pkgpush.Config
	var renderedJSON string
	switch channelType {
	case channelLark:
		config = pkgpush.Config{Channel: channelLark, URL: url, Secret: token}
		renderedJSON = other
	case channelEmail:
		config = pkgpush.Config{Channel: channelEmail, URL: url, Key: token, Secret: other}
	case channelTelegram:
		config = pkgpush.Config{Channel: channelTelegram, URL: url, Secret: token, Key: other}
	default:
		config = pkgpush.Config{Channel: channelCustom, URL: url}
		customPushReq := CustomPushRequest{
			Title:       "通道测试通知",
			Content:     "这是一条来自系统的消息通道连通性测试消息。",
			Description: "系统通道测试",
			URL:         "https://example.com",
			To:          req.Target,
		}
		renderedJSON = renderCustomPayload(other, customPushReq)
	}

	payload := SendPayload{
		EventKey: "test_channel",
		Config:   config,
		Target:   req.Target,
		Body: NotificationMessage{
			Title:   "通道测试通知",
			Content: "这是一条来自系统的消息通道连通性测试消息。",
			Level:   defaultLevelInfo,
		},
		Template: renderedJSON,
	}
	if err := enqueuePushTask(ctx, payload); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// CustomPushRequest 外部公开推送请求参数
type CustomPushRequest struct {
	Title       string `json:"title" form:"title"`
	Description string `json:"description" form:"description"`
	Content     string `json:"content" form:"content"`
	URL         string `json:"url" form:"url"`
	To          string `json:"to" form:"to"`
	Token       string `json:"token" form:"token"`
}

func escapeJSONString(s string) string {
	b, _ := json.Marshal(s)
	const minJSONLen = 2
	if len(b) >= minJSONLen {
		return string(b[1 : len(b)-1])
	}
	return s
}

func renderCustomPayload(template string, req CustomPushRequest) string {
	result := template
	result = strings.ReplaceAll(result, "$title", escapeJSONString(req.Title))
	result = strings.ReplaceAll(result, "$description", escapeJSONString(req.Description))
	result = strings.ReplaceAll(result, "$content", escapeJSONString(req.Content))
	result = strings.ReplaceAll(result, "$url", escapeJSONString(req.URL))
	result = strings.ReplaceAll(result, "$to", escapeJSONString(req.To))
	return result
}
