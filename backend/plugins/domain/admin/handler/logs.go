// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/service"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	defaultLimit = 200
	maxLimit     = 500
	maxPageSize  = 100
)

// wsMessage WebSocket 消息格式
type wsMessage struct {
	Type string          `json:"type"` // "log" | "error"
	Data json.RawMessage `json:"data"`
}

// GetLogs 获取历史日志
// @Summary 获取系统日志
// @Description 分页获取系统历史日志，cursor=0 获取最新日志，cursor>0 获取更早日志
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param cursor query int false "日志游标，0=获取最新" default(0)
// @Param limit query int false "每页条数" default(200)
// @Success 200 {object} response.Any{data=model.LogsResponse} "日志列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/logs [get]
func GetLogs(c *gin.Context) {
	cursorStr := c.DefaultQuery("cursor", "0")
	limitStr := c.DefaultQuery("limit", "200")

	var cursor, limit int
	if err := parsePositiveInt(cursorStr, &cursor); err != nil {
		response.AbortWithError(c, http.StatusBadRequest, errs.InvalidCursorParam)
		return
	}
	if err := parsePositiveInt(limitStr, &limit); err != nil || limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	c.JSON(http.StatusOK, response.OK(service.RecentSystemLogs(cursor, limit)))
}

// HandleLogWebSocket WebSocket 端点，实时推送系统日志
// @Summary 系统日志实时推送
// @Description 通过 WebSocket 实时推送系统日志，需要管理员权限
// @Tags admin
// @Router /api/v1/admin/logs/ws [get]
func HandleLogWebSocket(c *gin.Context) {
	upgrader := getUpgrader()

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	ch := logger.GlobalRingBuffer.Subscribe()
	defer logger.GlobalRingBuffer.Unsubscribe(ch)

	done := make(chan struct{})
	util.Go(func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	for {
		select {
		case <-done:
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(entry)
			msg := wsMessage{Type: "log", Data: data}
			payload, _ := json.Marshal(msg)
			if err := conn.WriteMessage(1, payload); err != nil {
				return
			}
		}
	}
}

// GetAccessLogs 获取 ClickHouse 异步采集的访问日志
// @Summary 获取用户访问日志
// @Description 分页并按照用户、接口路径、时间范围等维度检索用户访问日志列表（需要管理员权限）
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param username query string false "用户名模糊搜索"
// @Param path query string false "接口路径模糊搜索"
// @Param start_time query string false "起始时间（RFC3339 或 YYYY-MM-DD HH:MM:SS）"
// @Param end_time query string false "结束时间（RFC3339 或 YYYY-MM-DD HH:MM:SS）"
// @Success 200 {object} response.Any{data=model.AccessLogsResponse} "访问日志列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/logs/access [get]
func GetAccessLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	resp, err := service.AccessLogs(c.Request.Context(), model.AccessLogQuery{
		Username:  c.Query("username"),
		Path:      c.Query("path"),
		StartTime: c.Query("start_time"),
		EndTime:   c.Query("end_time"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

// GetLogsAnalytics 获取 ClickHouse 访问日志图表聚合指标
// @Summary 获取访问日志分析数据
// @Description 聚合统计最近 7 天的每日访问趋势、浏览器分布以及前 10 名最活跃用户排行（需要管理员权限）
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.LogsAnalyticsResponse} "分析统计数据"
// @Failure 500 {object} response.Any "内部错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/logs/analytics [get]
func GetLogsAnalytics(c *gin.Context) {
	resp, err := service.AccessLogAnalytics(c.Request.Context())
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

func getUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return service.IsAllowedLogOrigin(
				r.Context(),
				r.Header.Get("Origin"),
				r.Host,
				forwardedHosts(r)...,
			)
		},
	}
}

func forwardedHosts(r *http.Request) []string {
	raw := r.Header.Get("X-Forwarded-Host")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	hosts := make([]string, 0, len(parts))
	for _, part := range parts {
		if h := strings.TrimSpace(part); h != "" {
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// errNegativeParam 表示查询参数解析出了负数。
var errNegativeParam = errors.New("parameter must not be negative")

// parsePositiveInt 解析非负整数查询参数；返回错误时 result 保持调用前的值。
func parsePositiveInt(s string, result *int) error {
	if s == "" {
		*result = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	if n < 0 {
		return errNegativeParam
	}
	*result = n
	return nil
}
