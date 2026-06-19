// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package apply_log

import (
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)


// GetApplyLogs lists apply logs with pagination and optional node_id filter.
// @Summary 获取配置下发日志
// @Description 分页返回节点配置下发记录，支持按节点 ID 筛选，需要管理员权限
// @Tags openflare-apply-log
// @Produce json
// @Security SessionCookie
// @Param node_id query string false "节点 ID 筛选"
// @Param pageNo query int false "页码"
// @Param page_no query int false "页码（别名）"
// @Param pageSize query int false "每页数量"
// @Param page_size query int false "每页数量（别名）"
// @Success 200 {object} response.Any{data=apply_log.ListResult} "下发日志列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/apply-logs [get]
func GetApplyLogs(c *gin.Context) {
	result, err := ListPage(c.Request.Context(), ListQuery{
		NodeID:   c.Query("node_id"),
		PageNo:   readIntQuery(c, "pageNo", "page_no"),
		PageSize: readIntQuery(c, "pageSize", "page_size"),
	})
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// CleanupApplyLogs removes old apply logs or deletes all records.
// @Summary 清理配置下发日志
// @Description 按保留天数清理历史下发记录，或删除全部记录，需要管理员权限
// @Tags openflare-apply-log
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param body body apply_log.CleanupInput true "清理参数"
// @Success 200 {object} response.Any{data=apply_log.CleanupResult} "清理结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 404 {object} response.Any "无权限或不存在"
// @Router /api/v1/d/apply-logs/cleanup [post]
func CleanupApplyLogs(c *gin.Context) {
	var input CleanupInput
	if !apiutil.BindJSON(c, &input) {
		return
	}

	result, err := Cleanup(c.Request.Context(), input)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

func readIntQuery(c *gin.Context, primary, secondary string) int {
	value := c.Query(primary)
	if value == "" {
		value = c.Query(secondary)
	}
	parsed, _ := strconv.Atoi(value)
	return parsed
}