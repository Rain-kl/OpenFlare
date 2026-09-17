// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package consts defines constants, keys, and TTL values for the auth domain plugin.
package consts

// OAuth and Auth error messages
const (
	ErrInvalidState                    = "非法登录请求"
	ErrIDTokenVerifyFailed             = "ID Token 验证失败" //nolint:gosec // error message constant
	ErrIDTokenVerifyFailedFormat       = "%s: %w"
	ErrNonceMismatch                   = "nonce 不匹配，可能存在重放攻击"
	ErrNoActiveAuthSource              = "未配置可用认证源"
	ErrServerAddressMissing            = "服务器地址 (server_address) 未配置或配置为空，请在后台系统设置中配置后再试"
	ErrAuthSourceRequired              = "认证源不能为空"
	ErrDiscoveryURLRequired            = "OIDC 认证源必须配置 Discovery URL"
	ErrUsernameGenerateFailed          = "无法生成可用用户名"
	ErrUsernameFromSourceFailed        = "无法从认证源获取用户名"
	ErrAuthSourceDisabled              = "认证源未启用"
	ErrInvalidExternalAccountBindingID = "绑定记录 ID 无效"
	ErrTokenAuthNotAllowed             = "该端点不允许使用访问令牌进行身份验证" //nolint:gosec // error message constant
	ErrOAuthStateRateLimited           = "请求授权过于频繁，请稍后重试"
	ErrAuthSourceNameRequired          = "认证源名称不能为空"
	ErrAuthSourceNameInvalid           = "认证源名称格式不正确"
	ErrAuthSourceTypeUnsupported       = "不支持的认证源类型"
	ErrAuthSourceDiscoveryURLRequired  = "Discovery URL 不能为空"
	//nolint:gosec // error message constant
	ErrAuthSourceClientCredentialsRequired  = "启用认证源时必须配置 Client ID 和 Client Secret"
	ErrAuthSourceIDRequired                 = "认证源 ID 不能为空"
	ErrUserIDRequired                       = "用户 ID 不能为空"
	ErrExternalAccountBindingIncomplete     = "外部帐号绑定信息不完整"
	ErrExternalAccountAlreadyBoundToAnother = "该外部帐号已被其他用户绑定"
	ErrExternalAccountBindingIDRequired     = "外部帐号绑定记录 ID 不能为空"
	ErrInsufficientPermission               = "权限不足"
	ErrBannedAccount                        = "账号已被封禁"
	ErrUnAuthorized                         = "未登录"
)

// Service 层与鉴权中间件内部错误文案
const (
	ErrUserNotInContext          = "auth: user not found in context"
	ErrEmptyToken                = "auth: empty token"                   //nolint:gosec // error message constant
	ErrSystemUserTokenNotAllowed = "auth: system user token not allowed" //nolint:gosec // error message constant
	ErrUnauthorizedInternal      = "unauthorized"
	ErrSystemUserLoginNotAllowed = "system user is not allowed to login"
)

// OAuth 回调会话校验错误文案
const (
	ErrInvalidSessionContext   = "invalid session context"
	ErrSessionMismatchForOAuth = "session mismatch for oauth state"
	ErrUserContextMismatch     = "user context mismatch for oauth binding"
)
