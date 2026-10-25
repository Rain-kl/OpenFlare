// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package node defines node validation and management error messages.
package node

const (
	errNodeNameRequired          = "node name must not be empty"
	errNodeIPTooLong             = "Node IP must not exceed 64 characters"
	errNodeIPInvalid             = "Invalid node IP address format"
	errNodeIPManualRequired      = "A node IP is required when IP locking is enabled"
	errNodeGeoNameTooLong        = "Node location name must not exceed 128 characters"
	errNodeGeoCoordinateMismatch = "Both latitude and longitude are required"
	errNodeGeoLatitudeInvalid    = "Latitude must be between -90 and 90"
	errNodeGeoLongitudeInvalid   = "Longitude must be between -180 and 180"
	errNodeIDConflict            = "node identifier conflict; please try again"
	errNodeNotFound              = "node was not found"
	errNodeForceSyncFailed       = "node is offline or the synchronization command could not be sent over WebSocket"
	errNoActiveConfigVersion     = "no active configuration version"
	errAgentPreviewTagInvalid    = "the selected version is not a preview release"
	errAgentStableTagInvalid     = "a preview release cannot be selected for a stable update"
)
