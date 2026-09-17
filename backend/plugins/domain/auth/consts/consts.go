// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package consts defines constants, keys, and TTL values for the auth domain plugin.
package consts

import "time"

// Session and Context Keys
const (
	UserNameKey     = "username"
	UserIDKey       = "user_id"
	UserObjKey      = "user_obj"
	TokenAuthKey    = "token_auth"          // 标记当前请求是否通过 Access Token 鉴权
	TokenAdminKey   = "token_admin"         // Access Token 本身是否具有管理员权限
	SessionTokenKey = "oauth_session_token" //nolint:gosec // false positive: session state key
	PasswordHashKey = "password_hash"
	SystemUsername  = "system"
)

// OAuth State Cache Keys and Expirations
const (
	OAuthStateCacheKeyFormat     = "oauth:state:%s"
	OAuthStateCacheKeyExpiration = 10 * time.Minute
	OAuthStateLimitKeyFormat     = "oauth:state:limit:%s"
	OAuthStateLimitMax           = 10
)

// OAuth Purpose Constants
const (
	OAuthPurposeLogin = "login"
	OAuthPurposeBind  = "bind"
)

// Auth Source Types
const (
	AuthSourceTypeOIDC = "oidc"
)

// Cache TTLs
const (
	TokenCacheTTL = 5 * time.Minute
	UserCacheTTL  = 5 * time.Minute
)
