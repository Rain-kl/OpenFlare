// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type challengeRequest struct {
	Scope string `json:"scope" form:"scope"`
}

type redeemRequest struct {
	Token     string `json:"token" binding:"required"`
	Solutions []int  `json:"solutions" binding:"required"`
	Scope     string `json:"scope" form:"scope"`
}

// Challenge 生成 PoW 人机验证难题
// @Summary 生成人机验证难题
// @Description 客户端获取 PoW 难题和签名的 JWT Token，并在后台计算。
// @Tags cap
// @Accept json
// @Produce json
// @Param request body challengeRequest false "可选范围限制参数"
// @Success 200 {object} cap.ChallengeResponse "成功返回 PoW 难题"
// @Failure 500 {object} RedeemResponse "内部服务错误"
// @Router /api/cap/challenge [post]
func Challenge(c *gin.Context) {
	var req challengeRequest
	_ = c.ShouldBind(&req) // 允许不传 body，默认使用 login scope

	if req.Scope == "" {
		req.Scope = "login"
	}

	mgr := GetDefaultManager()
	resp, err := mgr.Generate(c.Request.Context(), req.Scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, RedeemResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Redeem 提交 PoW 解答并兑换一次性凭证 Token
// @Summary 校验人机验证解答
// @Description 提交 PoW 解答进行核销，成功后返回一次性 X-Cap-Token 凭证
// @Tags cap
// @Accept json
// @Produce json
// @Param request body redeemRequest true "难题 Token 与解答 solutions 数组"
// @Success 200 {object} RedeemResponse "核销成功，返回 X-Cap-Token"
// @Failure 400 {object} RedeemResponse "参数错误或核销失败"
// @Failure 500 {object} RedeemResponse "内部服务错误"
// @Router /api/cap/redeem [post]
func Redeem(c *gin.Context) {
	var req redeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, RedeemResponse{
			Success: false,
			Error:   "无效的参数",
		})
		return
	}

	if req.Scope == "" {
		req.Scope = "login"
	}

	mgr := GetDefaultManager()
	resp, err := mgr.Redeem(c.Request.Context(), req.Token, req.Solutions, req.Scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, RedeemResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if !resp.Success {
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}
