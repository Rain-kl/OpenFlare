// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package heartbeat sends periodic relay node status to the control plane.
package heartbeat

import (
	"context"
	"log/slog"

	edgeheartbeat "github.com/Rain-kl/Wavelet/internal/apps/edge/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/edge/nodeip"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/config"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/frps"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/httpclient"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/observability"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/state"
	"github.com/Rain-kl/Wavelet/internal/apps/relay/updater"
	service "github.com/Rain-kl/Wavelet/pkg/protocol"
)

// Service sends periodic heartbeat payloads to the server, updates the frps
// configuration from the server response, and triggers auto-update checks.
type Service struct {
	client      *httpclient.Client
	frpsManager *frps.Manager
	config      *config.Config
	stateStore  *state.Store
	updater     *updater.Service
}

// New constructs a Service using the provided HTTP client, frps manager,
// configuration, and persistent state store.
func New(client *httpclient.Client, manager *frps.Manager, cfg *config.Config, stateStore *state.Store) *Service {
	return &Service{
		client:      client,
		frpsManager: manager,
		config:      cfg,
		stateStore:  stateStore,
		updater:     updater.New(),
	}
}

// Run starts the heartbeat loop and blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	edgeheartbeat.RunLoop(ctx, s.config.HeartbeatInterval.Duration(), s.doHeartbeat)
}

func (s *Service) doHeartbeat(ctx context.Context) {
	slog.Debug("sending heartbeat")

	runtimeStatus := s.frpsManager.GetRuntimeStatus()

	ip := s.config.NodeIP
	if !s.config.NodeIPConfigured {
		ip = nodeip.DetectWithContext(ctx)
	}

	payload := service.RelayHeartbeatPayload{
		Version:         config.Version,
		ExtVersion:      s.frpsManager.GetVersion(ctx),
		RelayStatus:     runtimeStatus.Status,
		FrpsConnCount:   runtimeStatus.Connections,
		FrpsProxyCount:  runtimeStatus.ProxyCount,
		FrpsClientCount: runtimeStatus.ClientCount,
		FrpsProxies:     runtimeStatus.Proxies,
		Name:            s.config.NodeName,
		IP:              ip,
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
	if resp != nil {
		s.frpsManager.UpdateConfig(ctx, resp.RelayConfig)

		if resp.RelaySettings != nil {
			edgeheartbeat.TryAutoUpdate(ctx, s.updater, relaySettingsToAutoUpdate(resp.RelaySettings), "relay")
		}
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
