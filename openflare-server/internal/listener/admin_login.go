// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package listener provides domain event dispatch for cross-module integration.
// Core domains emit events here; operational modules (push, webhooks, etc.)
// subscribe at the application composition root.
package listener

import (
	"context"

	"github.com/Rain-kl/Wavelet/internal/model"
)

// AdminLoggedIn is emitted when an administrator successfully authenticates.
type AdminLoggedIn struct {
	User *model.User
	IP   string
}

// AdminLoggedInHandler handles administrator login domain events.
type AdminLoggedInHandler func(ctx context.Context, event AdminLoggedIn)

var adminLoggedInHandlers []AdminLoggedInHandler

// OnAdminLoggedIn registers a handler for administrator login events.
// Handlers must be registered during application bootstrap before serving traffic.
func OnAdminLoggedIn(handler AdminLoggedInHandler) {
	adminLoggedInHandlers = append(adminLoggedInHandlers, handler)
}

// EmitAdminLoggedIn dispatches an administrator login event to all registered handlers.
func EmitAdminLoggedIn(ctx context.Context, user *model.User, ip string) {
	if user == nil || !user.IsAdmin {
		return
	}

	event := AdminLoggedIn{User: user, IP: ip}
	for _, handler := range adminLoggedInHandlers {
		handler(ctx, event)
	}
}
