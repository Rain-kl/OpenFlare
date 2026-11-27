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

type dnsInstanceClient struct {
	wsClientCore
}

type dnsInstanceHub struct {
	mu      sync.RWMutex
	clients map[string]*dnsInstanceClient
}

var defaultDNSInstanceHub = &dnsInstanceHub{
	clients: make(map[string]*dnsInstanceClient),
}

// ServeDNSInstance upgrades an openflare-dns WebSocket connection and pumps snapshot wake notifications.
func ServeDNSInstance(c *gin.Context, instanceID string) {
	if instanceID == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &dnsInstanceClient{
		wsClientCore: wsClientCore{
			nodeID: instanceID,
			conn:   conn,
			send:   make(chan Message, wsChannelBuf),
			done:   make(chan struct{}),
		},
	}
	defaultDNSInstanceHub.register(client)
	defer defaultDNSInstanceHub.unregister(client)
	defer client.close()

	go client.writePump()
	client.readPump()
}

func (h *dnsInstanceHub) register(client *dnsInstanceClient) {
	h.mu.Lock()
	previous := h.clients[client.nodeID]
	h.clients[client.nodeID] = client
	h.mu.Unlock()
	if previous != nil && previous != client {
		previous.close()
	}
}

func (h *dnsInstanceHub) unregister(client *dnsInstanceClient) {
	h.mu.Lock()
	if h.clients[client.nodeID] == client {
		delete(h.clients, client.nodeID)
	}
	h.mu.Unlock()
}

// IsDNSInstanceConnected reports whether the SmartDNS instance holds an active control WebSocket.
func IsDNSInstanceConnected(instanceID string) bool {
	defaultDNSInstanceHub.mu.RLock()
	client := defaultDNSInstanceHub.clients[instanceID]
	defaultDNSInstanceHub.mu.RUnlock()
	return client != nil
}

// SendDNSMessage enqueues a WebSocket frame to a connected SmartDNS instance.
func SendDNSMessage(instanceID string, message Message) bool {
	defaultDNSInstanceHub.mu.RLock()
	client := defaultDNSInstanceHub.clients[instanceID]
	defaultDNSInstanceHub.mu.RUnlock()
	if client == nil {
		return false
	}
	return client.enqueue(message)
}

func wakeAllDNSInstances() {
	defaultDNSInstanceHub.mu.RLock()
	clients := make([]*dnsInstanceClient, 0, len(defaultDNSInstanceHub.clients))
	for _, client := range defaultDNSInstanceHub.clients {
		clients = append(clients, client)
	}
	defaultDNSInstanceHub.mu.RUnlock()

	msg := Message{Type: protocol.DNSWSMessageTypeSnapshotChanged}
	for _, client := range clients {
		client.enqueue(msg)
	}
}

func disconnectLocalDNSInstance(instanceID, reason string) bool {
	defaultDNSInstanceHub.mu.Lock()
	client := defaultDNSInstanceHub.clients[instanceID]
	if client != nil {
		delete(defaultDNSInstanceHub.clients, instanceID)
	}
	defaultDNSInstanceHub.mu.Unlock()
	if client == nil {
		return false
	}
	client.enqueue(Message{
		Type:    protocol.DNSWSMessageTypeDisconnect,
		Payload: map[string]string{"reason": reason},
	})
	go func() {
		time.Sleep(disconnectFlushGrace)
		client.close()
	}()
	return true
}

func (c *dnsInstanceClient) readPump() {
	runReadPump(c.nodeID, c.conn, c.close, "smartdns ws", func(id string) bool {
		return SendDNSMessage(id, Message{Type: messageTypePong})
	}, messageTypePong)
}

func (c *dnsInstanceClient) writePump() {
	runWritePump(c.nodeID, c.conn, c.done, c.send, c.close, "smartdns ws")
}
