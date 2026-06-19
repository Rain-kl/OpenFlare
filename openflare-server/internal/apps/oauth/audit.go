// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package oauth 提供 OAuth/OIDC 认证与会话管理
package oauth

import (
	"context"
	"encoding/json"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
)

// LogForAudit 将登录鉴权审计日志写入 Logger
func LogForAudit(ctx context.Context, user *model.User, c *gin.Context) {
	auditLog := loginRequiredAuditLog{
		UserID:     user.ID,
		Username:   user.Username,
		ClientIP:   c.ClientIP(),
		Method:     c.Request.Method,
		Path:       c.Request.URL.Path,
		RequestURI: c.Request.RequestURI,
		UserAgent:  c.Request.UserAgent(),
		Referer:    c.Request.Referer(),
	}
	auditJSON, err := json.Marshal(auditLog)
	if err != nil {
		logger.ErrorF(ctx, "[LoginRequiredAudit] marshal failed: %v", err)
		logger.DebugF(ctx, "[LoginRequiredAudit] %s %d %s", c.ClientIP(), user.ID, user.Username)
	} else {
		logger.DebugF(ctx, "[LoginRequiredAudit] %s", auditJSON)
	}
}
