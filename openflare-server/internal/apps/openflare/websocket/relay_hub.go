// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// RelayWSConnectedLastSeenValue is the sentinel last_seen_at value when relay WS is connected.
const RelayWSConnectedLastSeenValue = "__OPENFLARE_WS_CONNECTED__"

type relayClient struct {
	nodeID string
	conn   *websocket.Conn
	send   chan Message
	done   chan struct{}
	once   sync.Once
}

func (c *relayClient) close() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

type relayHub struct {
	mu      sync.RWMutex
	clients map[string]*relayClient
}

var defaultRelayHub = &relayHub{clients: make(map[string]*relayClient)}

// ServeRelay handles an upgraded relay websocket connection.
func ServeRelay(c *gin.Context, nodeID string) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Debug("relay ws upgrade failed", "node_id", nodeID, "error", err)
		return
	}

	client := &relayClient{
		nodeID: nodeID,
		conn:   conn,
		send:   make(chan Message, 16),
		done:   make(chan struct{}),
	}
	defaultRelayHub.register(client)
	defer defaultRelayHub.unregister(client)

	slog.Debug("relay ws connected", "node_id", nodeID, "remote", c.Request.RemoteAddr)

	go client.writePump()
	client.readPump()
}

func (h *relayHub) register(client *relayClient) {
	h.mu.Lock()
	if existing := h.clients[client.nodeID]; existing != nil {
		existing.close()
	}
	h.clients[client.nodeID] = client
	h.mu.Unlock()
}

func (h *relayHub) unregister(client *relayClient) {
	h.mu.Lock()
	if current := h.clients[client.nodeID]; current == client {
		delete(h.clients, client.nodeID)
	}
	h.mu.Unlock()
	client.close()
}

// IsRelayConnected reports whether a relay websocket is active.
func IsRelayConnected(nodeID string) bool {
	defaultRelayHub.mu.RLock()
	client := defaultRelayHub.clients[nodeID]
	defaultRelayHub.mu.RUnlock()
	if client == nil {
		return false
	}
	select {
	case <-client.done:
		return false
	default:
		return true
	}
}

// SendRelayPong enqueues a pong message for the relay node.
func SendRelayPong(nodeID string) bool {
	defaultRelayHub.mu.RLock()
	client := defaultRelayHub.clients[nodeID]
	defaultRelayHub.mu.RUnlock()
	if client == nil {
		return false
	}
	select {
	case <-client.done:
		return false
	case client.send <- Message{Type: messageTypePong}:
		return true
	default:
		return false
	}
}

func (c *relayClient) readPump() {
	defer c.close()
	_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			slog.Debug("relay ws read closed", "node_id", c.nodeID, "error", err)
			return
		}

		var message Message
		if err = json.Unmarshal(data, &message); err != nil {
			slog.Debug("relay ws invalid message", "node_id", c.nodeID, "error", err)
			continue
		}

		switch message.Type {
		case messageTypePing:
			_ = SendRelayPong(c.nodeID)
		case messageTypePong:
		default:
			slog.Debug("relay ws unsupported message", "node_id", c.nodeID, "type", message.Type)
		}
	}
}

func (c *relayClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case message := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteJSON(message); err != nil {
				slog.Debug("relay ws write failed", "node_id", c.nodeID, "error", err)
				c.close()
				return
			}
		case <-ticker.C:
			select {
			case <-c.done:
				return
			case c.send <- Message{Type: messageTypePing}:
			default:
			}
		}
	}
}
