package sync

import (
	"context"
	"log"

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
	Apply(ctx context.Context, content string, supportFiles []protocol.SupportFile) error
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

func (s *Service) SyncOnce(ctx context.Context) error {
	return s.sync(ctx, false)
}

func (s *Service) SyncOnStartup(ctx context.Context) error {
	return s.sync(ctx, true)
}

func (s *Service) sync(ctx context.Context, startup bool) error {
	mode := "periodic"
	if startup {
		mode = "startup"
	}
	log.Printf("sync started: mode=%s", mode)
	snapshot, err := s.stateStore.Load()
	if err != nil {
		return err
	}
	config, err := s.client.GetActiveConfig(ctx)
	if err != nil {
		log.Printf("fetch active config failed: mode=%s error=%v", mode, err)
		return err
	}
	log.Printf("active config fetched: mode=%s version=%s checksum=%s support_files=%d", mode, config.Version, config.Checksum, len(config.SupportFiles))
	currentChecksum, err := s.nginxManager.CurrentChecksum()
	if err != nil {
		return err
	}
	log.Printf("current local checksum loaded: mode=%s checksum=%s", mode, currentChecksum)
	if currentChecksum == config.Checksum {
		log.Printf("local openresty config already up to date: mode=%s version=%s", mode, config.Version)
		if startup {
			log.Printf("ensuring openresty runtime on startup: version=%s", config.Version)
			if err = s.nginxManager.EnsureRuntime(ctx, true); err != nil {
				return err
			}
			log.Printf("openresty runtime ensured on startup: version=%s", config.Version)
		}
		snapshot.CurrentVersion = config.Version
		snapshot.CurrentChecksum = config.Checksum
		snapshot.LastError = ""
		log.Printf("sync finished without changes: mode=%s version=%s", mode, config.Version)
		return s.stateStore.Save(snapshot)
	}
	if snapshot.CurrentVersion == config.Version && snapshot.CurrentChecksum == config.Checksum && !startup {
		log.Printf("skipping apply because state already records target version/checksum: version=%s checksum=%s", config.Version, config.Checksum)
		return nil
	}
	log.Printf("applying new openresty config: mode=%s from_version=%s to_version=%s old_checksum=%s new_checksum=%s", mode, snapshot.CurrentVersion, config.Version, currentChecksum, config.Checksum)
	if err = s.nginxManager.Apply(ctx, config.RenderedConfig, config.SupportFiles); err != nil {
		log.Printf("apply openresty config failed: mode=%s version=%s error=%v", mode, config.Version, err)
		snapshot.LastError = err.Error()
		_ = s.stateStore.Save(snapshot)
		reportErr := s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
			NodeID:  snapshot.NodeID,
			Version: config.Version,
			Result:  ApplyResultFailed,
			Message: err.Error(),
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
	if err = s.stateStore.Save(snapshot); err != nil {
		return err
	}
	if err = s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
		NodeID:  snapshot.NodeID,
		Version: config.Version,
		Result:  ApplyResultSuccess,
		Message: "apply success",
	}); err != nil {
		log.Printf("report successful apply log failed: version=%s error=%v", config.Version, err)
		return err
	}
	log.Printf("successful apply log reported: version=%s", config.Version)
	return nil
}
