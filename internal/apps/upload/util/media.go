// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"slices"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/upload/shared"
)

// IsImageExtension reports whether ext is a common image format.
func IsImageExtension(ext string) bool {
	return slices.Contains([]string{"jpg", "jpeg", "png", "webp", "gif"}, ext)
}

// IsArchiveExtension reports whether ext is a common archive format.
func IsArchiveExtension(ext string) bool {
	return slices.Contains([]string{"zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "xz"}, ext)
}

// IsDocumentExtension reports whether ext is a common document format.
func IsDocumentExtension(ext string) bool {
	return slices.Contains([]string{"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "md", "csv", "json", "yaml", "yml", "xml"}, ext)
}

// NormalizeImageQuality normalizes the requested image quality query parameter.
func NormalizeImageQuality(quality string) string {
	switch strings.ToLower(quality) {
	case shared.ImageQualityLow, shared.ImageQualityMedium, shared.ImageQualityHigh:
		return strings.ToLower(quality)
	default:
		return shared.ImageQualityOrigin
	}
}
