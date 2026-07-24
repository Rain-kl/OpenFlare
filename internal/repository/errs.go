// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

// Persistence and repository-layer parameter messages live here (unexported).
// Domain field validation used by model.Validate stays in internal/model/errs.go;
// repository may call model.Validate and return those errors as-is.
// Keep wording aligned with model where the same user-facing phrase applies,
// but do not import or re-export model unexported consts (would require exporting).
const (
	errDatabaseNotInitialized               = "database not initialized"
	errConfigIntParseFailed                 = "配置 %s 的值 '%s' 无法转换为整数: %w"
	errConfigDecimalParseFailed             = "配置 %s 的值 '%s' 无法转换为decimal: %w"
	errConfigBoolParseFailed                = "配置 %s 的值 '%s' 无法转换为布尔值: %w"
	errParseMenuDisplayConfigFailed         = "解析目录显示配置失败: %w"
	errAuthSourceNameRequired               = "认证源名称不能为空"
	errAuthSourceIDRequired                 = "认证源 ID 不能为空"
	errExternalAccountBindingIncomplete     = "外部账号绑定信息不完整"
	errExternalAccountAlreadyBoundToAnother = "该外部账号已绑定到其他用户"
	errUserIDRequired                       = "用户 ID 不能为空"
	errExternalAccountBindingIDRequired     = "绑定记录 ID 不能为空"
)

const colName = "name"

const colEnabled = "enabled"
