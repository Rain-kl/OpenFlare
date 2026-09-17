// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dto provides data transfer objects and views for the auth plugin.
package dto

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
type OAuthAuthorizeResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

// OAuthCallbackResult 回调处理结果
type OAuthCallbackResult struct {
	Status string         `json:"status"`
	User   *BasicUserInfo `json:"user,omitempty"`
}

// CallbackRequest OAuth 回调请求参数
type CallbackRequest struct {
	State string `json:"state" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// ExternalAccountView 外部帐号绑定视图（脱敏展示用）
type ExternalAccountView struct {
	ID               uint64 `json:"id"`
	AuthSourceID     uint64 `json:"auth_source_id"`
	AuthSourceName   string `json:"auth_source_name"`
	AuthSourceType   string `json:"auth_source_type"`
	AuthSourceLabel  string `json:"auth_source_label"`
	ExternalUsername string `json:"external_username"`
	Email            string `json:"email"`
	CreatedAt        string `json:"created_at"`
}
