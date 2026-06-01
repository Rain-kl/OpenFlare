package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"openflare-flared/internal/config"
	"openflare-flared/internal/flared"
	"openflare-flared/internal/frpc"
	"openflare-flared/internal/heartbeat"
	"openflare-flared/internal/httpclient"
	"openflare-flared/internal/sync"
	"openflare-flared/internal/wsclient"
)

func main() {
	// Setup simple structured logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(os.Getenv("LOG_LEVEL")),
	})))

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

func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
