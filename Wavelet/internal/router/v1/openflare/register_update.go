// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/update"
	"github.com/gin-gonic/gin"
)

func registerUpdateRoutes(apiGroup *gin.RouterGroup) {
	updateRoute := apiGroup.Group("/update")
	updateRoute.Use(apiutil.AdminRequired())
	{
		updateRoute.GET("/latest-release", update.GetLatestReleaseHandler)
		updateRoute.GET("/logs/ws", update.StreamServerUpgradeLogsHandler)
		updateRoute.POST("/manual-upload", update.UploadManualServerBinaryHandler)
		updateRoute.POST("/manual-upgrade", update.ConfirmManualServerUpgradeHandler)
		updateRoute.POST("/upgrade", update.UpgradeServerHandler)
	}
}
