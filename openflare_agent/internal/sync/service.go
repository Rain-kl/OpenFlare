package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"openflare-agent/internal/nginx"
	"openflare-agent/internal/protocol"
	"openflare-agent/internal/state"
)

const (
	ApplyResultSuccess = "success"
	ApplyResultWarning = "warning"
	ApplyResultFailed  = "failed"
)

type ConfigClient interface {
	GetActiveConfig(ctx context.Context) (*protocol.ActiveConfigResponse, error)
	ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error
}

type NginxManager interface {
	Apply(ctx context.Context, mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) nginx.ApplyOutcome
	EnsureRuntime(ctx context.Context, recreate bool) error
	CurrentChecksum() (string, error)
}

type Service struct {
	client       ConfigClient
	nginxManager NginxManager
	stateStore   *state.Store
}

func New(client ConfigClient, nginxManager NginxManager, stateStore *state.Store) *Service {
	return &Service{
		client:       client,
		nginxManager: nginxManager,
		stateStore:   stateStore,
	}
}

func (s *Service) SyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	return s.sync(ctx, false, target)
}

func (s *Service) SyncOnStartup(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	return s.sync(ctx, true, target)
}

func (s *Service) sync(ctx context.Context, startup bool, target *protocol.ActiveConfigMeta) error {
	mode := "periodic"
	if startup {
		mode = "startup"
	}
	snapshot, err := s.stateStore.Load()
	if err != nil {
		return err
	}
	currentChecksum, err := s.nginxManager.CurrentChecksum()
	if err != nil {
		return err
	}

	if target != nil {
		target.Version = strings.TrimSpace(target.Version)
		target.Checksum = strings.TrimSpace(target.Checksum)
	}

	if target == nil || target.Version == "" || target.Checksum == "" {
		if !startup {
			slog.Debug("skipping sync because heartbeat returned no active config summary", "mode", mode)
			return nil
		}
		slog.Debug("sync startup fallback: active config summary unavailable, fetching active config directly")
		config, fetchErr := s.client.GetActiveConfig(ctx)
		if fetchErr != nil {
			slog.Error("fetch active config failed", "mode", mode, "error", fetchErr)
			return fetchErr
		}
		target = &protocol.ActiveConfigMeta{
			Version:  config.Version,
			Checksum: config.Checksum,
		}
		return s.applyIfNeeded(ctx, mode, startup, snapshot, currentChecksum, target, config)
	}

	if currentChecksum == target.Checksum {
		slog.Debug("local openresty config already up to date", "mode", mode, "version", target.Version)
		if startup {
			slog.Debug("ensuring openresty runtime on startup", "version", target.Version)
			if err = s.nginxManager.EnsureRuntime(ctx, true); err != nil {
				snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
				snapshot.OpenrestyMessage = err.Error()
				_ = s.stateStore.Save(snapshot)
				return err
			}
			slog.Debug("openresty runtime ensured on startup", "version", target.Version)
			snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
			snapshot.OpenrestyMessage = ""
		}
		snapshot.CurrentVersion = target.Version
		snapshot.CurrentChecksum = target.Checksum
		clearBlockedTarget(snapshot)
		snapshot.LastError = ""
		slog.Debug("sync finished without changes", "mode", mode, "version", target.Version)
		return s.stateStore.Save(snapshot)
	}
	if isBlockedTarget(snapshot, target.Version, target.Checksum) {
		slog.Warn("skipping blocked config version after previous failed apply", "mode", mode, "version", target.Version, "checksum", target.Checksum)
		if startup {
			if err = s.ensureRuntimeForCurrentConfig(ctx, mode, snapshot, currentChecksum); err != nil {
				return err
			}
			return s.stateStore.Save(snapshot)
		}
		return nil
	}
	if hasBlockedTarget(snapshot) {
		clearBlockedTarget(snapshot)
	}
	if snapshot.CurrentVersion == target.Version && snapshot.CurrentChecksum == target.Checksum && !startup {
		slog.Debug("skipping config fetch because state already records target version/checksum", "version", target.Version, "checksum", target.Checksum)
		return s.stateStore.Save(snapshot)
	}

	config, err := s.client.GetActiveConfig(ctx)
	if err != nil {
		slog.Error("fetch active config failed", "mode", mode, "error", err)
		return err
	}
	return s.applyIfNeeded(ctx, mode, startup, snapshot, currentChecksum, target, config)
}

func (s *Service) applyIfNeeded(ctx context.Context, mode string, startup bool, snapshot *state.Snapshot, currentChecksum string, target *protocol.ActiveConfigMeta, config *protocol.ActiveConfigResponse) error {
	if currentChecksum == config.Checksum {
		slog.Debug("local openresty config already up to date", "mode", mode, "version", config.Version)
		if startup {
			slog.Debug("ensuring openresty runtime on startup", "version", config.Version)
			if err := s.nginxManager.EnsureRuntime(ctx, true); err != nil {
				snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
				snapshot.OpenrestyMessage = err.Error()
				_ = s.stateStore.Save(snapshot)
				return err
			}
			slog.Debug("openresty runtime ensured on startup", "version", config.Version)
			snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
			snapshot.OpenrestyMessage = ""
		}
		snapshot.CurrentVersion = config.Version
		snapshot.CurrentChecksum = config.Checksum
		clearBlockedTarget(snapshot)
		snapshot.LastError = ""
		slog.Debug("sync finished without changes", "mode", mode, "version", config.Version)
		return s.stateStore.Save(snapshot)
	}
	if target != nil && (target.Version != config.Version || target.Checksum != config.Checksum) {
		slog.Warn("active config changed between heartbeat and fetch", "heartbeat_version", target.Version, "heartbeat_checksum", target.Checksum, "fetched_version", config.Version, "fetched_checksum", config.Checksum)
	}
	if isBlockedTarget(snapshot, config.Version, config.Checksum) {
		slog.Warn("skipping blocked config after fetch because the same version previously failed", "mode", mode, "version", config.Version, "checksum", config.Checksum)
		if startup {
			if err := s.ensureRuntimeForCurrentConfig(ctx, mode, snapshot, currentChecksum); err != nil {
				return err
			}
			return s.stateStore.Save(snapshot)
		}
		return nil
	}
	if hasBlockedTarget(snapshot) {
		clearBlockedTarget(snapshot)
	}
	if snapshot.CurrentVersion == config.Version && snapshot.CurrentChecksum == config.Checksum && !startup {
		slog.Debug("skipping apply because state already records target version/checksum", "version", config.Version, "checksum", config.Checksum)
		return s.stateStore.Save(snapshot)
	}
	routeConfig := config.RouteConfig
	if routeConfig == "" {
		routeConfig = config.RenderedConfig
	}
	mainConfigChecksum := checksumString(config.MainConfig)
	routeConfigChecksum := checksumString(routeConfig)
	slog.Info("applying new openresty config", "mode", mode, "from_version", snapshot.CurrentVersion, "to_version", config.Version, "old_checksum", currentChecksum, "new_checksum", config.Checksum)
	outcome := s.nginxManager.Apply(ctx, config.MainConfig, routeConfig, config.SupportFiles)
	message := strings.TrimSpace(outcome.Message)
	if outcome.Status == "" {
		outcome.Status = nginx.ApplyStatusFatal
		if message == "" {
			message = "openresty apply returned empty outcome"
		}
	}

	reportResult := ApplyResultFailed
	switch outcome.Status {
	case nginx.ApplyStatusSuccess:
		slog.Info("openresty config applied successfully", "mode", mode, "version", config.Version)
		snapshot.CurrentVersion = config.Version
		snapshot.CurrentChecksum = config.Checksum
		clearBlockedTarget(snapshot)
		snapshot.LastError = ""
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
		snapshot.OpenrestyMessage = ""
		reportResult = ApplyResultSuccess
		if message == "" {
			message = "apply success"
		}
	case nginx.ApplyStatusWarning:
		if message == "" {
			message = "apply rolled back to previous config"
		}
		slog.Warn("openresty config apply rolled back", "mode", mode, "version", config.Version, "message", message)
		markBlockedTarget(snapshot, config.Version, config.Checksum, message)
		snapshot.LastError = message
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
		snapshot.OpenrestyMessage = message
		reportResult = ApplyResultWarning
	default:
		if message == "" {
			message = "openresty apply failed"
		}
		slog.Error("apply openresty config failed", "mode", mode, "version", config.Version, "message", message)
		markBlockedTarget(snapshot, config.Version, config.Checksum, message)
		snapshot.LastError = message
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
		snapshot.OpenrestyMessage = message
	}

	if err := s.stateStore.Save(snapshot); err != nil {
		return err
	}
	if err := s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
		NodeID:              snapshot.NodeID,
		Version:             config.Version,
		Result:              reportResult,
		Message:             message,
		Checksum:            config.Checksum,
		MainConfigChecksum:  mainConfigChecksum,
		RouteConfigChecksum: routeConfigChecksum,
		SupportFileCount:    len(config.SupportFiles),
	}); err != nil {
		slog.Error("report apply log failed", "version", config.Version, "result", reportResult, "error", err)
		return err
	}
	if reportResult == ApplyResultFailed {
		slog.Warn("failed apply log reported", "version", config.Version)
		return outcomeError(config.Version, message)
	}
	slog.Debug("apply log reported", "version", config.Version, "result", reportResult)
	return nil
}

func outcomeError(version string, message string) error {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		trimmed = "openresty apply failed"
	}
	return fmt.Errorf("apply version %s failed: %s", version, trimmed)
}

func (s *Service) ensureRuntimeForCurrentConfig(ctx context.Context, mode string, snapshot *state.Snapshot, currentChecksum string) error {
	if strings.TrimSpace(currentChecksum) == "" {
		slog.Warn("blocked config cannot be retried and no local checksum is available for runtime recovery", "mode", mode, "blocked_version", snapshot.BlockedVersion)
		return nil
	}
	slog.Info("ensuring runtime with current local config while active target remains blocked", "mode", mode, "current_version", snapshot.CurrentVersion, "current_checksum", currentChecksum, "blocked_version", snapshot.BlockedVersion)
	if err := s.nginxManager.EnsureRuntime(ctx, true); err != nil {
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
		snapshot.OpenrestyMessage = err.Error()
		_ = s.stateStore.Save(snapshot)
		return err
	}
	snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
	if strings.TrimSpace(snapshot.OpenrestyMessage) == strings.TrimSpace(snapshot.BlockedReason) {
		snapshot.OpenrestyMessage = ""
	}
	return nil
}

func markBlockedTarget(snapshot *state.Snapshot, version string, checksum string, reason string) {
	if snapshot == nil {
		return
	}
	snapshot.BlockedVersion = strings.TrimSpace(version)
	snapshot.BlockedChecksum = strings.TrimSpace(checksum)
	snapshot.BlockedReason = strings.TrimSpace(reason)
}

func clearBlockedTarget(snapshot *state.Snapshot) {
	if snapshot == nil {
		return
	}
	snapshot.BlockedVersion = ""
	snapshot.BlockedChecksum = ""
	snapshot.BlockedReason = ""
}

func hasBlockedTarget(snapshot *state.Snapshot) bool {
	return snapshot != nil && (strings.TrimSpace(snapshot.BlockedVersion) != "" || strings.TrimSpace(snapshot.BlockedChecksum) != "")
}

func isBlockedTarget(snapshot *state.Snapshot, version string, checksum string) bool {
	if snapshot == nil {
		return false
	}
	return strings.TrimSpace(snapshot.BlockedVersion) == strings.TrimSpace(version) &&
		strings.TrimSpace(snapshot.BlockedChecksum) == strings.TrimSpace(checksum) &&
		(strings.TrimSpace(version) != "" || strings.TrimSpace(checksum) != "")
}

func checksumString(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
