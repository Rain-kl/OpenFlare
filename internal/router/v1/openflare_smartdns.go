// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/smartdns"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/gin-gonic/gin"
)

func registerOpenFlareSmartDNSRoutes(apiGroup *gin.RouterGroup) {
	dnsRoute := apiGroup.Group("/smartdns")
	dnsRoute.Use(openFlareAdminMiddlewares()...)
	{
		dnsRoute.GET("/overview", smartdns.OverviewHandler)
		dnsRoute.GET("/settings", smartdns.GetSettingsHandler)
		dnsRoute.POST("/settings", smartdns.SaveSettingsHandler)
		dnsRoute.POST("/settings/verify-delegation", smartdns.VerifyDelegationHandler)
		dnsRoute.POST("/settings/apply-delegation", smartdns.ApplyDelegationHandler)

		dnsRoute.GET("/discovery-token", smartdns.GetDiscoveryTokenHandler)
		dnsRoute.POST("/discovery-token/rotate", smartdns.RotateDiscoveryTokenHandler)

		instancesRoute := dnsRoute.Group("/instances")
		{
			httpx.RegisterCollection(instancesRoute, "GET", smartdns.ListInstancesHandler)
			httpx.RegisterCollection(instancesRoute, "POST", smartdns.CreateInstanceHandler)
			instancesRoute.POST("/:id/update", smartdns.UpdateInstanceHandler)
			instancesRoute.POST("/remote-update", smartdns.RequestInstanceUpdatesHandler)
			instancesRoute.POST("/:id/delete", smartdns.DeleteInstanceHandler)
			instancesRoute.POST("/:id/quarantine", smartdns.QuarantineInstanceHandler)
			instancesRoute.POST("/:id/unquarantine", smartdns.UnquarantineInstanceHandler)
			instancesRoute.POST("/:id/rotate-token", smartdns.RotateInstanceTokenHandler)
		}

		policiesRoute := dnsRoute.Group("/policies")
		{
			httpx.RegisterCollection(policiesRoute, "GET", smartdns.ListPoliciesHandler)
			httpx.RegisterCollection(policiesRoute, "POST", smartdns.CreatePolicyHandler)
			policiesRoute.POST("/validate", smartdns.ValidatePolicyHandler)
			policiesRoute.GET("/:id", smartdns.GetPolicyHandler)
			policiesRoute.POST("/:id/update", smartdns.UpdatePolicyHandler)
			policiesRoute.POST("/:id/delete", smartdns.DeletePolicyHandler)
			policiesRoute.POST("/:id/set-default", smartdns.SetDefaultPolicyHandler)
		}

		dnsRoute.POST("/simulate", smartdns.SimulateHandler)
	}
}
