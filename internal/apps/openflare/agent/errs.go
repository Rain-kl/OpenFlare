// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

const (
	errMissingAgentToken      = "missing agent token"                                  //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errInvalidAgentToken      = "Operation not authorized: invalid agent token"        //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errInvalidDiscoveryToken  = "Operation not authorized: invalid registration token" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errNodeMissingFromContext = "Node object missing from context"
	errNoActiveConfig         = "no active configuration version"
	errNodeNotFound           = "node was not found"
	errNodeIDRequired         = "node_id must not be empty"
	errVersionRequired        = "version must not be empty"
	errInvalidApplyResult     = "result must be success, warning or failed"
	errIPRequired             = "ip must not be empty"
	errIPInvalid              = "invalid IP address format"
	errAgentVersionRequired   = "version must not be empty"
	errNodeIDConflict         = "node identifier conflict; please try again"
)
