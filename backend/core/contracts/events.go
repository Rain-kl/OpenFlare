// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

// ======================================================================
// Domain Event Topic Constants
// ======================================================================
//
// All cross-plugin domain event topics MUST be declared here so that
// producers and consumers share the same string values without importing
// each other's implementation packages.
// ======================================================================

// --- Auth & User Events ---
const (
	// EventTopicAdminLoggedIn fires when an admin user logs in.
	EventTopicAdminLoggedIn = "admin:logged_in"

	// EventTopicUserCreated fires when a new user account is created.
	EventTopicUserCreated = "user:created"

	// EventTopicUserUpdated fires when a user profile is updated.
	EventTopicUserUpdated = "user:updated"

	// EventTopicUserDeleted fires when a user account is deleted.
	EventTopicUserDeleted = "user:deleted"

	// EventTopicUserStatusChanged fires when a user account active status changes.
	EventTopicUserStatusChanged = "user:status_changed"

	// EventTopicTokenRevoked fires when an access token is revoked.
	// #nosec G101
	EventTopicTokenRevoked = "auth:token_revoked"
)

// --- Admin & System Events ---
const (
	// EventTopicConfigChanged fires when a system configuration value changes.
	EventTopicConfigChanged = "admin:config_changed"

	// EventTopicSystemCleanup fires when a periodic system cleanup completes.
	EventTopicSystemCleanup = "admin:system_cleanup"
)

// --- Task Events ---
const (
	// EventTopicTaskCompleted fires when an asynchronous background task execution finishes.
	EventTopicTaskCompleted = "task:completed"
)

// TaskCompletedEvent carries task execution outcome details.
type TaskCompletedEvent struct {
	TaskID    string `json:"task_id"`
	TaskName  string `json:"task_name"`
	TaskType  string `json:"task_type"`
	Status    string `json:"status"`
	Duration  int64  `json:"duration"`
	ErrorMsg  string `json:"error_msg,omitempty"`
	ResultMsg string `json:"result_msg,omitempty"`
	Payload   string `json:"payload,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// --- Upload / Storage Events ---
const (
	// EventTopicUploadCreated fires when a new file upload is recorded.
	EventTopicUploadCreated = "upload:created"

	// EventTopicUploadDeleted fires when a file upload is removed.
	EventTopicUploadDeleted = "upload:deleted"

	// EventTopicIngestComplete fires when a programmatic file ingest finishes.
	EventTopicIngestComplete = "upload:ingest_complete"
)

// --- Message Gateway Events ---
const (
	// EventTopicNotificationSent fires when a push notification is dispatched.
	EventTopicNotificationSent = "message:notification_sent"

	// EventTopicChannelBound fires when a user binds a messaging channel.
	EventTopicChannelBound = "message:channel_bound"

	// EventTopicChannelUnbound fires when a user unbinds a messaging channel.
	EventTopicChannelUnbound = "message:channel_unbound"
)

// --- Risk Control Events ---
const (
	// EventTopicAccessLogRecorded fires when a user access log entry is recorded.
	EventTopicAccessLogRecorded = "risk:access_log_recorded"
)

// ======================================================================
// Domain Event Payload DTOs
// ======================================================================

// AdminLoggedIn 管理员登录领域事件载荷
type AdminLoggedIn struct {
	User *UserDTO `json:"user"`
	IP   string   `json:"ip"`
}

// UserCreatedEvent fires when a new user account is created.
type UserCreatedEvent struct {
	User     *UserDTO `json:"user"`
	Password string   `json:"-"`
}

// ConfigChangedEvent fires when a system configuration value changes.
type ConfigChangedEvent struct {
	Key    string `json:"key"`
	OldVal any    `json:"old_val,omitempty"`
	NewVal any    `json:"new_val,omitempty"`
}

// UploadCreatedEvent fires when a new file upload is recorded.
type UploadCreatedEvent struct {
	UploadID uint64 `json:"upload_id,string"`
	UserID   uint64 `json:"user_id,string"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
}

// NotificationSentEvent fires when a push notification is dispatched.
type NotificationSentEvent struct {
	UserID    uint64 `json:"user_id,string"`
	Channel   string `json:"channel"`
	Title     string `json:"title"`
	Success   bool   `json:"success"`
	ErrorInfo string `json:"error_info,omitempty"`
}

// UserStatusChangedEvent fires when a user status is enabled/disabled.
type UserStatusChangedEvent struct {
	UserID   uint64 `json:"user_id,string"`
	IsActive bool   `json:"is_active"`
}

// TokenRevokedEvent fires when an access token is revoked.
type TokenRevokedEvent struct {
	UserID    uint64 `json:"user_id,string"`
	TokenHash string `json:"token_hash"`
}

// UserDeletedEvent fires when a user account is deleted.
type UserDeletedEvent struct {
	CurrentUserID uint64 `json:"current_user_id,string"`
	TargetUserID  uint64 `json:"target_user_id,string"`
}

// SystemCleanupEvent fires when a periodic system cleanup is triggered.
type SystemCleanupEvent struct {
	TriggeredAt string `json:"triggered_at"`
}
