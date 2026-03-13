package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"atsflare-agent/internal/agent"
	"atsflare-agent/internal/config"
	"atsflare-agent/internal/heartbeat"
	"atsflare-agent/internal/httpclient"
	"atsflare-agent/internal/logging"
	"atsflare-agent/internal/nginx"
	"atsflare-agent/internal/state"
	syncservice "atsflare-agent/internal/sync"
	"atsflare-agent/internal/updater"
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
	cfg.NginxVersion = nginx.DetectVersion(
		context.Background(),
		nginx.ExecutorOptions{
			NginxPath:       cfg.OpenrestyPath,
			DockerBinary:    cfg.DockerBinary,
			ContainerName:   cfg.OpenrestyContainerName,
			Image:           cfg.OpenrestyDockerImage,
			MainConfigPath:  cfg.MainConfigPath,
			RouteConfigPath: cfg.RouteConfigPath,
			CertDir:         cfg.CertDir,
			NginxCertDir:    cfg.OpenrestyCertDir,
		},
	)
	slog.Info("agent config loaded",
		"server", cfg.ServerURL,
		"node", cfg.NodeName,
		"ip", cfg.NodeIP,
		"heartbeat_interval", cfg.HeartbeatInterval,
		"route_config", cfg.RouteConfigPath,
		"cert_dir", cfg.CertDir,
	)

	client := httpclient.New(cfg.ServerURL, cfg.InitialAuthToken(), cfg.RequestTimeout.Duration())
	stateStore := state.NewStore(cfg.StatePath)
	runtimeRouteConfigPath := cfg.RouteConfigPath
	if cfg.OpenrestyPath == "" {
		runtimeRouteConfigPath = nginx.DockerRouteConfigPath
	}
	runtimeManager := &nginx.Manager{
		MainConfigPath:         cfg.MainConfigPath,
		RouteConfigPath:        cfg.RouteConfigPath,
		RuntimeRouteConfigPath: runtimeRouteConfigPath,
		CertDir:                cfg.CertDir,
		NginxCertDir:           cfg.OpenrestyCertDir,
		Executor: nginx.NewExecutor(nginx.ExecutorOptions{
			NginxPath:       cfg.OpenrestyPath,
			DockerBinary:    cfg.DockerBinary,
			ContainerName:   cfg.OpenrestyContainerName,
			Image:           cfg.OpenrestyDockerImage,
			MainConfigPath:  cfg.MainConfigPath,
			RouteConfigPath: cfg.RouteConfigPath,
			CertDir:         cfg.CertDir,
			NginxCertDir:    cfg.OpenrestyCertDir,
		}),
	}
	runner := &agent.Runner{
		Config:           cfg,
		StateStore:       stateStore,
		HeartbeatService: heartbeat.New(client),
		SyncService:      syncservice.New(client, runtimeManager, stateStore),
		Updater:          updater.New(),
		RuntimeManager:   runtimeManager,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	slog.Info("agent process started")

	if err = runner.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("agent process exited with error", "error", err)
		os.Exit(1)
	}
	slog.Info("agent process stopped")
}
