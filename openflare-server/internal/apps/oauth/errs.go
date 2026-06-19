// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

// OAuth 认证相关错误消息
const (
	errInvalidState                    = "非法登录请求"
	errIDTokenVerifyFailed             = "ID Token 验证失败" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errIDTokenVerifyFailedFormat       = "%s: %w"
	errNonceMismatch                   = "nonce 不匹配，可能存在重放攻击"
	errNoActiveAuthSource              = "未配置可用认证源"
	errServerAddressMissing            = "服务器地址 (server_address) 未配置或配置为空，请在后台系统设置中配置后再试"
	errAuthSourceRequired              = "认证源不能为空"
	errDiscoveryURLRequired            = "OIDC 认证源必须配置 Discovery URL"
	errUsernameGenerateFailed          = "无法生成可用用户名"
	errUsernameFromSourceFailed        = "无法从认证源获取用户名"
	errAuthSourceDisabled              = "认证源未启用"
	errInvalidExternalAccountBindingID = "绑定记录 ID 无效"
	ErrTokenAuthNotAllowed             = "该端点不允许使用访问令牌进行身份验证" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
)
