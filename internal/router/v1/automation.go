// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/automation"
	"github.com/gin-gonic/gin"
)

func registerAutomationRoutes(apiV1Router *gin.RouterGroup) {
	automation.RegisterAPI(apiV1Router.Group("/automation"))
}
