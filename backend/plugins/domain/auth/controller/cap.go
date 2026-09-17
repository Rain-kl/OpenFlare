// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP handlers and middlewares for the auth plugin.
package controller

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/model/dto"
	"Wavelet/plugins/domain/auth/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CaptchaHandler handles CAPTCHA challenge and redeem endpoints.
type CaptchaHandler struct {
	capMgr *service.CaptchaManager
}

// NewCaptchaHandler creates a new CaptchaHandler.
func NewCaptchaHandler(mgr *service.CaptchaManager) *CaptchaHandler {
	return &CaptchaHandler{
		capMgr: mgr,
	}
}

// Challenge 生成 PoW 人机验证难题
// @Summary 生成人机验证难题
// @Description 客户端获取 PoW 难题和签名的 JWT Token，并在后台计算。
// @Tags cap
// @Accept json
// @Produce json
// @Param request body dto.ChallengeRequest false "可选范围限制参数"
// @Success 200 {object} response.Any{data=dto.ChallengeResponse} "成功返回 PoW 难题"
// @Failure 500 {object} response.Any "内部服务错误"
// @Router /api/v1/cap/challenge [get]
// @Router /api/v1/cap/challenge [post]
func (h *CaptchaHandler) Challenge(c *gin.Context) {
	var req dto.ChallengeRequest
	_ = c.ShouldBind(&req) // 允许不传 body，默认使用 login scope

	if req.Scope == "" {
		req.Scope = "login"
	}

	if h.capMgr == nil {
		response.AbortInternal(c, consts.ErrCapNotConfigured)
		return
	}
	resp, err := h.capMgr.Generate(c.Request.Context(), req.Scope)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "Generate cap challenge failed: %v", err)
		response.AbortInternal(c, consts.ErrChallengeGenerateFailed)
		return
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

// Redeem 提交 PoW 解答并兑换一次性凭证 Token
// @Summary 校验人机验证解答
// @Description 提交 PoW 解答进行核销，成功后返回一次性 X-Cap-Token 凭证
// @Tags cap
// @Accept json
// @Produce json
// @Param request body dto.RedeemRequest true "难题 Token 与解答 solutions 数组"
// @Success 200 {object} response.Any{data=dto.RedeemResponse} "核销成功，返回 X-Cap-Token"
// @Failure 400 {object} response.Any "参数错误或核销失败"
// @Failure 500 {object} response.Any "内部服务错误"
// @Router /api/v1/cap/redeem [post]
func (h *CaptchaHandler) Redeem(c *gin.Context) {
	var req dto.RedeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrInvalidRequestParams)
		return
	}

	if req.Scope == "" {
		req.Scope = "login"
	}

	if h.capMgr == nil {
		response.AbortInternal(c, consts.ErrCapNotConfigured)
		return
	}
	resp, err := h.capMgr.Redeem(c.Request.Context(), req.Token, req.Solutions, req.Scope)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "Redeem cap solutions failed: %v", err)
		response.AbortInternal(c, consts.ErrSolutionVerifyFailed)
		return
	}

	if !resp.Success {
		response.AbortBadRequest(c, resp.Error)
		return
	}

	c.JSON(http.StatusOK, response.OK(resp))
}
