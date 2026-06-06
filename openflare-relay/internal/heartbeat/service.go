package heartbeat

import (
	"context"
	"log/slog"
	"time"

	"github.com/rain-kl/openflare/openflare-relay/internal/config"
	"github.com/rain-kl/openflare/openflare-relay/internal/frps"
	"github.com/rain-kl/openflare/openflare-relay/internal/httpclient"
	"github.com/rain-kl/openflare/openflare-relay/internal/observability"
	"github.com/rain-kl/openflare/openflare-relay/internal/state"
	"github.com/rain-kl/openflare/openflare-relay/internal/updater"
	"github.com/rain-kl/openflare/openflare-server/service"
)

type Service struct {
	client      *httpclient.Client
	frpsManager *frps.Manager
	config      *config.Config
	stateStore  *state.Store
	updater     *updater.Service
}

func New(client *httpclient.Client, manager *frps.Manager, cfg *config.Config, stateStore *state.Store) *Service {
	return &Service{
		client:      client,
		frpsManager: manager,
		config:      cfg,
		stateStore:  stateStore,
		updater:     updater.New(),
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
		Version:         config.Version,
		ExtVersion:      s.frpsManager.GetVersion(),
		RelayStatus:     runtimeStatus.Status,
		FrpsConnCount:   runtimeStatus.Connections,
		FrpsProxyCount:  runtimeStatus.ProxyCount,
		FrpsClientCount: runtimeStatus.ClientCount,
		FrpsProxies:     runtimeStatus.Proxies,
		Name:            s.config.NodeName,
		IP:              s.config.NodeIP,
		Profile:         observability.BuildProfile(s.config, s.stateStore),
		Snapshot:        observability.BuildSnapshot(s.config, s.stateStore),
		HealthEvents:    observability.BuildHealthEvents(runtimeStatus),
	}

	resp, err := s.client.Heartbeat(ctx, payload)
	if err != nil {
		slog.Error("heartbeat failed", "error", err)
		return
	}
	slog.Debug("heartbeat succeeded")

	// Update configs if changed
	s.frpsManager.UpdateConfig(resp.RelayConfig)

	if resp != nil && resp.RelaySettings != nil {
		s.tryAutoUpdate(ctx, resp.RelaySettings)
	}
}

func (s *Service) tryAutoUpdate(ctx context.Context, settings *service.RelaySettings) {
	if settings == nil || s.updater == nil {
		return
	}
	force := settings.UpdateNow
	shouldCheck := settings.AutoUpdate || force
	if !shouldCheck || settings.UpdateRepo == "" {
		return
	}
	channel := "stable"
	if force && settings.UpdateChannel != "" {
		channel = settings.UpdateChannel
	}
	slog.Info("checking for relay updates", "repo", settings.UpdateRepo, "channel", channel, "force", force)
	err := s.updater.CheckAndUpdate(ctx, settings.UpdateRepo, updater.UpdateOptions{
		Channel: channel,
		TagName: settings.UpdateTag,
		Force:   force,
	})
	if err != nil {
		slog.Error("relay update check failed", "error", err)
	}
}
