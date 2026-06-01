package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	openrestyrender "openflare/utils/render/openresty"
	"sort"
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
	SyncWAFIPGroups(ctx context.Context, payload protocol.WAFIPGroupSyncRequest) (*protocol.WAFIPGroupSyncResponse, error)
}

type NginxManager interface {
	Apply(ctx context.Context, mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) nginx.ApplyOutcome
	EnsureRuntime(ctx context.Context, recreate bool) error
	EnsureSafeFallbackRuntime(ctx context.Context, reason string) error
	CurrentChecksum() (string, error)
	WAFIPGroupChecksums() (map[string]string, error)
	SyncWAFIPGroups(groups []protocol.WAFIPGroup) error
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
		if startup {
			config, fetchErr := s.client.GetActiveConfig(ctx)
			if fetchErr != nil {
				slog.Error("fetch active config failed", "mode", mode, "error", fetchErr)
				return fetchErr
			}
			return s.applyIfNeeded(ctx, mode, startup, snapshot, currentChecksum, target, config)
		}
		slog.Debug("local openresty config already up to date", "mode", mode, "version", target.Version)
		shouldReport := shouldReportNoopApply(snapshot, target.Version, target.Checksum)
		if shouldReport {
			if err = s.reportNoopApply(ctx, snapshot.NodeID, target.Version, target.Checksum, "", "", 0); err != nil {
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

func (s *Service) ForceSyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	snapshot, err := s.stateStore.Load()
	if err != nil {
		return err
	}
	if hasBlockedTarget(snapshot) {
		clearBlockedTarget(snapshot)
		_ = s.stateStore.Save(snapshot)
	}
	currentChecksum, err := s.nginxManager.CurrentChecksum()
	if err != nil {
		return err
	}
	config, err := s.client.GetActiveConfig(ctx)
	if err != nil {
		slog.Error("fetch active config failed", "mode", "force", "error", err)
		return err
	}
	return s.applyIfNeeded(ctx, "force", true, snapshot, currentChecksum, target, config)
}

func (s *Service) WAFIPGroupChecksums() (map[string]string, error) {
	if s.nginxManager == nil {
		return map[string]string{}, nil
	}
	return s.nginxManager.WAFIPGroupChecksums()
}

func (s *Service) ApplyWAFIPGroups(ctx context.Context, groups []protocol.WAFIPGroup) error {
	if len(groups) == 0 || s.nginxManager == nil {
		return nil
	}
	return s.nginxManager.SyncWAFIPGroups(groups)
}

func (s *Service) applyIfNeeded(ctx context.Context, mode string, startup bool, snapshot *state.Snapshot, currentChecksum string, target *protocol.ActiveConfigMeta, config *protocol.ActiveConfigResponse) error {
	if currentChecksum == config.Checksum && !startup {
		slog.Debug("local openresty config already up to date", "mode", mode, "version", config.Version)
		shouldReport := shouldReportNoopApply(snapshot, config.Version, config.Checksum)
		if shouldReport {
			rendered, renderErr := renderActiveConfig(config)
			if renderErr != nil {
				return renderErr
			}
			if err := s.reportNoopApply(ctx, snapshot.NodeID, config.Version, config.Checksum, checksumString(rendered.mainConfig), checksumString(rendered.routeConfig), len(rendered.supportFiles)); err != nil {
				return err
			}
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
	rendered, err := renderActiveConfig(config)
	if err != nil {
		return err
	}
	mainConfigChecksum := checksumString(rendered.mainConfig)
	routeConfigChecksum := checksumString(rendered.routeConfig)
	slog.Info("applying new openresty config", "mode", mode, "from_version", snapshot.CurrentVersion, "to_version", config.Version, "old_checksum", currentChecksum, "new_checksum", config.Checksum)
	outcome := s.nginxManager.Apply(ctx, rendered.mainConfig, rendered.routeConfig, rendered.supportFiles)
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
		SupportFileCount:    len(rendered.supportFiles),
	}); err != nil {
		slog.Error("report apply log failed", "version", config.Version, "result", reportResult, "error", err)
		return err
	}
	if reportResult == ApplyResultFailed {
		slog.Warn("failed apply log reported", "version", config.Version)
		return outcomeError(config.Version, message)
	}
	if err := s.syncReferencedWAFIPGroups(ctx, rendered.supportFiles); err != nil {
		slog.Error("sync referenced waf ip groups failed", "version", config.Version, "error", err)
		return err
	}
	slog.Debug("apply log reported", "version", config.Version, "result", reportResult)
	return nil
}

func (s *Service) syncReferencedWAFIPGroups(ctx context.Context, supportFiles []protocol.SupportFile) error {
	ids := referencedWAFIPGroupIDs(supportFiles)
	if len(ids) == 0 {
		return nil
	}
	checksums, err := s.WAFIPGroupChecksums()
	if err != nil {
		return err
	}
	response, err := s.client.SyncWAFIPGroups(ctx, protocol.WAFIPGroupSyncRequest{
		IDs:       ids,
		Checksums: checksums,
	})
	if err != nil {
		return err
	}
	if response == nil || len(response.Groups) == 0 {
		return nil
	}
	return s.ApplyWAFIPGroups(ctx, response.Groups)
}

type renderedActiveConfig struct {
	mainConfig   string
	routeConfig  string
	supportFiles []protocol.SupportFile
}

func renderActiveConfig(config *protocol.ActiveConfigResponse) (*renderedActiveConfig, error) {
	if config == nil {
		return nil, errors.New("active config is nil")
	}
	sourceJSON := strings.TrimSpace(config.SourceConfigJSON)
	if sourceJSON == "" {
		return nil, errors.New("active config source_config_json is empty")
	}
	rendered, err := openrestyrender.RenderJSON(sourceJSON, toOpenRestySupportFiles(config.SupportFiles))
	if err != nil {
		return nil, err
	}
	files := fromOpenRestySupportFiles(rendered.SupportFiles)
	files = append(files, protocol.SupportFile{Path: openrestyrender.SourceConfigFileName, Content: sourceJSON})
	return &renderedActiveConfig{
		mainConfig:   rendered.MainConfig,
		routeConfig:  rendered.RouteConfig,
		supportFiles: files,
	}, nil
}

func toOpenRestySupportFiles(files []protocol.SupportFile) []openrestyrender.SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]openrestyrender.SupportFile, 0, len(files))
	for _, file := range files {
		result = append(result, openrestyrender.SupportFile{Path: file.Path, Content: file.Content})
	}
	return result
}

func fromOpenRestySupportFiles(files []openrestyrender.SupportFile) []protocol.SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]protocol.SupportFile, 0, len(files))
	for _, file := range files {
		result = append(result, protocol.SupportFile{Path: file.Path, Content: file.Content})
	}
	return result
}

func referencedWAFIPGroupIDs(supportFiles []protocol.SupportFile) []uint {
	var content string
	for _, file := range supportFiles {
		if file.Path == "waf_config.json" {
			content = strings.TrimSpace(file.Content)
			break
		}
	}
	if content == "" {
		return []uint{}
	}
	var payload struct {
		RuleGroups []struct {
			IPWhitelistGroups []uint `json:"ip_whitelist_group_ids"`
			IPBlacklistGroups []uint `json:"ip_blacklist_group_ids"`
		} `json:"rule_groups"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		slog.Debug("decode waf_config.json for ip group references failed", "error", err)
		return []uint{}
	}
	seen := make(map[uint]struct{})
	for _, group := range payload.RuleGroups {
		for _, id := range group.IPWhitelistGroups {
			if id > 0 {
				seen[id] = struct{}{}
			}
		}
		for _, id := range group.IPBlacklistGroups {
			if id > 0 {
				seen[id] = struct{}{}
			}
		}
	}
	ids := make([]uint, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func shouldReportNoopApply(snapshot *state.Snapshot, version string, checksum string) bool {
	if snapshot == nil {
		return false
	}
	return strings.TrimSpace(snapshot.CurrentVersion) != strings.TrimSpace(version) ||
		strings.TrimSpace(snapshot.CurrentChecksum) != strings.TrimSpace(checksum)
}

func (s *Service) reportNoopApply(ctx context.Context, nodeID string, version string, checksum string, mainConfigChecksum string, routeConfigChecksum string, supportFileCount int) error {
	message := "local config already matches active version; apply skipped"
	if err := s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
		NodeID:              nodeID,
		Version:             strings.TrimSpace(version),
		Result:              ApplyResultSuccess,
		Message:             message,
		Checksum:            strings.TrimSpace(checksum),
		MainConfigChecksum:  strings.TrimSpace(mainConfigChecksum),
		RouteConfigChecksum: strings.TrimSpace(routeConfigChecksum),
		SupportFileCount:    supportFileCount,
	}); err != nil {
		slog.Error("report noop apply log failed", "version", version, "error", err)
		return err
	}
	slog.Debug("noop apply log reported", "version", version)
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
		reason := fmt.Sprintf("blocked config %s has no valid local config available for runtime recovery", strings.TrimSpace(snapshot.BlockedVersion))
		if err := s.nginxManager.EnsureSafeFallbackRuntime(ctx, reason); err != nil {
			snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
			snapshot.OpenrestyMessage = err.Error()
			_ = s.stateStore.Save(snapshot)
			return err
		}
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
		snapshot.OpenrestyMessage = "safe default fallback runtime started"
		return nil
	}
	slog.Info("ensuring runtime with current local config while active target remains blocked", "mode", mode, "current_version", snapshot.CurrentVersion, "current_checksum", currentChecksum, "blocked_version", snapshot.BlockedVersion)
	if err := s.nginxManager.EnsureRuntime(ctx, true); err != nil {
		if strings.TrimSpace(snapshot.CurrentChecksum) == "" {
			reason := fmt.Sprintf("blocked config %s has no historical config and current local config cannot start: %v", strings.TrimSpace(snapshot.BlockedVersion), err)
			if fallbackErr := s.nginxManager.EnsureSafeFallbackRuntime(ctx, reason); fallbackErr == nil {
				snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
				snapshot.OpenrestyMessage = "safe default fallback runtime started"
				return nil
			} else {
				err = fmt.Errorf("%v; fallback recovery failed: %w", err, fallbackErr)
			}
		}
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
