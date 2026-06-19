// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/option"
	"github.com/gin-gonic/gin"
)

func registerOptionRoutes(apiGroup *gin.RouterGroup) {
	apiGroup.GET("/status", option.GetStatusHandler)
	apiGroup.GET("/notice", option.GetNoticeHandler)

	optionRoute := apiGroup.Group("/option")
	optionRoute.Use(apiutil.AdminMiddlewares()...)
	{
		apiutil.RegisterCollection(optionRoute, "GET", option.ListOptionsHandler)
		optionRoute.POST("/update", option.UpdateOptionHandler)
		optionRoute.POST("/update-batch", option.UpdateOptionsBatchHandler)
		optionRoute.POST("/geoip/lookup", option.LookupGeoIPHandler)
		optionRoute.POST("/database/cleanup", option.CleanupDatabaseHandler)
	}

	uptimeKumaRoute := apiGroup.Group("/uptimekuma")
	uptimeKumaRoute.Use(apiutil.AdminMiddlewares()...)
	{
		uptimeKumaRoute.POST("/sync", option.SyncUptimeKumaHandler)
	}
}
