package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"atsflare-agent/internal/agent"
	"atsflare-agent/internal/config"
	"atsflare-agent/internal/heartbeat"
	"atsflare-agent/internal/httpclient"
	"atsflare-agent/internal/nginx"
	"atsflare-agent/internal/state"
	syncservice "atsflare-agent/internal/sync"
	"atsflare-agent/internal/updater"
)

func main() {
	configPath := flag.String("config", "./agent.json", "agent config path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
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
	log.Printf("agent config loaded: server=%s node=%s ip=%s heartbeat_interval=%s sync_interval=%s route_config=%s cert_dir=%s", cfg.ServerURL, cfg.NodeName, cfg.NodeIP, cfg.HeartbeatInterval, cfg.SyncInterval, cfg.RouteConfigPath, cfg.CertDir)

	client := httpclient.New(cfg.ServerURL, cfg.InitialAuthToken(), cfg.RequestTimeout.Duration())
	stateStore := state.NewStore(cfg.StatePath)
	runtimeManager := &nginx.Manager{
		MainConfigPath:  cfg.MainConfigPath,
		RouteConfigPath: cfg.RouteConfigPath,
		CertDir:         cfg.CertDir,
		NginxCertDir:    cfg.OpenrestyCertDir,
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
	log.Printf("agent process started")

	if err = runner.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
	log.Printf("agent process stopped")
}
