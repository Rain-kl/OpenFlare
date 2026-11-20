// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package waf

import (
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ListAttackLogsHandler lists blocking and observation records.
// @Summary 列出攻击日志记录
// @Tags openflare-waf
// @Security SessionCookie
// @Param filters query attackLogQuery false "日志筛选"
// @Success 200 {object} response.Any{data=attackLogPage[attackLogEntry]}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/attack-logs [get]
func ListAttackLogsHandler(c *gin.Context) {
	if filter, ok := bindAttackLogFilter(c, "all"); ok {
		result, err := listAttackLogs(c.Request.Context(), filter)
		respondAttackLogPage(c, result, err)
	}
}

// ListAttackEventsHandler groups attack logs by source IP, host, and five-minute window.
// @Summary 列出聚合攻击事件
// @Tags openflare-waf
// @Security SessionCookie
// @Param filters query attackLogQuery false "事件筛选"
// @Success 200 {object} response.Any{data=attackLogPage[model.OpenFlareAttackEvent]}
// @Failure 400 {object} response.Any
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/d/waf/attack-events [get]
func ListAttackEventsHandler(c *gin.Context) {
	if filter, ok := bindAttackLogFilter(c, "blocked"); ok {
		result, err := listAttackEvents(c.Request.Context(), filter)
		respondAttackLogPage(c, result, err)
	}
}

func bindAttackLogFilter(c *gin.Context, defaultAction string) (model.OpenFlareAccessLogQuery, bool) {
	var input attackLogQuery
	if err := c.ShouldBindQuery(&input); err != nil {
		response.AbortBadRequest(c, errAttackLogFilters)
		return model.OpenFlareAccessLogQuery{}, false
	}
	filter, err := input.filter(defaultAction, time.Now().UTC())
	if err != nil {
		response.AbortBadRequest(c, errAttackLogFilters)
		return filter, false
	}
	return filter, true
}

func respondAttackLogPage[T any](c *gin.Context, result attackLogPage[T], err error) {
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[OpenFlareWAF] read attack logs: %v", err)
		response.AbortInternal(c, errAttackLogRead)
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}
