// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/nettools"
	"github.com/gin-gonic/gin"
)

func registerOpenFlareNetworkToolsRoutes(apiGroup *gin.RouterGroup) {
	toolsRoute := apiGroup.Group("/network-tools")
	toolsRoute.Use(openFlareAdminMiddlewares()...)
	{
		toolsRoute.POST("/ping", nettools.DispatchPingHandler)
		toolsRoute.POST("/traceroute", nettools.DispatchTracerouteHandler)
		toolsRoute.GET("/tasks/:task_id", nettools.GetTaskHandler)
		toolsRoute.GET("/traceroute/history", nettools.ListTracerouteHistoryHandler)
		toolsRoute.POST("/dns", nettools.QueryDNSHandler)
		toolsRoute.GET("/lookup/ip", nettools.LookupIPHandler)
		toolsRoute.GET("/lookup/asn/:asn", nettools.LookupASNHandler)
		toolsRoute.GET("/lookup/asn/:asn/prefixes", nettools.ListASNPrefixesHandler)
		toolsRoute.POST("/as-path", nettools.TraceASPathHandler)
		toolsRoute.GET("/quality-matrix", nettools.ListQualityMatrixHandler)
		toolsRoute.GET("/netintel/status", nettools.GetNetIntelStatusHandler)
		toolsRoute.POST("/netintel/sync", nettools.TriggerNetIntelSyncHandler)
	}
}
