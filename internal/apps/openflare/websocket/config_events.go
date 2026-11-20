// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"sync"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/google/uuid"
)

const configChangedChannel = "edge:config_changed"

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

// StartConfigChangeListener forwards cross-process changes to local edge WebSockets.
func StartConfigChangeListener(ctx context.Context) {
	client := db.Redis
	if client == nil {
		return
	}
	configSubscriberOnce.Do(func() {
		subscriberCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		subscription := client.Subscribe(subscriberCtx, db.PrefixedKey(configChangedChannel))
		lifecycle.OnShutdown("edge configuration notifications", func(context.Context) error {
			cancel()
			return subscription.Close()
		})
		go func() {
			for message := range subscription.Channel() {
				if message.Payload != configPublisherID {
					wakeAgents()
				}
			}
		}()
	})
}
