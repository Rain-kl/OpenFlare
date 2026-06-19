// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package v1 contains router registrations for API V1
package v1

import (
	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	admin_auth_source "github.com/Rain-kl/Wavelet/internal/apps/admin/auth_source"
	admin_cache "github.com/Rain-kl/Wavelet/internal/apps/admin/cache"
	admin_db_manage "github.com/Rain-kl/Wavelet/internal/apps/admin/db_manage"
	admin_logs "github.com/Rain-kl/Wavelet/internal/apps/admin/logs"
	admin_push "github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	admin_status "github.com/Rain-kl/Wavelet/internal/apps/admin/status"
	"github.com/Rain-kl/Wavelet/internal/apps/admin/system_config"
	admin_task "github.com/Rain-kl/Wavelet/internal/apps/admin/task"
	admin_template "github.com/Rain-kl/Wavelet/internal/apps/admin/template"
	admin_updater "github.com/Rain-kl/Wavelet/internal/apps/admin/updater"
	admin_user "github.com/Rain-kl/Wavelet/internal/apps/admin/user"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes registers all admin-related routes with sub-group categorizations.
func RegisterAdminRoutes(apiV1Router *gin.RouterGroup) {
	adminRouter := apiV1Router.Group("/admin")
	adminRouter.Use(oauth.LoginRequired(), admin.LoginAdminRequired())
	{
		// 1. Diagnostics & Infrastructure Management
		registerAdminDiagnosticRoutes(adminRouter)

		// 2. Identity & Access Management (IAM)
		registerAdminIAMRoutes(adminRouter)

		// 3. System Configuration & Templates Settings
		registerAdminConfigRoutes(adminRouter)

		// 4. Storage & Asset Management
		registerAdminStorageRoutes(adminRouter)

		// 5. Task Orchestration & Automation
		registerAdminTaskRoutes(adminRouter)

		// 6. Messaging & Push Notifications
		registerAdminPushRoutes(adminRouter)
	}
}

// registerAdminDiagnosticRoutes registers infrastructure, system status, caching, database, and logs diagnostics.
func registerAdminDiagnosticRoutes(adminRouter *gin.RouterGroup) {
	// System status
	adminRouter.GET("/status", admin_status.GetSystemStatus)

	// Database basic info & backup export
	adminRouter.GET("/db-info", admin_status.GetDatabaseInfo)
	adminRouter.GET("/db-export", admin_status.ExportDatabase)

	// Database management & interactive browser
	dbManage := adminRouter.Group("/db-manage")
	{
		dbManage.GET("/overview", admin_db_manage.GetDBOverview)
		dbManage.GET("/tables", admin_db_manage.ListDBTables)
		dbManage.GET("/table-data", admin_db_manage.GetDBTableData)
		dbManage.POST("/query", admin_db_manage.ExecuteSQL)
	}

	// Cache management (TTL, LRU eviction and clear operations)
	cache := adminRouter.Group("/cache")
	{
		cache.GET("/status", admin_cache.GetCacheStatus)
		cache.POST("/config", admin_cache.UpdateCacheConfig)
		cache.POST("/clear", admin_cache.ClearCache)
	}

	// Application updater
	update := adminRouter.Group("/update")
	{
		update.GET("", admin_updater.GetUpdateStatus)
		update.POST("/apply", admin_updater.ApplyUpdate)
	}

	// System & access logs analytics
	logs := adminRouter.Group("/logs")
	{
		logs.GET("", admin_logs.GetLogs)
		logs.GET("/access", admin_logs.GetAccessLogs)
		logs.GET("/analytics", admin_logs.GetLogsAnalytics)
		logs.GET("/ws", admin_logs.HandleLogWebSocket)
	}
}

// registerAdminIAMRoutes registers Identity & Access Management endpoints (Users & Auth Sources).
func registerAdminIAMRoutes(adminRouter *gin.RouterGroup) {
	// Users management
	users := adminRouter.Group("/users")
	{
		users.GET("", admin_user.ListUsers)
		users.POST("", admin_user.CreateUser)
		users.GET("/:id", admin_user.GetUser)
		users.PUT("/:id/status", admin_user.UpdateUserStatus)
		users.DELETE("/:id", admin_user.DeleteUser)
	}

	// Authentication Sources (LDAP, OAuth sources, etc.)
	authSources := adminRouter.Group("/auth-sources")
	{
		authSources.GET("", admin_auth_source.ListAuthSources)
		authSources.POST("", admin_auth_source.CreateAuthSource)
		authSources.PUT("/:id", admin_auth_source.UpdateAuthSource)
		authSources.PUT("/:id/toggle", admin_auth_source.ToggleAuthSource)
		authSources.DELETE("/:id", admin_auth_source.DeleteAuthSource)
	}
}

// registerAdminConfigRoutes registers system configurations and template settings.
func registerAdminConfigRoutes(adminRouter *gin.RouterGroup) {
	// System configs
	configs := adminRouter.Group("/system-configs")
	{
		configs.GET("", system_config.ListSystemConfigs)
		configs.POST("", system_config.CreateSystemConfig)
		configs.POST("/smtp/test", system_config.TestSMTP)

		keyGroup := configs.Group("/:key")
		{
			keyGroup.GET("", system_config.GetSystemConfig)
			keyGroup.PUT("", system_config.UpdateSystemConfig)
		}
	}

	// Email/Notification Templates
	templates := adminRouter.Group("/templates")
	{
		templates.GET("", admin_template.ListTemplates)
		templates.POST("", admin_template.CreateTemplate)

		keyGroup := templates.Group("/:key")
		{
			keyGroup.GET("", admin_template.GetTemplate)
			keyGroup.PUT("", admin_template.UpdateTemplate)
			keyGroup.DELETE("", admin_template.DeleteTemplate)
		}
	}
}

// registerAdminStorageRoutes registers file and asset storage administration.
func registerAdminStorageRoutes(adminRouter *gin.RouterGroup) {
	uploads := adminRouter.Group("/uploads")
	{
		uploads.GET("", upload.ListFiles)
		uploads.GET("/stats", upload.GetFileStats)
		uploads.DELETE("/:id", upload.DeleteFile)
		uploads.GET("/download/:id", upload.DownloadFile)
		uploads.POST("/download/batch", upload.BatchDownloadFiles)
		uploads.GET("/types", upload.GetDistinctUploadTypes)
	}
}

// registerAdminTaskRoutes registers task orchestrations, execution logs and schedules.
func registerAdminTaskRoutes(adminRouter *gin.RouterGroup) {
	tasks := adminRouter.Group("/tasks")
	{
		// Task dispatch & metadata
		tasks.GET("/types", admin_task.ListTaskTypes)
		tasks.POST("/dispatch", admin_task.DispatchTask)

		// Task execution logs & manual retry
		executions := tasks.Group("/executions")
		{
			executions.GET("", admin_task.ListTaskExecutions)
			executions.GET("/:id", admin_task.GetTaskExecution)
			executions.POST("/:id/retry", admin_task.RetryTask)
		}

		// Cron scheduler settings
		schedules := tasks.Group("/schedules")
		{
			schedules.GET("", admin_task.ListSchedules)
			schedules.POST("", admin_task.CreateSchedule)
			schedules.PUT("/:id", admin_task.UpdateSchedule)
			schedules.DELETE("/:id", admin_task.DeleteSchedule)
		}
	}
}

// registerAdminPushRoutes registers messaging channels and push notification events.
func registerAdminPushRoutes(adminRouter *gin.RouterGroup) {
	push := adminRouter.Group("/push")
	{
		// Push Events
		events := push.Group("/events")
		{
			events.GET("", admin_push.ListEvents)
			events.GET("/builtin", admin_push.ListBuiltInEvents)
			events.POST("", admin_push.CreateEvent)
			events.PUT("/:id", admin_push.UpdateEvent)
			events.DELETE("/:id", admin_push.DeleteEvent)
			events.POST("/:id/toggle", admin_push.ToggleEvent)
		}

		// Delivery histories and diagnostics test
		push.GET("/histories", admin_push.ListHistories)
		push.POST("/test", admin_push.TestPush)

		// Message Channels CRUD
		channels := push.Group("/channels")
		{
			channels.GET("/definitions", admin_push.ListChannelDefinitions)
			channels.GET("", admin_push.ListChannels)
			channels.POST("", admin_push.CreateChannel)
			channels.PUT("/:id", admin_push.UpdateChannel)
			channels.DELETE("/:id", admin_push.DeleteChannel)
			channels.POST("/test", admin_push.TestChannel)
		}
	}
}
