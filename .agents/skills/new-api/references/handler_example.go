// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package references

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/service"
	"github.com/Rain-kl/Wavelet/internal/util"
	"github.com/gin-gonic/gin"
)

// customRequest 客户端请求体 DTO
type customRequest struct {
	Payload string `json:"payload" binding:"required,min=1,max=100"`
}

// customResponse API 响应体 DTO
type customResponse struct {
	Result string `json:"result"`
}

// HandleCustomBusiness 示例 API Handler
// @Summary 示例定制业务接口
// @Description 接收数据载荷，调用 Service 执行核心逻辑，并返回统一格式的 JSON 结果。
// @Tags custom
// @Accept json
// @Produce json
// @Param request body customRequest true "业务请求参数"
// @Success 200 {object} util.ResponseAny{data=customResponse} "操作成功"
// @Router /api/v1/custom/business [post]
func HandleCustomBusiness(c *gin.Context) {
	// 1. 参数绑定与校验
	var req customRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Err("参数校验失败：载荷不能为空且在 1-100 字符内"))
		return
	}

	// 2. 模拟获取当前上下文与已登录用户（例如从 Session 中提取）
	// 通常结合 oauth.LoginRequired() 等中间件使用
	userID := int64(9527)

	// 3. 实例化业务 Service 并调用核心逻辑
	// 注意传入 c.Request.Context() 以正确传递 OpenTelemetry Tracing 等上下文信息
	svc := service.NewCustomService()
	resText, err := svc.ProcessBusinessData(c.Request.Context(), userID, req.Payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Err(err.Error()))
		return
	}

	// 4. 返回符合外层形状规范 { "error_msg": "", "data": ... } 的统一成功响应
	c.JSON(http.StatusOK, util.OK(customResponse{
		Result: resText,
	}))
}
