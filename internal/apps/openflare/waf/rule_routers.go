// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func handleRuleError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	var validation *RuleValidationError
	switch {
	case errors.As(err, &validation):
		response.AbortBadRequest(c, validation.Error())
	case errors.Is(err, model.ErrWAFRuleRevisionConflict):
		response.AbortConflict(c, "规则已被其他操作更新，请重新加载")
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.AbortNotFound(c, "WAF 规则不存在")
	default:
		logger.ErrorF(c.Request.Context(), "[OpenFlareWAF] rule API failed: %v", err)
		response.AbortInternal(c, "WAF 规则操作失败")
	}
	return true
}

// ListRulesHandler lists orchestrated WAF rules.
// @Summary 列出 WAF 规则
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]waf.RuleView} "规则列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups [get]
func ListRulesHandler(c *gin.Context) {
	rules, err := ListRules(c.Request.Context())
	if handleRuleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(rules))
}

// GetRuleHandler gets an orchestrated WAF rule.
// @Summary 获取 WAF 规则详情
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Param id path int true "规则 ID"
// @Success 200 {object} response.Any{data=waf.RuleView} "规则详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups/{id} [get]
func GetRuleHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	rule, err := GetRule(c.Request.Context(), id)
	if handleRuleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(rule))
}

// CreateRuleHandler creates an orchestrated WAF rule from a name only.
// @Summary 创建 WAF 规则
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body waf.CreateRuleInput true "规则名称"
// @Success 200 {object} response.Any{data=waf.RuleView} "创建成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups [post]
func CreateRuleHandler(c *gin.Context) {
	var input CreateRuleInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	rule, err := CreateRule(c.Request.Context(), input)
	if handleRuleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(rule))
}

// UpdateRuleMetaHandler updates rule name and enabled state.
// @Summary 更新 WAF 规则元数据
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "规则 ID"
// @Param request body waf.UpdateRuleMetaInput true "规则元数据"
// @Success 200 {object} response.Any{data=waf.RuleView} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups/{id}/meta [post]
func UpdateRuleMetaHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input UpdateRuleMetaInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	rule, err := UpdateRuleMeta(c.Request.Context(), id, input)
	if handleRuleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(rule))
}

// SaveRuleGraphHandler saves a complete versioned rule graph.
// @Summary 保存 WAF 规则图
// @Tags openflare-waf
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "规则 ID"
// @Param request body waf.SaveRuleGraphInput true "规则图和修订号"
// @Success 200 {object} response.Any{data=waf.RuleView} "保存成功"
// @Failure 400 {object} response.Any "参数或规则图错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 409 {object} response.Any "修订冲突"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups/{id}/graph [post]
func SaveRuleGraphHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input SaveRuleGraphInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	rule, err := SaveRuleGraph(c.Request.Context(), id, input)
	if handleRuleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(rule))
}

// DeleteRuleHandler deletes a non-global WAF rule.
// @Summary 删除 WAF 规则
// @Tags openflare-waf
// @Produce json
// @Security SessionCookie
// @Param id path int true "规则 ID"
// @Success 200 {object} response.Any "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/waf/rule-groups/{id}/delete [post]
func DeleteRuleHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	if err := DeleteRuleGroup(c.Request.Context(), id); handleRuleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
