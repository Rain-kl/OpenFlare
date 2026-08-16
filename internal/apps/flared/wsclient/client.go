// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package wsclient provides a WebSocket client for flared control-plane communication.
package wsclient

import (
	"context"
	"time"

	edgews "github.com/Rain-kl/Wavelet/internal/apps/edge/wsclient"
)

// WSMessage is a WebSocket message exchanged with the control plane.
type WSMessage = edgews.WSMessage

// MessageHandler processes incoming WebSocket messages.
type MessageHandler = edgews.MessageHandler

// Connection represents an active WebSocket connection.
type Connection = edgews.Connection

// Client connects to the flared WebSocket endpoint on the control plane.
type Client struct {
	inner *edgews.Client
}

// New creates a WebSocket client for the flared control-plane endpoint.
func New(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		inner: edgews.New(edgews.PresetFlared, baseURL, token, timeout),
	}
}

// SetToken updates the authentication token used for the WebSocket connection.
func (c *Client) SetToken(token string) {
	c.inner.SetToken(token)
}

// Connect establishes a WebSocket connection to the control plane.
func (c *Client) Connect(ctx context.Context) (*Connection, error) {
	return c.inner.Connect(ctx)
}
