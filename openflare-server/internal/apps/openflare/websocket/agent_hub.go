// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// AgentWSConnectedLastSeenValue is the sentinel last_seen_at value when agent WS is connected.
	AgentWSConnectedLastSeenValue = "__OPENFLARE_WS_CONNECTED__"

	agentMessageTypeStatus          = "status"
	agentMessageTypeSettings        = "settings"
	agentMessageTypeActiveConfig    = "active_config"
	agentMessageTypeForceSyncConfig = "force_sync_config"
	agentMessageTypeWAFIPGroups     = "waf_ip_groups"
)

// AgentStatusHandler processes inbound agent websocket status payloads.
type AgentStatusHandler func(ctx context.Context, nodeID, remoteAddr string, payload json.RawMessage)

type agentClient struct {
	nodeID     string
	remoteAddr string
	conn       *websocket.Conn
	send       chan Message
	done       chan struct{}
	onStatus   AgentStatusHandler
	once       sync.Once
}

func (c *agentClient) close() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

type agentHub struct {
	mu      sync.RWMutex
	clients map[string]*agentClient
}

var defaultAgentHub = &agentHub{clients: make(map[string]*agentClient)}

// ServeAgent handles an upgraded agent websocket connection.
func ServeAgent(c *gin.Context, nodeID string, onStatus AgentStatusHandler) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Debug("agent ws upgrade failed", "node_id", nodeID, "error", err)
		return
	}

	client := &agentClient{
		nodeID:     nodeID,
		remoteAddr: c.Request.RemoteAddr,
		conn:       conn,
		send:       make(chan Message, 16),
		done:       make(chan struct{}),
		onStatus:   onStatus,
	}
	defaultAgentHub.register(client)
	defer defaultAgentHub.unregister(client)

	slog.Debug("agent ws connected", "node_id", nodeID, "remote", client.remoteAddr)

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

// SendAgentSettings pushes agent settings to a connected agent.
func SendAgentSettings(nodeID string, payload any) bool {
	return sendAgentMessage(nodeID, Message{Type: agentMessageTypeSettings, Payload: payload})
}

// SendAgentActiveConfig pushes active config metadata to a connected agent.
func SendAgentActiveConfig(nodeID string, payload any) bool {
	return sendAgentMessage(nodeID, Message{Type: agentMessageTypeActiveConfig, Payload: payload})
}

// SendAgentWAFIPGroups pushes WAF IP group updates to a connected agent.
func SendAgentWAFIPGroups(nodeID string, payload any) bool {
	return sendAgentMessage(nodeID, Message{Type: agentMessageTypeWAFIPGroups, Payload: payload})
}

// BroadcastWAFIPGroups pushes changed WAF IP groups to all connected agents.
func BroadcastWAFIPGroups(payload any) int {
	if payload == nil {
		return 0
	}
	message := Message{Type: agentMessageTypeWAFIPGroups, Payload: payload}
	defaultAgentHub.mu.RLock()
	clients := make([]*agentClient, 0, len(defaultAgentHub.clients))
	for _, client := range defaultAgentHub.clients {
		clients = append(clients, client)
	}
	defaultAgentHub.mu.RUnlock()

	success := 0
	for _, client := range clients {
		if client.enqueue(message) {
			success++
		}
	}
	return success
}

// SendForceSyncConfig notifies an agent to force sync configuration.
func SendForceSyncConfig(nodeID string, payload any) bool {
	return sendAgentMessage(nodeID, Message{Type: agentMessageTypeForceSyncConfig, Payload: payload})
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
		case agentMessageTypeStatus:
			if c.onStatus != nil {
				c.onStatus(context.Background(), c.nodeID, c.remoteAddr, inbound.Payload)
			}
		case messageTypePing:
			_ = c.enqueue(Message{Type: messageTypePong})
		case messageTypePong:
		default:
			slog.Debug("agent ws unsupported message type", "node_id", c.nodeID, "type", inbound.Type)
		}
	}
}

func agentWSReadTimeout() time.Duration {
	timeout := 90 * time.Second
	if timeout < 30*time.Second {
		return 30 * time.Second
	}
	return timeout
}

func (c *agentClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case message := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteJSON(message); err != nil {
				slog.Debug("agent ws write failed", "node_id", c.nodeID, "error", err)
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

func (c *agentClient) enqueue(message Message) bool {
	select {
	case <-c.done:
		return false
	case c.send <- message:
		return true
	default:
		return false
	}
}
