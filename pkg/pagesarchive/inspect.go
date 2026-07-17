// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// InspectOptions controls package inspection.
type InspectOptions struct {
	// RootDir is an optional project root subdirectory that must contain EntryFile.
	RootDir string
	// EntryFile is the required entry file name (e.g. index.html).
	EntryFile string
	// Limits bounds files and sizes.
	Limits Limits
}

// InspectFile opens path and inspects it as a Pages deployment package.
func InspectFile(filePath string, format Format, opts InspectOptions) (*Manifest, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is a controlled temp upload path
	if err != nil {
		return nil, err
	}
	return InspectBytes(data, format, opts)
}

// InspectBytes inspects an in-memory deployment package.
func InspectBytes(data []byte, format Format, opts InspectOptions) (*Manifest, error) {
	if format == "" {
		var err error
		format, err = DetectFormat("", data)
		if err != nil {
			return nil, err
		}
	}
	entries, err := listEntries(data, format)
	if err != nil {
		return nil, err
	}
	return buildManifest(entries, opts)
}

func buildManifest(entries []Entry, opts InspectOptions) (*Manifest, error) {
	limits := normalizeLimits(opts.Limits)
	commonPrefix := FindCommonRootPrefix(collectFileNames(entries))
	targetEntryPath := resolveTargetEntryPath(opts.RootDir, opts.EntryFile)

	manifest := &Manifest{Files: make([]FileEntry, 0)}
	entrySeen := false

	for _, entry := range entries {
		normalizedPath, skip, err := prepareEntryPath(entry, commonPrefix)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if exceedsFileByteLimit(entry.Size, limits.MaxFileBytes) {
			return nil, fmt.Errorf("pages file too large: %s", normalizedPath)
		}

		fileEntry, err := inspectRegularFile(entry, normalizedPath, limits)
		if err != nil {
			return nil, err
		}
		manifest.FileCount++
		if manifest.FileCount > limits.MaxFiles {
			return nil, fmt.Errorf("pages deployment file count exceeds %d", limits.MaxFiles)
		}
		manifest.TotalSize += fileEntry.Size
		if manifest.TotalSize > limits.MaxTotalBytes {
			return nil, fmt.Errorf("pages extracted size exceeds limit")
		}
		if normalizedPath == targetEntryPath {
			entrySeen = true
		}
		manifest.Files = append(manifest.Files, fileEntry)
	}

	if manifest.FileCount == 0 {
		return nil, fmt.Errorf("pages package is empty")
	}
	if !entrySeen {
		return nil, fmt.Errorf("pages package is missing entry file %s", targetEntryPath)
	}
	return manifest, nil
}

func collectFileNames(entries []Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir || entry.IsSymlink {
			continue
		}
		names = append(names, entry.Name)
	}
	return names
}

func resolveTargetEntryPath(rootDir, entryFile string) string {
	normalizedEntry := strings.TrimSpace(entryFile)
	if normalizedEntry == "" {
		normalizedEntry = "index.html"
	}
	normalizedRoot := strings.Trim(strings.TrimSpace(rootDir), "/")
	if normalizedRoot == "" {
		return normalizedEntry
	}
	return path.Join(normalizedRoot, normalizedEntry)
}

func prepareEntryPath(entry Entry, commonPrefix string) (string, bool, error) {
	normalizedPath, skip, err := NormalizeEntryPath(entry.Name)
	if err != nil {
		return "", false, err
	}
	if skip || entry.IsDir {
		return "", true, nil
	}
	normalizedPath = StripPrefix(normalizedPath, commonPrefix)
	if entry.IsSymlink {
		return "", false, fmt.Errorf("pages package contains unsupported symlink: %s", normalizedPath)
	}
	return normalizedPath, false, nil
}

func inspectRegularFile(entry Entry, normalizedPath string, limits Limits) (FileEntry, error) {
	src, err := entry.Open()
	if err != nil {
		return FileEntry{}, fmt.Errorf("%s: %w", normalizedPath, err)
	}
	checksum, fileSize, checksumErr := checksumReader(src, entry.Size, limits.MaxFileBytes)
	_ = src.Close()
	if checksumErr != nil {
		return FileEntry{}, fmt.Errorf("%s: %w", normalizedPath, checksumErr)
	}
	return FileEntry{
		Path:     normalizedPath,
		Size:     fileSize,
		Checksum: checksum,
	}, nil
}
