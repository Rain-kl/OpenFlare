// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pow

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const errPolicyOperation = "PoW policy operation failed"
const errSiteNotFound = "Site was not found"

func handleError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	var validation *ValidationError
	switch {
	case errors.As(err, &validation):
		response.AbortBadRequest(c, validation.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.AbortNotFound(c, errSiteNotFound)
	default:
		logger.ErrorF(c.Request.Context(), "[OpenFlarePoW] policy operation failed: %v", err)
		response.AbortInternal(c, errPolicyOperation)
	}
	return true
}

// ListPoliciesHandler lists independent PoW policies.
// @Summary 列出独立 PoW 站点策略
// @Tags openflare-pow
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]pow.Policy} "策略列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pow/policies [get]
func ListPoliciesHandler(c *gin.Context) {
	policies, err := ListPolicies(c.Request.Context())
	if handleError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(policies))
}

// SavePolicyHandler saves a site's independent PoW policy.
// @Summary 保存独立 PoW 站点策略
// @Tags openflare-pow
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "站点 ID"
// @Param request body pow.Policy true "PoW 策略"
// @Success 200 {object} response.Any "保存成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或站点不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/pow/policies/{id} [put]
func SavePolicyHandler(c *gin.Context) {
	id, ok := httpx.IDParam(c)
	if !ok {
		return
	}
	var input Policy
	if !httpx.BindJSON(c, &input) {
		return
	}
	if handleError(c, SavePolicy(c.Request.Context(), id, input)) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
