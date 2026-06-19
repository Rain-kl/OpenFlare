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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
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
	queryExtraArgs = 2 // pageSize + offset
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

// buildAccessLogFilters 构建 ClickHouse 访问日志查询过滤条件
func buildAccessLogFilters(ctx context.Context, c *gin.Context) ([]string, []interface{}, []uint64, error) {
	var conditions []string
	var args []interface{}
	var userIDs []uint64

	// 按用户名过滤
	username := c.Query("username")
	if username != "" {
		err := db.DB(ctx).Model(&model.User{}).
			Where("username LIKE ?", "%"+username+"%").
			Pluck("id", &userIDs).Error
		if err != nil {
			return nil, nil, nil, fmt.Errorf("查询用户信息失败: %w", err)
		}
		if len(userIDs) == 0 {
			return nil, nil, nil, nil // 无匹配用户
		}
	}

	if len(userIDs) > 0 {
		placeholders := make([]string, len(userIDs))
		for i := range userIDs {
			placeholders[i] = "?"
			args = append(args, userIDs[i])
		}
		conditions = append(conditions, fmt.Sprintf("user_id IN (%s)", strings.Join(placeholders, ",")))
	}

	if path := c.Query("path"); path != "" {
		conditions = append(conditions, "path LIKE ?")
		args = append(args, "%"+path+"%")
	}

	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			conditions = append(conditions, "created_at >= ?")
			args = append(args, t)
		} else if t, err := time.Parse("2006-01-02 15:04:05", startTime); err == nil {
			conditions = append(conditions, "created_at >= ?")
			args = append(args, t)
		}
	}

	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			conditions = append(conditions, "created_at <= ?")
			args = append(args, t)
		} else if t, err := time.Parse("2006-01-02 15:04:05", endTime); err == nil {
			conditions = append(conditions, "created_at <= ?")
			args = append(args, t)
		}
	}

	return conditions, args, userIDs, nil
}

// fetchAccessLogDetails 查询 ClickHouse 访问日志明细并填充用户名
func fetchAccessLogDetails(ctx context.Context, whereClause string, args []interface{}, pageSize int, offset int) ([]accessLogItem, error) {
	dataQuery := fmt.Sprintf(`
		SELECT id, user_id, path, method, ip, user_agent, headers, status, latency, created_at
		FROM w_user_access_logs
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	selectArgs := make([]interface{}, len(args), len(args)+queryExtraArgs)
	copy(selectArgs, args)
	selectArgs = append(selectArgs, pageSize, offset)

	rows, err := db.ChConn.Query(ctx, dataQuery, selectArgs...)
	if err != nil {
		return nil, fmt.Errorf("查询 ClickHouse 日志明细失败: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []accessLogItem
	var fetchUserIDs []uint64

	for rows.Next() {
		var item accessLogItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.UserID, &item.Path, &item.Method, &item.IP, &item.UserAgent, &item.Headers, &item.Status, &item.Latency, &createdAt); err != nil {
			return nil, fmt.Errorf("读取 ClickHouse 结果失败: %w", err)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		list = append(list, item)
		fetchUserIDs = append(fetchUserIDs, item.UserID)
	}

	// 反查 Postgres 关联 Username 和 Nickname
	if len(fetchUserIDs) > 0 {
		userMap := make(map[uint64]struct{ Username, Nickname string })
		var users []model.User
		if err := db.DB(ctx).Where("id IN ?", fetchUserIDs).Find(&users).Error; err == nil {
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

	return list, nil
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
	if db.ChConn == nil {
		response.AbortWithError(c, http.StatusInternalServerError, "ClickHouse 未初始化，无法检索访问日志")
		return
	}

	// 2. 解析分页参数
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
	offset := (page - 1) * pageSize

	// 3. 构建过滤条件
	conditions, args, userIDs, err := buildAccessLogFilters(c.Request.Context(), c)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if userIDs != nil && len(userIDs) == 0 {
		c.JSON(http.StatusOK, response.OK(accessLogsResponse{Total: 0, List: []accessLogItem{}}))
		return
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 4. 查询日志总数
	var total uint64
	countQuery := fmt.Sprintf("SELECT count() FROM w_user_access_logs %s", whereClause)
	if err := db.ChConn.QueryRow(c.Request.Context(), countQuery, args...).Scan(&total); err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, "查询 ClickHouse 日志统计失败: "+err.Error())
		return
	}
	if total == 0 {
		c.JSON(http.StatusOK, response.OK(accessLogsResponse{Total: 0, List: []accessLogItem{}}))
		return
	}

	// 5. 分页查询明细数据
	list, err := fetchAccessLogDetails(c.Request.Context(), whereClause, args, pageSize, offset)
	if err != nil {
		response.AbortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

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
	if db.ChConn == nil {
		response.AbortWithError(c, http.StatusInternalServerError, "ClickHouse 未初始化，无法获取分析数据")
		return
	}

	ctx := c.Request.Context()
	// 7 天前 00:00:00
	startTime := time.Now().AddDate(0, 0, -(analyticsDays - 1)).Truncate(hoursInDay * time.Hour)

	trendList := queryAccessTrend(ctx, startTime)
	browserList := queryBrowserDistribution(ctx, startTime)
	topUsers := queryTopActiveUsers(ctx, startTime)

	c.JSON(http.StatusOK, response.OK(logsAnalyticsResponse{
		Trend:    trendList,
		Browsers: browserList,
		TopUsers: topUsers,
	}))
}

// queryAccessTrend 查询最近 7 天的访问趋势
func queryAccessTrend(ctx context.Context, startTime time.Time) []trendItem {
	trendRows, err := db.ChConn.Query(ctx, `
		SELECT toDate(created_at) as date, count() as count
		FROM w_user_access_logs
		WHERE created_at >= ?
		GROUP BY date
		ORDER BY date ASC
	`, startTime)

	trendMap := make(map[string]uint64)
	for i := 0; i < analyticsDays; i++ {
		dStr := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		trendMap[dStr] = 0
	}

	if err == nil {
		defer func() { _ = trendRows.Close() }()
		for trendRows.Next() {
			var dt time.Time
			var cnt uint64
			if errScan := trendRows.Scan(&dt, &cnt); errScan == nil {
				dStr := dt.Format("2006-01-02")
				trendMap[dStr] = cnt
			}
		}
	}

	var trendList []trendItem
	for i := analyticsDays - 1; i >= 0; i-- {
		dStr := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		trendList = append(trendList, trendItem{
			Date:  dStr,
			Count: trendMap[dStr],
		})
	}
	return trendList
}

// queryBrowserDistribution 查询浏览器分布排行
func queryBrowserDistribution(ctx context.Context, startTime time.Time) []browserItem {
	uaRows, err := db.ChConn.Query(ctx, `
		SELECT user_agent, count() as count
		FROM w_user_access_logs
		WHERE created_at >= ?
		GROUP BY user_agent
	`, startTime)

	browserCounts := make(map[string]uint64)
	if err == nil {
		defer func() { _ = uaRows.Close() }()
		for uaRows.Next() {
			var ua string
			var cnt uint64
			if errScan := uaRows.Scan(&ua, &cnt); errScan == nil {
				browser := parseBrowserName(ua)
				browserCounts[browser] += cnt
			}
		}
	}

	var browserList []browserItem
	for b, cnt := range browserCounts {
		browserList = append(browserList, browserItem{
			Browser: b,
			Count:   cnt,
		})
	}
	sort.Slice(browserList, func(i, j int) bool {
		return browserList[i].Count > browserList[j].Count
	})
	return browserList
}

// queryTopActiveUsers 查询活跃用户 Top 10
func queryTopActiveUsers(ctx context.Context, startTime time.Time) []topUserItem {
	userRows, err := db.ChConn.Query(ctx, `
		SELECT user_id, count() as count
		FROM w_user_access_logs
		WHERE created_at >= ? AND user_id > 0
		GROUP BY user_id
		ORDER BY count DESC
		LIMIT 10
	`, startTime)

	var topUsers []topUserItem
	var userIDs []uint64
	userCountMap := make(map[uint64]uint64)

	if err == nil {
		defer func() { _ = userRows.Close() }()
		for userRows.Next() {
			var uid uint64
			var cnt uint64
			if errScan := userRows.Scan(&uid, &cnt); errScan == nil {
				userIDs = append(userIDs, uid)
				userCountMap[uid] = cnt
			}
		}
	}

	// 反查 Postgres 补全活跃用户的用户名和昵称
	userProfileMap := make(map[uint64]struct {
		Username string
		Nickname string
	})
	if len(userIDs) > 0 {
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
	}

	for _, uid := range userIDs {
		profile := userProfileMap[uid]
		topUsers = append(topUsers, topUserItem{
			UserID:   uid,
			Username: profile.Username,
			Nickname: profile.Nickname,
			Count:    userCountMap[uid],
		})
	}
	return topUsers
}

// parseBrowserName 简易的 User-Agent 浏览器类型识别
func parseBrowserName(ua string) string {
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "micromessenger") {
		return "WeChat"
	}
	if strings.Contains(uaLower, "postman") {
		return "Postman"
	}
	if strings.Contains(uaLower, "edg/") || strings.Contains(uaLower, "edge") {
		return "Edge"
	}
	if strings.Contains(uaLower, "firefox") {
		return "Firefox"
	}
	if strings.Contains(uaLower, "chrome") {
		return "Chrome"
	}
	if strings.Contains(uaLower, "safari") {
		return "Safari"
	}
	return "Other"
}
