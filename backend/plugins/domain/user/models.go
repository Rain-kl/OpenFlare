// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"Wavelet/pkg/util"
	"errors"
	"strings"
	"time"
)

// AccessToken 个人访问令牌实体
type AccessToken struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `json:"user_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"size:128;not null"`
	TokenHash   string    `json:"-" gorm:"size:64;uniqueIndex;not null"`
	MaskedToken string    `json:"masked_token" gorm:"size:64;not null"`
	IsAdmin     bool      `json:"is_admin" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (AccessToken) TableName() string {
	return "w_access_tokens"
}

// User 用户表实体
type User struct {
	ID                 uint64    `json:"id,string" gorm:"primaryKey;not null"`
	Username           string    `json:"username" gorm:"size:64;uniqueIndex"`
	Password           string    `json:"password,omitempty" gorm:"size:255"`
	Nickname           string    `json:"nickname" gorm:"size:255"`
	Email              string    `json:"email" gorm:"size:255;index"`
	AvatarURL          string    `json:"avatar_url" gorm:"size:255"`
	IsActive           bool      `json:"is_active" gorm:"default:true;index"`
	IsAdmin            bool      `json:"is_admin" gorm:"default:false"`
	NeedChangePassword bool      `json:"need_change_password,omitempty" gorm:"-"`
	Bio                string    `json:"bio" gorm:"size:500"`
	Phone              string    `json:"phone" gorm:"size:32"`
	Gender             string    `json:"gender" gorm:"size:16"`
	Website            string    `json:"website" gorm:"size:255"`
	Location           string    `json:"location" gorm:"size:255"`
	LastLoginAt        time.Time `json:"last_login_at" gorm:"index"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 表名
func (User) TableName() string {
	return "w_users"
}

// IsPlaintextPassword 检查当前密码是否为未加密的明文密码（如初始默认密码）
func (u *User) IsPlaintextPassword() bool {
	if u.Password == "" {
		return false
	}
	return !strings.HasPrefix(u.Password, "$2a$") &&
		!strings.HasPrefix(u.Password, "$2b$") &&
		!strings.HasPrefix(u.Password, "$2y$") &&
		!strings.HasPrefix(u.Password, "$2x$")
}

// SetEncryptedPassword 设置加密密码
func (u *User) SetEncryptedPassword(password string) error {
	trimmed := strings.TrimSpace(password)
	if trimmed == "" {
		return errors.New(errPasswordEmpty)
	}
	hash, err := util.HashPassword(trimmed)
	if err != nil {
		return err
	}
	u.Password = hash
	return nil
}

// CheckPassword 校验密码（支持 bcrypt 哈希校验与初始明文密码校验）
func (u *User) CheckPassword(password string) bool {
	if u.Password == "" {
		util.DummyCheckPassword(password)
		return false
	}
	if !u.IsPlaintextPassword() {
		return util.CheckPasswordHash(u.Password, password)
	}

	// 明文密码兼容比对（识别初始默认密码向用户警告修改密码）
	if u.Password == password {
		u.NeedChangePassword = true
		return true
	}
	return false
}

// loginRequest 登录请求参数
type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// registerRequest 注册请求参数
type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

type sendEmailCodeRequest struct {
	Email string `json:"email" binding:"required"`
}

// changePasswordRequest 修改密码请求参数
type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// updateProfileRequest 资料更新请求参数
type updateProfileRequest struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	Phone     string `json:"phone"`
	Gender    string `json:"gender"`
	Website   string `json:"website"`
	Location  string `json:"location"`
}

// createAccessTokenRequest 创建访问令牌请求参数
type createAccessTokenRequest struct {
	Name      string     `json:"name" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at"`
	IsAdmin   bool       `json:"is_admin"`
}

// userAdminFlags 用户写操作前置校验所需的最小列投影
type userAdminFlags struct {
	ID      uint64
	IsAdmin bool
}
