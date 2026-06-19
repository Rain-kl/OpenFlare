// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/dashboard"
	"github.com/gin-gonic/gin"
)

func registerDashboardRoutes(apiGroup *gin.RouterGroup) {
	dashboardRoute := apiGroup.Group("/dashboard")
	dashboardRoute.Use(apiutil.AdminMiddlewares()...)
	{
		dashboardRoute.GET("/overview", dashboard.GetOverviewHandler)
	}
}
