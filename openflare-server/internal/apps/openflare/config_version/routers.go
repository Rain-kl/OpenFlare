// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)


func handleLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	return apiutil.AbortNotFoundIfMissing(c, err, "记录不存在")
}

// ListConfigVersionsHandler lists config versions.
// @Summary 获取配置版本列表
// @Description 返回所有已发布的 OpenResty 配置版本摘要，需要管理员权限
// @Tags openflare-config-version
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.ConfigVersionSummary} "配置版本列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/config-versions [get]
func ListConfigVersionsHandler(c *gin.Context) {
	versions, err := ListConfigVersions(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(versions))
}

// GetConfigVersionHandler returns a config version by id.
// @Summary 获取配置版本详情
// @Description 返回指定配置版本的完整快照与渲染内容，需要管理员权限
// @Tags openflare-config-version
// @Produce json
// @Security SessionCookie
// @Param id path int true "配置版本 ID"
// @Success 200 {object} response.Any{data=model.ConfigVersion} "配置版本详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或版本不存在"
// @Router /api/v1/d/config-versions/{id} [get]
func GetConfigVersionHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	version, err := GetConfigVersionDetail(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(version))
}

// GetActiveConfigVersionHandler returns the active config version.
// @Summary 获取当前活跃配置版本
// @Description 返回当前正在使用的配置版本，需要管理员权限
// @Tags openflare-config-version
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.ConfigVersion} "活跃配置版本"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限、不存在或无活跃版本"
// @Router /api/v1/d/config-versions/active [get]
func GetActiveConfigVersionHandler(c *gin.Context) {
	version, err := GetActiveConfigVersion(c.Request.Context())
	if apiutil.AbortNotFoundIfMissing(c, err, errNoActiveVersion) {
		return
	}
	c.JSON(http.StatusOK, response.OK(version))
}

// PreviewConfigVersionHandler previews the current draft configuration.
// @Summary 预览当前草稿配置
// @Description 渲染并返回当前草稿配置的预览结果，需要管理员权限
// @Tags openflare-config-version
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=config_version.ConfigPreviewResult} "配置预览"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/config-versions/preview [get]
func PreviewConfigVersionHandler(c *gin.Context) {
	preview, err := PreviewConfigVersion(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(preview))
}

// DiffConfigVersionHandler diffs the current draft against the active version.
// @Summary 对比草稿与活跃配置
// @Description 对比当前草稿配置与活跃版本之间的差异，需要管理员权限
// @Tags openflare-config-version
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=config_version.ConfigDiffResult} "配置差异"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/config-versions/diff [get]
func DiffConfigVersionHandler(c *gin.Context) {
	diff, err := DiffConfigVersion(c.Request.Context())
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(diff))
}

// PublishConfigVersionHandler publishes a new config version.
// @Summary 发布配置版本
// @Description 将当前草稿配置发布为新版本，需要管理员权限
// @Tags openflare-config-version
// @Produce json
// @Security SessionCookie
// @Param force query bool false "是否强制发布"
// @Success 200 {object} response.Any{data=model.ConfigVersion} "发布成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/config-versions/publish [post]
func PublishConfigVersionHandler(c *gin.Context) {
	username := c.GetString("username")
	force := c.Query("force") == "true"
	version, err := PublishConfigVersion(c.Request.Context(), username, force)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(version))
}

// ActivateConfigVersionHandler activates an existing config version.
// @Summary 激活配置版本
// @Description 将指定历史版本设为当前活跃配置，需要管理员权限
// @Tags openflare-config-version
// @Produce json
// @Security SessionCookie
// @Param id path int true "配置版本 ID"
// @Success 200 {object} response.Any{data=model.ConfigVersion} "激活成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或版本不存在"
// @Router /api/v1/d/config-versions/{id}/activate [post]
func ActivateConfigVersionHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	version, err := ActivateConfigVersion(c.Request.Context(), id)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(version))
}

// CleanupConfigVersionsHandler removes old inactive config versions.
// @Summary 清理历史配置版本
// @Description 删除超出保留数量的非活跃配置版本，需要管理员权限
// @Tags openflare-config-version
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body config_version.CleanupInput true "清理参数"
// @Success 200 {object} response.Any{data=config_version.CleanupResult} "清理结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/config-versions/cleanup [post]
func CleanupConfigVersionsHandler(c *gin.Context) {
	var input CleanupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	result, err := CleanupConfigVersions(c.Request.Context(), input.KeepCount)
	if handleLogicError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}