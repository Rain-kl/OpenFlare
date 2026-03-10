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
)

func main() {
	configPath := flag.String("config", "./agent.json", "agent config path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	client := httpclient.New(cfg.ServerURL, cfg.AgentToken, cfg.RequestTimeout)
	stateStore := state.NewStore(cfg.StatePath)
	runner := &agent.Runner{
		Config:           cfg,
		StateStore:       stateStore,
		HeartbeatService: heartbeat.New(client),
		SyncService: syncservice.New(client, &nginx.Manager{
			RouteConfigPath: cfg.RouteConfigPath,
			CertDir:         cfg.CertDir,
			NginxCertDir:    cfg.NginxCertDir,
			Executor: nginx.NewExecutor(nginx.ExecutorOptions{
				NginxPath:       cfg.NginxPath,
				DockerBinary:    cfg.DockerBinary,
				ContainerName:   cfg.NginxContainerName,
				Image:           cfg.NginxDockerImage,
				RouteConfigPath: cfg.RouteConfigPath,
				CertDir:         cfg.CertDir,
				NginxCertDir:    cfg.NginxCertDir,
			}),
		}, stateStore),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err = runner.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
