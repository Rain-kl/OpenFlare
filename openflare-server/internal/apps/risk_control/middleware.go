// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package risk_control 提供风险控制中间件
package risk_control

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
)

// RiskControlMiddleware 全局日志采集中间件
func RiskControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未启用 ClickHouse，直接放行
		if !config.Config.ClickHouse.Enabled {
			c.Next()
			return
		}

		// 1. 限流背压检测（检测本地缓冲队列是否已满）
		if IsBufferFull() {
			response.AbortTooManyRequests(c, "系统繁忙，请稍后再试")
			return
		}

		start := time.Now()

		// 2. 执行后续请求（穿过业务处理和认证中间件）
		c.Next()

		// 3. 后置身份检查：仅记录通过认证的请求
		userObj, exists := oauth.GetFromContext[*model.User](c, oauth.UserObjKey)
		if !exists || userObj == nil {
			return
		}

		// 4. 计算耗时并异步推送到缓冲队列
		latency := time.Since(start).Milliseconds()

		var headersStr string
		if c.Request.Header != nil {
			// 克隆 Header，避免污染原 HTTP 请求的 Header 对象
			clonedHeaders := make(http.Header)
			for k, v := range c.Request.Header {
				clonedHeaders[k] = v
			}
			clonedHeaders.Del("Cookie")

			if headersBytes, err := json.Marshal(clonedHeaders); err == nil {
				headersStr = string(headersBytes)
			}
		}

		const maxHTTPStatus = 999
		status := c.Writer.Status()
		if status < 0 {
			status = 0
		} else if status > maxHTTPStatus {
			status = maxHTTPStatus
		}

		logItem := &UserAccessLog{
			ID:        idgen.NextUint64ID(),
			UserID:    userObj.ID, // 直接从 Context 获取已登录用户ID，避免数据库查询
			Path:      c.Request.URL.Path,
			Method:    c.Request.Method,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			Headers:   headersStr,
			Status:    int32(status),
			Latency:   latency,
			CreatedAt: time.Now(),
		}

		// 非阻塞地推入缓存队列
		QueueAccessLog(logItem)
	}
}
