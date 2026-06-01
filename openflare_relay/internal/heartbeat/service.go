package heartbeat

import (
	"context"
	"log/slog"
	"time"

	"openflare-relay/internal/config"
	"openflare-relay/internal/frps"
	"openflare-relay/internal/httpclient"
	"openflare-relay/internal/observability"
	"openflare-relay/internal/state"
	"openflare/service"
)

type Service struct {
	client      *httpclient.Client
	frpsManager *frps.Manager
	config      *config.Config
	stateStore  *state.Store
}

func New(client *httpclient.Client, manager *frps.Manager, cfg *config.Config, stateStore *state.Store) *Service {
	return &Service{
		client:      client,
		frpsManager: manager,
		config:      cfg,
		stateStore:  stateStore,
	}
}

func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(s.config.HeartbeatInterval.Duration())
	defer ticker.Stop()

	// initial heartbeat
	s.doHeartbeat(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.doHeartbeat(ctx)
		}
	}
}

func (s *Service) doHeartbeat(ctx context.Context) {
	slog.Debug("sending heartbeat")

	runtimeStatus := s.frpsManager.GetRuntimeStatus()
	payload := service.RelayHeartbeatPayload{
		Version:        config.Version,
		ExtVersion:     s.frpsManager.GetVersion(),
		RelayStatus:    runtimeStatus.Status,
		FrpsConnCount:  runtimeStatus.Connections,
		FrpsProxyCount: runtimeStatus.ProxyCount,
		Name:           s.config.NodeName,
		IP:             s.config.NodeIP,
		Profile:        observability.BuildProfile(s.config, s.stateStore),
		Snapshot:       observability.BuildSnapshot(s.config, s.stateStore),
		HealthEvents:   observability.BuildHealthEvents(runtimeStatus),
	}

	resp, err := s.client.Heartbeat(ctx, payload)
	if err != nil {
		slog.Error("heartbeat failed", "error", err)
		return
	}
	slog.Debug("heartbeat succeeded")

	// Update configs if changed
	s.frpsManager.UpdateConfig(resp.RelayConfig)
}
