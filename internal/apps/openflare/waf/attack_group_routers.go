// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// ListAttackGroupsHandler lists selectable attack protection groups.
// @Summary 列出攻击防护规则组
// @Tags openflare-waf
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.OpenFlareAttackGroup}
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/attack-groups [get]
func ListAttackGroupsHandler(c *gin.Context) {
	groups, err := repository.ListOpenFlareAttackGroups(c.Request.Context(), nil)
	if !handleRuleError(c, err) {
		c.JSON(http.StatusOK, response.OK(groups))
	}
}

// CreateAttackGroupHandler creates a selectable attack protection group.
// @Summary 创建攻击防护规则组
// @Description 可按类别配置防护等级。rule_overrides 以规则 ID 为键，支持 off（关闭）、enabled（开启）、observe（记录），优先于类别等级。
// @Tags openflare-waf
// @Security SessionCookie
// @Param request body AttackGroupInput true "规则组"
// @Success 200 {object} response.Any{data=model.OpenFlareAttackGroup}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/attack-groups [post]
func CreateAttackGroupHandler(c *gin.Context) { saveAttackGroupHandler(c, 0) }

// UpdateAttackGroupHandler updates a group's name, levels and optional rule overrides.
// @Summary 更新攻击防护规则组
// @Description 替换规则组等级配置，省略的类别按禁用处理。rule_overrides 支持 off、enabled、observe；省略字段时保留已有逐条设置，传入空对象时清空。
// @Tags openflare-waf
// @Security SessionCookie
// @Param id path int true "规则组 ID"
// @Param request body AttackGroupInput true "规则组"
// @Success 200 {object} response.Any{data=model.OpenFlareAttackGroup}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/attack-groups/{id}/update [post]
func UpdateAttackGroupHandler(c *gin.Context) {
	if id, ok := httpx.IDParam(c); ok {
		saveAttackGroupHandler(c, id)
	}
}

func saveAttackGroupHandler(c *gin.Context, id uint) {
	var input AttackGroupInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	group, err := SaveAttackGroup(c.Request.Context(), id, input)
	if !handleRuleError(c, err) {
		c.JSON(http.StatusOK, response.OK(group))
	}
}
