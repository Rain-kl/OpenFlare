// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import "context"

// PushNotificationTemplate defines notification message template payload.
type PushNotificationTemplate struct {
	Title   string
	Content string
	Level   string
	Ext     map[string]any
}

// PushEventMeta defines metadata for a system push event.
type PushEventMeta struct {
	Key             string
	Name            string
	Description     string
	DefaultTemplate PushNotificationTemplate
}

// PushRegistry defines the interface for registering built-in events.
type PushRegistry interface {
	RegisterBuiltInEvent(meta PushEventMeta)
	SyncEvents(ctx context.Context) error
}
