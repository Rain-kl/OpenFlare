// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package custom_events

import (
	"github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/listener"
)

// Register wires push notification handlers for domain events and registers
// built-in event metadata. Must be called once during application bootstrap
// before push.SyncEvents.
func Register() {
	push.RegisterBuiltInEvent(AdminLogin)
	listener.OnAdminLoggedIn(handleAdminLogin)
}
