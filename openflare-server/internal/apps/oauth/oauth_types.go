// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

// AuthSourceView 登录源展示信息
type AuthSourceView struct {
	ID                     uint64 `json:"id"`
	Name                   string `json:"name"`
	Type                   string `json:"type"`
	DisplayName            string `json:"display_name"`
	IsActive               bool   `json:"is_active"`
	IconURL                string `json:"icon_url"`
	ClientSecretConfigured bool   `json:"client_secret_configured"`
}

// OAuthAuthorizeResponse 授权 URL 响应
//
//nolint:revive // OAuth 前缀保持包内语义清晰
type OAuthAuthorizeResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

// OAuthCallbackResult 回调处理结果
//
//nolint:revive // OAuth 前缀保持包内语义清晰
type OAuthCallbackResult struct {
	Status string         `json:"status"`
	User   *BasicUserInfo `json:"user,omitempty"`
}

// CallbackRequest OAuth 回调请求参数
type CallbackRequest struct {
	State string `json:"state" binding:"required"`
	Code  string `json:"code" binding:"required"`
}
