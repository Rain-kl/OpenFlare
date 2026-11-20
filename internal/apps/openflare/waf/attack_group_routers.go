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
// @Tags openflare-waf
// @Security SessionCookie
// @Param request body attackGroupInput true "规则组"
// @Success 200 {object} response.Any{data=model.OpenFlareAttackGroup}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/attack-groups [post]
func CreateAttackGroupHandler(c *gin.Context) { saveAttackGroupHandler(c, 0) }

// UpdateAttackGroupHandler replaces a group's name and protection levels.
// @Summary 更新攻击防护规则组
// @Tags openflare-waf
// @Security SessionCookie
// @Param id path int true "规则组 ID"
// @Param request body attackGroupInput true "规则组"
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
	var input attackGroupInput
	if !httpx.BindJSON(c, &input) {
		return
	}
	group, err := saveAttackGroup(c.Request.Context(), id, input)
	if !handleRuleError(c, err) {
		c.JSON(http.StatusOK, response.OK(group))
	}
}
