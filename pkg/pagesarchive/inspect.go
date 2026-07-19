// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
)

// InspectOptions controls package inspection.
type InspectOptions struct {
	// RootDir is an optional project root subdirectory that must contain EntryFile.
	RootDir string
	// EntryFile is the required entry file name (e.g. index.html).
	EntryFile string
	// Limits bounds files and actual extracted sizes.
	Limits Limits
	// VerifySizes is retained for source compatibility. Inspection now always
	// streams regular members and verifies actual bytes against declared sizes.
	VerifySizes bool
}

type measuredFile struct {
	path string
	size int64
}

type measuredArchive struct {
	files     []measuredFile
	fileCount int
	totalSize int64
}

// InspectFile opens path and inspects it as a Pages deployment package without
// loading the whole archive or any tar member body into memory.
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
		head := make([]byte, formatDetectHeadBytes)
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
	limits := normalizeLimits(opts.Limits)
	var (
		measured *measuredArchive
		err      error
	)
	if isTarFamily(format) {
		measured, err = scanTarFamilyAt(ra, size, format, limits, true)
	} else {
		var entries []Entry
		entries, err = listRandomAccessEntriesAt(ra, size, format)
		if err == nil {
			measured, err = inspectRandomAccessEntries(entries, limits)
		}
	}
	if err != nil {
		return nil, err
	}
	return buildMeasuredManifest(measured, opts)
}

func inspectRandomAccessEntries(entries []Entry, limits Limits) (*measuredArchive, error) {
	measured := &measuredArchive{files: make([]measuredFile, 0, len(entries))}
	for _, entry := range entries {
		normalizedPath, skip, err := validateArchiveEntry(entry)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if err := prepareMeasuredFile(measured, normalizedPath, entry.Size, limits, true); err != nil {
			return nil, err
		}
		if entry.Open == nil {
			return nil, fmt.Errorf("%s: pages archive member cannot be opened", normalizedPath)
		}
		src, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", normalizedPath, err)
		}
		maxBytes := effectiveFileLimit(limits, measured.totalSize, true)
		actual, copyErr := copyAndVerifySize(io.Discard, src, entry.Size, maxBytes)
		closeErr := src.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return nil, fmt.Errorf("%s: %w", normalizedPath, err)
		}
		appendMeasuredFile(measured, normalizedPath, actual)
	}
	return measured, nil
}

func scanTarFamilyAt(
	ra io.ReaderAt,
	size int64,
	format Format,
	limits Limits,
	enforceLimits bool,
) (*measuredArchive, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid pages package size")
	}
	tarReader, closeReader, err := openTarFamilyReader(io.NewSectionReader(ra, 0, size), format)
	if err != nil {
		return nil, err
	}
	measured, scanErr := scanTarReader(tarReader, limits, enforceLimits)
	if closeErr := closeReader(); closeErr != nil {
		scanErr = errors.Join(scanErr, closeErr)
	}
	return measured, scanErr
}

func scanTarReader(tarReader *tar.Reader, limits Limits, enforceLimits bool) (*measuredArchive, error) {
	measured := &measuredArchive{files: make([]measuredFile, 0)}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar pages package: %w", err)
		}
		entry := entryFromTarHeader(header)
		normalizedPath, skip, err := validateArchiveEntry(entry)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if err := prepareMeasuredFile(measured, normalizedPath, entry.Size, limits, enforceLimits); err != nil {
			return nil, err
		}
		maxBytes := effectiveFileLimit(limits, measured.totalSize, enforceLimits)
		actual, err := copyAndVerifySize(io.Discard, tarReader, entry.Size, maxBytes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", normalizedPath, err)
		}
		appendMeasuredFile(measured, normalizedPath, actual)
	}
	return measured, nil
}

func buildMeasuredManifest(measured *measuredArchive, opts InspectOptions) (*Manifest, error) {
	if measured == nil || measured.fileCount == 0 {
		return nil, fmt.Errorf("pages package is empty")
	}
	targetEntryPath, err := resolveTargetEntryPath(opts.RootDir, opts.EntryFile)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(measured.files))
	for _, file := range measured.files {
		paths = append(paths, file.path)
	}
	commonPrefix := FindCommonRootPrefix(paths)
	manifest := &Manifest{
		Files:     make([]FileEntry, 0, measured.fileCount),
		FileCount: measured.fileCount,
		TotalSize: measured.totalSize,
	}
	entrySeen := false
	for _, file := range measured.files {
		normalizedPath := StripPrefix(file.path, commonPrefix)
		if normalizedPath == targetEntryPath {
			entrySeen = true
		}
		manifest.Files = append(manifest.Files, FileEntry{
			Path: normalizedPath,
			Size: file.size,
		})
	}
	if !entrySeen {
		return nil, fmt.Errorf("pages package is missing entry file %s", targetEntryPath)
	}
	return manifest, nil
}

func prepareMeasuredFile(measured *measuredArchive, normalizedPath string, declaredSize uint64, limits Limits, enforceLimits bool) error {
	if declaredSize > uint64(math.MaxInt64) {
		return fmt.Errorf("%s: pages file size out of bounds", normalizedPath)
	}
	if !enforceLimits {
		return nil
	}
	if measured.fileCount >= limits.MaxFiles {
		return fmt.Errorf("pages deployment file count exceeds %d", limits.MaxFiles)
	}
	if exceedsFileByteLimit(declaredSize, limits.MaxFileBytes) {
		return fmt.Errorf("pages file too large: %s", normalizedPath)
	}
	remaining := limits.MaxTotalBytes - measured.totalSize
	if remaining < 0 || declaredSize > uint64(remaining) { //nolint:gosec // remaining is checked non-negative
		return fmt.Errorf("pages extracted size exceeds limit")
	}
	return nil
}

func appendMeasuredFile(measured *measuredArchive, normalizedPath string, actual int64) {
	measured.files = append(measured.files, measuredFile{path: normalizedPath, size: actual})
	measured.fileCount++
	measured.totalSize += actual
}

func effectiveFileLimit(limits Limits, totalSize int64, enforceLimits bool) int64 {
	if !enforceLimits {
		return -1
	}
	remaining := limits.MaxTotalBytes - totalSize
	if remaining < limits.MaxFileBytes {
		return remaining
	}
	return limits.MaxFileBytes
}

func resolveTargetEntryPath(rootDir, entryFile string) (string, error) {
	normalizedRoot, err := NormalizeLogicalPath(rootDir, true)
	if err != nil {
		return "", fmt.Errorf("invalid pages root directory: %w", err)
	}
	if entryFile == "" {
		entryFile = "index.html"
	}
	normalizedEntry, err := NormalizeLogicalPath(entryFile, false)
	if err != nil {
		return "", fmt.Errorf("invalid pages entry file: %w", err)
	}
	if normalizedRoot == "" {
		return normalizedEntry, nil
	}
	return path.Join(normalizedRoot, normalizedEntry), nil
}

func validateArchiveEntry(entry Entry) (string, bool, error) {
	normalizedPath, skip, err := NormalizeEntryPath(entry.Name)
	if err != nil {
		return "", false, err
	}
	if entry.IsSymlink {
		return "", false, fmt.Errorf("pages package contains unsupported symlink: %s", normalizedPath)
	}
	if entry.IsHardlink {
		return "", false, fmt.Errorf("pages package contains unsupported hardlink: %s", normalizedPath)
	}
	if entry.IsSpecial {
		return "", false, fmt.Errorf("pages package contains unsupported special entry: %s", normalizedPath)
	}
	if skip || entry.IsDir {
		return normalizedPath, true, nil
	}
	return normalizedPath, false, nil
}

func isTarFamily(format Format) bool {
	switch format {
	case FormatTar, FormatTarGz, FormatTarXz, FormatTarBz2:
		return true
	default:
		return false
	}
}
