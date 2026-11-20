// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package websocket manages persistent WebSocket connections between the OpenFlare server and its agents.
package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// AgentWSConnectedLastSeenValue is the sentinel last_seen_at value when agent WS is connected.
	AgentWSConnectedLastSeenValue = "__OPENFLARE_WS_CONNECTED__"

	agentMessageTypeConfigChanged = "config_changed"
)

type agentClient struct {
	wsClientCore
}

type agentHub struct {
	mu      sync.RWMutex
	clients map[string]*agentClient
}

var defaultAgentHub = &agentHub{clients: make(map[string]*agentClient)}

// ServeAgent handles an upgraded agent websocket connection.
func ServeAgent(c *gin.Context, nodeID string) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Debug("agent ws upgrade failed", "node_id", nodeID, "error", err)
		return
	}

	client := &agentClient{
		wsClientCore: wsClientCore{
			nodeID: nodeID,
			conn:   conn,
			send:   make(chan Message, wsChannelBuf),
			done:   make(chan struct{}),
		},
	}
	defaultAgentHub.register(client)
	defer defaultAgentHub.unregister(client)

	slog.Debug("agent ws connected", "node_id", nodeID, "remote", c.Request.RemoteAddr)

	go client.writePump()
	client.readPump()
}

func (h *agentHub) register(client *agentClient) {
	h.mu.Lock()
	if existing := h.clients[client.nodeID]; existing != nil {
		existing.close()
	}
	h.clients[client.nodeID] = client
	h.mu.Unlock()
}

func (h *agentHub) unregister(client *agentClient) {
	h.mu.Lock()
	if current := h.clients[client.nodeID]; current == client {
		delete(h.clients, client.nodeID)
	}
	h.mu.Unlock()
	client.close()
}

// IsAgentConnected reports whether an agent websocket is active.
func IsAgentConnected(nodeID string) bool {
	defaultAgentHub.mu.RLock()
	client := defaultAgentHub.clients[nodeID]
	defaultAgentHub.mu.RUnlock()
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

func wakeAgents() {
	message := Message{Type: agentMessageTypeConfigChanged}
	defaultAgentHub.mu.RLock()
	clients := make([]*agentClient, 0, len(defaultAgentHub.clients))
	for _, client := range defaultAgentHub.clients {
		clients = append(clients, client)
	}
	defaultAgentHub.mu.RUnlock()
	for _, client := range clients {
		client.enqueue(message)
	}
}

// SendForceSyncConfig asks a specific node to reload its active configuration.
func SendForceSyncConfig(nodeID string) bool {
	return sendAgentMessage(nodeID, Message{Type: agentMessageTypeConfigChanged, Payload: struct {
		Force bool `json:"force"`
	}{Force: true}})
}

func sendAgentMessage(nodeID string, message Message) bool {
	defaultAgentHub.mu.RLock()
	client := defaultAgentHub.clients[nodeID]
	defaultAgentHub.mu.RUnlock()
	if client == nil {
		return false
	}
	return client.enqueue(message)
}

func (c *agentClient) readPump() {
	defer c.close()

	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(agentWSReadTimeout()))
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			slog.Debug("agent ws read closed", "node_id", c.nodeID, "error", err)
			return
		}

		var inbound struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload,omitempty"`
		}
		if err = json.Unmarshal(data, &inbound); err != nil {
			slog.Debug("agent ws invalid message", "node_id", c.nodeID, "error", err)
			continue
		}

		slog.Debug("agent ws message received", "node_id", c.nodeID, "type", inbound.Type)
		switch inbound.Type {
		case messageTypePing:
			_ = c.enqueue(Message{Type: messageTypePong})
		case messageTypePong:
		default:
			slog.Debug("agent ws unsupported message type", "node_id", c.nodeID, "type", inbound.Type)
		}
	}
}

func agentWSReadTimeout() time.Duration {
	timeout := wsReadDeadline
	if timeout < minAgentWSReadTimeout {
		return minAgentWSReadTimeout
	}
	return timeout
}

func (c *agentClient) writePump() {
	runWritePump(c.nodeID, c.conn, c.done, c.send, c.close, "agent ws")
}
