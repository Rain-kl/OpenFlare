// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// formatDetectHeadBytes is the sniff window used for archive format detection.
const formatDetectHeadBytes = 512

// ExtractOptions controls package extraction.
type ExtractOptions struct {
	// Limits bounds actual files and sizes during extraction when EnforceLimits is true.
	Limits Limits
	// StripCommonRoot strips a single shared top-level directory when present.
	StripCommonRoot bool
	// EnforceLimits enables MaxFiles / MaxFileBytes / MaxTotalBytes checks.
	// Path, member type, and declared/actual-size validation always remain enabled.
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
	return extractFromReaderAt(bytes.NewReader(data), int64(len(data)), format, destDir, opts)
}

// ExtractFile opens path and extracts it into destDir without buffering the
// whole archive or tar member bodies in memory.
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
	return extractFromReaderAt(file, info.Size(), format, destDir, opts)
}

func extractFromReaderAt(ra io.ReaderAt, size int64, format Format, destDir string, opts ExtractOptions) error {
	if isTarFamily(format) {
		return extractTarFamilyAt(ra, size, format, destDir, opts)
	}
	entries, err := listRandomAccessEntriesAt(ra, size, format)
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
	commonPrefix, err := commonRootForEntries(entries, opts.StripCommonRoot)
	if err != nil {
		return err
	}

	measured := &measuredArchive{files: make([]measuredFile, 0, len(entries))}
	for _, entry := range entries {
		normalizedPath, skip, err := validateArchiveEntry(entry)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		normalizedPath = StripPrefix(normalizedPath, commonPrefix)
		if normalizedPath == "" {
			continue
		}
		if err := prepareMeasuredFile(measured, normalizedPath, entry.Size, limits, opts.EnforceLimits); err != nil {
			return err
		}
		if entry.Open == nil {
			return fmt.Errorf("%s: pages archive member cannot be opened", normalizedPath)
		}
		src, err := entry.Open()
		if err != nil {
			return fmt.Errorf("%s: %w", normalizedPath, err)
		}
		maxBytes := effectiveFileLimit(limits, measured.totalSize, opts.EnforceLimits)
		targetPath, err := safeExtractionTarget(destDir, normalizedPath)
		if err != nil {
			_ = src.Close()
			return err
		}
		actual, writeErr := writeEntryFile(targetPath, src, entry.Size, maxBytes, filePerm)
		closeErr := src.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return fmt.Errorf("%s: %w", normalizedPath, err)
		}
		appendMeasuredFile(measured, normalizedPath, actual)
	}
	if measured.fileCount == 0 {
		return fmt.Errorf("pages package is empty")
	}
	return nil
}

func extractTarFamilyAt(ra io.ReaderAt, size int64, format Format, destDir string, opts ExtractOptions) error {
	limits := Limits{}
	if opts.EnforceLimits {
		limits = normalizeLimits(opts.Limits)
	}
	firstPass, err := scanTarFamilyAt(ra, size, format, limits, opts.EnforceLimits)
	if err != nil {
		return err
	}
	if firstPass.fileCount == 0 {
		return fmt.Errorf("pages package is empty")
	}
	commonPrefix := ""
	if opts.StripCommonRoot {
		paths := make([]string, 0, len(firstPass.files))
		for _, file := range firstPass.files {
			paths = append(paths, file.path)
		}
		commonPrefix = FindCommonRootPrefix(paths)
	}

	tarReader, closeReader, err := openTarFamilyReader(io.NewSectionReader(ra, 0, size), format)
	if err != nil {
		return err
	}
	secondPass, extractErr := extractTarReader(tarReader, destDir, commonPrefix, limits, opts.EnforceLimits)
	if closeErr := closeReader(); closeErr != nil {
		extractErr = errors.Join(extractErr, closeErr)
	}
	if extractErr != nil {
		return extractErr
	}
	if secondPass.fileCount != firstPass.fileCount || secondPass.totalSize != firstPass.totalSize {
		return fmt.Errorf("pages tar package changed between validation and extraction")
	}
	return nil
}

func extractTarReader(
	tarReader *tar.Reader,
	destDir string,
	commonPrefix string,
	limits Limits,
	enforceLimits bool,
) (*measuredArchive, error) {
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
		normalizedPath = StripPrefix(normalizedPath, commonPrefix)
		if normalizedPath == "" {
			continue
		}
		if err := prepareMeasuredFile(measured, normalizedPath, entry.Size, limits, enforceLimits); err != nil {
			return nil, err
		}
		targetPath, err := safeExtractionTarget(destDir, normalizedPath)
		if err != nil {
			return nil, err
		}
		maxBytes := effectiveFileLimit(limits, measured.totalSize, enforceLimits)
		actual, err := writeEntryFile(targetPath, tarReader, entry.Size, maxBytes, filePerm)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", normalizedPath, err)
		}
		appendMeasuredFile(measured, normalizedPath, actual)
	}
	return measured, nil
}

func commonRootForEntries(entries []Entry, strip bool) (string, error) {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		normalizedPath, skip, err := validateArchiveEntry(entry)
		if err != nil {
			return "", err
		}
		if !skip {
			paths = append(paths, normalizedPath)
		}
	}
	if !strip {
		return "", nil
	}
	return FindCommonRootPrefix(paths), nil
}

func safeExtractionTarget(destDir, relativePath string) (string, error) {
	targetPath := filepath.Join(destDir, filepath.FromSlash(relativePath))
	if !isWithinDir(destDir, targetPath) {
		return "", fmt.Errorf("pages package path escapes directory: %s", relativePath)
	}
	return targetPath, nil
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
