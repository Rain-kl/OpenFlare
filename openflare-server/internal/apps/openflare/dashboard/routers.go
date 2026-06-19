// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)

// GetOverviewHandler 获取仪表盘概览数据。
// @Summary 获取仪表盘概览
// @Description 聚合节点与可观测性数据，返回 OpenFlare 控制台仪表盘概览，需要管理员权限
// @Tags openflare-dashboard
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=dashboard.OverviewPayload} "仪表盘概览"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/dashboard/overview [get]
func GetOverviewHandler(c *gin.Context) {
	overview, err := GetOverview(c.Request.Context())
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(overview))
}