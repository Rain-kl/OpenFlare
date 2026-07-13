// Command agent runs the OpenFlare edge agent daemon.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/geoipupdate"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/logging"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/nginx"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/runtimeuser"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	syncservice "github.com/Rain-kl/Wavelet/internal/apps/agent/sync"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/updater"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/wsclient"
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
	if err = runtimeuser.EnsureProcessUser(); err != nil {
		slog.Error("ensure runtime user failed", "error", err)
		os.Exit(1)
	}
	if err = runtimeuser.EnsurePathOwnership(cfg.DataDir, runtimeuser.DefaultDirPerm, runtimeuser.DefaultFilePerm); err != nil {
		slog.Error("ensure data dir ownership failed", "error", err, "data_dir", cfg.DataDir)
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
		"city_mmdb_path", cfg.CityMMDBPath,
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
		MMDBPath:                     cfg.MMDBPath,
		CityMMDBPath:                 cfg.CityMMDBPath,
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
	heartbeatService := heartbeat.New(client)
	updateService := updater.New()
	runner := &agent.Runner{
		Config:     cfg,
		StateStore: stateStore,
		HeartbeatCycle: &heartbeat.Cycle{
			Config:              cfg,
			StateStore:          stateStore,
			ObservabilityBuffer: observabilityBuffer,
			Heartbeat:           heartbeatService,
			Sync:                syncService,
			Updater:             updateService,
		},
		HeartbeatService: heartbeatService,
		SyncService:      syncService,
		RuntimeManager:   runtimeManager,
		WebSocketService: wsClient,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	geoIPUpdater := newGeoIPUpdater(cfg)
	if err = geoIPUpdater.EnsureInitialDatabases(ctx); err != nil {
		slog.Warn("failed to prepare GeoIP databases before agent startup", "error", err)
	}
	go geoIPUpdater.Run(ctx)
	slog.Info("agent process started")

	if err = runner.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("agent process exited with error", "error", err)
		stop()
		os.Exit(1)
	}
	stop()
	slog.Info("agent process stopped")
}

func newGeoIPUpdater(cfg *config.Config) *geoipupdate.Updater {
	return &geoipupdate.Updater{
		MMDBPath:        cfg.MMDBPath,
		DownloadURL:     cfg.MMDBDownloadURL,
		CityMMDBPath:    cfg.CityMMDBPath,
		CityDownloadURL: cfg.CityMMDBDownloadURL,
		UpdateInterval:  cfg.MMDBUpdateInterval.Duration(),
	}
}
