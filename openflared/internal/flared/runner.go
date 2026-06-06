package flared

import (
	"context"
	"log/slog"
	"time"

	"github.com/rain-kl/openflare/openflared/internal/config"
	"github.com/rain-kl/openflare/openflared/internal/frpc"
	"github.com/rain-kl/openflare/openflared/internal/heartbeat"
	"github.com/rain-kl/openflare/openflared/internal/httpclient"
	"github.com/rain-kl/openflare/openflared/internal/sync"
	"github.com/rain-kl/openflare/openflared/internal/wsclient"
)

type Runner struct {
	Config           *config.Config
	HeartbeatService *heartbeat.Service
	FrpcManager      *frpc.Manager
	SyncService      *sync.Service
	WebSocketService *wsclient.Client
	HttpClient       *httpclient.Client
}

func (r *Runner) Run(ctx context.Context) error {
	// Start background services
	go r.HeartbeatService.Run(ctx)
	go r.SyncService.Run(ctx)

	// WebSocket reconnection loop
	for {
		select {
		case <-ctx.Done():
			r.FrpcManager.Stop()
			return ctx.Err()
		default:
		}

		conn, err := r.WebSocketService.Connect(ctx)
		if err != nil {
			slog.Error("flared ws connect failed, will retry", "error", err)
			r.sleepContext(ctx, 5*time.Second)
			continue
		}

		r.handleConnection(ctx, conn)
		_ = conn.Close()
		slog.Info("flared ws connection closed, reconnecting...")
		r.sleepContext(ctx, 2*time.Second)
	}
}

type flaredWSHandler struct {
	runner *Runner
}

func (h *flaredWSHandler) OnConnect(ctx context.Context) error {
	return nil
}

func (h *flaredWSHandler) HandleMessage(ctx context.Context, msg wsclient.WSMessage) error {
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

func (r *Runner) handleConnection(ctx context.Context, conn *wsclient.Connection) {
	_ = conn.RunReceiveLoop(ctx, &flaredWSHandler{runner: r})
}

func (r *Runner) sleepContext(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
