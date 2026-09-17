// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"log/slog"
	"sync"

	"github.com/gin-gonic/gin"

	"Wavelet/pkg/util"
)

const (
	// FlaredWSConnectedLastSeenValue is the sentinel last_seen_at value when flared WS is connected.
	FlaredWSConnectedLastSeenValue = "__OPENFLARE_FLARED_WS_CONNECTED__"

	flaredMessageTypeActiveConfig = "active_config"
	flaredMessageTypeForceSync    = "force_sync"
	flaredMessageTypePong         = "pong"
)

type flaredClient struct {
	wsClientCore
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
		wsClientCore: wsClientCore{
			nodeID: nodeID,
			conn:   conn,
			send:   make(chan Message, wsChannelBuf),
			done:   make(chan struct{}),
		},
	}
	defaultFlaredHub.register(client)
	defer defaultFlaredHub.unregister(client)

	slog.Debug("flared ws connected", "node_id", nodeID, "remote", c.Request.RemoteAddr)

	util.Go(client.writePump)
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
	// 委托 enqueue：closed 检查与发送不能合并在同一个 select（两 case 同时
	// 就绪时 Go 随机选择，close 后仍可能投递成功）。
	return client.enqueue(Message{Type: flaredMessageTypePong})
}

func (c *flaredClient) readPump() {
	runReadPump(c.nodeID, c.conn, c.close, "flared ws", SendFlaredPong, flaredMessageTypePong)
}

func (c *flaredClient) writePump() {
	runWritePump(c.nodeID, c.conn, c.done, c.send, c.close, "flared ws")
}
