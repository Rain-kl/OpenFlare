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

const (
	// FlaredWSConnectedLastSeenValue is the sentinel last_seen_at value when flared WS is connected.
	FlaredWSConnectedLastSeenValue = "__OPENFLARE_FLARED_WS_CONNECTED__"

	flaredMessageTypeActiveConfig = "active_config"
	flaredMessageTypeForceSync    = "force_sync"
	flaredMessageTypePong         = "pong"
)

type flaredClient struct {
	nodeID string
	conn   *websocket.Conn
	send   chan Message
	done   chan struct{}
	once   sync.Once
}

func (c *flaredClient) close() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

type flaredHub struct {
	mu      sync.RWMutex
	clients map[string]*flaredClient
}

var defaultFlaredHub = &flaredHub{clients: make(map[string]*flaredClient)}

// ServeFlared handles an upgraded flared websocket connection.
func ServeFlared(c *gin.Context, nodeID string) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Debug("flared ws upgrade failed", "node_id", nodeID, "error", err)
		return
	}

	client := &flaredClient{
		nodeID: nodeID,
		conn:   conn,
		send:   make(chan Message, 16),
		done:   make(chan struct{}),
	}
	defaultFlaredHub.register(client)
	defer defaultFlaredHub.unregister(client)

	slog.Debug("flared ws connected", "node_id", nodeID, "remote", c.Request.RemoteAddr)

	go client.writePump()
	client.readPump()
}

func (h *flaredHub) register(client *flaredClient) {
	h.mu.Lock()
	if existing := h.clients[client.nodeID]; existing != nil {
		existing.close()
	}
	h.clients[client.nodeID] = client
	h.mu.Unlock()
}

func (h *flaredHub) unregister(client *flaredClient) {
	h.mu.Lock()
	if current := h.clients[client.nodeID]; current == client {
		delete(h.clients, client.nodeID)
	}
	h.mu.Unlock()
	client.close()
}

// DisconnectFlaredClient forcefully disconnects a flared websocket client.
func DisconnectFlaredClient(nodeID string) {
	defaultFlaredHub.mu.Lock()
	client := defaultFlaredHub.clients[nodeID]
	if client != nil {
		delete(defaultFlaredHub.clients, nodeID)
	}
	defaultFlaredHub.mu.Unlock()
	if client != nil {
		client.close()
	}
}

// IsFlaredConnected reports whether a flared websocket is active.
func IsFlaredConnected(nodeID string) bool {
	defaultFlaredHub.mu.RLock()
	client := defaultFlaredHub.clients[nodeID]
	defaultFlaredHub.mu.RUnlock()
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

// SendFlaredPong enqueues a pong message for the flared node.
func SendFlaredPong(nodeID string) bool {
	defaultFlaredHub.mu.RLock()
	client := defaultFlaredHub.clients[nodeID]
	defaultFlaredHub.mu.RUnlock()
	if client == nil {
		return false
	}
	select {
	case <-client.done:
		return false
	case client.send <- Message{Type: flaredMessageTypePong}:
		return true
	default:
		return false
	}
}

func (c *flaredClient) readPump() {
	defer c.close()
	_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			slog.Debug("flared ws read closed", "node_id", c.nodeID, "error", err)
			return
		}

		var message Message
		if err = json.Unmarshal(data, &message); err != nil {
			slog.Debug("flared ws invalid message", "node_id", c.nodeID, "error", err)
			continue
		}

		switch message.Type {
		case messageTypePing:
			_ = SendFlaredPong(c.nodeID)
		case flaredMessageTypePong:
		default:
			slog.Debug("flared ws unsupported message", "node_id", c.nodeID, "type", message.Type)
		}
	}
}

func (c *flaredClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case message := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteJSON(message); err != nil {
				slog.Debug("flared ws write failed", "node_id", c.nodeID, "error", err)
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
