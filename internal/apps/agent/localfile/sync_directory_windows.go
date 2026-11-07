// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package localfile

import "os"

// Windows does not support a portable directory fsync through os.File.Sync.
// The file is flushed before replacement; persisting the directory entry is best effort.
func syncDirectory(_ *os.File) error {
	return nil
}
