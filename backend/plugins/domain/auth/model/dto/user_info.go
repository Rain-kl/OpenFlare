// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dto provides data transfer objects and views for the auth plugin.
package dto

import (
	"Wavelet/core/contracts"
	"strconv"
)

// BasicUserInfo 用户基本信息结构体
type BasicUserInfo struct {
	ID                 uint64 `json:"id,string"`
	Username           string `json:"username"`
	Nickname           string `json:"nickname"`
	Email              string `json:"email"`
	AvatarURL          string `json:"avatar_url"`
	IsAdmin            bool   `json:"is_admin"`
	NeedChangePassword bool   `json:"need_change_password"`
	Bio                string `json:"bio"`
	Phone              string `json:"phone"`
	Gender             string `json:"gender"`
	Website            string `json:"website"`
	Location           string `json:"location"`
}

// BuildBasicUserInfo 将 UserDTO 转换为 BasicUserInfo
func BuildBasicUserInfo(user *contracts.UserDTO, needChange bool) BasicUserInfo {
	if user == nil {
		return BasicUserInfo{}
	}
	return BasicUserInfo{
		ID:                 user.ID,
		Username:           user.Username,
		Nickname:           user.Nickname,
		Email:              user.Email,
		AvatarURL:          user.AvatarURL,
		IsAdmin:            user.IsAdmin,
		NeedChangePassword: needChange || user.NeedChangePassword,
		Bio:                user.Bio,
		Phone:              user.Phone,
		Gender:             user.Gender,
		Website:            user.Website,
		Location:           user.Location,
	}
}

// LoginRequiredAuditLog 审计日志结构体
type LoginRequiredAuditLog struct {
	UserID     uint64 `json:"user_id"`
	Username   string `json:"username"`
	ClientIP   string `json:"client_ip"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	RequestURI string `json:"request_uri"`
	UserAgent  string `json:"user_agent"`
	Referer    string `json:"referer"`
}

// ParseUserID parses a string, int, or float64 user ID representation.
func ParseUserID(v any) uint64 {
	switch val := v.(type) {
	case uint64:
		return val
	case int64:
		if val > 0 {
			return uint64(val)
		}
	case int:
		if val > 0 {
			return uint64(val)
		}
	case float64:
		if val > 0 {
			return uint64(val)
		}
	case string:
		if id, err := strconv.ParseUint(val, 10, 64); err == nil {
			return id
		}
	}
	return 0
}
