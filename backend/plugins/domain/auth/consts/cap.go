// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package consts defines constants, keys, and TTL values for the auth domain plugin.
package consts

import "time"

// CAP 默认参数
const (
	DefaultCapChallengeCount      = 1
	DefaultCapChallengeSize       = 32
	DefaultCapChallengeDifficulty = 4
	DefaultCapChallengeTTL        = 10 * time.Minute
	DefaultCapTokenTTL            = 20 * time.Minute

	RedeemTokenIDLength  = 8  // 兑换 Token ID 字节长度
	RedeemVerTokenLength = 15 // 兑换验证 Token 字节长度
	TokenPartsCount      = 2  // 兑换 Token 由两部分组成 (id:token)
	ValuePartsCount      = 2  // 存储值由 scope 和过期时间组成 (expNano|scope)
)

// CAP 动态配置键常量
const (
	ConfigKeyCapLoginEnabled        = "cap_login_enabled"
	ConfigKeyCapChallengeCount      = "cap_challenge_count"
	ConfigKeyCapChallengeSize       = "cap_challenge_size"
	ConfigKeyCapChallengeDifficulty = "cap_challenge_difficulty"
	ConfigKeyCapChallengeTTL        = "cap_challenge_ttl"
	// ConfigKeyCapTokenTTL 验证码 Token 过期时间键
	// #nosec G101
	ConfigKeyCapTokenTTL = "cap_token_ttl"
)

// HTTP 响应错误文案
const (
	ErrCapTokenMissing          = "验证码验证失败，缺少验证码凭证" //nolint:gosec // error message constant
	ErrCapTokenInvalidOrExpired = "验证码校验失败或已过期，请重试" //nolint:gosec // error message constant
	ErrCapNotConfigured         = "captcha is not configured"
	ErrChallengeGenerateFailed  = "生成验证难题失败，请稍后再试"
	ErrInvalidRequestParams     = "无效的参数"
	ErrSolutionVerifyFailed     = "校验验证解答失败，请稍后再试"
)

// Redeem 结果码，属于 redeem 响应 JSON 的对外契约取值，禁止改写取值
const (
	RedeemErrInvalidToken     = "invalid_token"
	RedeemErrNonceStoreFailed = "nonce_store_error"
	RedeemErrAlreadyRedeemed  = "already_redeemed"
	RedeemErrSettingsLoad     = "settings_load_error"
	RedeemErrTokenStoreFailed = "token_store_error" //nolint:gosec // error code constant
)
