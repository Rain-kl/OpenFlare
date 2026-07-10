// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/chwriter"
	"github.com/Rain-kl/Wavelet/internal/apps/risk_control"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/db/batchwriter"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"github.com/gin-gonic/gin"
)

// GetClickHouseStatus returns ClickHouse operational metrics for administrators.
// @Summary 获取 ClickHouse 运行指标
// @Description 返回 ClickHouse parts、mutation、async_insert 队列及进程内 batch writer 指标，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=analyticsrepo.ClickHouseOperationalStats} "获取成功"
// @Failure 400 {object} response.Any "ClickHouse 未启用"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/status/clickhouse [get]
func GetClickHouseStatus(c *gin.Context) {
	if !config.Config.ClickHouse.Enabled || !db.ChConnReady() {
		response.AbortWithError(c, http.StatusBadRequest, "ClickHouse 存储服务未启用")
		return
	}

	stats, err := analyticsrepo.GetClickHouseOperationalStats(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, "获取 ClickHouse 运行指标失败")
		return
	}
	stats.BatchWriters = collectBatchWriterStats()
	c.JSON(http.StatusOK, response.OK(stats))
}

func collectBatchWriterStats() []batchwriter.Stats {
	out := chwriter.WriterStats()
	if out == nil {
		out = make([]batchwriter.Stats, 0, 1)
	}
	out = append(out, risk_control.LogWriterStats())
	return out
}
