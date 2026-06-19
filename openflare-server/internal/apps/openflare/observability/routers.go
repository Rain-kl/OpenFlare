// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)

// GetAccessLogsHandler 分页列出访问日志。
// @Summary 列出访问日志
// @Description 分页返回 OpenFlare 访问日志，支持按节点、IP、主机与路径筛选，需要管理员权限
// @Tags openflare-observability
// @Produce json
// @Security SessionCookie
// @Param node_id query string false "节点 ID"
// @Param remote_addr query string false "客户端 IP"
// @Param host query string false "请求 Host"
// @Param path query string false "请求路径"
// @Param p query int false "页码"
// @Param page_size query int false "每页条数"
// @Param sort_by query string false "排序字段"
// @Param sort_order query string false "排序方向"
// @Success 200 {object} response.Any{data=observability.AccessLogList} "访问日志列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/access-logs [get]
func GetAccessLogsHandler(c *gin.Context) {
	logs, err := ListAccessLogs(c.Request.Context(), readAccessLogQuery(c))
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(logs))
}

// getFoldedAccessLogsHandler 分页列出折叠访问日志。
// @Summary 列出折叠访问日志
// @Description 按时间桶聚合访问日志并分页返回，需要管理员权限
// @Tags openflare-observability
// @Produce json
// @Security SessionCookie
// @Param node_id query string false "节点 ID"
// @Param remote_addr query string false "客户端 IP"
// @Param host query string false "请求 Host"
// @Param path query string false "请求路径"
// @Param fold_minutes query int false "折叠时间窗口（分钟）"
// @Param p query int false "页码"
// @Param page_size query int false "每页条数"
// @Param sort_by query string false "排序字段"
// @Param sort_order query string false "排序方向"
// @Success 200 {object} response.Any{data=observability.FoldedAccessLogList} "折叠访问日志列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/access-logs/folds [get]
func GetFoldedAccessLogsHandler(c *gin.Context) {
	query := readAccessLogQuery(c)
	query.FoldMinutes = readQueryInt(c, "fold_minutes")
	logs, err := ListFoldedAccessLogs(c.Request.Context(), query)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(logs))
}

// getFoldedAccessLogIPsHandler 列出折叠桶内的 IP 汇总。
// @Summary 列出折叠访问日志 IP 汇总
// @Description 在指定时间桶内按 IP 聚合访问统计，需要管理员权限
// @Tags openflare-observability
// @Produce json
// @Security SessionCookie
// @Param node_id query string false "节点 ID"
// @Param remote_addr query string false "客户端 IP"
// @Param host query string false "请求 Host"
// @Param path query string false "请求路径"
// @Param bucket_started_at query string false "时间桶起始时间"
// @Param fold_minutes query int false "折叠时间窗口（分钟）"
// @Param p query int false "页码"
// @Param page_size query int false "每页条数"
// @Param sort_by query string false "排序字段"
// @Param sort_order query string false "排序方向"
// @Success 200 {object} response.Any{data=observability.FoldedAccessLogIPList} "折叠 IP 汇总列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/access-logs/folds/ip-summary [get]
func GetFoldedAccessLogIPsHandler(c *gin.Context) {
	result, err := ListFoldedAccessLogIPs(c.Request.Context(), FoldedAccessLogIPQuery{
		NodeID:          c.Query("node_id"),
		RemoteAddr:      c.Query("remote_addr"),
		Host:            c.Query("host"),
		Path:            c.Query("path"),
		BucketStartedAt: c.Query("bucket_started_at"),
		FoldMinutes:     readQueryInt(c, "fold_minutes"),
		Page:            readQueryInt(c, "p"),
		PageSize:        readQueryInt(c, "page_size"),
		SortBy:          c.Query("sort_by"),
		SortOrder:       c.Query("sort_order"),
	})
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// getAccessLogIPSummariesHandler 列出访问日志 IP 汇总。
// @Summary 列出访问日志 IP 汇总
// @Description 按 IP 聚合访问日志统计并分页返回，需要管理员权限
// @Tags openflare-observability
// @Produce json
// @Security SessionCookie
// @Param node_id query string false "节点 ID"
// @Param remote_addr query string false "客户端 IP"
// @Param host query string false "请求 Host"
// @Param p query int false "页码"
// @Param page_size query int false "每页条数"
// @Param sort_by query string false "排序字段"
// @Param sort_order query string false "排序方向"
// @Success 200 {object} response.Any{data=observability.AccessLogIPSummaryList} "IP 汇总列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/access-logs/ip-summary [get]
func GetAccessLogIPSummariesHandler(c *gin.Context) {
	result, err := ListAccessLogIPSummaries(c.Request.Context(), AccessLogIPSummaryQuery{
		NodeID:     c.Query("node_id"),
		RemoteAddr: c.Query("remote_addr"),
		Host:       c.Query("host"),
		Page:       readQueryInt(c, "p"),
		PageSize:   readQueryInt(c, "page_size"),
		SortBy:     c.Query("sort_by"),
		SortOrder:  c.Query("sort_order"),
	})
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// getAccessLogIPTrendHandler 获取 IP 访问趋势。
// @Summary 获取访问日志 IP 趋势
// @Description 返回指定 IP 在时间范围内的访问趋势数据，需要管理员权限
// @Tags openflare-observability
// @Produce json
// @Security SessionCookie
// @Param node_id query string false "节点 ID"
// @Param remote_addr query string false "客户端 IP"
// @Param host query string false "请求 Host"
// @Param hours query int false "统计时间范围（小时）"
// @Param bucket_minutes query int false "时间桶粒度（分钟）"
// @Success 200 {object} response.Any{data=observability.AccessLogIPTrendView} "IP 访问趋势"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/access-logs/ip-summary/trend [get]
func GetAccessLogIPTrendHandler(c *gin.Context) {
	result, err := GetAccessLogIPTrend(c.Request.Context(), AccessLogIPTrendQuery{
		NodeID:        c.Query("node_id"),
		RemoteAddr:    c.Query("remote_addr"),
		Host:          c.Query("host"),
		Hours:         readQueryInt(c, "hours"),
		BucketMinutes: readQueryInt(c, "bucket_minutes"),
	})
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// cleanupAccessLogsHandler 清理过期访问日志。
// @Summary 清理访问日志
// @Description 按保留天数清理过期访问日志记录，需要管理员权限
// @Tags openflare-observability
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body observability.AccessLogCleanupInput true "清理参数"
// @Success 200 {object} response.Any{data=observability.AccessLogCleanupResult} "清理结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/d/access-logs/cleanup [post]
func CleanupAccessLogsHandler(c *gin.Context) {
	var input AccessLogCleanupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	result, err := CleanupAccessLogs(c.Request.Context(), input)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

func readAccessLogQuery(c *gin.Context) AccessLogQuery {
	return AccessLogQuery{
		NodeID:     c.Query("node_id"),
		RemoteAddr: c.Query("remote_addr"),
		Host:       c.Query("host"),
		Path:       c.Query("path"),
		Page:       readQueryInt(c, "p"),
		PageSize:   readQueryInt(c, "page_size"),
		SortBy:     c.Query("sort_by"),
		SortOrder:  c.Query("sort_order"),
	}
}

func readQueryInt(c *gin.Context, key string) int {
	value, _ := strconv.Atoi(c.DefaultQuery(key, "0"))
	return value
}