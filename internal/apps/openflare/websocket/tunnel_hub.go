// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"net/http"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/gin-gonic/gin"
)

const disconnectFlushGrace = 50 * time.Millisecond

type relayClient struct {
	wsClientCore
}

type connectorClient struct {
	wsClientCore
	tunnelID uint
}

type tunnelRelayHub struct {
	mu      sync.RWMutex
	clients map[string]*relayClient
}

type tunnelConnectorHub struct {
	mu       sync.RWMutex
	clients  map[string]*connectorClient
	byTunnel map[uint]map[string]*connectorClient
}

var (
	defaultTunnelRelayHub     = &tunnelRelayHub{clients: make(map[string]*relayClient)}
	defaultTunnelConnectorHub = &tunnelConnectorHub{
		clients:  make(map[string]*connectorClient),
		byTunnel: make(map[uint]map[string]*connectorClient),
	}
)

// ServeRelay upgrades an openflare-relay WebSocket connection and pumps control-plane wake signals.
func ServeRelay(c *gin.Context, relayID string) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &relayClient{
		wsClientCore: wsClientCore{
			nodeID: relayID,
			conn:   conn,
			send:   make(chan Message, wsChannelBuf),
			done:   make(chan struct{}),
		},
	}
	defaultTunnelRelayHub.register(client)
	defer defaultTunnelRelayHub.unregister(client)
	defer client.close()

	go client.writePump()
	client.readPump()
}

func (h *tunnelRelayHub) register(client *relayClient) {
	h.mu.Lock()
	previous := h.clients[client.nodeID]
	h.clients[client.nodeID] = client
	h.mu.Unlock()
	if previous != nil && previous != client {
		previous.close()
	}
}

func (h *tunnelRelayHub) unregister(client *relayClient) {
	h.mu.Lock()
	if h.clients[client.nodeID] == client {
		delete(h.clients, client.nodeID)
	}
	h.mu.Unlock()
}

// IsRelayConnected reports whether the relay currently maintains a live control WebSocket.
func IsRelayConnected(relayID string) bool {
	defaultTunnelRelayHub.mu.RLock()
	client := defaultTunnelRelayHub.clients[relayID]
	defaultTunnelRelayHub.mu.RUnlock()
	return client != nil
}

// SendRelayMessage enqueues a WebSocket frame to a connected relay.
func SendRelayMessage(relayID string, message Message) bool {
	defaultTunnelRelayHub.mu.RLock()
	client := defaultTunnelRelayHub.clients[relayID]
	defaultTunnelRelayHub.mu.RUnlock()
	if client == nil {
		return false
	}
	return client.enqueue(message)
}

func wakeRelay(relayID string) bool {
	return SendRelayMessage(relayID, Message{Type: protocol.TunnelWSMessageTypeConfigChanged})
}

func wakeAllRelays() {
	defaultTunnelRelayHub.mu.RLock()
	clients := make([]*relayClient, 0, len(defaultTunnelRelayHub.clients))
	for _, client := range defaultTunnelRelayHub.clients {
		clients = append(clients, client)
	}
	defaultTunnelRelayHub.mu.RUnlock()

	msg := Message{Type: protocol.TunnelWSMessageTypeConfigChanged}
	for _, client := range clients {
		client.enqueue(msg)
	}
}

func disconnectLocalRelay(relayID, reason string) bool {
	defaultTunnelRelayHub.mu.Lock()
	client := defaultTunnelRelayHub.clients[relayID]
	if client != nil {
		delete(defaultTunnelRelayHub.clients, relayID)
	}
	defaultTunnelRelayHub.mu.Unlock()
	if client == nil {
		return false
	}
	client.enqueue(Message{
		Type:    protocol.WSMessageTypeQuarantined,
		Payload: map[string]string{"reason": reason},
	})
	go func() {
		time.Sleep(disconnectFlushGrace)
		client.close()
	}()
	return true
}

func (c *relayClient) readPump() {
	runReadPump(c.nodeID, c.conn, c.close, "tunnel relay ws", func(id string) bool {
		return SendRelayMessage(id, Message{Type: messageTypePong})
	}, messageTypePong)
}

func (c *relayClient) writePump() {
	runWritePump(c.nodeID, c.conn, c.done, c.send, c.close, "tunnel relay ws")
}

// ServeConnector upgrades an openflared connector WebSocket connection and registers it under tunnelID.
func ServeConnector(c *gin.Context, tunnelID uint, connectorID string) {
	if tunnelID == 0 || connectorID == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &connectorClient{
		wsClientCore: wsClientCore{
			nodeID: connectorID,
			conn:   conn,
			send:   make(chan Message, wsChannelBuf),
			done:   make(chan struct{}),
		},
		tunnelID: tunnelID,
	}
	defaultTunnelConnectorHub.register(client)
	defer defaultTunnelConnectorHub.unregister(client)
	defer client.close()

	go client.writePump()
	client.readPump()
}

func (h *tunnelConnectorHub) register(client *connectorClient) {
	h.mu.Lock()
	previous := h.clients[client.nodeID]
	if previous != nil && previous != client {
		h.removeFromTunnelLocked(previous)
	}
	h.clients[client.nodeID] = client
	group := h.byTunnel[client.tunnelID]
	if group == nil {
		group = make(map[string]*connectorClient)
		h.byTunnel[client.tunnelID] = group
	}
	group[client.nodeID] = client
	h.mu.Unlock()

	if previous != nil && previous != client {
		previous.close()
	}
}

func (h *tunnelConnectorHub) unregister(client *connectorClient) {
	h.mu.Lock()
	if h.clients[client.nodeID] == client {
		delete(h.clients, client.nodeID)
		h.removeFromTunnelLocked(client)
	}
	h.mu.Unlock()
}

func (h *tunnelConnectorHub) removeFromTunnelLocked(client *connectorClient) {
	group := h.byTunnel[client.tunnelID]
	if group == nil {
		return
	}
	if group[client.nodeID] == client {
		delete(group, client.nodeID)
	}
	if len(group) == 0 {
		delete(h.byTunnel, client.tunnelID)
	}
}

// IsConnectorConnected reports whether a specific connector replica has an active control WebSocket.
func IsConnectorConnected(connectorID string) bool {
	defaultTunnelConnectorHub.mu.RLock()
	client := defaultTunnelConnectorHub.clients[connectorID]
	defaultTunnelConnectorHub.mu.RUnlock()
	return client != nil
}

// SendConnectorMessage enqueues a WebSocket message to a specific connector replica.
func SendConnectorMessage(connectorID string, message Message) bool {
	defaultTunnelConnectorHub.mu.RLock()
	client := defaultTunnelConnectorHub.clients[connectorID]
	defaultTunnelConnectorHub.mu.RUnlock()
	if client == nil {
		return false
	}
	return client.enqueue(message)
}

func wakeTunnelConnectors(tunnelID uint) {
	defaultTunnelConnectorHub.mu.RLock()
	group := defaultTunnelConnectorHub.byTunnel[tunnelID]
	clients := make([]*connectorClient, 0, len(group))
	for _, client := range group {
		clients = append(clients, client)
	}
	defaultTunnelConnectorHub.mu.RUnlock()

	msg := Message{Type: protocol.TunnelWSMessageTypeConfigChanged}
	for _, client := range clients {
		client.enqueue(msg)
	}
}

func wakeAllConnectors() {
	defaultTunnelConnectorHub.mu.RLock()
	clients := make([]*connectorClient, 0, len(defaultTunnelConnectorHub.clients))
	for _, client := range defaultTunnelConnectorHub.clients {
		clients = append(clients, client)
	}
	defaultTunnelConnectorHub.mu.RUnlock()

	msg := Message{Type: protocol.TunnelWSMessageTypeConfigChanged}
	for _, client := range clients {
		client.enqueue(msg)
	}
}

func disconnectLocalConnector(connectorID, reason string, revoked bool) bool {
	defaultTunnelConnectorHub.mu.Lock()
	client := defaultTunnelConnectorHub.clients[connectorID]
	if client != nil {
		delete(defaultTunnelConnectorHub.clients, connectorID)
		defaultTunnelConnectorHub.removeFromTunnelLocked(client)
	}
	defaultTunnelConnectorHub.mu.Unlock()
	if client == nil {
		return false
	}
	closeConnectorWithNotice(client, reason, revoked)
	return true
}

func disconnectLocalTunnelConnectors(tunnelID uint, reason string) int {
	defaultTunnelConnectorHub.mu.Lock()
	group := defaultTunnelConnectorHub.byTunnel[tunnelID]
	clients := make([]*connectorClient, 0, len(group))
	for id, client := range group {
		clients = append(clients, client)
		delete(defaultTunnelConnectorHub.clients, id)
	}
	delete(defaultTunnelConnectorHub.byTunnel, tunnelID)
	defaultTunnelConnectorHub.mu.Unlock()

	for _, client := range clients {
		closeConnectorWithNotice(client, reason, false)
	}
	return len(clients)
}

func closeConnectorWithNotice(client *connectorClient, reason string, revoked bool) {
	msgType := protocol.WSMessageTypeQuarantined
	if revoked {
		msgType = protocol.TunnelWSMessageTypeRevoked
	}
	client.enqueue(Message{
		Type:    msgType,
		Payload: map[string]string{"reason": reason},
	})
	go func() {
		time.Sleep(disconnectFlushGrace)
		client.close()
	}()
}

func (c *connectorClient) readPump() {
	runReadPump(c.nodeID, c.conn, c.close, "tunnel connector ws", func(id string) bool {
		return SendConnectorMessage(id, Message{Type: messageTypePong})
	}, messageTypePong)
}

func (c *connectorClient) writePump() {
	runWritePump(c.nodeID, c.conn, c.done, c.send, c.close, "tunnel connector ws")
}
