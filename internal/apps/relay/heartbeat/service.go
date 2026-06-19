package heartbeat

import (
	"context"
	"log/slog"

	"github.com/Rain-kl/Wavelet/internal/apps/relay/config"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/frps"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/observability"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/state"
	edgeheartbeat "github.com/Rain-kl/Wavelet/internal/apps/edge/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/updater"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
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
	edgeheartbeat.RunLoop(ctx, s.config.HeartbeatInterval.Duration(), s.doHeartbeat)
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
		edgeheartbeat.TryAutoUpdate(ctx, s.updater, relaySettingsToAutoUpdate(resp.RelaySettings), "relay")
	}
}

func relaySettingsToAutoUpdate(settings *service.RelaySettings) *edgeheartbeat.AutoUpdateSettings {
	if settings == nil {
		return nil
	}
	return &edgeheartbeat.AutoUpdateSettings{
		AutoUpdate:    settings.AutoUpdate,
		UpdateNow:     settings.UpdateNow,
		UpdateRepo:    settings.UpdateRepo,
		UpdateChannel: settings.UpdateChannel,
		UpdateTag:     settings.UpdateTag,
	}
}
