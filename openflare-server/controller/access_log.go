package controller

import (
	"strconv"

	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-gonic/gin"
)

// GetAccessLogs godoc
// @Summary List access logs
// @Tags AccessLogs
// @Produce json
// @Security OpenFlareTokenAuth
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
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, logs)
}

// GetFoldedAccessLogs godoc
// @Summary List folded access logs
// @Tags AccessLogs
// @Produce json
// @Security OpenFlareTokenAuth
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
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, logs)
}

// GetFoldedAccessLogIPs godoc
// @Summary List folded access log IP summaries
// @Tags AccessLogs
// @Produce json
// @Security OpenFlareTokenAuth
// @Param node_id query string false "Node ID"
// @Param remote_addr query string false "Remote address"
// @Param host query string false "Host"
// @Param path query string false "Path"
// @Param bucket_started_at query string true "Bucket started at"
// @Param fold_minutes query int true "Fold minutes"
// @Param p query int false "Page index"
// @Param page_size query int false "Page size"
// @Param sort_by query string false "Sort by"
// @Param sort_order query string false "Sort order"
// @Success 200 {object} map[string]interface{}
// @Router /api/access-logs/folds/ip-summary [get]
func GetFoldedAccessLogIPs(c *gin.Context) {
	result, err := service.ListFoldedAccessLogIPs(service.FoldedAccessLogIPQuery{
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
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}

// GetAccessLogIPSummaries godoc
// @Summary List access log IP summaries
// @Tags AccessLogs
// @Produce json
// @Security OpenFlareTokenAuth
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
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}

// GetAccessLogIPTrend godoc
// @Summary Get access log IP trend
// @Tags AccessLogs
// @Produce json
// @Security OpenFlareTokenAuth
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
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
}

// CleanupAccessLogs godoc
// @Summary Cleanup access logs by retention days
// @Tags AccessLogs
// @Accept json
// @Produce json
// @Security OpenFlareTokenAuth
// @Success 200 {object} map[string]interface{}
// @Router /api/access-logs/cleanup [post]
func CleanupAccessLogs(c *gin.Context) {
	var input service.AccessLogCleanupInput
	if !bind.JSON(c, &input) {
		return
	}
	result, err := service.CleanupAccessLogs(input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, result)
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
