// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package pagesarchive provides multi-format archive detection, inspection and
// extraction helpers for OpenFlare Pages deployment packages.
package pagesarchive

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

// Format identifies a supported Pages deployment package archive format.
type Format string

const (
	// FormatZip is a ZIP archive.
	FormatZip Format = "zip"
	// FormatTar is an uncompressed tar archive.
	FormatTar Format = "tar"
	// FormatTarGz is a gzip-compressed tar archive.
	FormatTarGz Format = "tar.gz"
	// FormatTarXz is an xz-compressed tar archive.
	FormatTarXz Format = "tar.xz"
	// FormatTarBz2 is a bzip2-compressed tar archive.
	FormatTarBz2 Format = "tar.bz2"
	// FormatSevenZip is a 7z archive.
	FormatSevenZip Format = "7z"

	ustarMagicOffset = 257
	ustarMagicMinLen = 262
)

// DetectFormatFromName returns the archive format inferred from a file name.
// Returns an empty Format and false when the extension is unsupported.
func DetectFormatFromName(fileName string) (Format, bool) {
	name := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		return FormatTarGz, true
	case strings.HasSuffix(name, ".tar.xz"), strings.HasSuffix(name, ".txz"):
		return FormatTarXz, true
	case strings.HasSuffix(name, ".tar.bz2"), strings.HasSuffix(name, ".tbz2"), strings.HasSuffix(name, ".tbz"):
		return FormatTarBz2, true
	case strings.HasSuffix(name, ".tar"):
		return FormatTar, true
	case strings.HasSuffix(name, ".7z"):
		return FormatSevenZip, true
	case strings.HasSuffix(name, ".zip"):
		return FormatZip, true
	default:
		return "", false
	}
}

// DetectFormatFromBytes returns the archive format inferred from magic bytes.
// Prefer DetectFormatFromName when a reliable file name is available.
func DetectFormatFromBytes(data []byte) (Format, bool) {
	if isZipMagic(data) {
		return FormatZip, true
	}
	if isSevenZipMagic(data) {
		return FormatSevenZip, true
	}
	if isXZMagic(data) {
		return FormatTarXz, true
	}
	if isGzipMagic(data) {
		return FormatTarGz, true
	}
	if isBzip2Magic(data) {
		return FormatTarBz2, true
	}
	if looksLikeTar(data) {
		return FormatTar, true
	}
	return "", false
}

// DetectFormat prefers the file name when present, otherwise magic bytes.
func DetectFormat(fileName string, data []byte) (Format, error) {
	if format, ok := DetectFormatFromName(fileName); ok {
		return format, nil
	}
	if format, ok := DetectFormatFromBytes(data); ok {
		return format, nil
	}
	return "", fmt.Errorf("unsupported pages package format")
}

// Extension returns the canonical file extension for a format (without leading dot).
func Extension(format Format) string {
	switch format {
	case FormatZip:
		return "zip"
	case FormatTar:
		return "tar"
	case FormatTarGz:
		return "tar.gz"
	case FormatTarXz:
		return "tar.xz"
	case FormatTarBz2:
		return "tar.bz2"
	case FormatSevenZip:
		return "7z"
	default:
		return "bin"
	}
}

// MIMEType returns a reasonable content type for the archive format.
func MIMEType(format Format) string {
	switch format {
	case FormatZip:
		return "application/zip"
	case FormatTar:
		return "application/x-tar"
	case FormatTarGz:
		return "application/gzip"
	case FormatTarXz:
		return "application/x-xz"
	case FormatTarBz2:
		return "application/x-bzip2"
	case FormatSevenZip:
		return "application/x-7z-compressed"
	default:
		return "application/octet-stream"
	}
}

// SupportedExtensions lists human-readable extensions for UI copy and accept attributes.
func SupportedExtensions() []string {
	return []string{".zip", ".tar.gz", ".tgz", ".tar.xz", ".txz", ".tar.bz2", ".tbz2", ".tar", ".7z"}
}

// AcceptAttribute returns a comma-separated accept list for file inputs.
func AcceptAttribute() string {
	return strings.Join(SupportedExtensions(), ",")
}

// NormalizeNameExtension returns a storage-safe extension for the given format/name.
func NormalizeNameExtension(fileName string, format Format) string {
	if format != "" {
		return Extension(format)
	}
	if formatFromName, ok := DetectFormatFromName(fileName); ok {
		return Extension(formatFromName)
	}
	ext := strings.TrimPrefix(filepath.Ext(fileName), ".")
	if ext == "" {
		return "bin"
	}
	return strings.ToLower(ext)
}

func isZipMagic(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 0x50 && data[1] == 0x4b &&
		(data[2] == 0x03 || data[2] == 0x05 || data[2] == 0x07) &&
		(data[3] == 0x04 || data[3] == 0x06 || data[3] == 0x08)
}

func isSevenZipMagic(data []byte) bool {
	return len(data) >= 6 &&
		data[0] == 0x37 && data[1] == 0x7a && data[2] == 0xbc &&
		data[3] == 0xaf && data[4] == 0x27 && data[5] == 0x1c
}

func isXZMagic(data []byte) bool {
	return len(data) >= 6 &&
		data[0] == 0xfd && data[1] == 0x37 && data[2] == 0x7a &&
		data[3] == 0x58 && data[4] == 0x5a && data[5] == 0x00
}

func isGzipMagic(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

func isBzip2Magic(data []byte) bool {
	return len(data) >= 3 && data[0] == 0x42 && data[1] == 0x5a && data[2] == 0x68
}

func looksLikeTar(data []byte) bool {
	// POSIX ustar magic at offset 257 ("ustar\0" or "ustar ").
	if len(data) < ustarMagicMinLen {
		return false
	}
	return bytes.Equal(data[ustarMagicOffset:ustarMagicOffset+5], []byte("ustar"))
}
