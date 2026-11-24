// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ping

import (
	"context"
	"sync"
	"time"

	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

type pingScheduler struct {
	mu      sync.Mutex
	started bool
	baseCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

var defaultScheduler = &pingScheduler{}

// StartScheduler starts the interval-grouped latency probe scheduler and registers shutdown cleanup.
func StartScheduler(ctx context.Context) {
	defaultScheduler.mu.Lock()
	if defaultScheduler.started {
		defaultScheduler.mu.Unlock()
		return
	}
	defaultScheduler.started = true
	defaultScheduler.baseCtx = context.WithoutCancel(ctx)
	defaultScheduler.mu.Unlock()

	lifecycle.OnShutdown("ping task scheduler", func(context.Context) error {
		StopScheduler()
		return nil
	})

	ReloadSchedule(ctx)
}

// ReloadSchedule reloads all ping tasks from the database and restarts interval tickers.
func ReloadSchedule(ctx context.Context) {
	defaultScheduler.mu.Lock()
	if !defaultScheduler.started {
		defaultScheduler.mu.Unlock()
		return
	}
	baseCtx := defaultScheduler.baseCtx
	defaultScheduler.mu.Unlock()

	tasks, err := repository.ListOpenFlarePingTasks(ctx)
	if err != nil {
		logger.WarnF(ctx, "[Ping] failed to load ping tasks for scheduler reload: %v", err)
		return
	}

	defaultScheduler.replaceLocked(baseCtx, tasks)
}

// StopScheduler stops all active ping interval goroutines and waits for them to exit.
func StopScheduler() {
	defaultScheduler.mu.Lock()
	cancel := defaultScheduler.cancel
	defaultScheduler.cancel = nil
	defaultScheduler.started = false
	defaultScheduler.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	defaultScheduler.wg.Wait()
}

func (s *pingScheduler) replaceLocked(baseCtx context.Context, tasks []*model.OpenFlarePingTask) {
	s.mu.Lock()
	prevCancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()

	if prevCancel != nil {
		prevCancel()
	}
	s.wg.Wait()

	groups := make(map[int][]*model.OpenFlarePingTask)
	for _, task := range tasks {
		if task == nil || task.Interval < minPingIntervalSeconds || len(task.Clients) == 0 {
			continue
		}
		groups[task.Interval] = append(groups[task.Interval], task)
	}
	if len(groups) == 0 {
		return
	}

	runCtx, cancel := context.WithCancel(baseCtx)
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		cancel()
		return
	}
	s.cancel = cancel
	for intervalSec, groupTasks := range groups {
		interval := time.Duration(intervalSec) * time.Second
		cloned := append([]*model.OpenFlarePingTask(nil), groupTasks...)
		s.wg.Add(1)
		go s.runIntervalLoop(runCtx, interval, cloned)
	}
	s.mu.Unlock()
}

func (s *pingScheduler) runIntervalLoop(ctx context.Context, interval time.Duration, tasks []*model.OpenFlarePingTask) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, task := range tasks {
				dispatchTaskToLocalAgents(ctx, task)
			}
		}
	}
}

// dispatchTaskToLocalAgents dispatches a ping task only to edge nodes connected to this API replica,
// preventing duplicate probe dispatches in multi-replica deployments.
func dispatchTaskToLocalAgents(ctx context.Context, task *model.OpenFlarePingTask) {
	dispatch := protocol.PingTaskDispatch{
		PingTaskID: uint64(task.ID),
		PingType:   task.Type,
		PingTarget: task.Target,
	}
	for _, nodeID := range task.Clients {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !ofws.IsAgentConnected(nodeID) {
			continue
		}
		ofws.DispatchPingTask(ctx, nodeID, dispatch)
	}
}
