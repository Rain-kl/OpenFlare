// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pages"
	"github.com/gin-gonic/gin"
)

func registerPagesRoutes(apiGroup *gin.RouterGroup) {
	pagesRoute := apiGroup.Group("/pages")
	pagesRoute.Use(apiutil.AdminMiddlewares()...)
	{
		apiutil.RegisterCollection(pagesRoute, "GET", pages.ListProjectsHandler)
		pagesRoute.GET("/:id", pages.GetProjectHandler)
		apiutil.RegisterCollection(pagesRoute, "POST", pages.CreateProjectHandler)
		pagesRoute.POST("/:id/update", pages.UpdateProjectHandler)
		pagesRoute.POST("/:id/delete", pages.DeleteProjectHandler)
		pagesRoute.GET("/:id/deployments", pages.ListDeploymentsHandler)
		pagesRoute.POST("/:id/deployments/upload", pages.UploadDeploymentHandler)
		pagesRoute.POST("/:id/deployments/:deployment_id/activate", pages.ActivateDeploymentHandler)
		pagesRoute.POST("/:id/deployments/:deployment_id/delete", pages.DeleteDeploymentHandler)
		pagesRoute.GET("/deployments/:deployment_id/files", pages.ListDeploymentFilesHandler)
	}
}
