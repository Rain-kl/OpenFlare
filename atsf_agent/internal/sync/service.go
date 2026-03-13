package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
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
			log.Printf("skipping sync because heartbeat returned no active config summary: mode=%s", mode)
			return nil
		}
		log.Printf("sync startup fallback: active config summary unavailable, fetching active config directly")
		config, fetchErr := s.client.GetActiveConfig(ctx)
		if fetchErr != nil {
			log.Printf("fetch active config failed: mode=%s error=%v", mode, fetchErr)
			return fetchErr
		}
		target = &protocol.ActiveConfigMeta{
			Version:  config.Version,
			Checksum: config.Checksum,
		}
		return s.applyIfNeeded(ctx, mode, startup, snapshot, currentChecksum, target, config)
	}

	if currentChecksum == target.Checksum {
		log.Printf("local openresty config already up to date: mode=%s version=%s", mode, target.Version)
		if startup {
			log.Printf("ensuring openresty runtime on startup: version=%s", target.Version)
			if err = s.nginxManager.EnsureRuntime(ctx, true); err != nil {
				snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
				snapshot.OpenrestyMessage = err.Error()
				_ = s.stateStore.Save(snapshot)
				return err
			}
			log.Printf("openresty runtime ensured on startup: version=%s", target.Version)
			snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
			snapshot.OpenrestyMessage = ""
		}
		snapshot.CurrentVersion = target.Version
		snapshot.CurrentChecksum = target.Checksum
		snapshot.LastError = ""
		log.Printf("sync finished without changes: mode=%s version=%s", mode, target.Version)
		return s.stateStore.Save(snapshot)
	}
	if snapshot.CurrentVersion == target.Version && snapshot.CurrentChecksum == target.Checksum && !startup {
		log.Printf("skipping config fetch because state already records target version/checksum: version=%s checksum=%s", target.Version, target.Checksum)
		return nil
	}

	config, err := s.client.GetActiveConfig(ctx)
	if err != nil {
		log.Printf("fetch active config failed: mode=%s error=%v", mode, err)
		return err
	}
	return s.applyIfNeeded(ctx, mode, startup, snapshot, currentChecksum, target, config)
}

func (s *Service) applyIfNeeded(ctx context.Context, mode string, startup bool, snapshot *state.Snapshot, currentChecksum string, target *protocol.ActiveConfigMeta, config *protocol.ActiveConfigResponse) error {
	if currentChecksum == config.Checksum {
		log.Printf("local openresty config already up to date: mode=%s version=%s", mode, config.Version)
		if startup {
			log.Printf("ensuring openresty runtime on startup: version=%s", config.Version)
			if err := s.nginxManager.EnsureRuntime(ctx, true); err != nil {
				snapshot.OpenrestyStatus = protocol.OpenrestyStatusUnhealthy
				snapshot.OpenrestyMessage = err.Error()
				_ = s.stateStore.Save(snapshot)
				return err
			}
			log.Printf("openresty runtime ensured on startup: version=%s", config.Version)
			snapshot.OpenrestyStatus = protocol.OpenrestyStatusHealthy
			snapshot.OpenrestyMessage = ""
		}
		snapshot.CurrentVersion = config.Version
		snapshot.CurrentChecksum = config.Checksum
		snapshot.LastError = ""
		log.Printf("sync finished without changes: mode=%s version=%s", mode, config.Version)
		return s.stateStore.Save(snapshot)
	}
	if target != nil && (target.Version != config.Version || target.Checksum != config.Checksum) {
		log.Printf("active config changed between heartbeat and fetch: heartbeat_version=%s heartbeat_checksum=%s fetched_version=%s fetched_checksum=%s", target.Version, target.Checksum, config.Version, config.Checksum)
	}
	if snapshot.CurrentVersion == config.Version && snapshot.CurrentChecksum == config.Checksum && !startup {
		log.Printf("skipping apply because state already records target version/checksum: version=%s checksum=%s", config.Version, config.Checksum)
		return nil
	}
	routeConfig := config.RouteConfig
	if routeConfig == "" {
		routeConfig = config.RenderedConfig
	}
	mainConfigChecksum := checksumString(config.MainConfig)
	routeConfigChecksum := checksumString(routeConfig)
	log.Printf("applying new openresty config: mode=%s from_version=%s to_version=%s old_checksum=%s new_checksum=%s", mode, snapshot.CurrentVersion, config.Version, currentChecksum, config.Checksum)
	if err := s.nginxManager.Apply(ctx, config.MainConfig, routeConfig, config.SupportFiles); err != nil {
		log.Printf("apply openresty config failed: mode=%s version=%s error=%v", mode, config.Version, err)
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
			log.Printf("report failed apply log failed: version=%s error=%v", config.Version, reportErr)
			return reportErr
		}
		log.Printf("failed apply log reported: version=%s", config.Version)
		return err
	}
	log.Printf("openresty config applied successfully: mode=%s version=%s", mode, config.Version)
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
		log.Printf("report successful apply log failed: version=%s error=%v", config.Version, err)
		return err
	}
	log.Printf("successful apply log reported: version=%s", config.Version)
	return nil
}

func checksumString(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
