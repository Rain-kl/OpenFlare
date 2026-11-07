// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cap 提供人机验证中间件
package cap

const (
	errCapTokenMissing          = "CAPTCHA verification failed: missing verification credential" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errCapTokenInvalidOrExpired = "CAPTCHA verification failed or expired; please try again"     //nolint:gosec // false positive: this is an error message, not hardcoded credentials
)
