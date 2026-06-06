package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rain-kl/openflare/openflare-agent/internal/agent"
	"github.com/rain-kl/openflare/openflare-agent/internal/config"
	"github.com/rain-kl/openflare/openflare-agent/internal/geoipupdate"
	"github.com/rain-kl/openflare/openflare-agent/internal/heartbeat"
	"github.com/rain-kl/openflare/openflare-agent/internal/httpclient"
	"github.com/rain-kl/openflare/openflare-agent/internal/logging"
	"github.com/rain-kl/openflare/openflare-agent/internal/nginx"
	"github.com/rain-kl/openflare/openflare-agent/internal/state"
	syncservice "github.com/rain-kl/openflare/openflare-agent/internal/sync"
	"github.com/rain-kl/openflare/openflare-agent/internal/updater"
	"github.com/rain-kl/openflare/openflare-agent/internal/wsclient"
)

func main() {
	logging.Setup()

	configPath := flag.String("config", "./agent.json", "agent config path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load agent config failed", "error", err)
		os.Exit(1)
	}
	cfg.ExtVersion = nginx.DetectVersion(
		context.Background(),
		nginx.ExecutorOptions{
			NginxPath:                  cfg.OpenrestyPath,
			MainConfigPath:             cfg.MainConfigPath,
			RouteConfigPath:            cfg.RouteConfigPath,
			CertDir:                    cfg.CertDir,
			NginxCertDir:               cfg.OpenrestyCertDir,
			LuaDir:                     cfg.LuaDir,
			NginxLuaDir:                cfg.OpenrestyLuaDir,
			OpenrestyObservabilityPort: cfg.OpenrestyObservabilityPort,
		},
	)
	slog.Info("agent config loaded",
		"server", cfg.ServerURL,
		"node", cfg.NodeName,
		"ip", cfg.NodeIP,
		"heartbeat_interval", cfg.HeartbeatInterval,
		"route_config", cfg.RouteConfigPath,
		"access_log", cfg.AccessLogPath,
		"cert_dir", cfg.CertDir,
		"lua_dir", cfg.LuaDir,
		"runtime_config_dir", cfg.RuntimeConfigDir,
		"mmdb_path", cfg.MMDBPath,
	)

	client := httpclient.New(cfg.ServerURL, cfg.InitialAuthToken(), cfg.RequestTimeout.Duration())
	wsClient := wsclient.New(cfg.ServerURL, cfg.InitialAuthToken(), cfg.RequestTimeout.Duration())
	stateStore := state.NewStore(cfg.StatePath)
	observabilityBuffer := state.NewObservabilityBufferStore(cfg.ObservabilityBufferPath)
	runtimeManager := &nginx.Manager{
		MainConfigPath:               cfg.MainConfigPath,
		RouteConfigPath:              cfg.RouteConfigPath,
		AccessLogPath:                cfg.AccessLogPath,
		CertDir:                      cfg.CertDir,
		NginxCertDir:                 cfg.OpenrestyCertDir,
		LuaDir:                       cfg.LuaDir,
		NginxLuaDir:                  cfg.OpenrestyLuaDir,
		RuntimeConfigDir:             cfg.RuntimeConfigDir,
		PagesDir:                     cfg.PagesDir,
		OpenrestyObservabilityListen: nginx.ObservabilityListenAddress(cfg.OpenrestyObservabilityPort),
		OpenrestyObservabilityPort:   cfg.OpenrestyObservabilityPort,
		OpenrestyResolverDirective:   "",
		Executor: nginx.NewExecutor(nginx.ExecutorOptions{
			NginxPath:                  cfg.OpenrestyPath,
			MainConfigPath:             cfg.MainConfigPath,
			RouteConfigPath:            cfg.RouteConfigPath,
			CertDir:                    cfg.CertDir,
			NginxCertDir:               cfg.OpenrestyCertDir,
			LuaDir:                     cfg.LuaDir,
			NginxLuaDir:                cfg.OpenrestyLuaDir,
			OpenrestyObservabilityPort: cfg.OpenrestyObservabilityPort,
		}),
	}
	if err = runtimeManager.EnsureLuaAssets(); err != nil {
		slog.Error("ensure managed lua assets failed", "error", err)
		os.Exit(1)
	}
	syncService := syncservice.New(client, runtimeManager, stateStore)
	syncService.SetPagesDir(cfg.PagesDir)
	runner := &agent.Runner{
		Config:              cfg,
		StateStore:          stateStore,
		ObservabilityBuffer: observabilityBuffer,
		HeartbeatService:    heartbeat.New(client),
		SyncService:         syncService,
		Updater:             updater.New(),
		RuntimeManager:      runtimeManager,
		WebSocketService:    wsClient,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	geoIPUpdater := &geoipupdate.Updater{
		MMDBPath:       cfg.MMDBPath,
		DownloadURL:    cfg.MMDBDownloadURL,
		UpdateInterval: cfg.MMDBUpdateInterval.Duration(),
	}
	go geoIPUpdater.Run(ctx)
	slog.Info("agent process started")

	if err = runner.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("agent process exited with error", "error", err)
		os.Exit(1)
	}
	slog.Info("agent process stopped")
}
