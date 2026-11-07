// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config_version defines shared error messages for configuration versions.
package config_version

const (
	errNoActiveVersion       = "no active configuration version"
	errNoEnabledRoutes       = "no enabled rules are available to publish"
	errNoChangesToPublish    = "the current rules have not changed and cannot be published again"
	errVersionConflict       = "version identifier conflict; please try again"
	errInvalidSnapshotFormat = "invalid historical version snapshot format"
)
