// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package listener

import "context"

// NodeTokenRevoked is emitted when an edge node's access token is rotated or the node is quarantined.
type NodeTokenRevoked struct {
	NodeID   string
	OldToken string
	Reason   string
}

// NodeTokenRevokedHandler handles node credential revocation events.
type NodeTokenRevokedHandler func(ctx context.Context, event NodeTokenRevoked)

var nodeTokenRevokedHandlers []NodeTokenRevokedHandler

// OnNodeTokenRevoked registers a handler for node credential revocation events.
func OnNodeTokenRevoked(handler NodeTokenRevokedHandler) {
	if handler == nil {
		return
	}
	nodeTokenRevokedHandlers = append(nodeTokenRevokedHandlers, handler)
}

// EmitNodeTokenRevoked dispatches a node credential revocation event to all registered handlers.
func EmitNodeTokenRevoked(ctx context.Context, event NodeTokenRevoked) {
	for _, handler := range nodeTokenRevokedHandlers {
		handler(ctx, event)
	}
}
