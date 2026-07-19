// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

import (
	"errors"
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
	// IsHardlink marks hard links (unsupported for Pages).
	IsHardlink bool
	// IsSpecial marks device, FIFO, socket, and other non-regular entries.
	IsSpecial bool
	// Size is the archive-declared uncompressed size; 0 means an empty member.
	Size uint64
	// Open returns a reader for the entry body. Caller must Close it.
	Open func() (io.ReadCloser, error)
}

// copyLimited copies actual bytes from src. maxBytes < 0 disables the byte cap;
// maxBytes == 0 permits only an empty stream.
func copyLimited(dst io.Writer, src io.Reader, maxBytes int64) (int64, error) {
	if maxBytes < 0 {
		return io.Copy(dst, src)
	}

	readLimit := maxBytes
	if maxBytes < math.MaxInt64 {
		readLimit++
	}
	written, err := io.Copy(dst, io.LimitReader(src, readLimit))
	if err != nil {
		return written, err
	}
	if written > maxBytes {
		return written, fmt.Errorf("pages file size out of bounds")
	}
	return written, nil
}

func copyAndVerifySize(dst io.Writer, src io.Reader, declaredSize uint64, maxBytes int64) (int64, error) {
	if declaredSize > uint64(math.MaxInt64) {
		return 0, fmt.Errorf("pages file size out of bounds")
	}
	written, err := copyLimited(dst, src, maxBytes)
	if err != nil {
		return written, err
	}
	//nolint:gosec // declaredSize is bounded to MaxInt64 above
	if written != int64(declaredSize) {
		return written, fmt.Errorf("pages declared size %d does not match actual %d", declaredSize, written)
	}
	return written, nil
}

func writeEntryFile(targetPath string, src io.Reader, declaredSize uint64, maxBytes int64, perm os.FileMode) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), dirPerm); err != nil {
		return 0, err
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm) //nolint:gosec // caller validates path under release dir
	if err != nil {
		return 0, err
	}
	written, copyErr := copyAndVerifySize(target, src, declaredSize, maxBytes)
	closeErr := target.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = os.Remove(targetPath)
		return written, err
	}
	return written, nil
}
