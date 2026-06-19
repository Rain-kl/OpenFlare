// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package v1 contains router registrations for API V1
package v1

import (
	ofrouter "github.com/Rain-kl/Wavelet/internal/router/v1/openflare"
	"github.com/gin-gonic/gin"
)

// RegisterV1Routes registers all routes under API V1.
func RegisterV1Routes(apiV1Router *gin.RouterGroup, apiGroup *gin.RouterGroup) {
	// 1. User & Public routes (OAuth, User, Upload, CAPTCHA, Health, Config)
	RegisterUserRoutes(apiV1Router, apiGroup)

	// 2. Admin routes
	RegisterAdminRoutes(apiV1Router)

	// 3. OpenFlare management console APIs and Agent/Relay/Tunnel protocol routes
	ofrouter.RegisterV1Routes(apiV1Router)
	ofrouter.RegisterRoutes(apiV1Router)

	// 4. Custom business routes (example only)
	RegisterCustomRoutes(apiV1Router)
}
