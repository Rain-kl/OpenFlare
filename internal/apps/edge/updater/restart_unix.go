//go:build !windows

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package updater provides capabilities to check for, download, and apply updates.
package updater

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

func replaceAndRestart(execPath string, tmpPath string) error {
	backupPath := execPath + ".bak"
	if err := removeBackupBinary(backupPath); err != nil {
		return err
	}
	if err := os.Rename(execPath, backupPath); err != nil {
		renameErr := err
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			slog.Error("remove tmp binary failed", "path", tmpPath, "error", err)
			return fmt.Errorf("backup current binary: %w; remove tmp binary: %w", renameErr, err)
		}
		return fmt.Errorf("backup current binary: %w", renameErr)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		replaceErr := err
		if err := os.Rename(backupPath, execPath); err != nil {
			slog.Error("restore backup binary failed", "path", backupPath, "error", err)
			return fmt.Errorf("replace binary: %w; restore backup binary: %w", replaceErr, err)
		}
		return fmt.Errorf("replace binary: %w", replaceErr)
	}
	if err := removeBackupBinary(backupPath); err != nil {
		return err
	}
	if err := syscall.Exec(execPath, os.Args, os.Environ()); err != nil { //nolint:gosec // execPath is the validated edge updater binary path
		return fmt.Errorf("exec restart: %w", err)
	}
	return errors.New("unreachable after exec")
}

func removeBackupBinary(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.Error("remove backup binary failed", "path", path, "error", err)
		return err
	}
	return nil
}
