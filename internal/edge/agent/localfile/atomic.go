// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package localfile persists agent identity and configuration without truncating existing files.
package localfile

import (
	"errors"
	"os"
	"path/filepath"
)

// WriteAtomic syncs the complete contents, replaces path, then syncs its containing directory.
// An error from the final sync means the replacement is visible but its durability is unconfirmed.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	return writeAtomic(path, data, perm, SyncDirectory)
}

func writeAtomic(path string, data []byte, perm os.FileMode, syncDir func(string) error) error {
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()           // Best effort after an earlier write/close error.
		_ = os.Remove(file.Name()) // Best effort; rename normally removes the temporary path.
	}()
	if err = file.Chmod(perm); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// SyncDirectory confirms a directory entry update on platforms with directory flushing.
// Windows provides best-effort replacement without a portable directory flush guarantee.
func SyncDirectory(path string) (resultErr error) {
	directory, err := os.Open(path) //nolint:gosec // callers provide the directory of a managed local state/config file
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := directory.Close(); resultErr == nil {
			resultErr = closeErr
		}
	}()
	info, err := directory.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("cannot sync a non-directory path")
	}
	return syncDirectory(directory)
}
