// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// formatDetectHeadBytes is the sniff window used for archive format detection.
const formatDetectHeadBytes = 512

// ExtractOptions controls package extraction.
type ExtractOptions struct {
	// Limits bounds files and sizes during extraction when EnforceLimits is true.
	Limits Limits
	// StripCommonRoot strips a single shared top-level directory when present.
	StripCommonRoot bool
	// EnforceLimits enables MaxFiles / MaxFileBytes / MaxTotalBytes checks.
	// When false, the caller is assumed to have already validated the package
	// (e.g. Agent trusts control-plane inspection). Path-escape and symlink
	// guards still apply so local extraction cannot leave destDir.
	EnforceLimits bool
}

// ExtractBytes extracts a deployment package into destDir.
func ExtractBytes(data []byte, format Format, destDir string, opts ExtractOptions) error {
	if format == "" {
		var err error
		format, err = DetectFormat("", data)
		if err != nil {
			return err
		}
	}
	entries, err := listEntriesAt(bytes.NewReader(data), int64(len(data)), format, true)
	if err != nil {
		return err
	}
	return extractEntries(entries, destDir, opts)
}

// ExtractFile opens path and extracts it into destDir without buffering the
// whole archive as an intermediate []byte for zip/7z (ReaderAt). Tar-family
// formats still materialize member bodies so random Open works for extract.
func ExtractFile(filePath string, format Format, destDir string, opts ExtractOptions) error {
	file, err := os.Open(filePath) //nolint:gosec // controlled path
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if format == "" {
		head := make([]byte, formatDetectHeadBytes)
		n, readErr := file.ReadAt(head, 0)
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		format, err = DetectFormat(filePath, head[:n])
		if err != nil {
			return err
		}
	}
	entries, err := listEntriesAt(file, info.Size(), format, true)
	if err != nil {
		return err
	}
	return extractEntries(entries, destDir, opts)
}

func extractEntries(entries []Entry, destDir string, opts ExtractOptions) error {
	limits := Limits{}
	if opts.EnforceLimits {
		limits = normalizeLimits(opts.Limits)
	}
	commonPrefix := ""
	if opts.StripCommonRoot {
		commonPrefix = FindCommonRootPrefix(collectFileNames(entries))
	}

	var totalSize int64
	var fileCount int
	for _, entry := range entries {
		written, counted, err := extractSingleEntry(entry, destDir, commonPrefix, limits, opts.EnforceLimits)
		if err != nil {
			return err
		}
		if !counted {
			continue
		}
		fileCount++
		if opts.EnforceLimits && fileCount > limits.MaxFiles {
			return fmt.Errorf("pages deployment file count exceeds %d", limits.MaxFiles)
		}
		totalSize += written
		if opts.EnforceLimits && totalSize > limits.MaxTotalBytes {
			return fmt.Errorf("pages extracted size exceeds limit")
		}
	}
	if fileCount == 0 {
		return fmt.Errorf("pages package is empty")
	}
	return nil
}

func extractSingleEntry(
	entry Entry,
	destDir, commonPrefix string,
	limits Limits,
	enforceLimits bool,
) (written int64, counted bool, err error) {
	relativePath, skip, err := NormalizeEntryPath(entry.Name)
	if err != nil {
		return 0, false, err
	}
	if skip {
		return 0, false, nil
	}
	if commonPrefix != "" {
		relativePath = StripPrefix(relativePath, commonPrefix)
		if relativePath == "" {
			return 0, false, nil
		}
	}
	if entry.IsSymlink {
		return 0, false, fmt.Errorf("pages package contains unsupported symlink: %s", relativePath)
	}

	targetPath := filepath.Join(destDir, filepath.FromSlash(relativePath))
	if !isWithinDir(destDir, targetPath) {
		return 0, false, fmt.Errorf("pages package path escapes directory: %s", entry.Name)
	}

	if entry.IsDir {
		if err := os.MkdirAll(targetPath, dirPerm); err != nil {
			return 0, false, err
		}
		return 0, false, nil
	}

	maxFileBytes := int64(0) // unlimited when not enforcing
	if enforceLimits {
		if exceedsFileByteLimit(entry.Size, limits.MaxFileBytes) {
			return 0, false, fmt.Errorf("pages file too large: %s", relativePath)
		}
		maxFileBytes = limits.MaxFileBytes
	}
	src, err := entry.Open()
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", relativePath, err)
	}
	written, writeErr := writeEntryFile(targetPath, src, entry.Size, maxFileBytes, filePerm)
	_ = src.Close()
	if writeErr != nil {
		return 0, false, fmt.Errorf("%s: %w", relativePath, writeErr)
	}
	return written, true, nil
}

func isWithinDir(baseDir, targetPath string) bool {
	cleanBase := filepath.Clean(baseDir)
	cleanTarget := filepath.Clean(targetPath)
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return false
	}
	return rel != ".." && !hasParentRel(rel)
}

func hasParentRel(rel string) bool {
	if rel == ".." {
		return true
	}
	return len(rel) >= 3 && (rel[:3] == "../" || rel[:3] == "..\\")
}
