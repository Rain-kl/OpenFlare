// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package stats maintains incremental upload statistics and aggregations.
package stats

import (
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/upload/util"
)

const (
	// These are persisted counter keys, not display labels. Keep existing keys
	// stable; the frontend translates their presentation independently.
	catImage    = "图片"
	catVideo    = "视频"
	catAudio    = "音频"
	catDocument = "文档"
	catArchive  = "压缩包"
	catOther    = "其他"
)

// Categories returns the legacy persistent category keys in display order.
func Categories() []string {
	return []string{catImage, catVideo, catAudio, catDocument, catArchive, catOther}
}

// GetFileCategory classifies a file by mime type and extension.
func GetFileCategory(mimeType, ext string) string {
	mimeType = strings.ToLower(mimeType)
	ext = strings.ToLower(ext)

	if strings.HasPrefix(mimeType, "image/") || util.IsImageExtension(ext) {
		return catImage
	}
	if strings.HasPrefix(mimeType, "video/") {
		return catVideo
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return catAudio
	}
	if util.IsArchiveExtension(ext) || strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "tar") || strings.Contains(mimeType, "gzip") {
		return catArchive
	}
	if util.IsDocumentExtension(ext) || strings.HasPrefix(mimeType, "text/") || mimeType == "application/pdf" {
		return catDocument
	}
	return catOther
}
