// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package relay implements the relay node daemon runtime loop.
package relay

import (
	"context"
	"encoding/json"
	"log/slog"

	edgerunner "github.com/Rain-kl/Wavelet/internal/edge/common/runner"
	edgews "github.com/Rain-kl/Wavelet/internal/edge/common/wsclient"
	"github.com/Rain-kl/Wavelet/internal/edge/relay/config"
	"github.com/Rain-kl/Wavelet/internal/edge/relay/frps"
	"github.com/Rain-kl/Wavelet/internal/edge/relay/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/edge/relay/httpclient"
	"github.com/Rain-kl/Wavelet/internal/edge/relay/state"
	service "github.com/Rain-kl/Wavelet/internal/openflare/protocol"
)

// Runner manages the relay process.
type Runner struct {
	Config           *config.Config
	StateStore       *state.Store
	HeartbeatService *heartbeat.Service
	FrpsManager      *frps.Manager
	WebSocketService *edgews.Client
	HTTPClient       *httpclient.Client
}

// Run starts the relay process by initiating the heartbeat and WS reconnection loop.
func (r *Runner) Run(ctx context.Context) error {
	go r.HeartbeatService.Run(ctx)

	return edgerunner.RunWSReconnectLoop(ctx, edgerunner.WSReconnectConfig{
		ComponentName: "relay",
		OnShutdown:    r.FrpsManager.Stop,
	}, func(ctx context.Context) (edgerunner.WSConnection, error) {
		return r.WebSocketService.Connect(ctx)
	}, func(ctx context.Context, conn edgerunner.WSConnection) {
		r.handleConnection(ctx, conn)
	})
}

type relayWSHandler struct {
	runner *Runner
}

func (h *relayWSHandler) OnConnect(_ context.Context) error {
	return nil
}

func (h *relayWSHandler) HandleMessage(ctx context.Context, msg edgews.WSMessage) error {
	switch msg.Type {
	case "relay_config":
		var cfg service.RelayConfig
		if err := json.Unmarshal(msg.Payload, &cfg); err != nil {
			slog.Error("failed to unmarshal relay_config", "error", err)
			return nil
		}
		h.runner.FrpsManager.UpdateConfig(ctx, &cfg)
	default:
		slog.Debug("ignored unknown ws message type", "type", msg.Type)
	}
	return nil
}

func (h *relayWSHandler) OnClose(err error) {
	slog.Error("relay ws receive failed", "error", err)
}

func (r *Runner) handleConnection(ctx context.Context, conn edgerunner.WSConnection) {
	wsConn, ok := conn.(*edgews.Connection)
	if !ok {
		slog.Error("relay ws connection has unexpected type")
		return
	}
	_ = wsConn.RunReceiveLoop(ctx, &relayWSHandler{runner: r})
}
