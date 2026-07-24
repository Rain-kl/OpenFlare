// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package risk_control 提供风险控制中间件
package risk_control

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const maxAuditLogHeadersBytes = 2 * 1024

var auditLogHeaderAllowlist = map[string]struct{}{
	"Authorization":   {},
	"Cookie":          {},
	"X-Forwarded-For": {},
	"X-Real-Ip":       {},
	"User-Agent":      {},
	"Content-Type":    {},
}

func marshalAuditLogHeaders(headers http.Header) string {
	if headers == nil {
		return ""
	}

	filtered := make(http.Header)
	for key, values := range headers {
		if _, ok := auditLogHeaderAllowlist[key]; !ok {
			continue
		}
		filtered[key] = redactAuditLogHeaderValues(key, values)
	}

	headersBytes, err := json.Marshal(filtered)
	if err != nil {
		return ""
	}
	if len(headersBytes) <= maxAuditLogHeadersBytes {
		return string(headersBytes)
	}
	return string(headersBytes[:maxAuditLogHeadersBytes])
}

func redactAuditLogHeaderValues(key string, values []string) []string {
	switch key {
	case "Authorization", "Cookie":
		redacted := make([]string, len(values))
		for i, value := range values {
			redacted[i] = hashAuditLogSensitiveValue(value)
		}
		return redacted
	default:
		return values
	}
}

func hashAuditLogSensitiveValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

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

		headersStr := marshalAuditLogHeaders(c.Request.Header)

		const maxHTTPStatus = 999
		status := c.Writer.Status()
		if status < 0 {
			status = 0
		} else if status > maxHTTPStatus {
			status = maxHTTPStatus
		}

		logItem := &analytics.UserAccessLog{
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
