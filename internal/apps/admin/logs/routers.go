// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package logs 提供日志查询与分析功能
package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

const (
	defaultLimit   = 200
	maxLimit       = 500
	maxPageSize    = 100
	hoursInDay     = 24
	analyticsDays  = 7
	topActiveLimit = 10
)

// logsResponse 历史日志查询响应
type logsResponse struct {
	Lines      []logger.LogEntry `json:"lines"`
	HasMore    bool              `json:"has_more"`
	NextCursor int               `json:"next_cursor"` // 用于加载更早日志的 cursor
}

// GetLogs 获取历史日志
// @Summary 获取系统日志
// @Description 分页获取系统历史日志，cursor=0 获取最新日志，cursor>0 获取更早日志
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param cursor query int false "日志游标，0=获取最新" default(0)
// @Param limit query int false "每页条数" default(200)
// @Success 200 {object} response.Any{data=logs.logsResponse} "日志列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/logs [get]
func GetLogs(c *gin.Context) {
	cursorStr := c.DefaultQuery("cursor", "0")
	limitStr := c.DefaultQuery("limit", "200")

	var cursor, limit int
	if _, err := parsePositiveInt(cursorStr, &cursor); err != nil {
		response.AbortWithError(c, http.StatusBadRequest, admin.InvalidCursorParam)
		return
	}
	if _, err := parsePositiveInt(limitStr, &limit); err != nil || limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	entries, hasMore := logger.GlobalRingBuffer.Query(cursor, limit)

	resp := logsResponse{
		Lines:   entries,
		HasMore: hasMore,
	}
	if len(entries) > 0 {
		resp.NextCursor = entries[0].Index
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

// wsMessage WebSocket 消息格式
type wsMessage struct {
	Type string          `json:"type"` // "log" | "error"
	Data json.RawMessage `json:"data"`
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

	// 订阅 ring buffer
	ch := logger.GlobalRingBuffer.Subscribe()
	defer logger.GlobalRingBuffer.Unsubscribe(ch)

	// 在独立 goroutine 中读取客户端消息（保持连接活跃 + 检测断开）
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// 主循环：推送日志
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

// accessLogItem 访问日志单条数据
type accessLogItem struct {
	ID        uint64 `json:"id,string"`
	UserID    uint64 `json:"user_id,string"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Path      string `json:"path"`
	Method    string `json:"method"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Headers   string `json:"headers"`
	Status    int32  `json:"status"`
	Latency   int64  `json:"latency"`
	CreatedAt string `json:"created_at"`
}

// accessLogsResponse 访问日志查询响应
type accessLogsResponse struct {
	Total uint64          `json:"total"`
	List  []accessLogItem `json:"list"`
}

func buildAccessLogFilter(ctx context.Context, c *gin.Context) (analyticsrepo.AccessLogFilter, error) {
	filter := analyticsrepo.AccessLogFilter{}

	username := c.Query("username")
	if username != "" {
		var userIDs []uint64
		err := db.DB(ctx).Model(&model.User{}).
			Where("username LIKE ?", "%"+username+"%").
			Pluck("id", &userIDs).Error
		if err != nil {
			return filter, fmt.Errorf("查询用户信息失败: %w", err)
		}
		filter.UserIDs = userIDs
	}

	if path := c.Query("path"); path != "" {
		filter.Path = path
	}

	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := parseAccessLogTime(startTime); err == nil {
			filter.StartTime = &t
		}
	}

	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := parseAccessLogTime(endTime); err == nil {
			filter.EndTime = &t
		}
	}

	return filter, nil
}

func parseAccessLogTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", value)
}

func enrichAccessLogsWithUsers(ctx context.Context, list []accessLogItem) {
	if len(list) == 0 {
		return
	}

	userIDs := make([]uint64, 0, len(list))
	seen := make(map[uint64]struct{}, len(list))
	for _, item := range list {
		if _, ok := seen[item.UserID]; ok {
			continue
		}
		seen[item.UserID] = struct{}{}
		userIDs = append(userIDs, item.UserID)
	}

	userMap := make(map[uint64]struct{ Username, Nickname string })
	var users []model.User
	if err := db.DB(ctx).Where("id IN ?", userIDs).Find(&users).Error; err == nil {
		for _, u := range users {
			userMap[u.ID] = struct{ Username, Nickname string }{Username: u.Username, Nickname: u.Nickname}
		}
	}
	for i := range list {
		if info, ok := userMap[list[i].UserID]; ok {
			list[i].Username = info.Username
			list[i].Nickname = info.Nickname
		}
	}
}

// GetAccessLogs 获取 ClickHouse 异步采集的访问日志
// @Summary 获取用户访问日志
// @Description 分页并按照用户、接口路径、时间范围等维度检索 ClickHouse 用户访问日志列表（需要管理员权限，ClickHouse 未启用时报错）
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param username query string false "用户名模糊搜索"
// @Param path query string false "接口路径模糊搜索"
// @Param start_time query string false "起始时间（RFC3339 或 YYYY-MM-DD HH:MM:SS）"
// @Param end_time query string false "结束时间（RFC3339 或 YYYY-MM-DD HH:MM:SS）"
// @Success 200 {object} response.Any{data=logs.accessLogsResponse} "访问日志列表"
// @Failure 400 {object} response.Any "ClickHouse 未启用或参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/logs/access [get]
func GetAccessLogs(c *gin.Context) {
	ctx := c.Request.Context()
	if !config.Config.ClickHouse.Enabled || !db.ChConnReady() {
		response.AbortWithError(c, http.StatusBadRequest, "ClickHouse 存储服务未启用，无法检索访问日志")
		return
	}

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

	filter, err := buildAccessLogFilter(ctx, c)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if filter.UserIDs != nil && len(filter.UserIDs) == 0 {
		c.JSON(http.StatusOK, response.OK(accessLogsResponse{Total: 0, List: []accessLogItem{}}))
		return
	}

	logs, total, err := analyticsrepo.ListAccessLogs(ctx, filter, page, pageSize)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if total == 0 {
		c.JSON(http.StatusOK, response.OK(accessLogsResponse{Total: 0, List: []accessLogItem{}}))
		return
	}

	list := make([]accessLogItem, len(logs))
	for i, logItem := range logs {
		list[i] = accessLogItem{
			ID:        logItem.ID,
			UserID:    logItem.UserID,
			Path:      logItem.Path,
			Method:    logItem.Method,
			IP:        logItem.IP,
			UserAgent: logItem.UserAgent,
			Headers:   logItem.Headers,
			Status:    logItem.Status,
			Latency:   logItem.Latency,
			CreatedAt: logItem.CreatedAt.Format(time.RFC3339),
		}
	}
	enrichAccessLogsWithUsers(ctx, list)

	c.JSON(http.StatusOK, response.OK(accessLogsResponse{
		Total: total,
		List:  list,
	}))
}

// trendItem 趋势图数据点
type trendItem struct {
	Date  string `json:"date"`
	Count uint64 `json:"count"`
}

// browserItem 浏览器占比排行
type browserItem struct {
	Browser string `json:"browser"`
	Count   uint64 `json:"count"`
}

// topUserItem 活跃用户数据
type topUserItem struct {
	UserID   uint64 `json:"user_id,string"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Count    uint64 `json:"count"`
}

// logsAnalyticsResponse 访问日志数据分析结果
type logsAnalyticsResponse struct {
	Trend    []trendItem   `json:"trend"`
	Browsers []browserItem `json:"browsers"`
	TopUsers []topUserItem `json:"top_users"`
}

// GetLogsAnalytics 获取 ClickHouse 访问日志图表聚合指标
// @Summary 获取访问日志分析数据
// @Description 聚合统计最近 7 天的每日访问趋势、浏览器分布以及前 10 名最活跃用户排行（需要管理员权限，ClickHouse 未启用时报错）
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=logs.logsAnalyticsResponse} "分析统计数据"
// @Failure 400 {object} response.Any "ClickHouse 未启用"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/logs/analytics [get]
func GetLogsAnalytics(c *gin.Context) {
	ctx := c.Request.Context()
	if !config.Config.ClickHouse.Enabled || !db.ChConnReady() {
		response.AbortWithError(c, http.StatusBadRequest, "ClickHouse 存储服务未启用，无法获取分析数据")
		return
	}

	startTime := time.Now().AddDate(0, 0, -(analyticsDays - 1)).Truncate(hoursInDay * time.Hour)

	trendPoints, err := analyticsrepo.GetDailyTrend(ctx, analyticsDays)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, "查询访问趋势失败: "+err.Error())
		return
	}
	trendList := make([]trendItem, len(trendPoints))
	for i, point := range trendPoints {
		trendList[i] = trendItem{
			Date:  point.Date,
			Count: point.Count,
		}
	}

	browserPoints, err := analyticsrepo.GetBrowserDistribution(ctx, startTime)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, "查询浏览器分布失败: "+err.Error())
		return
	}
	browserList := make([]browserItem, len(browserPoints))
	for i, point := range browserPoints {
		browserList[i] = browserItem{
			Browser: point.Browser,
			Count:   point.Count,
		}
	}

	topUserPoints, err := analyticsrepo.GetTopActiveUsers(ctx, startTime, topActiveLimit)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, "查询活跃用户失败: "+err.Error())
		return
	}

	topUsers := make([]topUserItem, len(topUserPoints))
	userIDs := make([]uint64, len(topUserPoints))
	for i, point := range topUserPoints {
		topUsers[i] = topUserItem{
			UserID: point.UserID,
			Count:  point.Count,
		}
		userIDs[i] = point.UserID
	}

	if len(userIDs) > 0 {
		userProfileMap := make(map[uint64]struct {
			Username string
			Nickname string
		})
		var users []model.User
		if errProfile := db.DB(ctx).Where("id IN ?", userIDs).Find(&users).Error; errProfile == nil {
			for _, u := range users {
				userProfileMap[u.ID] = struct {
					Username string
					Nickname string
				}{
					Username: u.Username,
					Nickname: u.Nickname,
				}
			}
		}
		for i := range topUsers {
			if profile, ok := userProfileMap[topUsers[i].UserID]; ok {
				topUsers[i].Username = profile.Username
				topUsers[i].Nickname = profile.Nickname
			}
		}
	}

	c.JSON(http.StatusOK, response.OK(logsAnalyticsResponse{
		Trend:    trendList,
		Browsers: browserList,
		TopUsers: topUsers,
	}))
}
