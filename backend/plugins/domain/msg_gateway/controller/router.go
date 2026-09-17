// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP endpoints for msg_gateway.
package controller

import (
	"Wavelet/core/extpoints"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes mounts user-facing message gateway endpoints.
func RegisterUserRoutes(r extpoints.RouterExtension, loginMW gin.HandlerFunc) {
	mg := r.Group("/message-gateway", loginMW)
	{
		mg.GET("/channels", ListChannels)
		mg.GET("/bindings", ListBindings)
		mg.POST("/bindings", BindBinding)
		mg.DELETE("/bindings/:id", UnbindBinding)
	}
}

// RegisterAdminRoutes mounts admin message-gateway APIs under /admin.
func RegisterAdminRoutes(adminRouter extpoints.RouterExtension, loginMW, adminMW gin.HandlerFunc) {
	g := adminRouter.Group("/message-gateway", loginMW, adminMW)
	{
		g.GET("/channels/definitions", ListAdminChannelDefinitions)
		g.GET("/channels", ListAdminChannels)
		g.POST("/channels", CreateAdminChannel)
		g.PATCH("/channels/:id", UpdateAdminChannel)
		g.DELETE("/channels/:id", DeleteAdminChannel)
		g.POST("/channels/:id/test", TestAdminChannel)
	}
}

// RegisterAdminPushRoutes mounts admin push notification APIs under /admin.
func RegisterAdminPushRoutes(adminRouter extpoints.RouterExtension, loginMW, adminMW gin.HandlerFunc) {
	adminPushGroup := adminRouter.Group("/push", loginMW, adminMW)
	{
		events := adminPushGroup.Group("/events")
		{
			events.GET("", ListPushEvents)
			events.GET("/builtin", ListBuiltInPushEvents)
			events.POST("", CreatePushEvent)
			events.PUT("/:id", UpdatePushEvent)
			events.DELETE("/:id", DeletePushEvent)
			events.POST("/:id/toggle", TogglePushEvent)
		}

		adminPushGroup.GET("/histories", ListPushHistories)
		adminPushGroup.POST("/test", TestPush)

		channels := adminPushGroup.Group("/channels")
		{
			channels.GET("/definitions", ListPushChannelDefinitions)
			channels.GET("", ListPushChannels)
			channels.POST("", CreatePushChannel)
			channels.PUT("/:id", UpdatePushChannel)
			channels.DELETE("/:id", DeletePushChannel)
			channels.POST("/test", TestPushChannel)
		}
	}
}
