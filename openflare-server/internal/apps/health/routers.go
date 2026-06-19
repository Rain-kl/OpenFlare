// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package health 提供健康检查端点
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// Health 健康检查
// @Summary 健康检查
// @Description 检查服务是否正常运行，可用于负载均衡存活探测
// @Tags health
// @Produce json
// @Success 200 {object} response.Any{data=string} "服务正常"
// @Router /api/health [get]
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, response.OKNil())
}
