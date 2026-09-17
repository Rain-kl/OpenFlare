// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetPublicConfig 获取公共配置
// @Summary 获取公共配置
// @Description 返回系统配置表中 visibility 为 1 的扁平键值集合（如 cap_login_enabled）
// @Tags config
// @Accept json
// @Produce json
// @Success 200 {object} response.Any
// @Router /api/v1/config/public [get]
func GetPublicConfig(c *gin.Context) {
	resp, err := service.PublicSystemConfigs(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

// GetRobotsTXT 动态生成 robots.txt
// @Summary 获取 robots.txt
// @Description 根据系统配置决定是否允许搜索引擎检索，并返回相应的 robots.txt 文件内容
// @Tags config
// @Produce text/plain
// @Success 200 {string} string "robots.txt 内容"
// @Router /robots.txt [get]
func GetRobotsTXT(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(service.RobotsTxtBody(c.Request.Context())))
}

// CreateSystemConfig 创建系统配置
// @Summary 创建系统配置
// @Description 创建一条新的系统配置项，配置键不可重复，同时将新配置同步到 Redis，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body model.CreateSystemConfigRequest true "创建请求参数"
// @Success 200 {object} response.Any{data=string} "创建成功"
// @Failure 400 {object} response.Any "参数错误或配置键已存在"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs [post]
func CreateSystemConfig(c *gin.Context) {
	var req model.CreateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := service.CreateAdminSystemConfig(c.Request.Context(), req); err != nil {
		if errors.Is(err, errs.ErrProtectedConfigKey) || errors.Is(err, errs.ErrConfigKeyExists) {
			response.AbortBadRequest(c, err.Error())
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// ListSystemConfigs 获取系统配置列表
// @Summary 获取系统配置列表
// @Description 返回所有系统配置列表，支持按配置类型（system/business）过滤，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param type query string false "配置类型（system/business）"
// @Success 200 {object} response.Any{data=[]model.SystemConfig} "系统配置列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs [get]
func ListSystemConfigs(c *gin.Context) {
	configs, err := service.ListAdminSystemConfigs(c.Request.Context(), c.Query("type"))
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(configs))
}

// GetSystemConfig 获取单个系统配置
// @Summary 获取单个系统配置
// @Description 根据配置键获取对应的系统配置详情，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param key path string true "配置键"
// @Success 200 {object} response.Any{data=model.SystemConfig} "系统配置详情"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "配置不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs/{key} [get]
func GetSystemConfig(c *gin.Context) {
	config, err := service.GetAdminSystemConfig(c.Request.Context(), c.Param("key"))
	if err != nil {
		if errors.Is(err, errs.ErrSystemConfigNotFound) {
			response.AbortNotFound(c, errs.SystemConfigNotFound)
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(config))
}

// UpdateSystemConfig 更新系统配置
// @Summary 更新系统配置
// @Description 根据配置键更新对应的配置内容，同时将更新同步到 Redis，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param key path string true "配置键"
// @Param request body model.UpdateSystemConfigRequest true "更新请求参数"
// @Success 200 {object} response.Any{data=string} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "配置不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs/{key} [put]
func UpdateSystemConfig(c *gin.Context) {
	var req model.UpdateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	key := c.Param("key")
	if err := service.UpdateAdminSystemConfig(c.Request.Context(), key, req); err != nil {
		if errors.Is(err, errs.ErrSystemConfigNotFound) {
			response.AbortNotFound(c, errs.SystemConfigNotFound)
			return
		}
		if errors.Is(err, errs.ErrProtectedConfigKey) || errs.IsStorageConfigValidationError(err) {
			response.AbortBadRequest(c, err.Error())
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// TestSMTP 测试 SMTP 邮件发送
// @Summary 测试 SMTP 邮件发送
// @Description 使用传入的配置进行 SMTP 邮件发送测试，支持使用 ****** 占位符使用保存的数据库密码
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body model.TestSMTPRequest true "测试请求参数"
// @Success 200 {object} response.Any{data=model.TestSMTPResponse} "测试执行完毕"
// @Failure 400 {object} response.Any "参数错误"
// @Router /api/v1/admin/system-configs/smtp/test [post]
func TestSMTP(c *gin.Context) {
	var req model.TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(service.TestSMTP(c.Request.Context(), req)))
}
