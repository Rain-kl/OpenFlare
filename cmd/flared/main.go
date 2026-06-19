package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	edgelogging "github.com/Rain-kl/Wavelet/internal/apps/edge/logging"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/config"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/flared"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/frpc"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/sync"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/wsclient"
)

func main() {
	edgelogging.Setup(edgelogging.Options{})

	configPath := flag.String("config", "./flared.json", "flared config path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load flared config failed", "error", err)
		os.Exit(1)
	}

	slog.Info("flared config loaded",
		"server", cfg.ServerURL,
		"frpc_path", cfg.FrpcPath,
		"data_dir", cfg.DataDir,
		"heartbeat_interval", cfg.HeartbeatInterval,
		"sync_interval", cfg.SyncInterval,
	)

	frpcManager := frpc.NewManager(cfg)
	_ = frpcManager.LoadState()

	slog.Info("detected frpc version", "version", frpcManager.GetVersion())

	httpClient := httpclient.New(cfg.ServerURL, cfg.InitialAuthToken(), cfg.RequestTimeout.Duration())
	wsClient := wsclient.New(cfg.ServerURL, cfg.InitialAuthToken(), cfg.RequestTimeout.Duration())

	syncService := sync.New(httpClient, frpcManager, cfg)
	heartbeatService := heartbeat.New(httpClient, frpcManager, cfg)

	runner := &flared.Runner{
		Config:           cfg,
		FrpcManager:      frpcManager,
		HttpClient:       httpClient,
		WebSocketService: wsClient,
		HeartbeatService: heartbeatService,
		SyncService:      syncService,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("flared process started")

	if err := runner.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("flared process exited with error", "error", err)
		os.Exit(1)
	}
	slog.Info("flared process stopped")
}
