// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package shared

// Upload size, path, media quality, and cache constants shared across subpackages.
const (
	MaxUploadSize           = 32 * 1024 * 1024 // 32MB
	DetectContentBytes      = 512              // http.DetectContentType 需要的最小字节数
	UploadDirPerm           = 0755             // 上传目录权限
	UploadFilePerm          = 0644             // 上传文件权限
	ImageQualityLow         = "low"
	ImageQualityMedium      = "medium"
	ImageQualityHigh        = "high"
	ImageQualityOrigin      = "origin"
	DefaultPublicUploadType = "avatar"
	FileStatsTrendDays      = 7
	MaxS3KeyLength          = 1024
	AccessCacheTTL          = 5 // seconds; multiplied by time.Second at use site
	// ReservedPagesDeploymentType is managed exclusively by the Pages domain.
	ReservedPagesDeploymentType = "openflare_pages_deployment"
)
