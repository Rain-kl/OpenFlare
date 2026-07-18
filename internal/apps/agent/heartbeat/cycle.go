// Package heartbeat implements the periodic heartbeat cycle executed by the agent,
// including payload preparation, config sync, WAF IP group application, and observability buffering.
package heartbeat

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/observability"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/updater"
	edgeheartbeat "github.com/Rain-kl/Wavelet/internal/apps/edge/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/edge/nodeip"
)

// SyncService is the interface used by Cycle to sync active configuration and WAF IP groups.
type SyncService interface {
	SyncOnStartup(ctx context.Context, target *protocol.ActiveConfigMeta) error
	SyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error
	WAFIPGroupChecksums() (map[string]string, error)
	ApplyWAFIPGroups(ctx context.Context, groups []protocol.WAFIPGroup) error
}

// SettingsApplier is the interface used by Cycle to apply agent settings received from the server.
type SettingsApplier interface {
	Apply(settings *protocol.AgentSettings) (intervalChanged bool)
	RestartOpenrestyIfNeeded(ctx context.Context)
}

// Cycle holds the dependencies required to execute a single agent heartbeat cycle.
type Cycle struct {
	Config              *config.Config
	StateStore          *state.Store
	ObservabilityBuffer *state.ObservabilityBufferStore
	Heartbeat           API
	Sync                SyncService
	Updater             *updater.Service
	RecordSyncError     func(err error)
}

// Perform executes one complete heartbeat cycle: sends the heartbeat, syncs config, and applies settings.
func (c *Cycle) Perform(ctx context.Context, nodeID string, startup bool, settings SettingsApplier) (bool, error) {
	payload, ackWindows := c.PrepareHeartbeatPayload(ctx, nodeID)
	heartbeatResult, err := c.Heartbeat.Heartbeat(ctx, payload)
	if err != nil {
		return false, err
	}
	c.AckObservabilityWindows(ackWindows)
	if heartbeatResult == nil {
		heartbeatResult = &protocol.HeartbeatResult{}
	}
	mode := "periodic"
	if startup {
		mode = "startup"
	}
	slog.Debug("agent heartbeat succeeded", "mode", mode, "node_id", nodeID)

	var changed bool
	if settings != nil {
		changed = settings.Apply(heartbeatResult.AgentSettings)
	}
	c.ApplyWAFIPGroups(ctx, heartbeatResult.WAFIPGroups)
	if startup {
		if err = c.Sync.SyncOnStartup(ctx, heartbeatResult.ActiveConfig); err != nil {
			c.recordSyncError(err)
			slog.Error("agent startup sync failed", "error", err)
		} else {
			slog.Debug("agent startup sync completed")
		}
	} else if err = c.Sync.SyncOnce(ctx, heartbeatResult.ActiveConfig); err != nil {
		c.recordSyncError(err)
		slog.Error("agent sync failed", "error", err)
	}
	if settings != nil {
		settings.RestartOpenrestyIfNeeded(ctx)
	}
	edgeheartbeat.TryAutoUpdate(ctx, c.Updater, agentSettingsToAutoUpdate(heartbeatResult.AgentSettings), "agent")
	return changed, nil
}

// NodePayload builds and returns the full NodePayload to be sent in a heartbeat request.
func (c *Cycle) NodePayload(ctx context.Context, nodeID string) protocol.NodePayload {
	snapshot, _ := c.StateStore.Load()
	openrestyStatus := strings.TrimSpace(snapshot.OpenrestyStatus)
	if openrestyStatus == "" {
		openrestyStatus = protocol.OpenrestyStatusUnknown
	}
	profile := observability.BuildProfile(c.Config, c.StateStore)
	edgeSnapshot := observability.CollectEdgeHealth(ctx, c.Config)
	accessLogs := observability.CollectAccessLogs(c.Config, c.StateStore)
	metricSnapshot := observability.BuildSnapshot(c.Config, c.StateStore)
	edgeHealth := observability.BuildEdgeHealth(edgeSnapshot, openrestyStatus, snapshot.OpenrestyMessage)
	healthEvents := observability.BuildHealthEvents(snapshot)

	ip := c.Config.NodeIP
	if !c.Config.NodeIPConfigured {
		ip = nodeip.DetectWithContext(ctx)
	}

	payload := protocol.NodePayload{
		SchemaVersion:    2,
		NodeID:           nodeID,
		Name:             c.Config.NodeName,
		IP:               ip,
		Version:          c.Config.Version,
		ExtVersion:       c.Config.ExtVersion,
		CurrentVersion:   snapshot.CurrentVersion,
		LastError:        snapshot.LastError,
		OpenrestyStatus:  openrestyStatus,
		OpenrestyMessage: snapshot.OpenrestyMessage,
		Profile:          profile,
		HostMetrics:      metricSnapshot,
		EdgeHealth:       edgeHealth,
		AccessLogs:       accessLogs,
		HealthEvents:     healthEvents,
	}
	if c.Sync != nil {
		checksums, err := c.Sync.WAFIPGroupChecksums()
		if err != nil {
			slog.Debug("load local waf ip group checksums failed", "error", err)
		} else if len(checksums) > 0 {
			payload.WAFIPGroupChecksums = checksums
		}
	}
	return payload
}

// PrepareHeartbeatPayload constructs the heartbeat payload with buffered observability records and returns the window timestamps to acknowledge.
func (c *Cycle) PrepareHeartbeatPayload(ctx context.Context, nodeID string) (protocol.NodePayload, []int64) {
	payload := c.NodePayload(ctx, nodeID)
	if c.ObservabilityBuffer == nil || (payload.HostMetrics == nil && payload.EdgeHealth == nil && len(payload.AccessLogs) == 0) {
		return payload, nil
	}
	now := time.Now().UTC()
	retainAfterUnix := now.Add(-time.Duration(c.Config.ObservabilityReplayMinutes) * time.Minute).Unix()
	windowStartedAtUnix := state.ObservabilityWindowStartedAt(payload.HostMetrics, payload.EdgeHealth)
	if windowStartedAtUnix <= 0 {
		return payload, nil
	}

	record := state.ObservabilityBufferRecord{
		WindowStartedAtUnix: windowStartedAtUnix,
		HostMetrics:         payload.HostMetrics,
		EdgeHealth:          payload.EdgeHealth,
		AccessLogs:          payload.AccessLogs,
		QueuedAtUnix:        now.Unix(),
	}
	if err := c.ObservabilityBuffer.Upsert(record, retainAfterUnix); err != nil {
		slog.Error("upsert observability buffer failed", "error", err)
		return payload, nil
	}

	records, err := c.ObservabilityBuffer.Replayable(windowStartedAtUnix, retainAfterUnix)
	if err != nil {
		slog.Error("load replayable observability buffer failed", "error", err)
		return payload, []int64{windowStartedAtUnix}
	}

	ackWindows := make([]int64, 0, len(records)+1)
	buffered := make([]protocol.BufferedObservabilityRecord, 0, len(records))
	for _, item := range records {
		if item.WindowStartedAtUnix <= 0 {
			continue
		}
		buffered = append(buffered, protocol.BufferedObservabilityRecord{
			CapturedAtUnix: item.WindowStartedAtUnix,
			HostMetrics:    item.HostMetrics,
			EdgeHealth:     item.EdgeHealth,
			AccessLogs:     item.AccessLogs,
		})
		ackWindows = append(ackWindows, item.WindowStartedAtUnix)
	}
	payload.Buffered = buffered
	ackWindows = append(ackWindows, windowStartedAtUnix)
	return payload, ackWindows
}

// AckObservabilityWindows acknowledges the given observability window timestamps in the buffer store.
func (c *Cycle) AckObservabilityWindows(windowStartedAtUnix []int64) {
	if c.ObservabilityBuffer == nil || len(windowStartedAtUnix) == 0 {
		return
	}
	retainAfterUnix := time.Now().UTC().Add(-time.Duration(c.Config.ObservabilityReplayMinutes) * time.Minute).Unix()
	if err := c.ObservabilityBuffer.Ack(windowStartedAtUnix, retainAfterUnix); err != nil {
		slog.Error("ack observability buffer failed", "error", err)
	}
}

// ApplyWAFIPGroups applies the WAF IP groups received from the server via the SyncService.
func (c *Cycle) ApplyWAFIPGroups(ctx context.Context, groups []protocol.WAFIPGroup) {
	if len(groups) == 0 || c.Sync == nil {
		return
	}
	if err := c.Sync.ApplyWAFIPGroups(ctx, groups); err != nil {
		c.recordSyncError(err)
		slog.Error("agent apply waf ip groups failed", "error", err)
	}
}

func (c *Cycle) recordSyncError(err error) {
	if c.RecordSyncError != nil {
		c.RecordSyncError(err)
	}
}

// AgentSettingsToAutoUpdate converts AgentSettings to an AutoUpdateSettings value used by the edge heartbeat updater.
func AgentSettingsToAutoUpdate(settings *protocol.AgentSettings) *edgeheartbeat.AutoUpdateSettings {
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

func agentSettingsToAutoUpdate(settings *protocol.AgentSettings) *edgeheartbeat.AutoUpdateSettings {
	return AgentSettingsToAutoUpdate(settings)
}
