// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package root

import (
	_ "github.com/Rain-kl/Wavelet/docs" // Swagger documentation generation setup
	publicconfig "github.com/Rain-kl/Wavelet/internal/apps/config"
	"github.com/Rain-kl/Wavelet/internal/apps/health"
	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterDefaultRootRoutes registers default routes that belong to the root path.
func RegisterDefaultRootRoutes(r *gin.Engine) {
	// 1. Serve files by ID
	r.GET("/f/:id", upload.ServeFileByID)

	// 2. Dynamic robots.txt serving
	r.GET("/robots.txt", publicconfig.GetRobotsTXT)

	// 3. Swagger routes (Non-production only)
	if !config.Config.App.IsProduction() {
		r.GET(config.Config.App.APIPrefix+"/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// 4. Health check
	r.GET(config.Config.App.APIPrefix+"/health", health.Health)
}
