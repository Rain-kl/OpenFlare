// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"log/slog"
	"sync"

	"github.com/gin-gonic/gin"

	"Wavelet/pkg/util"
)

// RelayWSConnectedLastSeenValue is the sentinel last_seen_at value when relay WS is connected.
const RelayWSConnectedLastSeenValue = "__OPENFLARE_WS_CONNECTED__"

type relayClient struct {
	wsClientCore
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
		wsClientCore: wsClientCore{
			nodeID: nodeID,
			conn:   conn,
			send:   make(chan Message, wsChannelBuf),
			done:   make(chan struct{}),
		},
	}
	defaultRelayHub.register(client)
	defer defaultRelayHub.unregister(client)

	slog.Debug("relay ws connected", "node_id", nodeID, "remote", c.Request.RemoteAddr)

	util.Go(client.writePump)
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
	// 委托 enqueue：closed 检查与发送不能合并在同一个 select（两 case 同时
	// 就绪时 Go 随机选择，close 后仍可能投递成功）。
	return client.enqueue(Message{Type: messageTypePong})
}

func (c *relayClient) readPump() {
	runReadPump(c.nodeID, c.conn, c.close, "relay ws", SendRelayPong, messageTypePong)
}

func (c *relayClient) writePump() {
	runWritePump(c.nodeID, c.conn, c.done, c.send, c.close, "relay ws")
}
