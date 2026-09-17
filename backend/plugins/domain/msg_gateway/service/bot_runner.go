// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"context"
	"sync"
)

// Handler processes one inbound message.
type Handler func(ctx context.Context, msg do.InboundMessage) error

// Factory constructs a Channel from decrypted config.
type Factory func(cfg do.ChannelConfig, onInbound Handler) (Channel, error)

// Channel is one connected messaging adapter.
type Channel interface {
	Type() string
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Send(ctx context.Context, to do.Recipient, msg do.OutboundMessage) error
	Capabilities() do.Capability
}

var (
	factoriesMu sync.RWMutex
	factories   = map[string]Factory{}
)

// Register stores a channel factory under typ.
func Register(typ string, fn Factory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[typ] = fn
}

// Lookup returns a previously registered factory.
func Lookup(typ string) (Factory, bool) {
	factoriesMu.RLock()
	defer factoriesMu.RUnlock()
	fn, ok := factories[typ]
	return fn, ok
}

// Runner manages lifecycle for long-lived channel adapters (WebSocket, long-polling, etc.).
type Runner struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

// GlobalRunner is the default global runner instance.
var GlobalRunner = &Runner{}

// Start starts all background long-lived channel runners.
func Start(ctx context.Context) error {
	GlobalRunner.mu.Lock()
	defer GlobalRunner.mu.Unlock()

	if GlobalRunner.running {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	GlobalRunner.cancel = cancel
	GlobalRunner.running = true

	logger.InfoF(runCtx, "[MessageGateway] Starting bot channel runners...")
	return nil
}

// Stop stops the channel runner.
func Stop() {
	GlobalRunner.mu.Lock()
	defer GlobalRunner.mu.Unlock()

	if !GlobalRunner.running {
		return
	}

	if GlobalRunner.cancel != nil {
		GlobalRunner.cancel()
	}
	GlobalRunner.running = false
}
