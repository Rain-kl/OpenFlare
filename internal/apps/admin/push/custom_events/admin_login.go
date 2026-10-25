// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package custom_events defines custom push notification events.
package custom_events

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	"github.com/Rain-kl/Wavelet/internal/listener"
)

// AdminLogin is the metadata definition for the admin login event.
var AdminLogin = push.EventMetadata{
	Key:  "admin_login",
	Name: "Administrator login",
	DefaultTemplate: push.NotificationMessage{
		Title:   "Administrator login notification",
		Content: "Administrator {{user.username}} signed in at {{time}} from IP {{ip}}.",
		Level:   "INFO",
	},
	Description: "Triggered when an administrator signs in successfully",
}

func handleAdminLogin(ctx context.Context, event listener.AdminLoggedIn) {
	if event.User == nil {
		return
	}

	body := map[string]any{
		"user": event.User,
		"ip":   event.IP,
		"time": time.Now().Format("2006-01-02 15:04:05"),
	}
	push.DefaultTrigger.Trigger(ctx, AdminLogin, body)
}
