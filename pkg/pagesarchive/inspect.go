// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"bytes"
	"fmt"
	"io"
	"math"
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
	// VerifySizes, when true, streams each regular file and compares the actual
	// byte count against the archive-declared size (no content hashing).
	// Default false: trust zip central directory / tar header sizes.
	VerifySizes bool
}

// InspectFile opens path and inspects it as a Pages deployment package without
// loading the whole archive into memory. File inventory uses declared sizes;
// per-file content hashes are not computed.
func InspectFile(filePath string, format Format, opts InspectOptions) (*Manifest, error) {
	file, err := os.Open(filePath) //nolint:gosec // filePath is a controlled temp upload path
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if format == "" {
		head := make([]byte, 512)
		n, readErr := file.ReadAt(head, 0)
		if readErr != nil && readErr != io.EOF {
			return nil, readErr
		}
		format, err = DetectFormat(filePath, head[:n])
		if err != nil {
			return nil, err
		}
	}
	return inspectFromReaderAt(file, info.Size(), format, opts)
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
	return inspectFromReaderAt(bytes.NewReader(data), int64(len(data)), format, opts)
}

func inspectFromReaderAt(ra io.ReaderAt, size int64, format Format, opts InspectOptions) (*Manifest, error) {
	// Default: zip/7z use central directory only; tar streams headers and discards bodies.
	// VerifySizes needs openable tar bodies, so materialize only when requested.
	entries, err := listEntriesAt(ra, size, format, opts.VerifySizes)
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

		fileEntry, err := inspectRegularFile(entry, normalizedPath, limits, opts.VerifySizes)
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

func inspectRegularFile(entry Entry, normalizedPath string, limits Limits, verifySizes bool) (FileEntry, error) {
	if entry.Size > uint64(math.MaxInt64) {
		return FileEntry{}, fmt.Errorf("%s: pages file size out of bounds", normalizedPath)
	}
	//nolint:gosec // bounded to MaxInt64 above
	declaredSize := int64(entry.Size)

	if !verifySizes {
		return FileEntry{
			Path:     normalizedPath,
			Size:     declaredSize,
			Checksum: "",
		}, nil
	}

	if entry.Open == nil {
		return FileEntry{}, fmt.Errorf("%s: cannot verify size without entry open", normalizedPath)
	}
	src, err := entry.Open()
	if err != nil {
		return FileEntry{}, fmt.Errorf("%s: %w", normalizedPath, err)
	}
	actual, measureErr := measureReader(src, entry.Size, limits.MaxFileBytes)
	_ = src.Close()
	if measureErr != nil {
		return FileEntry{}, fmt.Errorf("%s: %w", normalizedPath, measureErr)
	}
	if declaredSize > 0 && actual != declaredSize {
		return FileEntry{}, fmt.Errorf("%s: declared size %d does not match actual %d", normalizedPath, declaredSize, actual)
	}
	return FileEntry{
		Path:     normalizedPath,
		Size:     actual,
		Checksum: "",
	}, nil
}

// measureReader counts bytes without hashing, enforcing maxBytes when positive.
func measureReader(src io.Reader, declaredSize uint64, maxBytes int64) (int64, error) {
	return copyLimited(io.Discard, src, declaredSize, maxBytes)
}
