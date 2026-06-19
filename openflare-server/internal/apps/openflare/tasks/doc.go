// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package tasks hosts shared OpenFlare background job business logic.
//
// Scheduled execution is handled by the Wavelet Asynq task framework; handlers
// live in internal/apps/openflare/async_tasks.go and are registered via
// bootstrap.RegisterTasks().
package tasks