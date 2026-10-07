// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/protocol"
)

const (
	configSyncRetryInitialDelay = 10 * time.Second
	configSyncRetryMaxDelay     = 5 * time.Minute
)

type configSyncRetryEvent struct {
	target  *protocol.ActiveConfigMeta
	resolve bool
}

type configSyncRetry struct {
	syncService SyncService
	recordError func(error)
	mu          sync.Mutex
	pending     *configSyncRetryEvent
	wake        chan struct{}
}

func newConfigSyncRetry(syncService SyncService, recordError func(error)) *configSyncRetry {
	return &configSyncRetry{
		syncService: syncService,
		recordError: recordError,
		wake:        make(chan struct{}, 1),
	}
}

func (r *configSyncRetry) schedule(target *protocol.ActiveConfigMeta) {
	r.notify(configSyncRetryEvent{target: cloneActiveConfigMeta(target)})
}

func (r *configSyncRetry) resolve() {
	r.notify(configSyncRetryEvent{resolve: true})
}

func (r *configSyncRetry) notify(event configSyncRetryEvent) {
	r.mu.Lock()
	r.pending = &event
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *configSyncRetry) takePending() (configSyncRetryEvent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending == nil {
		return configSyncRetryEvent{}, false
	}
	event := *r.pending
	r.pending = nil
	return event, true
}

func (r *configSyncRetry) run(ctx context.Context) {
	var target *protocol.ActiveConfigMeta
	var timer *time.Timer
	var timerC <-chan time.Time
	attempt := 0
	pending := false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
			event, ok := r.takePending()
			if !ok {
				continue
			}
			if event.resolve {
				target = nil
				attempt = 0
				pending = false
				if timer != nil {
					timer.Stop()
				}
				timerC = nil
				continue
			}
			if !pending || !sameConfigTarget(target, event.target) {
				target = event.target
				attempt = 0
				pending = true
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(configSyncRetryDelay(attempt))
				timerC = timer.C
			}
		case <-timerC:
			if err := r.syncService.ForceSyncOnce(ctx, target); err != nil {
				attempt++
				delay := configSyncRetryDelay(attempt)
				if r.recordError != nil {
					r.recordError(err)
				}
				slog.Warn("agent forced config sync retry failed", "attempt", attempt, "retry_after", delay, "error", err)
				timer.Reset(delay)
				timerC = timer.C
				continue
			}
			slog.Info("agent forced config sync retry succeeded", "attempt", attempt+1)
			target = nil
			attempt = 0
			pending = false
			timerC = nil
		}
	}
}

func configSyncRetryDelay(attempt int) time.Duration {
	delay := configSyncRetryInitialDelay
	for i := 0; i < attempt && delay < configSyncRetryMaxDelay; i++ {
		delay *= 2
		if delay > configSyncRetryMaxDelay {
			return configSyncRetryMaxDelay
		}
	}
	return delay
}

func sameConfigTarget(left, right *protocol.ActiveConfigMeta) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return strings.TrimSpace(left.Version) == strings.TrimSpace(right.Version) &&
		strings.TrimSpace(left.Checksum) == strings.TrimSpace(right.Checksum)
}

func cloneActiveConfigMeta(target *protocol.ActiveConfigMeta) *protocol.ActiveConfigMeta {
	if target == nil {
		return nil
	}
	cloned := *target
	return &cloned
}
