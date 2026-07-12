// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/zone"
	"github.com/gin-gonic/gin"
)

func registerZoneRoutes(apiGroup *gin.RouterGroup) {
	zoneGroup := apiGroup.Group("/zones")
	zoneGroup.Use(apiutil.AdminMiddlewares()...)
	apiutil.RegisterCollection(zoneGroup, "GET", zone.ListHandler)
	apiutil.RegisterCollection(zoneGroup, "POST", zone.CreateHandler)
	zoneGroup.GET("/:id/overview", zone.GetOverviewHandler)
	zoneGroup.POST("/:id/update", zone.UpdateHandler)
	zoneGroup.POST("/:id/delete", zone.DeleteHandler)
	zoneGroup.POST("/:id/domains", zone.CreateDomainHandler)
	zoneGroup.POST("/:id/domains/:domainId/update", zone.UpdateDomainHandler)
	zoneGroup.POST("/:id/domains/:domainId/delete", zone.DeleteDomainHandler)
}
