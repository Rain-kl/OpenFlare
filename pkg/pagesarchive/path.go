// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// NormalizeEntryPath cleans an archive entry path and rejects zip-slip / absolute paths.
// skip=true means the entry should be ignored (empty path or directory marker).
func NormalizeEntryPath(raw string) (cleaned string, skip bool, err error) {
	name := strings.TrimSpace(filepath.ToSlash(raw))
	if name == "" {
		return "", true, nil
	}
	if strings.HasSuffix(name, "/") {
		return "", true, nil
	}
	if strings.HasPrefix(name, "/") || path.IsAbs(name) {
		return "", false, fmt.Errorf("pages package contains absolute path: %s", raw)
	}
	// Reject Windows drive / UNC-style paths that may appear after ToSlash.
	if len(name) >= 2 && name[1] == ':' {
		return "", false, fmt.Errorf("pages package contains absolute path: %s", raw)
	}
	cleanedPath := path.Clean(name)
	if cleanedPath == "." {
		return "", true, nil
	}
	if cleanedPath == ".." || strings.HasPrefix(cleanedPath, "../") || strings.Contains(cleanedPath, "/../") {
		return "", false, fmt.Errorf("pages package path escapes directory: %s", raw)
	}
	return cleanedPath, false, nil
}

// FindCommonRootPrefix returns a trailing-slash directory prefix shared by all file paths.
// When files do not share a single root folder the result is empty.
func FindCommonRootPrefix(paths []string) string {
	var firstFilePath string
	hasMultipleFiles := false
	for _, item := range paths {
		normalizedPath, skip, err := NormalizeEntryPath(item)
		if err != nil || skip {
			continue
		}
		if firstFilePath == "" {
			firstFilePath = normalizedPath
		} else {
			hasMultipleFiles = true
		}
	}
	if firstFilePath == "" {
		return ""
	}
	parts := strings.Split(firstFilePath, "/")
	if len(parts) <= 1 {
		return ""
	}
	commonPrefix := parts[0] + "/"
	if !hasMultipleFiles {
		return commonPrefix
	}
	for _, item := range paths {
		normalizedPath, skip, err := NormalizeEntryPath(item)
		if err != nil || skip {
			continue
		}
		if !strings.HasPrefix(normalizedPath, commonPrefix) {
			return ""
		}
	}
	return commonPrefix
}

// StripPrefix removes a directory prefix from a cleaned path when present.
func StripPrefix(normalizedPath, prefix string) string {
	if prefix == "" {
		return normalizedPath
	}
	return strings.TrimPrefix(normalizedPath, prefix)
}
