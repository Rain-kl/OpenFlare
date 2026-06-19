package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/config"
	agentheartbeat "github.com/Rain-kl/Wavelet/internal/apps/agent/heartbeat"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/wsclient"
	edgeheartbeat "github.com/Rain-kl/Wavelet/internal/apps/edge/heartbeat"
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

type RuntimeManager interface {
	CheckHealth(ctx context.Context) error
	Restart(ctx context.Context) error
}

type WebSocketService interface {
	Connect(ctx context.Context) (protocol.WebSocketConnection, error)
	SetToken(token string)
	URL() string
}

type Runner struct {
	Config              *config.Config
	StateStore          *state.Store
	HeartbeatCycle      *agentheartbeat.Cycle
	HeartbeatService    HeartbeatService
	SyncService         SyncService
	RuntimeManager      RuntimeManager
	WebSocketService    WebSocketService

	restartOpenrestyNow     bool
	websocketUpgradeEnabled bool
}

func (r *Runner) Run(ctx context.Context) error {
	if r.HeartbeatCycle != nil {
		r.HeartbeatCycle.RecordSyncError = r.recordSyncError
	}
	nodeID, err := r.StateStore.EnsureNodeID()
	if err != nil {
		return err
	}
	slog.Info("agent runner started", "node_id", nodeID, "node", r.Config.NodeName, "ip", r.Config.NodeIP)
	if r.hasAccessToken() {
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
			if r.hasAccessToken() {
				if _, hbErr := r.performHeartbeatCycle(ctx, nodeID, false); hbErr != nil {
					slog.Error("agent heartbeat after ws disconnect failed", "error", hbErr)
				}
			}
		case <-heartbeatTicker.C:
			if wsDone != nil {
				continue
			}
			if !r.hasAccessToken() {
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
	return r.HeartbeatCycle.Perform(ctx, nodeID, startup, r)
}

func (r *Runner) Apply(settings *protocol.AgentSettings) bool {
	return r.applySettings(settings)
}

func (r *Runner) RestartOpenrestyIfNeeded(ctx context.Context) {
	r.tryRestartOpenresty(ctx)
}

func (r *Runner) shouldUseWebSocket() bool {
	enabled := r.WebSocketService != nil && r.websocketUpgradeEnabled && r.hasAccessToken()
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

type agentWSHandler struct {
	runner       *Runner
	conn         protocol.WebSocketConnection
	nodeID       string
	statusTicker *time.Ticker
}

func (h *agentWSHandler) OnConnect(ctx context.Context) error {
	return h.runner.sendWebSocketStatus(ctx, h.nodeID, h.conn)
}

func (h *agentWSHandler) HandleMessage(ctx context.Context, msg wsclient.WSMessage) error {
	var payloadBytes []byte
	if msg.Payload != nil {
		payloadBytes = []byte(msg.Payload)
	}
	protoMsg := protocol.WSMessage{
		Type:    msg.Type,
		Payload: payloadBytes,
	}
	changed, err := h.runner.handleWebSocketMessage(ctx, protoMsg, h.conn)
	if err != nil {
		return err
	}
	if changed {
		h.statusTicker.Reset(h.runner.Config.HeartbeatInterval.Duration())
	}
	return nil
}

func (h *agentWSHandler) OnClose(err error) {
	slog.Error("agent ws receive failed", "error", err)
}

func (r *Runner) runWebSocket(ctx context.Context, nodeID string, conn protocol.WebSocketConnection) error {
	slog.Debug("agent ws connected", "url", conn.URL(), "node_id", nodeID)
	statusTicker := time.NewTicker(r.Config.HeartbeatInterval.Duration())
	defer statusTicker.Stop()

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for {
			select {
			case <-childCtx.Done():
				return
			case <-statusTicker.C:
				if err := r.sendWebSocketStatus(childCtx, nodeID, conn); err != nil {
					slog.Error("agent ws send status failed", "error", err)
					_ = conn.Close()
					return
				}
			}
		}
	}()

	wsConn, ok := conn.(*wsclient.Connection)
	if !ok {
		return errors.New("invalid websocket connection type")
	}

	return wsConn.RunReceiveLoop(childCtx, &agentWSHandler{
		runner:       r,
		conn:         conn,
		nodeID:       nodeID,
		statusTicker: statusTicker,
	})
}

func (r *Runner) sendWebSocketStatus(ctx context.Context, nodeID string, conn protocol.WebSocketConnection) error {
	r.refreshOpenrestyHealth(ctx)
	payload, ackWindows := r.HeartbeatCycle.PrepareHeartbeatPayload(nodeID)
	if err := conn.SendStatus(payload); err != nil {
		return err
	}
	r.HeartbeatCycle.AckObservabilityWindows(ackWindows)
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
		edgeheartbeat.TryAutoUpdate(ctx, r.HeartbeatCycle.Updater, agentheartbeat.AgentSettingsToAutoUpdate(&settings), "agent")
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
		r.HeartbeatCycle.ApplyWAFIPGroups(ctx, groups)
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

func (r *Runner) hasAccessToken() bool {
	return strings.TrimSpace(r.Config.AccessToken) != ""
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

func (r *Runner) tryRegister(ctx context.Context, nodeID *string) error {
	if strings.TrimSpace(r.Config.DiscoveryToken) == "" {
		return errors.New("agent_token 为空且未配置 discovery_token")
	}
	slog.Info("agent discovery registration started")
	response, err := r.HeartbeatService.Register(ctx, r.HeartbeatCycle.NodePayload(*nodeID))
	if err != nil {
		return err
	}
	if response == nil || strings.TrimSpace(response.AccessToken) == "" || strings.TrimSpace(response.NodeID) == "" {
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
	r.Config.AccessToken = response.AccessToken
	r.Config.DiscoveryToken = ""
	if err = r.Config.Save(); err != nil {
		return err
	}
	r.HeartbeatService.SetToken(response.AccessToken)
	if r.WebSocketService != nil {
		r.WebSocketService.SetToken(response.AccessToken)
	}
	*nodeID = response.NodeID
	slog.Info("agent discovery registration succeeded", "node_id", response.NodeID)
	r.refreshOpenrestyHealth(ctx)
	if _, err = r.HeartbeatCycle.Perform(ctx, *nodeID, true, r); err != nil {
		slog.Error("agent post-register heartbeat failed", "error", err)
	}
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