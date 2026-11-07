// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package admin 提供管理后台功能
package admin

// 管理后台错误消息常量
const (
	AdminRequired          = "Unauthorized access"
	TokenAdminRequired     = "This access token has no administrator privileges and cannot access admin endpoints" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	InvalidAuthSourceID    = "invalid authentication source ID"
	InvalidCursorParam     = "Invalid cursor parameter"
	InvalidTaskExecutionID = "Invalid task execution ID"
)
