// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// GetStatusHandler 获取公开运行状态。
// @Summary 获取 OpenFlare 公开状态
// @Description 返回版本、认证源与系统公开配置，无需登录
// @Tags openflare-option
// @Produce json
// @Success 200 {object} response.Any{data=option.statusView} "公开状态"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/status [get]
func GetStatusHandler(c *gin.Context) {
	view := getStatus(c.Request.Context(), "/api/v1/d")
	c.JSON(http.StatusOK, response.OK(view))
}

// ListOptionsHandler 列出全部配置项。
// @Summary 列出 OpenFlare 配置项
// @Description 返回全部非敏感 OpenFlare 配置项，需要管理员权限
// @Tags openflare-option
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.OpenFlareOption} "配置项列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option [get]
func ListOptionsHandler(c *gin.Context) {
	options, err := listOptions(c.Request.Context())
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(options))
}

// UpdateOptionHandler 更新单个配置项。
// @Summary 更新 OpenFlare 配置项
// @Description 更新单个 OpenFlare 配置项，需要管理员权限
// @Tags openflare-option
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body model.OpenFlareOption true "配置项"
// @Success 200 {object} response.Any "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option/update [post]
func UpdateOptionHandler(c *gin.Context) {
	var option model.OpenFlareOption
	if !apiutil.BindJSON(c, &option) {
		return
	}
	if apiutil.AbortBadRequestOnError(c, updateOption(c.Request.Context(), option)) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// UpdateOptionsBatchHandler 批量更新配置项。
// @Summary 批量更新 OpenFlare 配置项
// @Description 批量更新多个 OpenFlare 配置项，需要管理员权限
// @Tags openflare-option
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body option.optionBatchPayload true "批量配置项"
// @Success 200 {object} response.Any "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option/update-batch [post]
func UpdateOptionsBatchHandler(c *gin.Context) {
	var payload optionBatchPayload
	if !apiutil.BindJSON(c, &payload) {
		return
	}
	if apiutil.AbortBadRequestOnError(c, updateOptionsBatch(c.Request.Context(), payload)) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// LookupGeoIPHandler 查询 GeoIP 信息。
// @Summary GeoIP 地址查询
// @Description 按提供商与 IP 查询地理位置信息，需要管理员权限
// @Tags openflare-option
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body option.geoIPLookupRequest true "查询参数"
// @Success 200 {object} response.Any{data=option.geoIPLookupView} "GeoIP 查询结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option/geoip/lookup [post]
func LookupGeoIPHandler(c *gin.Context) {
	var request geoIPLookupRequest
	if !apiutil.BindJSON(c, &request) {
		return
	}
	view, err := lookupGeoIP(c.Request.Context(), request.Provider, request.IP)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// SyncUptimeKumaHandler 同步 Uptime Kuma 监控。
// @Summary 同步 Uptime Kuma
// @Description 将 OpenFlare 节点同步到 Uptime Kuma，需要管理员权限
// @Tags openflare-option
// @Accept json
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=string} "同步成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/uptimekuma/sync [post]
func SyncUptimeKumaHandler(c *gin.Context) {
	if apiutil.AbortBadRequestOnError(c, syncUptimeKuma(c.Request.Context())) {
		return
	}
	c.JSON(http.StatusOK, response.OK("同步成功"))
}
