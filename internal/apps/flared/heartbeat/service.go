package heartbeat

import (
	"context"
	"log/slog"

	edgeheartbeat "github.com/Rain-kl/Wavelet/internal/apps/edge/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/edge/nodeip"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/config"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/frpc"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/flared/updater"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

type Service struct {
	client      *httpclient.Client
	frpcManager *frpc.Manager
	config      *config.Config
	updater     *updater.Service
}

func New(client *httpclient.Client, manager *frpc.Manager, cfg *config.Config) *Service {
	return &Service{
		client:      client,
		frpcManager: manager,
		config:      cfg,
		updater:     updater.New(),
	}
}

func (s *Service) Run(ctx context.Context) {
	edgeheartbeat.RunLoop(ctx, s.config.HeartbeatInterval.Duration(), s.doHeartbeat)
}

func (s *Service) doHeartbeat(ctx context.Context) {
	slog.Debug("sending flared heartbeat")

	payload := service.FlaredHeartbeatPayload{
		ClientVersion:   config.Version,
		FrpVersion:      s.frpcManager.GetVersion(),
		IP:              nodeip.Detect(),
		TunnelStatus:    "running",
		ConnectedRelays: s.frpcManager.GetConnectedRelays(),
		CurrentVersion:  s.frpcManager.GetCurrentConfigVersion(),
		CurrentChecksum: s.frpcManager.GetCurrentConfigChecksum(),
	}

	resp, err := s.client.Heartbeat(ctx, payload)
	if err != nil {
		slog.Error("flared heartbeat failed", "error", err)
		return
	}
	slog.Debug("flared heartbeat succeeded")

	if resp != nil && resp.TunnelSettings != nil {
		edgeheartbeat.TryAutoUpdate(ctx, s.updater, tunnelSettingsToAutoUpdate(resp.TunnelSettings), "flared")
	}
}

func tunnelSettingsToAutoUpdate(settings *service.RelaySettings) *edgeheartbeat.AutoUpdateSettings {
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