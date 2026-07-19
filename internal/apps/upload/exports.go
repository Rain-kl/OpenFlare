// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"github.com/Rain-kl/Wavelet/internal/apps/upload/cache"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/filesrv"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/handler"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/ingest"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
	uploadstats "github.com/Rain-kl/Wavelet/internal/apps/upload/stats"
	uploadtask "github.com/Rain-kl/Wavelet/internal/apps/upload/task"
	"github.com/Rain-kl/Wavelet/internal/apps/upload/util"
	"github.com/Rain-kl/Wavelet/internal/task"
)

// HTTP handlers
var (
	UploadFile             = handler.UploadFile
	DownloadFile           = handler.DownloadFile
	BatchDownloadFiles     = handler.BatchDownloadFiles
	ListFiles              = handler.ListFiles
	DeleteFile             = handler.DeleteFile
	GetDistinctUploadTypes = handler.GetDistinctUploadTypes
	ListMyFiles            = handler.ListMyFiles
	DeleteMyFile           = handler.DeleteMyFile
	UpdateMyFile           = handler.UpdateMyFile
	GetFileStats           = handler.GetFileStats
	ServeFileByID          = filesrv.ServeFileByID
)

// Programmatic ingest API
var (
	Ingest                    = ingest.Ingest
	Remove                    = ingest.Remove
	RemoveOwned               = ingest.RemoveOwned
	RemoveLockedTx            = ingest.RemoveLockedTx
	InvalidateUploadMetaCache = ingest.InvalidateUploadMetaCache
	FindByHash                = ingest.FindByHash
	GetActiveUpload           = ingest.GetActive
	OpenStoredUpload          = ingest.OpenActiveObject
	ActiveUploadHash          = ingest.ActiveHash
	ResolveLocalFile          = ingest.ResolveLocalFile
	IngestFromLocalPath       = ingest.FromLocalPath
)

type (
	// OpenedUploadObject is the upload-domain view of a stored object stream.
	OpenedUploadObject = ingest.OpenedObject
	// LocalFileCandidateRequest describes filesystem locations that may host a legacy blob.
	LocalFileCandidateRequest = ingest.LocalFileCandidateRequest
)

// Ingest policy constants
const (
	PolicyCreate          = ingest.PolicyCreate
	PolicyDedupNewRecord  = ingest.PolicyDedupNewRecord
	PolicyResolveExisting = ingest.PolicyResolveExisting
	// ReservedPagesDeploymentType is managed exclusively by the Pages domain.
	ReservedPagesDeploymentType = shared.ReservedPagesDeploymentType
)

type (
	// IngestRequest is the programmatic upload ingest payload.
	IngestRequest = ingest.Request
	// IngestResult reports ingest side effects.
	IngestResult = ingest.Result
	// IngestPolicy controls hash-collision behavior during ingest.
	IngestPolicy = ingest.Policy
)

// Ingest errors
var (
	ErrIngestForbidden       = ingest.ErrForbidden
	ErrIngestStorageReadOnly = ingest.ErrStorageReadOnly
	ErrReservedUploadType    = ingest.ErrReservedUploadType
)

// Cache management
var (
	ResetAccessCaches              = cache.ResetAccessCaches
	PublishAccessCacheInvalidation = cache.PublishAccessCacheInvalidation
)

// Stats
var (
	// Deprecated: use upload.Ingest or upload.Remove; stats are applied internally.
	ApplyUploadStatsAdd = uploadstats.ApplyUploadStatsAdd
	// Deprecated: use upload.Ingest or upload.Remove; stats are applied internally.
	ApplyUploadStatsRemove = uploadstats.ApplyUploadStatsRemove
	RebuildUploadStats     = uploadstats.RebuildUploadStats
)

// Utilities
var (
	CompressImageToWebP = util.CompressImageToWebP
	ValidateS3Key       = util.ValidateS3Key
)

// Task identifiers and metadata
const (
	StorageMigrationTask   = uploadtask.StorageMigrationTask
	SystemCleanupTask      = uploadtask.SystemCleanupTask
	WarmImageCacheTask     = uploadtask.WarmImageCacheTask
	RebuildUploadStatsTask = uploadtask.RebuildUploadStatsTask
)

var (
	// StorageMigrationMeta describes the storage migration async task.
	StorageMigrationMeta = uploadtask.StorageMigrationMeta
	// SystemCleanupMeta describes the orphaned upload cleanup task.
	SystemCleanupMeta = uploadtask.SystemCleanupMeta
	// WarmImageCacheMeta describes the image compression cache warmup task.
	WarmImageCacheMeta = uploadtask.WarmImageCacheMeta
	// RebuildUploadStatsMeta describes the upload stats rebuild task.
	RebuildUploadStatsMeta = uploadtask.RebuildUploadStatsMeta
)

// MigrationHandler executes storage migration tasks.
type MigrationHandler = uploadtask.MigrationHandler

// SystemCleanupHandler removes orphaned upload files.
type SystemCleanupHandler = uploadtask.SystemCleanupHandler

// WarmImageCacheHandler pre-warms compressed image caches.
type WarmImageCacheHandler = uploadtask.WarmImageCacheHandler

// RebuildUploadStatsHandler rebuilds upload stats from active records.
type RebuildUploadStatsHandler = uploadtask.RebuildUploadStatsHandler

// WarmImageCachePayload is the payload for image cache warmup tasks.
type WarmImageCachePayload = uploadtask.WarmImageCachePayload

// Ensure task handler types implement required interfaces.
var (
	_ task.TaskHandler = (*MigrationHandler)(nil)
	_ task.TaskHandler = (*SystemCleanupHandler)(nil)
	_ task.TaskHandler = (*RebuildUploadStatsHandler)(nil)
	_ interface {
		task.TaskHandler
		ValidatePayload([]byte) ([]byte, error)
	} = (*WarmImageCacheHandler)(nil)
)
