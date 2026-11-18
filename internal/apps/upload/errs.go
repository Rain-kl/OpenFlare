// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package upload 提供文件上传、存储管理、访问服务与统计功能
package upload

import "errors"

// 文件管理文案常量与摄取 sentinel 错误
const (
	ErrNoFileSelected                  = "Select a file to upload"
	ErrUnsupportedFormat               = "only JPG, PNG and WEBP images are supported"
	ErrProcessFileFailed               = "Failed to process file"
	ErrSaveFileFailed                  = "failed to save file"
	ErrOpenFileFailed                  = "Failed to open file"
	ErrSaveUploadRecordFailed          = "Failed to save upload record"
	ErrGenericFileTooLarge             = "File size must not exceed 32 MB"
	ErrFileContentExtensionMismatch    = "File content does not match its extension and may be unsafe"
	ErrFileValidationFailed            = "File validation failed"
	ErrInvalidMetadataJSON             = "Invalid metadata JSON format"
	ErrInvalidFileID                   = "Invalid file ID"
	ErrQueryUploadRecordFailed         = "Failed to query file record"
	ErrInvalidBatchDownloadRequest     = "Failed to bind parameters; provide a valid array of file IDs"
	ErrInvalidIDValueFormat            = "Invalid ID value: %s"
	ErrRetrieveUploadRecordsFailed     = "Failed to retrieve file records"
	ErrNoValidFilesForArchive          = "No valid file records were found to archive"
	ErrInvalidParams                   = "invalid parameters"
	ErrQueryFileCountFailed            = "Failed to query file count"
	ErrQueryFileListFailed             = "Failed to query file list"
	ErrDeleteFileFailed                = "Failed to delete file"
	errReservedUploadTypeMsg           = "system-reserved file types cannot be managed through the generic file API"
	ErrStorageReadOnly                 = "storage migration maintenance is active; files are read-only"
	ErrS3KeyRequired                   = "s3 key must not be empty"
	ErrS3KeyTooLongFormat              = "s3 key exceeds maximum length of %d"
	ErrS3KeyStartsWithSlash            = "s3 key must not start with /"
	ErrS3KeyContainsNullBytes          = "s3 key must not contain null bytes"
	ErrQueryUnusedUploadsFailed        = "failed to query unused uploads: %w"
	ErrImageCacheWarmupPayloadRequired = "image cache warmup parameters must not be empty"
	ErrInvalidImageCacheWarmupPayload  = "invalid image cache warmup parameters: %w"
	ErrInvalidImageCacheWarmupQuality  = "image quality must be low, medium or high"
	ErrParseImageCacheWarmupPayload    = "failed to parse image cache warmup parameters: %w"
	ErrQueryImagesForCacheWarmup       = "failed to query images for cache warmup: %w"
)

var (
	// ErrIngestForbidden indicates the caller is not allowed to mutate the upload record.
	ErrIngestForbidden = errors.New("upload forbidden")
	// ErrReservedUploadType indicates that a generic mutation targeted a domain-reserved upload type.
	ErrReservedUploadType = errors.New(errReservedUploadTypeMsg)
	// ErrIngestStorageReadOnly indicates the storage backend is in migration read-only mode.
	ErrIngestStorageReadOnly = errors.New(ErrStorageReadOnly)
)
