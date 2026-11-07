// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package relay provides relay node management and authentication for the OpenFlare platform.
package relay

const (
	//nolint:gosec // error message text, not a credential
	errAgentTokenInvalid     = "Operation not authorized: invalid agent token"
	errRelayNodeTypeMismatch = "This node is not a TunnelRelay"
)
