package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"

	"atsflare-agent/internal/protocol"
	"atsflare-agent/internal/state"
)

const (
	ApplyResultSuccess = "success"
	ApplyResultFailed  = "failed"
)

type ConfigClient interface {
	GetActiveConfig(ctx context.Context) (*protocol.ActiveConfigResponse, error)
	ReportApplyLog(ctx context.Context, payload protocol.ApplyLogPayload) error
}

type NginxManager interface {
	Apply(ctx context.Context, mainConfig string, routeConfig string, supportFiles []protocol.SupportFile) error
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
		slog.Info("sync startup fallback: active config summary unavailable, fetching active config directly")
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
		slog.Info("local openresty config already up to date", "mode", mode, "version", target.Version)
		if startup {
			slog.Info("ensuring openresty runtime on startup", "version", target.Version)
			if err = s.nginxManager.EnsureRuntime(ctx, true); err != nil {
				snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
				snapshot.OpenrestyMessage = err.Error()
				_ = s.stateStore.Save(snapshot)
				return err
			}
			slog.Info("openresty runtime ensured on startup", "version", target.Version)
			snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
			snapshot.OpenrestyMessage = ""
		}
		snapshot.CurrentVersion = target.Version
		snapshot.CurrentChecksum = target.Checksum
		snapshot.LastError = ""
		slog.Info("sync finished without changes", "mode", mode, "version", target.Version)
		return s.stateStore.Save(snapshot)
	}
	if snapshot.CurrentVersion == target.Version && snapshot.CurrentChecksum == target.Checksum && !startup {
		slog.Debug("skipping config fetch because state already records target version/checksum", "version", target.Version, "checksum", target.Checksum)
		return nil
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
		slog.Info("local openresty config already up to date", "mode", mode, "version", config.Version)
		if startup {
			slog.Info("ensuring openresty runtime on startup", "version", config.Version)
			if err := s.nginxManager.EnsureRuntime(ctx, true); err != nil {
				snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
				snapshot.OpenrestyMessage = err.Error()
				_ = s.stateStore.Save(snapshot)
				return err
			}
			slog.Info("openresty runtime ensured on startup", "version", config.Version)
			snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
			snapshot.OpenrestyMessage = ""
		}
		snapshot.CurrentVersion = config.Version
		snapshot.CurrentChecksum = config.Checksum
		snapshot.LastError = ""
		slog.Info("sync finished without changes", "mode", mode, "version", config.Version)
		return s.stateStore.Save(snapshot)
	}
	if target != nil && (target.Version != config.Version || target.Checksum != config.Checksum) {
		slog.Warn("active config changed between heartbeat and fetch", "heartbeat_version", target.Version, "heartbeat_checksum", target.Checksum, "fetched_version", config.Version, "fetched_checksum", config.Checksum)
	}
	if snapshot.CurrentVersion == config.Version && snapshot.CurrentChecksum == config.Checksum && !startup {
		slog.Debug("skipping apply because state already records target version/checksum", "version", config.Version, "checksum", config.Checksum)
		return nil
	}
	routeConfig := config.RouteConfig
	if routeConfig == "" {
		routeConfig = config.RenderedConfig
	}
	mainConfigChecksum := checksumString(config.MainConfig)
	routeConfigChecksum := checksumString(routeConfig)
	slog.Info("applying new openresty config", "mode", mode, "from_version", snapshot.CurrentVersion, "to_version", config.Version, "old_checksum", currentChecksum, "new_checksum", config.Checksum)
	if err := s.nginxManager.Apply(ctx, config.MainConfig, routeConfig, config.SupportFiles); err != nil {
		slog.Error("apply openresty config failed", "mode", mode, "version", config.Version, "error", err)
		snapshot.LastError = err.Error()
		snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
		snapshot.OpenrestyMessage = err.Error()
		_ = s.stateStore.Save(snapshot)
		reportErr := s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
			NodeID:              snapshot.NodeID,
			Version:             config.Version,
			Result:              ApplyResultFailed,
			Message:             err.Error(),
			Checksum:            config.Checksum,
			MainConfigChecksum:  mainConfigChecksum,
			RouteConfigChecksum: routeConfigChecksum,
			SupportFileCount:    len(config.SupportFiles),
		})
		if reportErr != nil {
			slog.Error("report failed apply log failed", "version", config.Version, "error", reportErr)
			return reportErr
		}
		slog.Warn("failed apply log reported", "version", config.Version)
		return err
	}
	slog.Info("openresty config applied successfully", "mode", mode, "version", config.Version)
	snapshot.CurrentVersion = config.Version
	snapshot.CurrentChecksum = config.Checksum
	snapshot.LastError = ""
	snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
	snapshot.OpenrestyMessage = ""
	if err := s.stateStore.Save(snapshot); err != nil {
		return err
	}
	if err := s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
		NodeID:              snapshot.NodeID,
		Version:             config.Version,
		Result:              ApplyResultSuccess,
		Message:             "apply success",
		Checksum:            config.Checksum,
		MainConfigChecksum:  mainConfigChecksum,
		RouteConfigChecksum: routeConfigChecksum,
		SupportFileCount:    len(config.SupportFiles),
	}); err != nil {
		slog.Error("report successful apply log failed", "version", config.Version, "error", err)
		return err
	}
	slog.Info("successful apply log reported", "version", config.Version)
	return nil
}

func checksumString(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
