package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Rain-kl/Wavelet/internal/relay/config"
	"github.com/Rain-kl/Wavelet/internal/relay/frps"
	"github.com/Rain-kl/Wavelet/internal/relay/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/relay/httpclient"
	"github.com/Rain-kl/Wavelet/internal/relay/relay"
	"github.com/Rain-kl/Wavelet/internal/relay/state"
	"github.com/Rain-kl/Wavelet/internal/relay/wsclient"
)

func main() {
	// Setup simple structured logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(os.Getenv("LOG_LEVEL")),
	})))

	configPath := flag.String("config", "./relay.json", "relay config path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load relay config failed", "error", err)
		os.Exit(1)
	}

	slog.Info("relay config loaded",
		"server", cfg.ServerURL,
		"node", cfg.NodeName,
		"ip", cfg.NodeIP,
		"frps_path", cfg.FrpsPath,
		"data_dir", cfg.DataDir,
		"heartbeat_interval", cfg.HeartbeatInterval,
	)

	stateStore := state.NewStore(cfg.StatePath)
	_ = stateStore // In the future we may use stateStore for auth caching

	frpsManager := frps.NewManager(cfg.FrpsPath, cfg.DataDir, cfg.InitialAuthToken())

	slog.Info("detected frps version", "version", frpsManager.GetVersion())

	httpClient := httpclient.New(cfg.ServerURL, cfg.InitialAuthToken(), cfg.RequestTimeout.Duration())
	wsClient := wsclient.New(cfg.ServerURL, cfg.InitialAuthToken(), cfg.RequestTimeout.Duration())

	runner := &relay.Runner{
		Config:           cfg,
		StateStore:       stateStore,
		FrpsManager:      frpsManager,
		HttpClient:       httpClient,
		WebSocketService: wsClient,
		HeartbeatService: heartbeat.New(httpClient, frpsManager, cfg, stateStore),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("relay process started")

	if err := runner.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("relay process exited with error", "error", err)
		os.Exit(1)
	}
	slog.Info("relay process stopped")
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
