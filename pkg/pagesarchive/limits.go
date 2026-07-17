// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pagesarchive

const (
	defaultMaxFiles      = 1000
	defaultMaxFileBytes  = 100 * 1024 * 1024
	defaultMaxTotalBytes = 100 * 1024 * 1024
	dirPerm              = 0o750
	filePerm             = 0o644
)

// normalizeLimits applies defaults for control-plane inspection.
// Callers that already validated the package should use EnforceLimits=false instead.
func normalizeLimits(limits Limits) Limits {
	if limits.MaxFiles <= 0 {
		limits.MaxFiles = defaultMaxFiles
	}
	if limits.MaxFileBytes <= 0 {
		limits.MaxFileBytes = defaultMaxFileBytes
	}
	if limits.MaxTotalBytes <= 0 {
		limits.MaxTotalBytes = defaultMaxTotalBytes
	}
	return limits
}

func exceedsFileByteLimit(size uint64, maxBytes int64) bool {
	// maxBytes <= 0 means unlimited (trusted extract path).
	if maxBytes <= 0 {
		return false
	}
	if size == 0 {
		return false
	}
	return size > uint64(maxBytes) //nolint:gosec // maxBytes is positive
}
