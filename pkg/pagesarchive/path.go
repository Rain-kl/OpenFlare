// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizeLogicalPath validates and normalizes a relative POSIX path.
// Empty input is returned unchanged only when allowEmpty is true.
func NormalizeLogicalPath(raw string, allowEmpty bool) (string, error) {
	if raw == "" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("pages path is required")
	}
	if err := validateLogicalPathText(raw); err != nil {
		return "", err
	}

	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == "" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("pages path is required")
	}
	if strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("pages path escapes directory: %s", raw)
	}
	return cleaned, nil
}

func validateLogicalPathText(raw string) error {
	if !utf8.ValidString(raw) {
		return fmt.Errorf("pages path is not valid UTF-8")
	}
	if strings.Contains(raw, "\\") {
		return fmt.Errorf("pages path must use POSIX separators: %s", raw)
	}
	if strings.HasPrefix(raw, "/") || path.IsAbs(raw) {
		return fmt.Errorf("pages path must be relative: %s", raw)
	}
	if err := validateLogicalPathRunes(raw); err != nil {
		return err
	}
	return validateLogicalPathSegments(raw)
}

func validateLogicalPathRunes(raw string) error {
	for _, r := range raw {
		if r == 0 || unicode.IsControl(r) {
			return fmt.Errorf("pages path contains a control character")
		}
		if r == '\'' || r == '"' || r == ';' {
			return fmt.Errorf("pages path contains an unsupported character: %s", raw)
		}
	}
	return nil
}

func validateLogicalPathSegments(raw string) error {
	for _, segment := range strings.Split(raw, "/") {
		if len(segment) >= 2 && segment[1] == ':' {
			return fmt.Errorf("pages path contains a Windows drive: %s", raw)
		}
		if segment == "." || segment == ".." {
			return fmt.Errorf("pages path escapes directory or contains a dot segment: %s", raw)
		}
	}
	return nil
}

// NormalizeEntryPath cleans an archive entry path and rejects zip-slip / absolute paths.
// skip=true means the entry should be ignored (empty path or directory marker).
func NormalizeEntryPath(raw string) (cleaned string, skip bool, err error) {
	if raw == "" {
		return "", true, nil
	}
	cleanedPath, normalizeErr := NormalizeLogicalPath(raw, false)
	if normalizeErr != nil {
		return "", false, fmt.Errorf("invalid pages package path %q: %w", raw, normalizeErr)
	}
	if strings.HasSuffix(raw, "/") {
		return cleanedPath, true, nil
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
