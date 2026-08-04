// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	cf "github.com/Rain-kl/Wavelet/internal/apps/openflare/cloudflare"
	"github.com/gin-gonic/gin"
)

func registerCloudflareRoutes(apiGroup *gin.RouterGroup) {
	route := apiGroup.Group("/cloudflare")
	route.Use(apiutil.AdminMiddlewares()...)
	route.GET("/connection", cf.GetConnectionHandler)
	route.PUT("/connection", cf.SaveConnectionHandler)
	route.POST("/connection/verify", cf.VerifyConnectionHandler)
	route.POST("/connection/clear", cf.ClearConnectionHandler)
	route.GET("/overview", cf.OverviewHandler)
	route.GET("/domains/available", cf.ListAvailableDomainsHandler)
	groups := route.Group("/groups")
	apiutil.RegisterCollection(groups, "GET", cf.ListGroupsHandler)
	apiutil.RegisterCollection(groups, "POST", cf.CreateGroupHandler)
	route.GET("/groups/:id", cf.GetGroupHandler)
	route.POST("/groups/:id/update", cf.UpdateGroupHandler)
	route.POST("/groups/:id/delete", cf.DeleteGroupHandler)
	route.POST("/groups/:id/sync", cf.SyncGroupHandler)
	route.GET("/groups/:id/members", cf.ListMembersHandler)
	route.POST("/groups/:id/members", cf.CreateMemberHandler)
	route.POST("/groups/:id/members/:memberId/update", cf.UpdateMemberHandler)
	route.POST("/groups/:id/members/:memberId/remove", cf.RemoveMemberHandler)
	route.POST("/groups/:id/members/:memberId/sync", cf.SyncMemberHandler)
}
