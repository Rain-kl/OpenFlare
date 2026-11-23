// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"encoding/json"
	"sync"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/listener"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/google/uuid"
)

const (
	configChangedChannel   = "edge:config_changed"
	nodeQuarantinedChannel = "edge:node_quarantined"
)

type nodeQuarantinedEvent struct {
	PublisherID string `json:"publisher_id"`
	NodeID      string `json:"node_id"`
	Reason      string `json:"reason,omitempty"`
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

// StartConfigChangeListener forwards cross-process changes to local edge WebSockets.
func StartConfigChangeListener(ctx context.Context) {
	client := db.Redis
	if client == nil {
		return
	}
	configSubscriberOnce.Do(func() {
		subscriberCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		configKey := db.PrefixedKey(configChangedChannel)
		quarantineKey := db.PrefixedKey(nodeQuarantinedChannel)
		subscription := client.Subscribe(subscriberCtx, configKey, quarantineKey)
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
				}
			}
		}()
	})
}
