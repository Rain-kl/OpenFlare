// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP handlers and middlewares for the auth plugin.
package controller

import (
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/service"

	"github.com/gin-gonic/gin"
)

// VerifyCaptchaMiddleware returns a Gin middleware that checks and consumes the X-Cap-Token header.
func VerifyCaptchaMiddleware(mgr *service.CaptchaManager, settingsMgr *service.CapSettingsManager, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if settingsMgr != nil && !settingsMgr.CapProtectionEnabled(c.Request.Context()) {
			c.Next()
			return
		}
		if mgr == nil {
			response.AbortBadRequest(c, consts.ErrCapTokenInvalidOrExpired)
			return
		}

		token := c.GetHeader("X-Cap-Token")
		if token == "" {
			response.AbortBadRequest(c, consts.ErrCapTokenMissing)
			return
		}

		valid, err := mgr.VerifyToken(c.Request.Context(), token, scope)
		if err != nil || !valid {
			response.AbortBadRequest(c, consts.ErrCapTokenInvalidOrExpired)
			return
		}

		c.Next()
	}
}
