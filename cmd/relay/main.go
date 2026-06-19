package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	edgelogging "github.com/Rain-kl/Wavelet/internal/apps/edge/logging"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/config"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/frps"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/relay"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/state"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/wsclient"
)

func main() {
	edgelogging.Setup(edgelogging.Options{})

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
