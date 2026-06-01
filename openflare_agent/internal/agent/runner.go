package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"openflare-agent/internal/config"
	"openflare-agent/internal/observability"
	"openflare-agent/internal/protocol"
	"openflare-agent/internal/state"
)

type HeartbeatService interface {
	Register(ctx context.Context, payload protocol.NodePayload) (*protocol.RegisterNodeResponse, error)
	Heartbeat(ctx context.Context, payload protocol.NodePayload) (*protocol.HeartbeatResult, error)
	SetToken(token string)
}

type SyncService interface {
	SyncOnStartup(ctx context.Context, target *protocol.ActiveConfigMeta) error
	SyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error
	ForceSyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error
	WAFIPGroupChecksums() (map[string]string, error)
	ApplyWAFIPGroups(ctx context.Context, groups []protocol.WAFIPGroup) error
}

type Updater interface {
	CheckAndUpdate(ctx context.Context, repo string, options UpdateOptions) error
}

type RuntimeManager interface {
	CheckHealth(ctx context.Context) error
	Restart(ctx context.Context) error
}

type WebSocketService interface {
	Connect(ctx context.Context) (protocol.WebSocketConnection, error)
	SetToken(token string)
	URL() string
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
	WebSocketService    WebSocketService

	autoUpdate              bool
	updateNow               bool
	updateRepo              string
	updateChan              string
	updateTag               string
	restartOpenrestyNow     bool
	websocketUpgradeEnabled bool
}

func (r *Runner) Run(ctx context.Context) error {
	nodeID, err := r.StateStore.EnsureNodeID()
	if err != nil {
		return err
	}
	slog.Info("agent runner started", "node_id", nodeID, "node", r.Config.NodeName, "ip", r.Config.NodeIP)
	if r.hasAgentToken() {
		if _, hbErr := r.performHeartbeatCycle(ctx, nodeID, true); hbErr != nil {
			slog.Error("agent startup heartbeat failed", "error", hbErr)
		}
	} else if err = r.tryRegister(ctx, &nodeID); err != nil {
		slog.Error("agent initial discovery register failed", "error", err)
	}

	heartbeatTicker := time.NewTicker(r.Config.HeartbeatInterval.Duration())
	defer heartbeatTicker.Stop()
	var wsDone <-chan error
	wsBackoff := newWebSocketBackoff()
	nextWSAttempt := time.Now()
	tryStartWebSocket := func() {
		if wsDone != nil || !r.shouldUseWebSocket() || time.Now().Before(nextWSAttempt) {
			return
		}
		done, startErr := r.startWebSocket(ctx, nodeID)
		if startErr != nil {
			delay := wsBackoff.Next()
			nextWSAttempt = time.Now().Add(delay)
			slog.Debug("agent ws upgrade failed; falling back to http heartbeat",
				"enabled", r.websocketUpgradeEnabled,
				"url", r.websocketURL(),
				"retry_after", delay,
				"error", startErr,
			)
			return
		}
		wsBackoff.Reset()
		wsDone = done
		slog.Debug("agent switched to websocket mode", "url", r.websocketURL())
	}
	tryStartWebSocket()

	for {
		select {
		case <-ctx.Done():
			slog.Info("agent runner shutting down", "error", ctx.Err())
			return ctx.Err()
		case wsErr := <-wsDone:
			wsDone = nil
			delay := wsBackoff.Next()
			nextWSAttempt = time.Now().Add(delay)
			slog.Debug("agent ws disconnected; resuming http heartbeat", "retry_after", delay, "error", wsErr)
			if r.hasAgentToken() {
				if _, hbErr := r.performHeartbeatCycle(ctx, nodeID, false); hbErr != nil {
					slog.Error("agent heartbeat after ws disconnect failed", "error", hbErr)
				}
			}
		case <-heartbeatTicker.C:
			if wsDone != nil {
				continue
			}
			if !r.hasAgentToken() {
				if err = r.tryRegister(ctx, &nodeID); err != nil {
					slog.Error("agent discovery register failed", "error", err)
				}
				continue
			}
			if changed, hbErr := r.performHeartbeatCycle(ctx, nodeID, false); hbErr != nil {
				slog.Error("agent heartbeat failed", "error", hbErr)
			} else {
				if changed {
					heartbeatTicker.Reset(r.Config.HeartbeatInterval.Duration())
				}
				tryStartWebSocket()
			}
		}
	}
}

func (r *Runner) performHeartbeatCycle(ctx context.Context, nodeID string, startup bool) (bool, error) {
	r.refreshOpenrestyHealth(ctx)
	payload, ackWindows := r.prepareHeartbeatPayload(nodeID)
	heartbeatResult, err := r.HeartbeatService.Heartbeat(ctx, payload)
	if err != nil {
		return false, err
	}
	r.ackObservabilityWindows(ackWindows)
	if heartbeatResult == nil {
		heartbeatResult = &protocol.HeartbeatResult{}
	}
	mode := "periodic"
	if startup {
		mode = "startup"
	}
	slog.Debug("agent heartbeat succeeded", "mode", mode, "node_id", nodeID)
	changed := r.applySettings(heartbeatResult.AgentSettings)
	r.applyWAFIPGroups(ctx, heartbeatResult.WAFIPGroups)
	if startup {
		if err = r.SyncService.SyncOnStartup(ctx, heartbeatResult.ActiveConfig); err != nil {
			r.recordSyncError(err)
			slog.Error("agent startup sync failed", "error", err)
		} else {
			slog.Debug("agent startup sync completed")
		}
	} else if err = r.SyncService.SyncOnce(ctx, heartbeatResult.ActiveConfig); err != nil {
		r.recordSyncError(err)
		slog.Error("agent sync failed", "error", err)
	}
	r.tryRestartOpenresty(ctx)
	r.tryAutoUpdate(ctx)
	return changed, nil
}

func (r *Runner) shouldUseWebSocket() bool {
	enabled := r.WebSocketService != nil && r.websocketUpgradeEnabled && r.hasAgentToken()
	slog.Debug("agent ws upgrade eligibility checked", "enabled", enabled, "server_enabled", r.websocketUpgradeEnabled, "url", r.websocketURL())
	return enabled
}

func (r *Runner) websocketURL() string {
	if r.WebSocketService == nil {
		return ""
	}
	return r.WebSocketService.URL()
}

func (r *Runner) startWebSocket(ctx context.Context, nodeID string) (<-chan error, error) {
	if r.WebSocketService == nil {
		return nil, errors.New("websocket service is not configured")
	}
	conn, err := r.WebSocketService.Connect(ctx)
	if err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() {
		defer func() {
			_ = conn.Close()
		}()
		done <- r.runWebSocket(ctx, nodeID, conn)
	}()
	return done, nil
}

func (r *Runner) runWebSocket(ctx context.Context, nodeID string, conn protocol.WebSocketConnection) error {
	slog.Debug("agent ws connected", "url", conn.URL(), "node_id", nodeID)
	statusTicker := time.NewTicker(r.Config.HeartbeatInterval.Duration())
	defer statusTicker.Stop()

	messages := make(chan protocol.WSMessage, 8)
	readDone := make(chan error, 1)
	go func() {
		for {
			message, err := conn.Receive()
			if err != nil {
				readDone <- err
				return
			}
			select {
			case messages <- message:
			case <-ctx.Done():
				readDone <- ctx.Err()
				return
			}
		}
	}()

	if err := r.sendWebSocketStatus(ctx, nodeID, conn); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readDone:
			return err
		case <-statusTicker.C:
			if err := r.sendWebSocketStatus(ctx, nodeID, conn); err != nil {
				return err
			}
		case message := <-messages:
			changed, err := r.handleWebSocketMessage(ctx, message, conn)
			if err != nil {
				return err
			}
			if changed {
				statusTicker.Reset(r.Config.HeartbeatInterval.Duration())
			}
		}
	}
}

func (r *Runner) sendWebSocketStatus(ctx context.Context, nodeID string, conn protocol.WebSocketConnection) error {
	r.refreshOpenrestyHealth(ctx)
	payload, ackWindows := r.prepareHeartbeatPayload(nodeID)
	if err := conn.SendStatus(payload); err != nil {
		return err
	}
	r.ackObservabilityWindows(ackWindows)
	return nil
}

func (r *Runner) handleWebSocketMessage(ctx context.Context, message protocol.WSMessage, conn protocol.WebSocketConnection) (bool, error) {
	switch message.Type {
	case protocol.WSMessageTypeSettings:
		var settings protocol.AgentSettings
		if err := json.Unmarshal(message.Payload, &settings); err != nil {
			slog.Debug("agent ws settings decode failed", "error", err)
			return false, nil
		}
		changed := r.applySettings(&settings)
		r.tryRestartOpenresty(ctx)
		r.tryAutoUpdate(ctx)
		if !r.websocketUpgradeEnabled {
			slog.Debug("agent ws disabled by server settings; falling back to http heartbeat")
			return changed, errors.New("websocket upgrade disabled by server")
		}
		return changed, nil
	case protocol.WSMessageTypeActiveConfig:
		var target protocol.ActiveConfigMeta
		if err := json.Unmarshal(message.Payload, &target); err != nil {
			slog.Debug("agent ws active config decode failed", "error", err)
			return false, nil
		}
		slog.Debug("agent ws active config received", "version", target.Version, "checksum", target.Checksum, "trigger_sync", true)
		if err := r.SyncService.SyncOnce(ctx, &target); err != nil {
			r.recordSyncError(err)
			slog.Error("agent ws triggered sync failed", "version", target.Version, "error", err)
		}
		return false, nil
	case protocol.WSMessageTypeForceSyncConfig:
		var target protocol.ActiveConfigMeta
		if err := json.Unmarshal(message.Payload, &target); err != nil {
			slog.Debug("agent ws force sync config decode failed", "error", err)
			return false, nil
		}
		slog.Debug("agent ws force sync config received", "version", target.Version, "checksum", target.Checksum, "trigger_sync", true)
		if err := r.SyncService.ForceSyncOnce(ctx, &target); err != nil {
			r.recordSyncError(err)
			slog.Error("agent ws triggered force sync failed", "version", target.Version, "error", err)
		}
		return false, nil
	case protocol.WSMessageTypeWAFIPGroups:
		var groups []protocol.WAFIPGroup
		if err := json.Unmarshal(message.Payload, &groups); err != nil {
			slog.Debug("agent ws waf ip groups decode failed", "error", err)
			return false, nil
		}
		r.applyWAFIPGroups(ctx, groups)
		return false, nil
	case protocol.WSMessageTypePing:
		slog.Debug("agent ws ping received")
		return false, conn.SendPong()
	case protocol.WSMessageTypePong:
		slog.Debug("agent ws pong received")
		return false, nil
	default:
		slog.Debug("agent ws unsupported message type", "type", message.Type)
		return false, nil
	}
}

type webSocketBackoff struct {
	delays []time.Duration
	index  int
}

func newWebSocketBackoff() *webSocketBackoff {
	return &webSocketBackoff{
		delays: []time.Duration{
			time.Second,
			2 * time.Second,
			5 * time.Second,
			10 * time.Second,
			30 * time.Second,
		},
	}
}

func (backoff *webSocketBackoff) Next() time.Duration {
	if backoff == nil || len(backoff.delays) == 0 {
		return 30 * time.Second
	}
	if backoff.index >= len(backoff.delays) {
		return backoff.delays[len(backoff.delays)-1]
	}
	delay := backoff.delays[backoff.index]
	backoff.index++
	return delay
}

func (backoff *webSocketBackoff) Reset() {
	if backoff != nil {
		backoff.index = 0
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
	if settings.WebsocketUpgradeEnabled != r.websocketUpgradeEnabled {
		slog.Debug("agent websocket upgrade setting updated", "from", r.websocketUpgradeEnabled, "to", settings.WebsocketUpgradeEnabled)
	}
	r.websocketUpgradeEnabled = settings.WebsocketUpgradeEnabled
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
	if r.WebSocketService != nil {
		r.WebSocketService.SetToken(response.AgentToken)
	}
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
	r.applyWAFIPGroups(ctx, heartbeatResult.WAFIPGroups)
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
		if strings.Contains(err.Error(), "openresty config not exists") {
			return
		}
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
	trafficReport, accessLogs, fallbackMetrics := observability.BuildTrafficObservability(r.Config, r.StateStore, managedOpenRestyMetrics)
	if managedOpenRestyMetrics == nil {
		managedOpenRestyMetrics = fallbackMetrics
	}
	metricSnapshot := observability.BuildSnapshot(r.Config, r.StateStore, managedOpenRestyMetrics)
	healthEvents := observability.BuildHealthEvents(snapshot)
	payload := protocol.NodePayload{
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
		AccessLogs:       accessLogs,
		HealthEvents:     healthEvents,
	}
	if r.SyncService != nil {
		checksums, err := r.SyncService.WAFIPGroupChecksums()
		if err != nil {
			slog.Debug("load local waf ip group checksums failed", "error", err)
		} else if len(checksums) > 0 {
			payload.WAFIPGroupChecksums = checksums
		}
	}
	return payload
}

func (r *Runner) applyWAFIPGroups(ctx context.Context, groups []protocol.WAFIPGroup) {
	if len(groups) == 0 || r.SyncService == nil {
		return
	}
	if err := r.SyncService.ApplyWAFIPGroups(ctx, groups); err != nil {
		r.recordSyncError(err)
		slog.Error("agent apply waf ip groups failed", "error", err)
	}
}

func (r *Runner) prepareHeartbeatPayload(nodeID string) (protocol.NodePayload, []int64) {
	payload := r.nodePayload(nodeID)
	if r.ObservabilityBuffer == nil || (payload.Snapshot == nil && payload.TrafficReport == nil && len(payload.AccessLogs) == 0) {
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
		AccessLogs:          payload.AccessLogs,
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
			AccessLogs:          item.AccessLogs,
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
