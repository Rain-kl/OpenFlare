// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package updater

import "context"

// GetStatus returns the current build and the newest compatible upstream release.
func GetStatus(ctx context.Context) (Status, error) {
	status, _, err := defaultManager.status(ctx)
	return status, err
}

// PrepareUpgrade downloads and stages the upgrade binary for the current platform.
func PrepareUpgrade(ctx context.Context) (executable string, stagedBinary string, status Status, err error) {
	status, _, err = defaultManager.status(ctx)
	if err != nil {
		return "", "", Status{}, err
	}

	executable, stagedBinary, err = defaultManager.prepareUpgrade(ctx)
	return executable, stagedBinary, status, err
}

// ApplyPreparedUpgrade replaces the running binary and restarts the process.
func ApplyPreparedUpgrade(executable, stagedBinary string) error {
	return replaceAndRestart(executable, stagedBinary)
}

// FinishUpgrade clears the in-progress upgrade flag after a failed restart.
func FinishUpgrade() {
	defaultManager.finishUpgrade()
}

// IsUpgrading reports whether an upgrade task is currently running.
func IsUpgrading() bool {
	defaultManager.mu.Lock()
	defer defaultManager.mu.Unlock()
	return defaultManager.upgrading
}
