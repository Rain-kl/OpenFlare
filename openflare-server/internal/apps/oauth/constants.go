// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"encoding/json"
	"time"
)

// Session 用户信息字段 Key
const (
	UserNameKey     = "username"
	UserIDKey       = "user_id"
	UserObjKey      = "user_obj"
	TokenAuthKey    = "token_auth"          // 标记当前请求是否通过 Access Token 鉴权
	TokenAdminKey   = "token_admin"         // Access Token 本身是否具有管理员权限
	SessionTokenKey = "oauth_session_token" //nolint:gosec // false positive: this is a session key, not hardcoded credentials
	PasswordHashKey = "password_hash"
)

// OAuth State 缓存 Key 格式与过期时间
const (
	OAuthStateCacheKeyFormat     = "oauth:state:%s"
	OAuthStateCacheKeyExpiration = 10 * time.Minute
)

// OAuth 授权用途常量
const (
	OAuthPurposeLogin = "login"
	OAuthPurposeBind  = "bind"
)

type oauthStatePayload struct {
	SourceName  string `json:"source_name"`
	Purpose     string `json:"purpose"`
	UserID      uint64 `json:"user_id,omitempty"`
	SessionHash string `json:"session_hash"`
}

func encodeOAuthStatePayload(payload oauthStatePayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeOAuthStatePayload(value string) (oauthStatePayload, error) {
	var payload oauthStatePayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return oauthStatePayload{}, err
	}
	return payload, nil
}
