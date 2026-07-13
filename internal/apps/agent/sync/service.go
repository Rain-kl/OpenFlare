package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/nginx"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
	"github.com/Rain-kl/Wavelet/internal/apps/agent/state"
)

// Apply result constants indicate the outcome reported back to the server.
const (
	ApplyResultSuccess = "success"
	ApplyResultWarning = "warning"
	ApplyResultFailed  = "failed"
)

// ConfigClient is the interface for communicating with the server control plane.
type ConfigClient interface {
	GetActiveConfig(ctx context.Context) (*protocol.ActiveConfigResponse, error)
	GetPagesDeploymentHash(ctx context.Context, deploymentID uint) (string, error)
	DownloadPagesDeploymentPackage(ctx context.Context, deploymentID uint) ([]byte, error)
	ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error
	SyncWAFIPGroups(ctx context.Context, payload protocol.WAFIPGroupSyncRequest) (*protocol.WAFIPGroupSyncResponse, error)
}

// NginxManager is the interface for managing the local OpenResty instance.
type NginxManager interface {
	Apply(ctx context.Context, mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) nginx.ApplyOutcome
	EnsureRuntime(ctx context.Context, recreate bool) error
	EnsureSafeFallbackRuntime(ctx context.Context, reason string) error
	CurrentChecksum() (string, error)
	WAFIPGroupChecksums() (map[string]string, error)
	ReconcileWAFIPGroups(targetIDs []uint, changed []protocol.WAFIPGroup) error
	UpdateExistingWAFIPGroups(changed []protocol.WAFIPGroup) error
	EnsureWorkerReadAccess() error
}

// Service orchestrates configuration synchronisation between the server and the local OpenResty instance.
type Service struct {
	client       ConfigClient
	nginxManager NginxManager
	stateStore   *state.Store
	pagesDir     string
	syncMu       sync.Mutex
}

// SetPagesDir sets the local directory used for pages deployment packages.
func (s *Service) SetPagesDir(path string) {
	s.pagesDir = strings.TrimSpace(path)
}

// New creates a new Service with the given client, nginx manager, and state store.
func New(client ConfigClient, nginxManager NginxManager, stateStore *state.Store) *Service {
	return &Service{
		client:       client,
		nginxManager: nginxManager,
		stateStore:   stateStore,
	}
}

// SyncOnce performs a single periodic sync against the given active config summary.
func (s *Service) SyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	return s.sync(ctx, false, target)
}

// SyncOnStartup performs an initial sync at agent startup, applying config even when checksums already match.
func (s *Service) SyncOnStartup(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	return s.sync(ctx, true, target)
}

func (s *Service) sync(ctx context.Context, startup bool, target *protocol.ActiveConfigMeta) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	mode := syncMode(startup)
	snapshot, currentChecksum, err := s.loadSyncState()
	if err != nil {
		return err
	}
	normalizeSyncTarget(target)

	if target == nil || target.Version == "" || target.Checksum == "" {
		return s.syncWithoutTarget(ctx, mode, startup, snapshot, currentChecksum)
	}
	if currentChecksum == target.Checksum {
		return s.syncMatchingChecksum(ctx, mode, startup, snapshot, currentChecksum, target)
	}
	return s.syncMismatchedChecksum(ctx, mode, startup, snapshot, currentChecksum, target)
}

// ForceSyncOnce clears any blocked target state then unconditionally fetches and applies the active config.
func (s *Service) ForceSyncOnce(ctx context.Context, target *protocol.ActiveConfigMeta) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

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

// WAFIPGroupChecksums returns the current per-group checksums held by the nginx manager.
func (s *Service) WAFIPGroupChecksums() (map[string]string, error) {
	if s.nginxManager == nil {
		return map[string]string{}, nil
	}
	return s.nginxManager.WAFIPGroupChecksums()
}

// ApplyWAFIPGroups applies real-time changes only to groups already in the local authoritative snapshot.
func (s *Service) ApplyWAFIPGroups(_ context.Context, groups []protocol.WAFIPGroup) error {
	if len(groups) == 0 || s.nginxManager == nil {
		return nil
	}
	return s.nginxManager.UpdateExistingWAFIPGroups(groups)
}

func (s *Service) applyIfNeeded(ctx context.Context, mode string, startup bool, snapshot *state.Snapshot, currentChecksum string, target *protocol.ActiveConfigMeta, config *protocol.ActiveConfigResponse) error {
	if currentChecksum == config.Checksum && !startup {
		return s.handleUpToDateConfig(ctx, mode, snapshot, config)
	}
	if target != nil && (target.Version != config.Version || target.Checksum != config.Checksum) {
		slog.Warn("active config changed between heartbeat and fetch", "heartbeat_version", target.Version, "heartbeat_checksum", target.Checksum, "fetched_version", config.Version, "fetched_checksum", config.Checksum)
	}
	if handled, err := s.handleBlockedConfigAfterFetch(ctx, mode, startup, snapshot, currentChecksum, config); handled {
		return err
	}
	if hasBlockedTarget(snapshot) {
		clearBlockedTarget(snapshot)
	}
	if snapshot.CurrentVersion == config.Version && snapshot.CurrentChecksum == config.Checksum && !startup {
		if err := s.syncPagesDeployments(ctx, snapshot, config); err != nil {
			return err
		}
		slog.Debug("skipping apply because state already records target version/checksum", "version", config.Version, "checksum", config.Checksum)
		return s.stateStore.Save(snapshot)
	}
	return s.applyRenderedConfig(ctx, mode, snapshot, currentChecksum, config)
}

func (s *Service) applyRenderedConfig(ctx context.Context, mode string, snapshot *state.Snapshot, currentChecksum string, config *protocol.ActiveConfigResponse) error {
	rendered, err := renderActiveConfig(config)
	if err != nil {
		return err
	}
	if err := s.syncPagesDeployments(ctx, snapshot, config); err != nil {
		return err
	}
	mainConfigChecksum := checksumString(rendered.mainConfig)
	routeConfigChecksum := checksumString(rendered.routeConfig)
	slog.Info("applying new openresty config", "mode", mode, "from_version", snapshot.CurrentVersion, "to_version", config.Version, "old_checksum", currentChecksum, "new_checksum", config.Checksum)
	alreadySynced := snapshotMatchesTarget(snapshot, config.Version, config.Checksum)
	outcome, message := normalizeApplyOutcome(s.nginxManager.Apply(ctx, rendered.mainConfig, rendered.routeConfig, rendered.supportFiles))
	applyResult := updateSnapshotFromApplyOutcome(mode, snapshot, config, outcome, message)

	if err := s.stateStore.Save(snapshot); err != nil {
		return err
	}
	if !shouldReportApplyLog(alreadySynced, applyResult.reportResult) {
		slog.Debug("skipping duplicate apply log report", "version", config.Version, "checksum", config.Checksum, "result", applyResult.reportResult)
		if applyResult.reportResult == ApplyResultFailed {
			return outcomeError(config.Version, applyResult.message)
		}
		if err := s.syncReferencedWAFIPGroups(ctx, rendered.supportFiles); err != nil {
			slog.Error("sync referenced waf ip groups failed", "version", config.Version, "error", err)
			return err
		}
		return nil
	}
	if err := s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
		NodeID:              snapshot.NodeID,
		Version:             config.Version,
		Result:              applyResult.reportResult,
		Message:             applyResult.message,
		Checksum:            config.Checksum,
		MainConfigChecksum:  mainConfigChecksum,
		RouteConfigChecksum: routeConfigChecksum,
		SupportFileCount:    len(rendered.supportFiles),
	}); err != nil {
		slog.Error("report apply log failed", "version", config.Version, "result", applyResult.reportResult, "error", err)
		return err
	}
	if applyResult.reportResult == ApplyResultFailed {
		slog.Warn("failed apply log reported", "version", config.Version)
		return outcomeError(config.Version, applyResult.message)
	}
	if err := s.syncReferencedWAFIPGroups(ctx, rendered.supportFiles); err != nil {
		slog.Error("sync referenced waf ip groups failed", "version", config.Version, "error", err)
		return err
	}
	slog.Debug("apply log reported", "version", config.Version, "result", applyResult.reportResult)
	return nil
}

func (s *Service) syncReferencedWAFIPGroups(ctx context.Context, supportFiles []protocol.SupportFile) error {
	ids, err := referencedWAFIPGroupIDs(supportFiles)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		if s.nginxManager == nil {
			return nil
		}
		return s.nginxManager.ReconcileWAFIPGroups([]uint{}, nil)
	}
	checksums, err := s.WAFIPGroupChecksums()
	if err != nil {
		return err
	}
	targetChecksums := make(map[string]string, len(ids))
	for _, id := range ids {
		key := strconv.FormatUint(uint64(id), 10)
		if value := strings.TrimSpace(checksums[key]); value != "" {
			targetChecksums[key] = value
		}
	}
	response, err := s.client.SyncWAFIPGroups(ctx, protocol.WAFIPGroupSyncRequest{
		IDs:       ids,
		Checksums: targetChecksums,
	})
	if err != nil {
		return err
	}
	if s.nginxManager == nil {
		return nil
	}
	var changed []protocol.WAFIPGroup
	if response != nil {
		changed = response.Groups
	}
	return s.nginxManager.ReconcileWAFIPGroups(ids, changed)
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

func referencedWAFIPGroupIDs(supportFiles []protocol.SupportFile) ([]uint, error) {
	var content string
	for _, file := range supportFiles {
		if file.Path == "waf_config.json" {
			content = strings.TrimSpace(file.Content)
			break
		}
	}
	if content == "" {
		return []uint{}, nil
	}
	var payload openrestyrender.WAFDocument
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return nil, fmt.Errorf("decode waf_config.json for ip group references: %w", err)
	}
	seen := make(map[uint]struct{})
	for _, group := range payload.RuleGroups {
		for _, legacyIDs := range [][]uint{group.IPWhitelistGroups, group.IPBlacklistGroups} {
			for _, id := range legacyIDs {
				if id > 0 {
					seen[id] = struct{}{}
				}
			}
		}
		for nodeID, node := range group.Graph.Nodes {
			if node.Type != "ip_match" {
				continue
			}
			var config *struct {
				IPGroupIDs []uint `json:"ip_group_ids"`
			}
			if err := json.Unmarshal(node.Config, &config); err != nil || config == nil {
				if err == nil {
					err = errors.New("config must be a JSON object")
				}
				return nil, fmt.Errorf("decode ip_match config for rule group %d node %s: %w", group.ID, nodeID, err)
			}
			for _, id := range config.IPGroupIDs {
				if id > 0 {
					seen[id] = struct{}{}
				}
			}
		}
	}
	ids := make([]uint, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
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
			fallbackErr := s.nginxManager.EnsureSafeFallbackRuntime(ctx, reason)
			if fallbackErr == nil {
				snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
				snapshot.OpenrestyMessage = "safe default fallback runtime started"
				return nil
			}
			err = fmt.Errorf("%v; fallback recovery failed: %w", err, fallbackErr)
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
