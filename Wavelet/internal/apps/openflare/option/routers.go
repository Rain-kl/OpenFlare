// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
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
	view, err := getStatus(c.Request.Context(), "/api/v1/d")
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(view))
}

// getNoticeHandler 获取系统公告。
// @Summary 获取系统公告
// @Description 返回 OpenFlare 控制台公告文本，无需登录
// @Tags openflare-option
// @Produce json
// @Success 200 {object} response.Any{data=string} "系统公告"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/notice [get]
// GetNoticeHandler returns the notice content.
func GetNoticeHandler(c *gin.Context) {
	notice, err := getNotice(c.Request.Context())
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(notice))
}

// listOptionsHandler 列出全部配置项。
// @Summary 列出 OpenFlare 配置项
// @Description 返回全部非敏感 OpenFlare 配置项，需要管理员权限
// @Tags openflare-option
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.OpenFlareOption} "配置项列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option [get]
// ListOptionsHandler lists OpenFlare options.
func ListOptionsHandler(c *gin.Context) {
	options, err := listOptions(c.Request.Context())
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(options))
}

// updateOptionHandler 更新单个配置项。
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
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option/update [post]
// UpdateOptionHandler updates a single option.
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

// updateOptionsBatchHandler 批量更新配置项。
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
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option/update-batch [post]
// UpdateOptionsBatchHandler updates options in batch.
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

// lookupGeoIPHandler 查询 GeoIP 信息。
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
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option/geoip/lookup [post]
// LookupGeoIPHandler performs a GeoIP lookup.
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

// cleanupDatabaseHandler 清理可观测性数据库数据。
// @Summary 清理可观测性数据库
// @Description 按目标与保留天数清理可观测性相关数据表，需要管理员权限
// @Tags openflare-option
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body option.databaseCleanupInput false "清理参数"
// @Success 200 {object} response.Any{data=option.databaseCleanupResult} "清理结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/option/database/cleanup [post]
// CleanupDatabaseHandler cleans up observability data.
func CleanupDatabaseHandler(c *gin.Context) {
	var input databaseCleanupInput
	if err := bindOptionalJSON(c.Request.Body, &input); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}
	result, err := cleanupDatabaseObservability(c.Request.Context(), input)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// syncUptimeKumaHandler 同步 Uptime Kuma 监控。
// @Summary 同步 Uptime Kuma
// @Description 将 OpenFlare 节点同步到 Uptime Kuma，需要管理员权限
// @Tags openflare-option
// @Accept json
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=string} "同步成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/uptimekuma/sync [post]
// SyncUptimeKumaHandler triggers UptimeKuma sync.
func SyncUptimeKumaHandler(c *gin.Context) {
	if apiutil.AbortBadRequestOnError(c, syncUptimeKuma(c.Request.Context())) {
		return
	}
	c.JSON(http.StatusOK, response.OK("同步成功"))
}

func bindOptionalJSON(body io.Reader, target any) error {
	if err := json.NewDecoder(body).Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}