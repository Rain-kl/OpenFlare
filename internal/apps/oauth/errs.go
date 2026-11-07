// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

// OAuth 认证相关错误消息
const (
	errInvalidState                    = "Invalid login request"
	errIDTokenVerifyFailed             = "ID token verification failed" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errIDTokenVerifyFailedFormat       = "%s: %w"
	errNonceMismatch                   = "nonce mismatch; possible replay attack"
	errNoActiveAuthSource              = "no available authentication source is configured"
	errServerAddressMissing            = "server address (server_address) is not configured; configure it in system settings and try again"
	errAuthSourceRequired              = "authentication source must not be empty"
	errDiscoveryURLRequired            = "OIDC authentication sources require a Discovery URL"
	errUsernameGenerateFailed          = "unable to generate an available username"
	errUsernameFromSourceFailed        = "unable to get username from authentication source"
	errAuthSourceDisabled              = "Authentication source is disabled"
	errInvalidExternalAccountBindingID = "Invalid account binding ID"
	ErrTokenAuthNotAllowed             = "This endpoint does not allow access-token authentication" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errOAuthStateRateLimited           = "too many login requests; please try again later"
)
