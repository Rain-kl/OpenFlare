// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package bootstrap wires cross-module integrations and process-level subsystem initialization.
// All registrations use sync.Once so entry points can call them safely without import-order side effects.
package bootstrap

import (
	"context"
	"sync"

	admin_push "github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/apps/admin/push/custom_events"
	"github.com/Rain-kl/Wavelet/internal/apps/risk_control"
	taskhandlers "github.com/Rain-kl/Wavelet/internal/task/handlers"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

// Options selects role-specific runtime bootstrap steps for the current process.
type Options struct {
	// API enables HTTP-only subsystems such as the ClickHouse access-log writer.
	API bool
}

var (
	registerTasksOnce            sync.Once
	registerPushDomainEventsOnce sync.Once
	registerTaskListenersOnce    sync.Once
	initRuntimeOnce              sync.Once
)

// RegisterTasks registers all built-in task handlers and metadata.
func RegisterTasks() {
	registerTasksOnce.Do(func() {
		taskhandlers.Register()
	})
}

// RegisterPushDomainEvents wires push notification handlers for domain events.
func RegisterPushDomainEvents() {
	registerPushDomainEventsOnce.Do(func() {
		custom_events.Register()
	})
}

// RegisterTaskListeners wires operational listeners to task framework hooks.
func RegisterTaskListeners() {
	registerTaskListenersOnce.Do(func() {
		admin_push.RegisterTaskListeners()
	})
}

// RegisterAPI wires integrations required by the HTTP API process.
func RegisterAPI() {
	RegisterTasks()
	RegisterPushDomainEvents()
}

// RegisterWorker wires integrations required by the task worker process.
func RegisterWorker() {
	RegisterTasks()
	RegisterTaskListeners()
}

// RegisterScheduler wires integrations required by the task scheduler process.
func RegisterScheduler() {
	RegisterTasks()
}

// RegisterAll wires integrations for fused mode (API + Worker + Scheduler).
func RegisterAll() {
	RegisterTasks()
	RegisterPushDomainEvents()
	RegisterTaskListeners()
}

// Init runs shared runtime bootstrap exactly once per process.
// Call from cmd entry points after wiring registration and database migration, not from router.
func Init(ctx context.Context, opts Options) {
	initRuntimeOnce.Do(func() {
		if err := admin_push.SyncEvents(ctx); err != nil {
			logger.ErrorF(ctx, "[Bootstrap] sync push events failed: %v", err)
		}
		if opts.API {
			risk_control.InitLogWriter(ctx)
		}
	})
}

// ResetInitRuntimeOnceForTest clears initRuntimeOnce so Init can run again in unit tests.
func ResetInitRuntimeOnceForTest() {
	initRuntimeOnce = sync.Once{}
}