// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package admin 提供管理后台功能
package admin

// 管理后台错误消息常量
const (
	AdminRequired          = "未经授权访问"
	TokenAdminRequired     = "该访问令牌没有管理员权限，无法访问管理端点" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	InvalidAuthSourceID    = "认证源 ID 无效"
	InvalidCursorParam     = "无效的 cursor 参数"
	InvalidTaskExecutionID = "无效的任务执行记录 ID"
)
