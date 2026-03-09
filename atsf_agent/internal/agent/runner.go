package agent

import (
	"context"
	"time"

	"atsflare-agent/internal/config"
	"atsflare-agent/internal/heartbeat"
	"atsflare-agent/internal/protocol"
	"atsflare-agent/internal/state"
	syncservice "atsflare-agent/internal/sync"
)

type Runner struct {
	Config           *config.Config
	StateStore       *state.Store
	HeartbeatService *heartbeat.Service
	SyncService      *syncservice.Service
}

func (r *Runner) Run(ctx context.Context) error {
	nodeID, err := r.StateStore.EnsureNodeID()
	if err != nil {
		return err
	}
	if err = r.SyncService.SyncOnStartup(ctx); err != nil {
		return err
	}
	if err = r.HeartbeatService.Register(ctx, r.nodePayload(nodeID)); err != nil {
		return err
	}

	heartbeatTicker := time.NewTicker(r.Config.HeartbeatInterval)
	defer heartbeatTicker.Stop()
	syncTicker := time.NewTicker(r.Config.SyncInterval)
	defer syncTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeatTicker.C:
			if err = r.HeartbeatService.Heartbeat(ctx, r.nodePayload(nodeID)); err != nil {
				return err
			}
		case <-syncTicker.C:
			if err = r.SyncService.SyncOnce(ctx); err != nil {
				return err
			}
		}
	}
}

func (r *Runner) nodePayload(nodeID string) protocol.NodePayload {
	snapshot, _ := r.StateStore.Load()
	return protocol.NodePayload{
		NodeID:         nodeID,
		Name:           r.Config.NodeName,
		IP:             r.Config.NodeIP,
		AgentVersion:   r.Config.AgentVersion,
		NginxVersion:   r.Config.NginxVersion,
		CurrentVersion: snapshot.CurrentVersion,
		LastError:      snapshot.LastError,
	}
}
