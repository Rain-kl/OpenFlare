//go:build !windows

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package localfile

import "os"

func syncDirectory(directory *os.File) error {
	return directory.Sync()
}
