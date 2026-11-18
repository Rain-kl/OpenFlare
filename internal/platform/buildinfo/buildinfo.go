// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package buildinfo exposes metadata injected by the release workflow.
package buildinfo

var (
	// Version is the application version.
	Version = "dev"
	// BuildTime is the UTC release build timestamp.
	BuildTime = ""
)
