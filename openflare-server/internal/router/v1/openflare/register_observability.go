// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/observability"
	"github.com/gin-gonic/gin"
)

func registerObservabilityRoutes(apiGroup *gin.RouterGroup) {
	accessLogRoute := apiGroup.Group("/access-logs")
	accessLogRoute.Use(apiutil.AdminMiddlewares()...)
	{
		apiutil.RegisterCollection(accessLogRoute, "GET", observability.GetAccessLogsHandler)
		accessLogRoute.GET("/folds", observability.GetFoldedAccessLogsHandler)
		accessLogRoute.GET("/folds/ip-summary", observability.GetFoldedAccessLogIPsHandler)
		accessLogRoute.GET("/ip-summary", observability.GetAccessLogIPSummariesHandler)
		accessLogRoute.GET("/ip-summary/trend", observability.GetAccessLogIPTrendHandler)
		accessLogRoute.POST("/cleanup", observability.CleanupAccessLogsHandler)
	}
}
