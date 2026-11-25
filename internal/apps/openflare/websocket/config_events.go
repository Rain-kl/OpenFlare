// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/listener"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/google/uuid"
)

const (
	configChangedChannel       = "edge:config_changed"
	nodeQuarantinedChannel     = "edge:node_quarantined"
	pingTaskDispatchChannel    = "edge:ping_task"
	remoteExecDispatchChannel  = "edge:remote_exec"
	pingScheduleReloadChannel  = "edge:ping_schedule_reload"
	tunnelRelayWakeChannel     = "tunnel:relay_wake"
	tunnelConnectorWakeChannel = "tunnel:connector_wake"
	tunnelDisconnectChannel    = "tunnel:disconnect"
)

type pingTaskDispatchEvent struct {
	PublisherID string                    `json:"publisher_id"`
	NodeID      string                    `json:"node_id"`
	Task        protocol.PingTaskDispatch `json:"task"`
}

type remoteExecDispatchEvent struct {
	PublisherID string                      `json:"publisher_id"`
	NodeID      string                      `json:"node_id"`
	Task        protocol.RemoteExecDispatch `json:"task"`
}

type nodeQuarantinedEvent struct {
	PublisherID string `json:"publisher_id"`
	NodeID      string `json:"node_id"`
	Reason      string `json:"reason,omitempty"`
}

type tunnelWakeEvent struct {
	PublisherID string `json:"publisher_id"`
	RelayID     string `json:"relay_id,omitempty"`
	TunnelID    uint   `json:"tunnel_id,omitempty"`
	All         bool   `json:"all,omitempty"`
}

type tunnelDisconnectEvent struct {
	PublisherID string `json:"publisher_id"`
	Kind        string `json:"kind"`
	TargetID    string `json:"target_id,omitempty"`
	TunnelID    uint   `json:"tunnel_id,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Revoked     bool   `json:"revoked,omitempty"`
}

var (
	configPublisherID    = uuid.NewString()
	configSubscriberOnce sync.Once
)

// NotifyAgentConfigChanged wakes connected nodes after a configuration transaction commits.
// The database remains authoritative when a notification is delayed or missed.
func NotifyAgentConfigChanged(ctx context.Context) {
	wakeAgents()
	if db.Redis == nil {
		return
	}
	if err := db.Redis.Publish(ctx, db.PrefixedKey(configChangedChannel), configPublisherID).Err(); err != nil {
		logger.WarnF(ctx, "could not broadcast edge configuration change: %v", err)
	}
}

// NotifyTunnelRelayChanged wakes one relay (or all relays when relayID is empty) to refresh runtime config.
func NotifyTunnelRelayChanged(ctx context.Context, relayID string) {
	if relayID == "" {
		wakeAllRelays()
	} else {
		wakeRelay(relayID)
	}
	publishTunnelEvent(ctx, tunnelRelayWakeChannel, tunnelWakeEvent{
		PublisherID: configPublisherID,
		RelayID:     relayID,
		All:         relayID == "",
	}, "relay wake")
}

// NotifyTunnelChanged wakes connectors for a specific logical tunnel (or all connectors when tunnelID == 0).
func NotifyTunnelChanged(ctx context.Context, tunnelID uint) {
	if tunnelID == 0 {
		wakeAllConnectors()
	} else {
		wakeTunnelConnectors(tunnelID)
	}
	publishTunnelEvent(ctx, tunnelConnectorWakeChannel, tunnelWakeEvent{
		PublisherID: configPublisherID,
		TunnelID:    tunnelID,
		All:         tunnelID == 0,
	}, "connector wake")
}

// NotifyTunnelPoolsChanged wakes all relays and connectors after relay pool or candidate availability changes.
func NotifyTunnelPoolsChanged(ctx context.Context) {
	NotifyTunnelRelayChanged(ctx, "")
	NotifyTunnelChanged(ctx, 0)
}

// DisconnectTunnelRelay notifies and drops an isolated relay WebSocket across controller replicas.
func DisconnectTunnelRelay(ctx context.Context, relayID, reason string) {
	disconnectLocalRelay(relayID, reason)
	publishTunnelEvent(ctx, tunnelDisconnectChannel, tunnelDisconnectEvent{
		PublisherID: configPublisherID,
		Kind:        "relay",
		TargetID:    relayID,
		Reason:      reason,
	}, "relay disconnect")
}

// DisconnectTunnelConnector notifies and drops a quarantined or revoked connector WebSocket across replicas.
func DisconnectTunnelConnector(ctx context.Context, connectorID, reason string, revoked bool) {
	disconnectLocalConnector(connectorID, reason, revoked)
	publishTunnelEvent(ctx, tunnelDisconnectChannel, tunnelDisconnectEvent{
		PublisherID: configPublisherID,
		Kind:        "connector",
		TargetID:    connectorID,
		Reason:      reason,
		Revoked:     revoked,
	}, "connector disconnect")
}

// DisconnectTunnel drops all live connector WebSockets belonging to a quarantined or disabled tunnel.
func DisconnectTunnel(ctx context.Context, tunnelID uint, reason string) {
	disconnectLocalTunnelConnectors(tunnelID, reason)
	publishTunnelEvent(ctx, tunnelDisconnectChannel, tunnelDisconnectEvent{
		PublisherID: configPublisherID,
		Kind:        "tunnel",
		TunnelID:    tunnelID,
		Reason:      reason,
	}, "tunnel disconnect")
}

func publishTunnelEvent(ctx context.Context, channel string, event any, label string) {
	if db.Redis == nil {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		logger.WarnF(ctx, "encode %s broadcast: %v", label, err)
		return
	}
	if err := db.Redis.Publish(ctx, db.PrefixedKey(channel), string(payload)).Err(); err != nil {
		logger.WarnF(ctx, "could not broadcast %s: %v", label, err)
	}
}

// DisconnectAgent notifies and disconnects a quarantined edge node across controller instances.
func DisconnectAgent(ctx context.Context, nodeID, reason string) {
	disconnectLocalAgent(nodeID, reason)
	if db.Redis == nil {
		return
	}
	payload, err := json.Marshal(nodeQuarantinedEvent{
		PublisherID: configPublisherID,
		NodeID:      nodeID,
		Reason:      reason,
	})
	if err != nil {
		logger.WarnF(ctx, "encode node quarantine broadcast: %v", err)
		return
	}
	if err := db.Redis.Publish(ctx, db.PrefixedKey(nodeQuarantinedChannel), string(payload)).Err(); err != nil {
		logger.WarnF(ctx, "could not broadcast edge node quarantine: %v", err)
	}
}

// DispatchPingTask delivers a latency probe task to a connected edge node, fanning out over Redis if needed.
func DispatchPingTask(ctx context.Context, nodeID string, task protocol.PingTaskDispatch) bool {
	if sendLocalPingTask(nodeID, task) {
		return true
	}
	if db.Redis == nil {
		return false
	}
	payload, err := json.Marshal(pingTaskDispatchEvent{
		PublisherID: configPublisherID,
		NodeID:      nodeID,
		Task:        task,
	})
	if err != nil {
		logger.WarnF(ctx, "encode ping task dispatch: %v", err)
		return false
	}
	if err := db.Redis.Publish(ctx, db.PrefixedKey(pingTaskDispatchChannel), string(payload)).Err(); err != nil {
		logger.WarnF(ctx, "could not broadcast edge ping task: %v", err)
		return false
	}
	return true
}

// DispatchRemoteExec delivers a remote command task to a connected edge node, fanning out over Redis if needed.
func DispatchRemoteExec(ctx context.Context, nodeID string, task protocol.RemoteExecDispatch) bool {
	if !config.Config.EnableWebShell {
		return false
	}
	if sendLocalRemoteExec(nodeID, task) {
		return true
	}
	if db.Redis == nil {
		return false
	}
	payload, err := json.Marshal(remoteExecDispatchEvent{
		PublisherID: configPublisherID,
		NodeID:      nodeID,
		Task:        task,
	})
	if err != nil {
		logger.WarnF(ctx, "encode remote exec dispatch: %v", err)
		return false
	}
	if err := db.Redis.Publish(ctx, db.PrefixedKey(remoteExecDispatchChannel), string(payload)).Err(); err != nil {
		logger.WarnF(ctx, "could not broadcast edge remote exec: %v", err)
		return false
	}
	return true
}

// NotifyPingScheduleChanged reloads the local ping scheduler and notifies peer controller replicas.
func NotifyPingScheduleChanged(ctx context.Context) {
	if handler := currentPingScheduleReloadHandler(); handler != nil {
		handler(ctx)
	}
	if db.Redis == nil {
		return
	}
	if err := db.Redis.Publish(ctx, db.PrefixedKey(pingScheduleReloadChannel), configPublisherID).Err(); err != nil {
		logger.WarnF(ctx, "could not broadcast ping schedule reload: %v", err)
	}
}

// StartConfigChangeListener forwards cross-process changes to local edge and tunnel WebSockets.
func StartConfigChangeListener(ctx context.Context) {
	client := db.Redis
	if client == nil {
		return
	}
	configSubscriberOnce.Do(func() {
		subscriberCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		configKey := db.PrefixedKey(configChangedChannel)
		quarantineKey := db.PrefixedKey(nodeQuarantinedChannel)
		pingTaskKey := db.PrefixedKey(pingTaskDispatchChannel)
		remoteExecKey := db.PrefixedKey(remoteExecDispatchChannel)
		pingReloadKey := db.PrefixedKey(pingScheduleReloadChannel)
		relayWakeKey := db.PrefixedKey(tunnelRelayWakeChannel)
		connectorWakeKey := db.PrefixedKey(tunnelConnectorWakeChannel)
		tunnelDisconnectKey := db.PrefixedKey(tunnelDisconnectChannel)
		subscription := client.Subscribe(subscriberCtx, configKey, quarantineKey, pingTaskKey, remoteExecKey, pingReloadKey, relayWakeKey, connectorWakeKey, tunnelDisconnectKey)
		lifecycle.OnShutdown("edge configuration notifications", func(context.Context) error {
			cancel()
			return subscription.Close()
		})
		go func() {
			for message := range subscription.Channel() {
				switch message.Channel {
				case configKey:
					if message.Payload != configPublisherID {
						wakeAgents()
					}
				case pingTaskKey:
					var event pingTaskDispatchEvent
					if err := json.Unmarshal([]byte(message.Payload), &event); err != nil || event.PublisherID == configPublisherID || event.NodeID == "" {
						continue
					}
					sendLocalPingTask(event.NodeID, event.Task)
				case remoteExecKey:
					var event remoteExecDispatchEvent
					if err := json.Unmarshal([]byte(message.Payload), &event); err != nil || event.PublisherID == configPublisherID || event.NodeID == "" {
						continue
					}
					sendLocalRemoteExec(event.NodeID, event.Task)
				case pingReloadKey:
					if message.Payload != configPublisherID {
						if handler := currentPingScheduleReloadHandler(); handler != nil {
							handler(subscriberCtx)
						}
					}
				case quarantineKey:
					var event nodeQuarantinedEvent
					if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
						continue
					}
					if event.PublisherID != configPublisherID && event.NodeID != "" {
						disconnectLocalAgent(event.NodeID, event.Reason)
						listener.EmitNodeTokenRevoked(subscriberCtx, listener.NodeTokenRevoked{
							NodeID: event.NodeID,
							Reason: event.Reason,
						})
					}
				case relayWakeKey:
					var event tunnelWakeEvent
					if err := json.Unmarshal([]byte(message.Payload), &event); err != nil || event.PublisherID == configPublisherID {
						continue
					}
					if event.All || event.RelayID == "" {
						wakeAllRelays()
					} else {
						wakeRelay(event.RelayID)
					}
				case connectorWakeKey:
					var event tunnelWakeEvent
					if err := json.Unmarshal([]byte(message.Payload), &event); err != nil || event.PublisherID == configPublisherID {
						continue
					}
					if event.All || event.TunnelID == 0 {
						wakeAllConnectors()
					} else {
						wakeTunnelConnectors(event.TunnelID)
					}
				case tunnelDisconnectKey:
					var event tunnelDisconnectEvent
					if err := json.Unmarshal([]byte(message.Payload), &event); err != nil || event.PublisherID == configPublisherID {
						continue
					}
					switch event.Kind {
					case "relay":
						if event.TargetID != "" {
							disconnectLocalRelay(event.TargetID, event.Reason)
						}
					case "connector":
						if event.TargetID != "" {
							disconnectLocalConnector(event.TargetID, event.Reason, event.Revoked)
						}
					case "tunnel":
						if event.TunnelID > 0 {
							disconnectLocalTunnelConnectors(event.TunnelID, event.Reason)
						}
					}
				}
			}
		}()
	})
}
