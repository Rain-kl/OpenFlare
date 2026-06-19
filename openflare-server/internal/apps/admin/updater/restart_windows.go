//go:build windows

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package updater

import "errors"

func replaceAndRestart(_, _ string) error {
	return errors.New(errAutomaticUpgradeBlocked)
}
