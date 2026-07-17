// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

// Limits bounds archive inspection / extraction work.
type Limits struct {
	// MaxFiles is the maximum number of regular files allowed.
	MaxFiles int
	// MaxFileBytes is the maximum size of a single extracted file.
	MaxFileBytes int64
	// MaxTotalBytes is the maximum sum of all extracted file sizes.
	MaxTotalBytes int64
}

// FileEntry is a regular file discovered inside a deployment package.
type FileEntry struct {
	Path string
	Size int64
	// Checksum is retained for API/schema compatibility and is left empty.
	// Integrity is enforced via the whole-package SHA-256 on the deployment record.
	Checksum string
}

// Manifest is the inspected content of a Pages deployment package.
type Manifest struct {
	Files     []FileEntry
	FileCount int
	TotalSize int64
}

// Entry describes one archive member for extraction.
type Entry struct {
	// Name is the original path inside the archive.
	Name string
	// IsDir marks directory entries.
	IsDir bool
	// IsSymlink marks symbolic links (unsupported for Pages).
	IsSymlink bool
	// Size is the declared uncompressed size when known; 0 means empty or unknown.
	Size uint64
	// Open returns a reader for the entry body. Caller must Close it.
	// May be unavailable for inspect-only tar listings (body not materialized).
	Open func() (io.ReadCloser, error)
}

// copyLimited copies src to dst.
// When maxBytes <= 0, size limits are not enforced (trusted extract path).
func copyLimited(dst io.Writer, src io.Reader, declaredSize uint64, maxBytes int64) (int64, error) {
	if maxBytes <= 0 {
		if declaredSize > 0 {
			if declaredSize > uint64(math.MaxInt64) {
				return 0, fmt.Errorf("pages file size out of bounds")
			}
			//nolint:gosec // declaredSize is bounded to MaxInt64 above
			return io.CopyN(dst, src, int64(declaredSize))
		}
		return io.Copy(dst, src)
	}
	if declaredSize > uint64(maxBytes) || declaredSize > uint64(math.MaxInt64) { //nolint:gosec // maxBytes positive
		return 0, fmt.Errorf("pages file size out of bounds")
	}
	if declaredSize > 0 {
		//nolint:gosec // declaredSize is bounded to MaxInt64 above
		return io.CopyN(dst, src, int64(declaredSize))
	}
	limited := io.LimitReader(src, maxBytes+1)
	written, err := io.Copy(dst, limited)
	if written > maxBytes {
		return written, fmt.Errorf("pages file size out of bounds")
	}
	return written, err
}

func writeEntryFile(targetPath string, src io.Reader, declaredSize uint64, maxBytes int64, perm os.FileMode) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), dirPerm); err != nil {
		return 0, err
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm) //nolint:gosec // caller validates path under release dir
	if err != nil {
		return 0, err
	}
	defer func() { _ = target.Close() }()
	return copyLimited(target, src, declaredSize, maxBytes)
}
