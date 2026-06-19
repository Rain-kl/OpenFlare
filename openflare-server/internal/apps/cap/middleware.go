// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/internal/common/response"
)

// VerifyMiddleware returns a Gin middleware that checks and consumes the X-Cap-Token header.
func VerifyMiddleware(mgr *Manager, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ProtectionEnabled(c.Request.Context()) {
			c.Next()
			return
		}

		token := c.GetHeader("X-Cap-Token")
		if token == "" {
			response.AbortUnauthorized(c, errCapTokenMissing)
			return
		}

		valid, err := mgr.VerifyToken(c.Request.Context(), token, scope)
		if err != nil || !valid {
			response.AbortUnauthorized(c, errCapTokenInvalidOrExpired)
			return
		}

		c.Next()
	}
}
