package controller

import (
	"net/http"
	"openflare/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetAccessLogs godoc
// @Summary List access logs
// @Tags AccessLogs
// @Produce json
// @Security BearerAuth
// @Param node_id query string false "Node ID"
// @Param remote_addr query string false "Remote address"
// @Param host query string false "Host"
// @Param path query string false "Path"
// @Param p query int false "Page index"
// @Param page_size query int false "Page size"
// @Param sort_by query string false "Sort by"
// @Param sort_order query string false "Sort order"
// @Success 200 {object} map[string]interface{}
// @Router /api/access-logs/ [get]
func GetAccessLogs(c *gin.Context) {
	logs, err := service.ListAccessLogs(readAccessLogQuery(c))
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, logs)
}

// GetFoldedAccessLogs godoc
// @Summary List folded access logs
// @Tags AccessLogs
// @Produce json
// @Security BearerAuth
// @Param node_id query string false "Node ID"
// @Param remote_addr query string false "Remote address"
// @Param host query string false "Host"
// @Param path query string false "Path"
// @Param p query int false "Page index"
// @Param page_size query int false "Page size"
// @Param sort_by query string false "Sort by"
// @Param sort_order query string false "Sort order"
// @Param fold_minutes query int false "Fold minutes"
// @Success 200 {object} map[string]interface{}
// @Router /api/access-logs/folds [get]
func GetFoldedAccessLogs(c *gin.Context) {
	query := readAccessLogQuery(c)
	query.FoldMinutes = readQueryInt(c, "fold_minutes")
	logs, err := service.ListFoldedAccessLogs(query)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, logs)
}

// GetAccessLogIPSummaries godoc
// @Summary List access log IP summaries
// @Tags AccessLogs
// @Produce json
// @Security BearerAuth
// @Param node_id query string false "Node ID"
// @Param remote_addr query string false "Remote address"
// @Param host query string false "Host"
// @Param p query int false "Page index"
// @Param page_size query int false "Page size"
// @Param sort_by query string false "Sort by"
// @Param sort_order query string false "Sort order"
// @Success 200 {object} map[string]interface{}
// @Router /api/access-logs/ip-summary [get]
func GetAccessLogIPSummaries(c *gin.Context) {
	result, err := service.ListAccessLogIPSummaries(service.AccessLogIPSummaryQuery{
		NodeID:     c.Query("node_id"),
		RemoteAddr: c.Query("remote_addr"),
		Host:       c.Query("host"),
		Page:       readQueryInt(c, "p"),
		PageSize:   readQueryInt(c, "page_size"),
		SortBy:     c.Query("sort_by"),
		SortOrder:  c.Query("sort_order"),
	})
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, result)
}

// GetAccessLogIPTrend godoc
// @Summary Get access log IP trend
// @Tags AccessLogs
// @Produce json
// @Security BearerAuth
// @Param node_id query string false "Node ID"
// @Param remote_addr query string true "Remote address"
// @Param host query string false "Host"
// @Param hours query int false "Hours"
// @Param bucket_minutes query int false "Bucket minutes"
// @Success 200 {object} map[string]interface{}
// @Router /api/access-logs/ip-summary/trend [get]
func GetAccessLogIPTrend(c *gin.Context) {
	result, err := service.GetAccessLogIPTrend(service.AccessLogIPTrendQuery{
		NodeID:        c.Query("node_id"),
		RemoteAddr:    c.Query("remote_addr"),
		Host:          c.Query("host"),
		Hours:         readQueryInt(c, "hours"),
		BucketMinutes: readQueryInt(c, "bucket_minutes"),
	})
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, result)
}

// CleanupAccessLogs godoc
// @Summary Cleanup access logs by retention days
// @Tags AccessLogs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/access-logs/cleanup [post]
func CleanupAccessLogs(c *gin.Context) {
	var input service.AccessLogCleanupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}
	result, err := service.CleanupAccessLogs(input)
	if err != nil {
		respondFailure(c, err.Error())
		return
	}
	respondSuccess(c, result)
}

func readAccessLogQuery(c *gin.Context) service.AccessLogQuery {
	return service.AccessLogQuery{
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
