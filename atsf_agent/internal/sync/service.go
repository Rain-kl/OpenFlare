package sync

import (
	"context"

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
	Apply(ctx context.Context, content string) error
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
	snapshot, err := s.stateStore.Load()
	if err != nil {
		return err
	}
	config, err := s.client.GetActiveConfig(ctx)
	if err != nil {
		return err
	}
	currentChecksum, err := s.nginxManager.CurrentChecksum()
	if err != nil {
		return err
	}
	if currentChecksum == config.Checksum {
		if startup {
			if err = s.nginxManager.EnsureRuntime(ctx, true); err != nil {
				return err
			}
		}
		snapshot.CurrentVersion = config.Version
		snapshot.CurrentChecksum = config.Checksum
		snapshot.LastError = ""
		return s.stateStore.Save(snapshot)
	}
	if snapshot.CurrentVersion == config.Version && snapshot.CurrentChecksum == config.Checksum && !startup {
		return nil
	}
	if err = s.nginxManager.Apply(ctx, config.RenderedConfig); err != nil {
		snapshot.LastError = err.Error()
		_ = s.stateStore.Save(snapshot)
		reportErr := s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
			NodeID:  snapshot.NodeID,
			Version: config.Version,
			Result:  ApplyResultFailed,
			Message: err.Error(),
		})
		if reportErr != nil {
			return reportErr
		}
		return err
	}
	snapshot.CurrentVersion = config.Version
	snapshot.CurrentChecksum = config.Checksum
	snapshot.LastError = ""
	if err = s.stateStore.Save(snapshot); err != nil {
		return err
	}
	return s.client.ReportApplyLog(ctx, protocol.ApplyLogPayload{
		NodeID:  snapshot.NodeID,
		Version: config.Version,
		Result:  ApplyResultSuccess,
		Message: "apply success",
	})
}
