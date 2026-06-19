// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/config_version"
	"github.com/gin-gonic/gin"
)

func registerConfigVersionRoutes(apiGroup *gin.RouterGroup) {
	configVersionGroup := apiGroup.Group("/config-versions")
	configVersionGroup.Use(apiutil.AdminMiddlewares()...)
	{
		apiutil.RegisterCollection(configVersionGroup, "GET", config_version.ListConfigVersionsHandler)
		configVersionGroup.GET("/active", config_version.GetActiveConfigVersionHandler)
		configVersionGroup.GET("/preview", config_version.PreviewConfigVersionHandler)
		configVersionGroup.GET("/diff", config_version.DiffConfigVersionHandler)
		configVersionGroup.GET("/:id", config_version.GetConfigVersionHandler)
		configVersionGroup.POST("/publish", config_version.PublishConfigVersionHandler)
		configVersionGroup.POST("/:id/activate", config_version.ActivateConfigVersionHandler)
		configVersionGroup.POST("/cleanup", config_version.CleanupConfigVersionsHandler)
	}
}
