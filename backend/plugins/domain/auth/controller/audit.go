// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP handlers and middlewares for the auth plugin.
package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/auth/model/dto"
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// LogForAudit 将登录鉴权审计日志写入 Logger
func LogForAudit(ctx context.Context, user *contracts.UserDTO, c *gin.Context) {
	if user == nil || c == nil {
		return
	}
	auditLog := dto.LoginRequiredAuditLog{
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
