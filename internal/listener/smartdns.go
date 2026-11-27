// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package listener

import "context"

// SmartDNSSnapshotDirty is emitted when routing configuration, policies, tags, or domain bindings change.
type SmartDNSSnapshotDirty struct {
	Reason string
}

// SmartDNSSnapshotDirtyHandler handles snapshot invalidation notifications.
type SmartDNSSnapshotDirtyHandler func(ctx context.Context, event SmartDNSSnapshotDirty)

var smartDNSSnapshotDirtyHandlers []SmartDNSSnapshotDirtyHandler

// OnSmartDNSSnapshotDirty registers a listener for SmartDNS snapshot invalidation events.
func OnSmartDNSSnapshotDirty(handler SmartDNSSnapshotDirtyHandler) {
	if handler == nil {
		return
	}
	smartDNSSnapshotDirtyHandlers = append(smartDNSSnapshotDirtyHandlers, handler)
}

// EmitSmartDNSSnapshotDirty dispatches a SmartDNS snapshot dirty event to all registered listeners.
func EmitSmartDNSSnapshotDirty(ctx context.Context, event SmartDNSSnapshotDirty) {
	for _, handler := range smartDNSSnapshotDirtyHandlers {
		handler(ctx, event)
	}
}
