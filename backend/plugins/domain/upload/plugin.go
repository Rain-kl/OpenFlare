// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package upload provides file uploading, storage abstraction, image transcoding, and caching domain plugin for Cordis.
package upload

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/ginutil"
	"Wavelet/plugins/domain/upload/filesrv"
	"Wavelet/plugins/domain/upload/handler"
	"Wavelet/plugins/domain/upload/shared"
	"Wavelet/plugins/domain/upload/task"
	"context"
	"embed"
	"reflect"

	"github.com/gin-gonic/gin"
)

//go:embed migrations/*/*.sql
var uploadMigrations embed.FS

// Plugin implements core.Plugin to provide file upload and media serving domain services.
type Plugin struct{}

// New creates a new upload domain plugin.
func New() *Plugin {
	return &Plugin{}
}

// Name returns the unique identifier for the upload domain plugin.
func (p *Plugin) Name() string {
	return "upload"
}

// Inject declares required dependencies for the upload domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		reflect.TypeFor[contracts.StorageService](),
		reflect.TypeFor[contracts.AuthService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "upload",
		Version:     "1.0.0",
		Description: "File upload, secure delivery, image transcoding, and storage management domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers upload routes, tasks, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	core.Bind[contracts.DBService](ctx, shared.SetDBService)
	core.Bind[contracts.CacheService](ctx, shared.SetCacheService)
	core.Bind[contracts.StorageService](ctx, shared.SetStorageService)
	core.Bind[contracts.TaskService](ctx, shared.SetTaskService)
	core.Bind[contracts.AuthService](ctx, shared.SetAuthService)
	core.Provide[contracts.UploadService](ctx, &uploadServiceImpl{})

	ctx.OnDispose(func() error {
		shared.ResetServices()
		return nil
	})

	denyAuth := ginutil.AuthUnavailable()
	loginMW := func(c *gin.Context) {
		if svc := shared.GetAuthService(c.Request.Context()); svc != nil {
			if mw, ok := svc.RequireAuthMiddleware().(gin.HandlerFunc); ok && mw != nil {
				mw(c)
				return
			}
		}
		denyAuth(c)
	}
	adminMW := func(c *gin.Context) {
		if svc := shared.GetAuthService(c.Request.Context()); svc != nil {
			if mw, ok := svc.RequireAdminMiddleware().(gin.HandlerFunc); ok && mw != nil {
				mw(c)
				return
			}
		}
		denyAuth(c)
	}

	// 0a. Register migrations
	ctx.Migrations().Register("upload", uploadMigrations)

	// 1. Register File Server Routes
	// TIP: loginMW populates AuthUserObjKey so private files can be checked for ownership.
	ctx.Router().GET("/f/:id", loginMW, filesrv.ServeFileByID)

	// 2. Register User/Admin Upload HTTP Routes
	uploadGroup := ctx.Router().Group("/api/v1/upload", loginMW)
	{
		uploadGroup.POST("", handler.UploadFile)
		uploadGroup.DELETE("/:id", handler.DeleteMyFile)
		uploadGroup.GET("/my", handler.ListMyFiles)
		uploadGroup.PUT("/:id", handler.UpdateMyFile)
		uploadGroup.GET("/download/:id", handler.DownloadFile)
		uploadGroup.POST("/download/batch", handler.BatchDownloadFiles)
	}

	adminUploadGroup := ctx.Router().Group("/api/v1/admin/uploads", loginMW, adminMW)
	{
		adminUploadGroup.GET("", handler.ListFiles)
		adminUploadGroup.GET("/stats", handler.GetFileStats)
		adminUploadGroup.DELETE("/:id", handler.DeleteFile)
		adminUploadGroup.GET("/download/:id", handler.DownloadFile)
		adminUploadGroup.POST("/download/batch", handler.BatchDownloadFiles)
		adminUploadGroup.GET("/types", handler.GetDistinctUploadTypes)
	}

	const (
		defaultCleanupRetry = 3
		defaultStatsRetry   = 2
		defaultSingleRetry  = 1
	)

	ctx.Task().Register(task.SystemCleanupTask, &task.SystemCleanupHandler{}, extpoints.WithTaskMeta(task.SystemCleanupMeta), extpoints.WithTaskRetry(defaultCleanupRetry))
	ctx.Task().Register(task.RebuildUploadStatsTask, &task.RebuildUploadStatsHandler{}, extpoints.WithTaskMeta(task.RebuildUploadStatsMeta), extpoints.WithTaskRetry(defaultStatsRetry))
	ctx.Task().Register(task.StorageMigrationTask, &task.MigrationHandler{}, extpoints.WithTaskMeta(task.StorageMigrationMeta), extpoints.WithTaskRetry(defaultSingleRetry))
	ctx.Task().Register(task.WarmImageCacheTask, &task.WarmImageCacheHandler{}, extpoints.WithTaskMeta(task.WarmImageCacheMeta), extpoints.WithTaskRetry(1))

	// 4. Register Event Listeners for domain events
	ctx.Events().On(contracts.EventTopicSystemCleanup, func(c context.Context, _ contracts.SystemCleanupEvent) error {
		_, _, err := task.CleanupOrphanUploads(c)
		return err
	})

	// 5. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "upload.max_file_size_mb",
		Default:     100,
		Description: "Maximum upload file size limit in MB",
		Type:        "integer",
		Category:    "storage",
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "upload.allowed_extensions",
		Default:     "png,jpg,jpeg,gif,webp,svg,pdf,zip,tar,gz",
		Description: "Allowed upload file extensions separated by commas",
		Type:        "string",
		Category:    "storage",
	})

	return nil
}
