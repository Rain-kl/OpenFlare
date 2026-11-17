// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pow"
	"github.com/gin-gonic/gin"
)

func registerPoWRoutes(apiGroup *gin.RouterGroup) {
	routes := apiGroup.Group("/pow")
	routes.Use(apiutil.AdminMiddlewares()...)
	routes.GET("/policies", pow.ListPoliciesHandler)
	routes.PUT("/policies/:id", pow.SavePolicyHandler)
}
