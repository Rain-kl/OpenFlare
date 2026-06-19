// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/origin"
	"github.com/gin-gonic/gin"
)

func registerOriginRoutes(apiGroup *gin.RouterGroup) {
	originRoute := apiGroup.Group("/origins")
	originRoute.Use(apiutil.AdminMiddlewares()...)
	{
		apiutil.RegisterCollection(originRoute, "GET", origin.GetOrigins)
		originRoute.GET("/:id", origin.GetOrigin)
		apiutil.RegisterCollection(originRoute, "POST", origin.CreateOriginHandler)
		originRoute.POST("/:id/update", origin.UpdateOriginHandler)
		originRoute.POST("/:id/delete", origin.DeleteOriginHandler)
	}
}
