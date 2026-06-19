package relay

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Rain-kl/Wavelet/internal/relay/config"
	"github.com/Rain-kl/Wavelet/internal/relay/frps"
	"github.com/Rain-kl/Wavelet/internal/relay/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/relay/httpclient"
	"github.com/Rain-kl/Wavelet/internal/relay/state"
	"github.com/Rain-kl/Wavelet/internal/relay/wsclient"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

type Runner struct {
	Config           *config.Config
	StateStore       *state.Store
	HeartbeatService *heartbeat.Service
	FrpsManager      *frps.Manager
	WebSocketService *wsclient.Client
	HttpClient       *httpclient.Client
}

func (r *Runner) Run(ctx context.Context) error {
	// Start heartbeat loop in background
	go r.HeartbeatService.Run(ctx)

	// WebSocket reconnection loop
	for {
		select {
		case <-ctx.Done():
			r.FrpsManager.Stop()
			return ctx.Err()
		default:
		}

		conn, err := r.WebSocketService.Connect(ctx)
		if err != nil {
			slog.Error("relay ws connect failed, will retry", "error", err)
			r.sleepContext(ctx, 5*time.Second)
			continue
		}

		r.handleConnection(ctx, conn)
		_ = conn.Close()
		slog.Info("relay ws connection closed, reconnecting...")
		r.sleepContext(ctx, 2*time.Second)
	}
}

type relayWSHandler struct {
	runner *Runner
}

func (h *relayWSHandler) OnConnect(ctx context.Context) error {
	return nil
}

func (h *relayWSHandler) HandleMessage(ctx context.Context, msg wsclient.WSMessage) error {
	switch msg.Type {
	case "relay_config":
		var cfg service.RelayConfig
		if err := json.Unmarshal(msg.Payload, &cfg); err != nil {
			slog.Error("failed to unmarshal relay_config", "error", err)
			return nil
		}
		h.runner.FrpsManager.UpdateConfig(&cfg)
	default:
		slog.Debug("ignored unknown ws message type", "type", msg.Type)
	}
	return nil
}

func (h *relayWSHandler) OnClose(err error) {
	slog.Error("relay ws receive failed", "error", err)
}

func (r *Runner) handleConnection(ctx context.Context, conn *wsclient.Connection) {
	_ = conn.RunReceiveLoop(ctx, &relayWSHandler{runner: r})
}

func (r *Runner) sleepContext(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
