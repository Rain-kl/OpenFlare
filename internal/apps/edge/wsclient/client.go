// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package wsclient provides WebSocket client abstractions for edge node communication.
package wsclient

import (
	"context"
	"time"

	pkgprotocol "github.com/Rain-kl/Wavelet/pkg/protocol"
	shared "github.com/Rain-kl/Wavelet/pkg/wsclient"
)

// WSMessage is an alias for the shared WebSocket message type.
type WSMessage = shared.WSMessage

// MessageHandler is an alias for the shared WebSocket message handler type.
type MessageHandler = shared.MessageHandler

// Preset represents a predefined connection configuration for a specific edge role.
type Preset int

const (
	// PresetAgent is the configuration preset for agent connections.
	PresetAgent Preset = iota
	// PresetRelay is the configuration preset for relay connections.
	PresetRelay
	// PresetFlared is the configuration preset for flared (tunnel) connections.
	PresetFlared
)

type presetConfig struct {
	HeaderKey string
	WSPath    string
}

var presets = map[Preset]presetConfig{
	PresetAgent:  {HeaderKey: "X-Agent-Token", WSPath: "/api/v1/agent/ws"},
	PresetRelay:  {HeaderKey: "X-Agent-Token", WSPath: "/api/v1/relay/ws"},
	PresetFlared: {HeaderKey: "X-Tunnel-Token", WSPath: "/api/v1/tunnel/ws"},
}

// PresetHeaderKey returns the HTTP header key used for authentication with the given preset.
func PresetHeaderKey(preset Preset) string {
	return presets[preset].HeaderKey
}

// PresetWSPath returns the WebSocket path used for the given preset.
func PresetWSPath(preset Preset) string {
	return presets[preset].WSPath
}

// Client is a WebSocket client configured for a specific edge preset.
type Client struct {
	sharedClient *shared.Client
}

// New creates a new Client for the given preset, base URL, token, and timeout.
func New(preset Preset, baseURL, token string, timeout time.Duration) *Client {
	cfg := presets[preset]
	return &Client{
		sharedClient: shared.New(shared.Config{
			BaseURL:   baseURL,
			Token:     token,
			Timeout:   timeout,
			HeaderKey: cfg.HeaderKey,
			WSPath:    cfg.WSPath,
		}),
	}
}

// SetToken updates the authentication token used by the client.
func (c *Client) SetToken(token string) {
	c.sharedClient.SetToken(token)
}

// URL returns the fully resolved WebSocket URL for this client.
func (c *Client) URL() string {
	return c.sharedClient.URL()
}

// Connection represents an established WebSocket connection to an edge node.
type Connection struct {
	sharedConn *shared.Connection
}

// Connect establishes a WebSocket connection using the client configuration.
func (c *Client) Connect(ctx context.Context) (*Connection, error) {
	conn, err := c.sharedClient.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &Connection{sharedConn: conn}, nil
}

// AgentConnection is a Connection specialized for agent node communication.
type AgentConnection struct {
	Connection
}

// ConnectAgent establishes a WebSocket connection and returns it as an AgentConnection.
func (c *Client) ConnectAgent(ctx context.Context) (*AgentConnection, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &AgentConnection{Connection: *conn}, nil
}

// URL returns the resolved WebSocket URL of this connection.
func (conn *Connection) URL() string {
	if conn == nil || conn.sharedConn == nil {
		return ""
	}
	return conn.sharedConn.URL
}

// SendPing sends a ping message over the connection.
func (conn *Connection) SendPing() error {
	return conn.sharedConn.SendMessage(pkgprotocol.WSMessageTypePing, nil)
}

// SendPong sends a pong message over the connection.
func (conn *Connection) SendPong() error {
	return conn.sharedConn.SendMessage(pkgprotocol.WSMessageTypePong, nil)
}

// SendMessage sends a typed message with an optional payload over the connection.
func (conn *Connection) SendMessage(msgType string, payload any) error {
	return conn.sharedConn.SendMessage(msgType, payload)
}

// Receive reads the next message from the connection.
func (conn *Connection) Receive() (pkgprotocol.WSMessage, error) {
	var message pkgprotocol.WSMessage
	if err := conn.sharedConn.Receive(&message); err != nil {
		return message, err
	}
	return message, nil
}

// RunReceiveLoop blocks and dispatches incoming messages to the handler until the context is canceled.
func (conn *Connection) RunReceiveLoop(ctx context.Context, handler MessageHandler) error {
	return conn.sharedConn.RunReceiveLoop(ctx, handler)
}

// Close gracefully closes the WebSocket connection.
func (conn *Connection) Close() error {
	if conn == nil || conn.sharedConn == nil {
		return nil
	}
	return conn.sharedConn.Close()
}

// SendStatus sends a node status payload over the agent connection.
func (conn *AgentConnection) SendStatus(payload pkgprotocol.NodePayload) error {
	return conn.sharedConn.SendMessage(pkgprotocol.WSMessageTypeStatus, payload)
}
