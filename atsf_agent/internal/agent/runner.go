package agent

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"atsflare-agent/internal/config"
	"atsflare-agent/internal/observability"
	"atsflare-agent/internal/protocol"
	"atsflare-agent/internal/state"
)

type HeartbeatService interface {
	Register(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error)
	Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error)
	SetToken(token string)
}

type SyncService interface {
	SyncOnStartup(ctx context.Context, target *protocol.ActiveConfigMeta) error
	SyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error
}

type Updater interface {
	CheckAndUpdate(ctx context.Context, repo string, options UpdateOptions) error
}

type RuntimeManager interface {
	CheckHealth(ctx context.Context) error
	Restart(ctx context.Context) error
}

type UpdateOptions struct {
	Channel string
	TagName string
	Force   bool
}

type Runner struct {
	Config              *config.Config
	StateStore          *state.Store
	ObservabilityBuffer *state.ObservabilityBufferStore
	HeartbeatService    HeartbeatService
	SyncService         SyncService
	Updater             Updater
	RuntimeManager      RuntimeManager

	autoUpdate          bool
	updateNow           bool
	updateRepo          string
	updateChan          string
	updateTag           string
	restartOpenrestyNow bool
}

func (r *Runner) Run(ctx context.Context) error {
	nodeID, err := r.StateStore.EnsureNodeID()
	if err != nil {
		return err
	}
	slog.Info("agent runner started", "node_id", nodeID, "node", r.Config.NodeName, "ip", r.Config.NodeIP)
	if r.hasAgentToken() {
		r.refreshOpenrestyHealth(ctx)
		payload, ackWindows := r.prepareHeartbeatPayload(nodeID)
		heartbeatResult, hbErr := r.HeartbeatService.Heartbeat(ctx, payload)
		if hbErr != nil {
			slog.Error("agent startup heartbeat failed", "error", hbErr)
		} else {
			r.ackObservabilityWindows(ackWindows)
			if heartbeatResult == nil {
				heartbeatResult = &protocol.HeartbeatResult{}
			}
			slog.Debug("agent startup heartbeat succeeded", "node_id", nodeID)
			r.applySettings(heartbeatResult.AgentSettings)
			if err = r.SyncService.SyncOnStartup(ctx, heartbeatResult.ActiveConfig); err != nil {
				r.recordSyncError(err)
				slog.Error("agent startup sync failed", "error", err)
			} else {
				slog.Debug("agent startup sync completed")
			}
			r.tryRestartOpenresty(ctx)
			r.tryAutoUpdate(ctx)
		}
	} else if err = r.tryRegister(ctx, &nodeID); err != nil {
		slog.Error("agent initial discovery register failed", "error", err)
	}

	heartbeatTicker := time.NewTicker(r.Config.HeartbeatInterval.Duration())
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("agent runner shutting down", "error", ctx.Err())
			return ctx.Err()
		case <-heartbeatTicker.C:
			if !r.hasAgentToken() {
				if err = r.tryRegister(ctx, &nodeID); err != nil {
					slog.Error("agent discovery register failed", "error", err)
				}
				continue
			}
			r.refreshOpenrestyHealth(ctx)
			payload, ackWindows := r.prepareHeartbeatPayload(nodeID)
			heartbeatResult, hbErr := r.HeartbeatService.Heartbeat(ctx, payload)
			if hbErr != nil {
				slog.Error("agent heartbeat failed", "error", hbErr)
			} else {
				r.ackObservabilityWindows(ackWindows)
				if heartbeatResult == nil {
					heartbeatResult = &protocol.HeartbeatResult{}
				}
				if changed := r.applySettings(heartbeatResult.AgentSettings); changed {
					heartbeatTicker.Reset(r.Config.HeartbeatInterval.Duration())
				}
				if err = r.SyncService.SyncOnce(ctx, heartbeatResult.ActiveConfig); err != nil {
					r.recordSyncError(err)
					slog.Error("agent sync failed", "error", err)
				}
				r.tryRestartOpenresty(ctx)
				r.tryAutoUpdate(ctx)
			}
		}
	}
}

func (r *Runner) hasAgentToken() bool {
	return strings.TrimSpace(r.Config.AgentToken) != ""
}

func (r *Runner) applySettings(settings *protocol.AgentSettings) bool {
	if settings == nil {
		return false
	}
	changed := false
	if settings.HeartbeatInterval > 0 {
		newInterval := config.MillisecondDuration(time.Duration(settings.HeartbeatInterval) * time.Millisecond)
		if newInterval != r.Config.HeartbeatInterval {
			slog.Info("agent heartbeat interval updated", "from", r.Config.HeartbeatInterval, "to", newInterval)
			r.Config.HeartbeatInterval = newInterval
			changed = true
		}
	}
	r.autoUpdate = settings.AutoUpdate
	r.updateNow = settings.UpdateNow
	r.updateRepo = strings.TrimSpace(settings.UpdateRepo)
	r.updateChan = strings.TrimSpace(settings.UpdateChannel)
	r.updateTag = strings.TrimSpace(settings.UpdateTag)
	r.restartOpenrestyNow = settings.RestartOpenrestyNow
	return changed
}

func (r *Runner) tryRestartOpenresty(ctx context.Context) {
	if !r.restartOpenrestyNow {
		return
	}
	r.restartOpenrestyNow = false
	if r.RuntimeManager == nil {
		return
	}
	slog.Info("agent openresty restart requested by server")
	if err := r.RuntimeManager.Restart(ctx); err != nil {
		slog.Error("agent openresty restart failed", "error", err)
		r.recordOpenrestyUnhealthy(err, false)
		return
	}
	slog.Info("agent openresty restart succeeded")
	r.recordOpenrestyHealthy()
}

func (r *Runner) tryAutoUpdate(ctx context.Context) {
	force := r.updateNow
	shouldCheck := r.autoUpdate || force
	r.updateNow = false
	r.updateTag = strings.TrimSpace(r.updateTag)
	if !shouldCheck || r.Updater == nil || r.updateRepo == "" {
		return
	}
	channel := "stable"
	if force && r.updateChan != "" {
		channel = r.updateChan
	}
	if err := r.Updater.CheckAndUpdate(ctx, r.updateRepo, UpdateOptions{
		Channel: channel,
		TagName: r.updateTag,
		Force:   force,
	}); err != nil {
		slog.Error("agent update check failed", "error", err)
	}
	if force {
		r.updateTag = ""
		r.updateChan = ""
	}
}

func (r *Runner) tryRegister(ctx context.Context, nodeID *string) error {
	if strings.TrimSpace(r.Config.DiscoveryToken) == "" {
		return errors.New("agent_token 为空且未配置 discovery_token")
	}
	slog.Info("agent discovery registration started")
	response, err := r.HeartbeatService.Register(ctx, r.nodePayload(*nodeID))
	if err != nil {
		return err
	}
	if response == nil || strings.TrimSpace(response.AgentToken) == "" || strings.TrimSpace(response.NodeID) == "" {
		return errors.New("discovery register response 缺少 node_id 或 agent_token")
	}
	snapshot, err := r.StateStore.Load()
	if err != nil {
		return err
	}
	snapshot.NodeID = response.NodeID
	if err = r.StateStore.Save(snapshot); err != nil {
		return err
	}
	r.Config.AgentToken = response.AgentToken
	r.Config.DiscoveryToken = ""
	if err = r.Config.Save(); err != nil {
		return err
	}
	r.HeartbeatService.SetToken(response.AgentToken)
	*nodeID = response.NodeID
	slog.Info("agent discovery registration succeeded", "node_id", response.NodeID)
	r.refreshOpenrestyHealth(ctx)
	payload, ackWindows := r.prepareHeartbeatPayload(*nodeID)
	heartbeatResult, heartbeatErr := r.HeartbeatService.Heartbeat(ctx, payload)
	if heartbeatErr != nil {
		slog.Error("agent post-register heartbeat failed", "error", heartbeatErr)
		return nil
	}
	r.ackObservabilityWindows(ackWindows)
	if heartbeatResult == nil {
		heartbeatResult = &protocol.HeartbeatResult{}
	}
	r.applySettings(heartbeatResult.AgentSettings)
	if err = r.SyncService.SyncOnStartup(ctx, heartbeatResult.ActiveConfig); err != nil {
		r.recordSyncError(err)
		slog.Error("agent post-register startup sync failed", "error", err)
	} else {
		slog.Debug("agent post-register startup sync completed")
	}
	r.tryRestartOpenresty(ctx)
	r.tryAutoUpdate(ctx)
	return nil
}

func (r *Runner) recordSyncError(err error) {
	if err == nil || r.StateStore == nil {
		return
	}
	snapshot, loadErr := r.StateStore.Load()
	if loadErr != nil {
		slog.Error("load state before recording sync error failed", "error", loadErr)
		return
	}
	snapshot.LastError = err.Error()
	slog.Warn("recording sync error into state", "error", snapshot.LastError)
	if saveErr := r.StateStore.Save(snapshot); saveErr != nil {
		slog.Error("save state after sync error failed", "error", saveErr)
	}
}

func (r *Runner) refreshOpenrestyHealth(ctx context.Context) {
	if r.RuntimeManager == nil || r.StateStore == nil {
		return
	}
	if err := r.RuntimeManager.CheckHealth(ctx); err != nil {
		r.recordOpenrestyUnhealthy(err, true)
		return
	}
	r.recordOpenrestyHealthy()
}

func (r *Runner) recordOpenrestyHealthy() {
	if r.StateStore == nil {
		return
	}
	snapshot, err := r.StateStore.Load()
	if err != nil {
		slog.Error("load state before recording openresty health failed", "error", err)
		return
	}
	if snapshot.OpenrestyStatus == protocol.OpenrestyStatusHealthy && strings.TrimSpace(snapshot.OpenrestyMessage) == "" {
		return
	}
	snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
	snapshot.OpenrestyMessage = ""
	if err = r.StateStore.Save(snapshot); err != nil {
		slog.Error("save state after recording openresty health failed", "error", err)
	}
}

func (r *Runner) recordOpenrestyUnhealthy(err error, fallbackOnly bool) {
	if err == nil || r.StateStore == nil {
		return
	}
	snapshot, loadErr := r.StateStore.Load()
	if loadErr != nil {
		slog.Error("load state before recording openresty error failed", "error", loadErr)
		return
	}
	message := strings.TrimSpace(err.Error())
	if !fallbackOnly || strings.TrimSpace(snapshot.OpenrestyMessage) == "" {
		snapshot.OpenrestyMessage = message
	}
	snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
	if saveErr := r.StateStore.Save(snapshot); saveErr != nil {
		slog.Error("save state after recording openresty error failed", "error", saveErr)
	}
}

func (r *Runner) nodePayload(nodeID string) protocol.NodePayload {
	snapshot, _ := r.StateStore.Load()
	openrestyStatus := strings.TrimSpace(snapshot.OpenrestyStatus)
	if openrestyStatus == "" {
		openrestyStatus = protocol.OpenrestyStatusUnknown
	}
	profile := observability.BuildProfile(r.Config, r.StateStore)
	managedOpenRestyMetrics := observability.CollectManagedOpenRestyMetrics(r.Config)
	metricSnapshot := observability.BuildSnapshot(r.Config, r.StateStore, managedOpenRestyMetrics)
	trafficReport := observability.BuildTrafficReport(r.Config, r.StateStore, managedOpenRestyMetrics)
	healthEvents := observability.BuildHealthEvents(snapshot)
	return protocol.NodePayload{
		NodeID:           nodeID,
		Name:             r.Config.NodeName,
		IP:               r.Config.NodeIP,
		AgentVersion:     r.Config.AgentVersion,
		NginxVersion:     r.Config.NginxVersion,
		CurrentVersion:   snapshot.CurrentVersion,
		LastError:        snapshot.LastError,
		OpenrestyStatus:  openrestyStatus,
		OpenrestyMessage: snapshot.OpenrestyMessage,
		Profile:          profile,
		Snapshot:         metricSnapshot,
		TrafficReport:    trafficReport,
		HealthEvents:     healthEvents,
	}
}

func (r *Runner) prepareHeartbeatPayload(nodeID string) (protocol.NodePayload, []int64) {
	payload := r.nodePayload(nodeID)
	if r.ObservabilityBuffer == nil || payload.Snapshot == nil {
		return payload, nil
	}
	now := time.Now().UTC()
	retainAfterUnix := now.Add(-time.Duration(r.Config.ObservabilityReplayMinutes) * time.Minute).Unix()
	windowStartedAtUnix := state.ObservabilityWindowStartedAt(payload.Snapshot, payload.TrafficReport)
	if windowStartedAtUnix <= 0 {
		return payload, nil
	}

	record := state.ObservabilityBufferRecord{
		WindowStartedAtUnix: windowStartedAtUnix,
		Snapshot:            payload.Snapshot,
		TrafficReport:       payload.TrafficReport,
		QueuedAtUnix:        now.Unix(),
	}
	if err := r.ObservabilityBuffer.Upsert(record, retainAfterUnix); err != nil {
		slog.Error("upsert observability buffer failed", "error", err)
		return payload, nil
	}

	records, err := r.ObservabilityBuffer.Replayable(windowStartedAtUnix, retainAfterUnix)
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
			WindowStartedAtUnix: item.WindowStartedAtUnix,
			Snapshot:            item.Snapshot,
			TrafficReport:       item.TrafficReport,
		})
		ackWindows = append(ackWindows, item.WindowStartedAtUnix)
	}
	payload.BufferedObservability = buffered
	ackWindows = append(ackWindows, windowStartedAtUnix)
	return payload, ackWindows
}

func (r *Runner) ackObservabilityWindows(windowStartedAtUnix []int64) {
	if r.ObservabilityBuffer == nil || len(windowStartedAtUnix) == 0 {
		return
	}
	retainAfterUnix := time.Now().UTC().Add(-time.Duration(r.Config.ObservabilityReplayMinutes) * time.Minute).Unix()
	if err := r.ObservabilityBuffer.Ack(windowStartedAtUnix, retainAfterUnix); err != nil {
		slog.Error("ack observability buffer failed", "error", err)
	}
}
