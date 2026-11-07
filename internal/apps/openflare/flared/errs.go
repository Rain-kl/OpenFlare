// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package flared defines shared error messages for tunnel client operations.
package flared

const (
	errTunnelTokenInvalid     = "Operation not authorized: invalid tunnel token" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errTunnelNodeTypeMismatch = "This node is not a TunnelClient"
)
