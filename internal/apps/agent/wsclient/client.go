// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package wsclient provides the agent-side WebSocket client for connecting to the OpenFlare server.
package wsclient

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	edgews "github.com/Rain-kl/Wavelet/internal/apps/edge/wsclient"
)

// WSMessage is an alias for the WebSocket message type.
type WSMessage = edgews.WSMessage

// MessageHandler is an alias for the WebSocket message handler function type.
type MessageHandler = edgews.MessageHandler

// Connection is an alias for the AgentConnection interface.
type Connection = edgews.AgentConnection

// Client wraps the connection client for agent WebSockets.
type Client struct {
	inner *edgews.Client
}

// New creates a new Client instance.
func New(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		inner: edgews.New(edgews.PresetAgent, baseURL, token, timeout),
	}
}

// SetToken updates the client's token.
func (c *Client) SetToken(token string) {
	c.inner.SetToken(token)
}

// URL returns the WebSocket client's target URL.
func (c *Client) URL() string {
	return c.inner.URL()
}

// Connect establishes a WebSocket connection to the server and returns the connection handle.
func (c *Client) Connect(ctx context.Context) (protocol.WebSocketConnection, error) {
	return c.inner.ConnectAgent(ctx)
}
