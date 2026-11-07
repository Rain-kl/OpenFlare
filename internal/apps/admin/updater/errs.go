// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package updater manages GitHub Release checks and in-place application upgrades.
package updater

const (
	errInvalidRepository       = "invalid upstream repository URL"
	errReleaseRequestFailed    = "Failed to fetch upstream version"
	errReleaseResponseInvalid  = "Invalid upstream version response"
	errNoCompatibleRelease     = "no compatible release was found"
	errNoCompatibleAsset       = "no release asset was found for the current platform"
	errDevelopmentBuild        = "development builds do not support automatic updates"
	errAlreadyUpToDate         = "already running the latest version"
	errUpgradeAlreadyRunning   = "an update is already in progress"
	errAutomaticUpgradeBlocked = "automatic binary replacement is not supported on this platform"
)
