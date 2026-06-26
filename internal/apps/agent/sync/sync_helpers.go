package sync

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/nginx"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
)

func syncMode(startup bool) string {
	if startup {
		return "startup"
	}
	return "periodic"
}

func normalizeSyncTarget(target *protocol.ActiveConfigMeta) {
	if target == nil {
		return
	}
	target.Version = strings.TrimSpace(target.Version)
	target.Checksum = strings.TrimSpace(target.Checksum)
}

func (s *Service) loadSyncState() (*state.Snapshot, string, error) {
	snapshot, err := s.stateStore.Load()
	if err != nil {
		return nil, "", err
	}
	currentChecksum, err := s.nginxManager.CurrentChecksum()
	if err != nil {
		return nil, "", err
	}
	return snapshot, currentChecksum, nil
}

func (s *Service) syncWithoutTarget(ctx context.Context, mode string, startup bool, snapshot *state.Snapshot, currentChecksum string) error {
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
	target := &protocol.ActiveConfigMeta{
		Version:  config.Version,
		Checksum: config.Checksum,
	}
	return s.applyIfNeeded(ctx, mode, startup, snapshot, currentChecksum, target, config)
}

func (s *Service) syncMatchingChecksum(ctx context.Context, mode string, startup bool, snapshot *state.Snapshot, currentChecksum string, target *protocol.ActiveConfigMeta) error {
	if startup {
		config, fetchErr := s.client.GetActiveConfig(ctx)
		if fetchErr != nil {
			slog.Error("fetch active config failed", "mode", mode, "error", fetchErr)
			return fetchErr
		}
		return s.applyIfNeeded(ctx, mode, startup, snapshot, currentChecksum, target, config)
	}
	return s.finishUpToDateSync(ctx, mode, snapshot, target)
}

func (s *Service) finishUpToDateSync(ctx context.Context, mode string, snapshot *state.Snapshot, target *protocol.ActiveConfigMeta) error {
	if err := s.reconcilePages(ctx, mode, snapshot); err != nil {
		return err
	}
	slog.Debug("local openresty config already up to date", "mode", mode, "version", target.Version)
	if shouldReportNoopApply(snapshot, target.Version, target.Checksum) {
		if err := s.reportNoopApply(ctx, snapshot.NodeID, target.Version, target.Checksum, "", "", 0); err != nil {
			return err
		}
	}
	snapshot.CurrentVersion = target.Version
	snapshot.CurrentChecksum = target.Checksum
	clearBlockedTarget(snapshot)
	snapshot.LastError = ""
	slog.Debug("sync finished without changes", "mode", mode, "version", target.Version)
	return s.stateStore.Save(snapshot)
}

func (s *Service) reconcilePages(ctx context.Context, mode string, snapshot *state.Snapshot) error {
	if !pagesReconcileNeeded(snapshot) {
		return nil
	}
	if pagesDiscoveryNeeded(snapshot) {
		config, err := s.client.GetActiveConfig(ctx)
		if err != nil {
			slog.Error("fetch active config failed", "mode", mode, "error", err)
			return err
		}
		return s.syncPagesDeployments(ctx, snapshot, config)
	}
	return s.syncPagesDeployments(ctx, snapshot, nil)
}

func (s *Service) syncMismatchedChecksum(ctx context.Context, mode string, startup bool, snapshot *state.Snapshot, currentChecksum string, target *protocol.ActiveConfigMeta) error {
	if isBlockedTarget(snapshot, target.Version, target.Checksum) {
		slog.Warn("skipping blocked config version after previous failed apply", "mode", mode, "version", target.Version, "checksum", target.Checksum)
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
	if snapshot.CurrentVersion == target.Version && snapshot.CurrentChecksum == target.Checksum && !startup {
		reconciled := pagesReconcileNeeded(snapshot)
		if err := s.reconcilePages(ctx, mode, snapshot); err != nil {
			return err
		}
		if !reconciled {
			slog.Debug("skipping config fetch because state already records target version/checksum", "version", target.Version, "checksum", target.Checksum)
		}
		return s.stateStore.Save(snapshot)
	}

	config, err := s.client.GetActiveConfig(ctx)
	if err != nil {
		slog.Error("fetch active config failed", "mode", mode, "error", err)
		return err
	}
	return s.applyIfNeeded(ctx, mode, startup, snapshot, currentChecksum, target, config)
}

func (s *Service) handleUpToDateConfig(ctx context.Context, mode string, snapshot *state.Snapshot, config *protocol.ActiveConfigResponse) error {
	if err := s.syncPagesDeployments(ctx, snapshot, config); err != nil {
		return err
	}
	rendered, renderErr := renderActiveConfig(config)
	if renderErr != nil {
		return renderErr
	}
	slog.Debug("local openresty config already up to date", "mode", mode, "version", config.Version)
	if shouldReportNoopApply(snapshot, config.Version, config.Checksum) {
		if err := s.reportNoopApply(
			ctx,
			snapshot.NodeID,
			config.Version,
			config.Checksum,
			checksumString(rendered.mainConfig),
			checksumString(rendered.routeConfig),
			len(rendered.supportFiles),
		); err != nil {
			return err
		}
	}
	if err := s.syncReferencedWAFIPGroups(ctx, rendered.supportFiles); err != nil {
		slog.Error("sync referenced waf ip groups failed", "version", config.Version, "error", err)
		return err
	}
	snapshot.CurrentVersion = config.Version
	snapshot.CurrentChecksum = config.Checksum
	clearBlockedTarget(snapshot)
	snapshot.LastError = ""
	slog.Debug("sync finished without changes", "mode", mode, "version", config.Version)
	return s.stateStore.Save(snapshot)
}

func (s *Service) handleBlockedConfigAfterFetch(ctx context.Context, mode string, startup bool, snapshot *state.Snapshot, currentChecksum string, config *protocol.ActiveConfigResponse) (bool, error) {
	if !isBlockedTarget(snapshot, config.Version, config.Checksum) {
		return false, nil
	}
	slog.Warn("skipping blocked config after fetch because the same version previously failed", "mode", mode, "version", config.Version, "checksum", config.Checksum)
	if startup {
		if err := s.ensureRuntimeForCurrentConfig(ctx, mode, snapshot, currentChecksum); err != nil {
			return true, err
		}
		return true, s.stateStore.Save(snapshot)
	}
	return true, nil
}

type applyOutcomeResult struct {
	reportResult string
	message      string
}

func normalizeApplyOutcome(outcome nginx.ApplyOutcome) (nginx.ApplyOutcome, string) {
	message := strings.TrimSpace(outcome.Message)
	if outcome.Status == "" {
		outcome.Status = nginx.ApplyStatusFatal
		if message == "" {
			message = "openresty apply returned empty outcome"
		}
	}
	return outcome, message
}

func updateSnapshotFromApplyOutcome(mode string, snapshot *state.Snapshot, config *protocol.ActiveConfigResponse, outcome nginx.ApplyOutcome, message string) applyOutcomeResult {
	result := applyOutcomeResult{reportResult: ApplyResultFailed, message: message}
	switch outcome.Status {
	case nginx.ApplyStatusSuccess:
		slog.Info("openresty config applied successfully", "mode", mode, "version", config.Version)
		snapshot.CurrentVersion = config.Version
		snapshot.CurrentChecksum = config.Checksum
		clearBlockedTarget(snapshot)
		snapshot.LastError = ""
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
		snapshot.OpenrestyMessage = ""
		result.reportResult = ApplyResultSuccess
		if result.message == "" {
			result.message = "apply success"
		}
	case nginx.ApplyStatusWarning:
		if result.message == "" {
			result.message = "apply rolled back to previous config"
		}
		slog.Warn("openresty config apply rolled back", "mode", mode, "version", config.Version, "message", result.message)
		markBlockedTarget(snapshot, config.Version, config.Checksum, result.message)
		snapshot.LastError = result.message
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
		snapshot.OpenrestyMessage = result.message
		result.reportResult = ApplyResultWarning
	default:
		if result.message == "" {
			result.message = "openresty apply failed"
		}
		slog.Error("apply openresty config failed", "mode", mode, "version", config.Version, "message", result.message)
		markBlockedTarget(snapshot, config.Version, config.Checksum, result.message)
		snapshot.LastError = result.message
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
		snapshot.OpenrestyMessage = result.message
	}
	return result
}

func snapshotMatchesTarget(snapshot *state.Snapshot, version string, checksum string) bool {
	if snapshot == nil {
		return false
	}
	return strings.TrimSpace(snapshot.CurrentVersion) == strings.TrimSpace(version) &&
		strings.TrimSpace(snapshot.CurrentChecksum) == strings.TrimSpace(checksum)
}

func shouldReportApplyLog(alreadySynced bool, result string) bool {
	if result != ApplyResultSuccess {
		return true
	}
	return !alreadySynced
}
