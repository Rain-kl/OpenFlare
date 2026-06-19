// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)

// GetLoginSources 获取可用登录源列表
// @Summary 获取可用登录源
// @Description 返回当前系统已启用的所有 OAuth 登录源，前端展示登录按钮列表时调用
// @Tags oauth
// @Produce json
// @Success 200 {object} response.Any{data=[]oauth.AuthSourceView} "登录源列表"
// @Router /api/v1/oauth/sources [get]
func GetLoginSources(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(activeLoginSources(c.Request.Context())))
}
