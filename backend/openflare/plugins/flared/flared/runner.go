// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package flared implements the tunnel client daemon runtime loop.
package flared

import (
	"context"
	"log/slog"

	"Wavelet/openflare/plugins/flared/config"
	"Wavelet/openflare/plugins/flared/frpc"
	"Wavelet/openflare/plugins/flared/heartbeat"
	"Wavelet/openflare/plugins/flared/httpclient"
	"Wavelet/openflare/plugins/flared/sync"
	"Wavelet/openflare/plugins/flared/wsclient"
	edgerunner "Wavelet/openflare/share/edge/runner"
	"Wavelet/pkg/util"
)

// Runner is the top-level orchestrator for the flared agent. It wires together
// heartbeat, sync, frpc management, and the WebSocket control-plane connection.
type Runner struct {
	Config           *config.Config
	HeartbeatService *heartbeat.Service
	FrpcManager      *frpc.Manager
	SyncService      *sync.Service
	WebSocketService *wsclient.Client
	HTTPClient       *httpclient.Client
}

// Run starts all background services and enters the WebSocket reconnect loop.
// It blocks until ctx is cancelled or an unrecoverable error occurs.
func (r *Runner) Run(ctx context.Context) error {
	util.Go(func() { r.HeartbeatService.Run(ctx) })
	util.Go(func() { r.SyncService.Run(ctx) })

	return edgerunner.RunWSReconnectLoop(ctx, edgerunner.WSReconnectConfig{
		ComponentName: "flared",
		OnShutdown:    r.FrpcManager.Stop,
	}, func(ctx context.Context) (edgerunner.WSConnection, error) {
		return r.WebSocketService.Connect(ctx)
	}, func(ctx context.Context, conn edgerunner.WSConnection) {
		r.handleConnection(ctx, conn)
	})
}

type flaredWSHandler struct {
	runner *Runner
}

func (h *flaredWSHandler) OnConnect(_ context.Context) error {
	return nil
}

func (h *flaredWSHandler) HandleMessage(_ context.Context, msg wsclient.WSMessage) error {
	switch msg.Type {
	case "active_config":
		slog.Info("received config update notification from server")
		h.runner.SyncService.Trigger()
	default:
		slog.Debug("ignored unknown ws message type", "type", msg.Type)
	}
	return nil
}

func (h *flaredWSHandler) OnClose(err error) {
	slog.Error("flared ws receive failed", "error", err)
}

func (r *Runner) handleConnection(ctx context.Context, conn edgerunner.WSConnection) {
	wsConn, ok := conn.(*wsclient.Connection)
	if !ok {
		slog.Error("flared ws connection has unexpected type")
		return
	}
	_ = wsConn.RunReceiveLoop(ctx, &flaredWSHandler{runner: r})
}
