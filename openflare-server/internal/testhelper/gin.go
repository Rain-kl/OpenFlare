// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package testhelper

import (
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)

// NewTestGinEngine 创建带 ErrorHandlerMiddleware 的 Gin 引擎，与生产环境错误响应行为一致。
func NewTestGinEngine(middlewares ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(response.ErrorHandlerMiddleware())
	for _, middleware := range middlewares {
		r.Use(middleware)
	}
	return r
}
