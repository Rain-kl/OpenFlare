// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cap 提供人机验证中间件
package cap

const (
	errCapTokenMissing          = "验证码验证失败，缺少验证码凭证" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errCapTokenInvalidOrExpired = "验证码校验失败或已过期，请重试" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
)
