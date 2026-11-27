// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package websocket manages persistent WebSocket connections between the OpenFlare server and its agents.
package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/gin-gonic/gin"
)

const (
	// AgentWSConnectedLastSeenValue is the sentinel last_seen_at value when agent WS is connected.
	AgentWSConnectedLastSeenValue = "__OPENFLARE_WS_CONNECTED__"

	agentMessageTypeConfigChanged = "config_changed"
	quarantineCloseDelay          = 150 * time.Millisecond
)

type agentClient struct {
	wsClientCore
	runtimeLogs agentRuntimeLogs
}

type agentHub struct {
	mu      sync.RWMutex
	clients map[string]*agentClient
}

var defaultAgentHub = &agentHub{clients: make(map[string]*agentClient)}

var (
	agentHandlersMu          sync.RWMutex
	pingResultHandler        func(ctx context.Context, nodeID string, payload protocol.PingResultPayload)
	pingScheduleReloadHandle func(ctx context.Context)
	netProbeResultHandler    func(ctx context.Context, nodeID string, payload protocol.NetProbeResultPayload)
)

// SetPingResultHandler registers the callback invoked when an agent reports a ping result over WebSocket.
func SetPingResultHandler(fn func(ctx context.Context, nodeID string, payload protocol.PingResultPayload)) {
	agentHandlersMu.Lock()
	pingResultHandler = fn
	agentHandlersMu.Unlock()
}

// SetPingScheduleReloadHandler registers the callback invoked when ping task schedules change across replicas.
func SetPingScheduleReloadHandler(fn func(ctx context.Context)) {
	agentHandlersMu.Lock()
	pingScheduleReloadHandle = fn
	agentHandlersMu.Unlock()
}

// SetNetProbeResultHandler registers the callback invoked when an agent reports a network probe result.
func SetNetProbeResultHandler(fn func(ctx context.Context, nodeID string, payload protocol.NetProbeResultPayload)) {
	agentHandlersMu.Lock()
	netProbeResultHandler = fn
	agentHandlersMu.Unlock()
}

// HandleNetProbeResult dispatches an agent network probe result to the registered handler if present.
func HandleNetProbeResult(ctx context.Context, nodeID string, payload protocol.NetProbeResultPayload) {
	handler := currentNetProbeResultHandler()
	if handler == nil {
		return
	}
	handler(ctx, nodeID, payload)
}

func currentPingResultHandler() func(ctx context.Context, nodeID string, payload protocol.PingResultPayload) {
	agentHandlersMu.RLock()
	defer agentHandlersMu.RUnlock()
	return pingResultHandler
}

func currentPingScheduleReloadHandler() func(ctx context.Context) {
	agentHandlersMu.RLock()
	defer agentHandlersMu.RUnlock()
	return pingScheduleReloadHandle
}

func currentNetProbeResultHandler() func(ctx context.Context, nodeID string, payload protocol.NetProbeResultPayload) {
	agentHandlersMu.RLock()
	defer agentHandlersMu.RUnlock()
	return netProbeResultHandler
}

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
	conn.SetReadLimit(maxAgentMessageBytes)
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

func sendLocalPingTask(nodeID string, task protocol.PingTaskDispatch) bool {
	return sendAgentMessage(nodeID, Message{
		Type:    protocol.WSMessageTypePingTask,
		Payload: task,
	})
}

func sendLocalNetProbeTask(nodeID string, task protocol.NetProbeTaskDispatch) bool {
	return sendAgentMessage(nodeID, Message{
		Type:    protocol.WSMessageTypeNetProbeTask,
		Payload: task,
	})
}

func sendLocalRemoteExec(nodeID string, task protocol.RemoteExecDispatch) bool {
	if !config.Config.EnableWebShell {
		return false
	}
	return sendAgentMessage(nodeID, Message{
		Type:    protocol.WSMessageTypeRemoteExec,
		Payload: task,
	})
}

func disconnectLocalAgent(nodeID, reason string) bool {
	defaultAgentHub.mu.Lock()
	client := defaultAgentHub.clients[nodeID]
	if client != nil {
		delete(defaultAgentHub.clients, nodeID)
	}
	defaultAgentHub.mu.Unlock()
	if client == nil {
		return false
	}
	if client.enqueue(Message{
		Type: protocol.WSMessageTypeQuarantined,
		Payload: struct {
			Reason string `json:"reason,omitempty"`
		}{Reason: reason},
	}) {
		time.AfterFunc(quarantineCloseDelay, client.close)
	} else {
		client.close()
	}
	return true
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
		case protocol.WSMessageTypePingResult:
			var payload protocol.PingResultPayload
			if err := json.Unmarshal(inbound.Payload, &payload); err != nil {
				slog.Debug("agent ws invalid ping_result payload", "node_id", c.nodeID, "error", err)
				continue
			}
			if handler := currentPingResultHandler(); handler != nil {
				handler(context.Background(), c.nodeID, payload)
			}
		case protocol.WSMessageTypeNetProbeResult:
			var payload protocol.NetProbeResultPayload
			if err := json.Unmarshal(inbound.Payload, &payload); err != nil {
				slog.Debug("agent ws invalid net_probe_result payload", "node_id", c.nodeID, "error", err)
				continue
			}
			HandleNetProbeResult(context.Background(), c.nodeID, payload)
		case protocol.WSMessageTypeRuntimeLogFrame:
			var frame protocol.RuntimeLogFrame
			if json.Unmarshal(inbound.Payload, &frame) == nil {
				c.dispatchRuntimeLogFrame(frame)
			}
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
