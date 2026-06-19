// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	Key         string `json:"key" binding:"required,max=80"`
	Name        string `json:"name" binding:"required,max=100"`
	Type        string `json:"type" binding:"required,max=20"`
	Subject     string `json:"subject" binding:"max=255"`
	Content     string `json:"content" binding:"required"`
	Description string `json:"description" binding:"max=255"`
}

// UpdateTemplateRequest 更新模板请求
type UpdateTemplateRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Type        string `json:"type" binding:"required,max=20"`
	Subject     string `json:"subject" binding:"max=255"`
	Content     string `json:"content" binding:"required"`
	Description string `json:"description" binding:"max=255"`
}

func abortTemplateLogicError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.AbortNotFound(c, TemplateNotFound)
		return true
	}
	msg := err.Error()
	switch msg {
	case TemplateKeyExists, SystemTemplateCannotDelete:
		response.AbortBadRequest(c, msg)
		return true
	}
	response.AbortInternal(c, msg)
	return true
}

// CreateTemplate 创建模板
// @Summary 创建模板
// @Description 创建一条新的自定义通知模板，模板标识符（Key）不可重复，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body template.CreateTemplateRequest true "创建请求参数"
// @Success 200 {object} response.Any{data=string} "创建成功"
// @Failure 400 {object} response.Any "参数错误或模板标识符已存在"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/templates [post]
func CreateTemplate(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	tmpl, err := createTemplate(c.Request.Context(), req)
	if abortTemplateLogicError(c, err) {
		return
	}

	c.JSON(http.StatusOK, response.OK(tmpl))
}

// ListTemplates 获取模板列表
// @Summary 获取模板列表
// @Description 返回所有通知模板列表，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.Template} "模板列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/templates [get]
func ListTemplates(c *gin.Context) {
	templates, err := listTemplates(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(templates))
}

// GetTemplate 获取单个模板
// @Summary 获取单个模板
// @Description 根据模板标识符获取对应的模板详情，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param key path string true "模板标识符"
// @Success 200 {object} response.Any{data=model.Template} "模板详情"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "模板不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/templates/{key} [get]
func GetTemplate(c *gin.Context) {
	tmpl, err := getTemplate(c.Request.Context(), c.Param("key"))
	if abortTemplateLogicError(c, err) {
		return
	}

	c.JSON(http.StatusOK, response.OK(tmpl))
}

// UpdateTemplate 更新模板
// @Summary 更新模板
// @Description 根据模板标识符更新对应的模板内容，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param key path string true "模板标识符"
// @Param request body template.UpdateTemplateRequest true "更新请求参数"
// @Success 200 {object} response.Any{data=model.Template} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "模板不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/templates/{key} [put]
func UpdateTemplate(c *gin.Context) {
	var req UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	tmpl, err := updateTemplate(c.Request.Context(), c.Param("key"), req)
	if abortTemplateLogicError(c, err) {
		return
	}

	c.JSON(http.StatusOK, response.OK(tmpl))
}

// DeleteTemplate 删除模板
// @Summary 删除模板
// @Description 根据模板标识符删除对应模板，系统预置模板不可删除，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param key path string true "模板标识符"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "不可删除系统模板"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "模板不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/templates/{key} [delete]
func DeleteTemplate(c *gin.Context) {
	if err := deleteTemplate(c.Request.Context(), c.Param("key")); abortTemplateLogicError(c, err) {
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}
