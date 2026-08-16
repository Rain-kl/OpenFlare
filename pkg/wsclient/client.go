// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package wsclient provides a WebSocket client for agent/server communication.
package wsclient

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

const (
	writeDeadlineSecs       = 5
	defaultReadDeadlineSecs = 75
)

// Config holds the configuration for a WebSocket client connection.
type Config struct {
	BaseURL   string
	Token     string
	Timeout   time.Duration
	HeaderKey string // e.g. "X-Agent-Token", "X-Tunnel-Token"
	WSPath    string // e.g. "/api/relay/ws", "/api/agent/ws", "/api/flared/ws"
}

// Client provides methods to connect and communicate over WebSocket.
type Client struct {
	cfg Config
}

// WSMessage represents a typed WebSocket message with an optional JSON payload.
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// MessageHandler handles WebSocket connection lifecycle and incoming messages.
type MessageHandler interface {
	OnConnect(ctx context.Context) error
	HandleMessage(ctx context.Context, msg WSMessage) error
	OnClose(err error)
}

// Connection represents an active WebSocket connection.
type Connection struct {
	Conn        *websocket.Conn
	URL         string
	ReadTimeout time.Duration
	writeMu     sync.Mutex
}

// New creates a new WebSocket client with the given configuration.
func New(cfg Config) *Client {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	cfg.Token = strings.TrimSpace(cfg.Token)
	cfg.HeaderKey = strings.TrimSpace(cfg.HeaderKey)
	cfg.WSPath = strings.TrimSpace(cfg.WSPath)
	return &Client{
		cfg: cfg,
	}
}

// SetToken updates the authentication token used for the WebSocket connection.
func (c *Client) SetToken(token string) {
	c.cfg.Token = strings.TrimSpace(token)
}

// URL returns the WebSocket URL for the configured endpoint, or empty string on error.
func (c *Client) URL() string {
	wsURL, err := c.BuildWebsocketURL()
	if err != nil {
		return ""
	}
	return wsURL
}

// BuildWebsocketURL constructs the WebSocket URL by converting the base URL scheme and appending the WS path.
func (c *Client) BuildWebsocketURL() (string, error) {
	parsed, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", errors.New("server_url scheme must be http, https, ws, or wss")
	}

	wsPath := c.cfg.WSPath
	if !strings.HasPrefix(wsPath, "/") {
		wsPath = "/" + wsPath
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + wsPath
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

// Connect establishes a new WebSocket connection to the configured server.
func (c *Client) Connect(ctx context.Context) (*Connection, error) {
	wsURL, err := c.BuildWebsocketURL()
	if err != nil {
		return nil, err
	}
	if c.cfg.Token == "" {
		return nil, errors.New("ws token is empty")
	}
	origin := c.cfg.BaseURL
	if origin == "" {
		origin = "http://localhost"
	}
	config, err := websocket.NewConfig(wsURL, origin)
	if err != nil {
		return nil, err
	}
	config.Header = http.Header{}
	if c.cfg.HeaderKey != "" {
		config.Header.Set(c.cfg.HeaderKey, c.cfg.Token)
	}
	if c.cfg.Timeout > 0 {
		config.Dialer = &net.Dialer{Timeout: c.cfg.Timeout}
	}
	slog.Debug("ws dialing server", "url", wsURL)
	conn, err := config.DialContext(ctx)
	if err != nil {
		return nil, err
	}
	slog.Debug("ws dial succeeded", "url", wsURL)
	return &Connection{Conn: conn, URL: wsURL, ReadTimeout: websocketReadTimeout(c.cfg.Timeout)}, nil
}

// SendMessage sends a typed message with an optional payload over the WebSocket connection.
func (conn *Connection) SendMessage(msgType string, payload any) error {
	if conn == nil || conn.Conn == nil {
		return errors.New("ws connection is nil")
	}
	slog.Debug("ws sending message", "type", msgType)

	// Create the outbound message wrapper
	message := struct {
		Type    string `json:"type"`
		Payload any    `json:"payload,omitempty"`
	}{
		Type:    msgType,
		Payload: payload,
	}

	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()

	_ = conn.Conn.SetWriteDeadline(time.Now().Add(writeDeadlineSecs * time.Second))
	return websocket.JSON.Send(conn.Conn, message)
}

// Receive reads a single message from the WebSocket connection into target.
func (conn *Connection) Receive(target any) error {
	if conn == nil || conn.Conn == nil {
		return errors.New("ws connection is nil")
	}
	if conn.ReadTimeout > 0 {
		_ = conn.Conn.SetReadDeadline(time.Now().Add(conn.ReadTimeout))
	}
	err := websocket.JSON.Receive(conn.Conn, target)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			slog.Debug("ws receive timeout waiting for server message", "timeout", conn.ReadTimeout)
		}
		return err
	}
	return nil
}

func websocketReadTimeout(requestTimeout time.Duration) time.Duration {
	timeout := requestTimeout * 6
	if timeout < defaultReadDeadlineSecs*time.Second {
		return defaultReadDeadlineSecs * time.Second
	}
	return timeout
}

// RunReceiveLoop continuously receives messages and dispatches them to the handler until the context is cancelled.
func (conn *Connection) RunReceiveLoop(ctx context.Context, handler MessageHandler) error {
	doneChan := make(chan struct{})
	defer close(doneChan)

	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-doneChan:
		}
	}()

	if err := handler.OnConnect(ctx); err != nil {
		handler.OnClose(err)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var raw WSMessage
		if err := conn.Receive(&raw); err != nil {
			handler.OnClose(err)
			return err
		}

		switch raw.Type {
		case "ping":
			slog.Debug("ws received ping from server, replying with pong")
			if err := conn.SendMessage("pong", nil); err != nil {
				slog.Error("ws send pong response failed", "error", err)
			}
		case "pong":
			slog.Debug("ws received pong response from server")
		default:
			if err := handler.HandleMessage(ctx, raw); err != nil {
				slog.Error("ws handler failed to process message", "type", raw.Type, "error", err)
				return err
			}
		}
	}
}

// Close gracefully closes the WebSocket connection.
func (conn *Connection) Close() error {
	if conn == nil || conn.Conn == nil {
		return nil
	}
	return conn.Conn.Close()
}
