package flared

import (
	"context"
	"log/slog"

	edgerunner "github.com/Rain-kl/Wavelet/internal/apps/edge/runner"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/config"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/frpc"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/sync"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/wsclient"
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
	go r.HeartbeatService.Run(ctx)
	go r.SyncService.Run(ctx)

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

func (r *Runner) handleConnection(ctx context.Context, conn edgerunner.WSConnection) {
	wsConn, ok := conn.(*wsclient.Connection)
	if !ok {
		slog.Error("flared ws connection has unexpected type")
		return
	}
	_ = wsConn.RunReceiveLoop(ctx, &flaredWSHandler{runner: r})
}