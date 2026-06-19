// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/common"
	"github.com/Rain-kl/Wavelet/internal/db/idgen"
	"github.com/Rain-kl/Wavelet/pkg/util"
	"gorm.io/gorm"
)

// OAuthUserInfo 用户信息结构（同时支持 OIDC ID Token claims 和 UserEndpoint 响应）
type OAuthUserInfo struct {
	ID                uint64 `json:"id"`
	Sub               string `json:"sub"`
	Username          string `json:"username"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	Active            bool   `json:"active"`
	AvatarURL         string `json:"avatar_url"`
}

// GetID 获取用户 ID
func (u *OAuthUserInfo) GetID() uint64 {
	if u.ID != 0 {
		return u.ID
	}
	// 从 sub 解析（OIDC 格式）
	if u.Sub != "" {
		if id, err := strconv.ParseUint(u.Sub, 10, 64); err == nil {
			return id
		}
	}
	return 0
}

// User 用户表实体
type User struct {
	ID          uint64    `json:"id,string" gorm:"primaryKey;not null"`
	Username    string    `json:"username" gorm:"size:64;uniqueIndex"`
	Password    string    `json:"password,omitempty" gorm:"size:255"`
	Nickname    string    `json:"nickname" gorm:"size:255"`
	Email       string    `json:"email" gorm:"size:255;index"`
	AvatarURL   string    `json:"avatar_url" gorm:"size:255"`
	IsActive    bool      `json:"is_active" gorm:"default:true;index"`
	IsAdmin     bool      `json:"is_admin" gorm:"default:false"`
	Bio         string    `json:"bio" gorm:"size:500"`
	Phone       string    `json:"phone" gorm:"size:32"`
	Gender      string    `json:"gender" gorm:"size:16"`
	Website     string    `json:"website" gorm:"size:255"`
	Location    string    `json:"location" gorm:"size:255"`
	LastLoginAt time.Time `json:"last_login_at" gorm:"index"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 表名
func (User) TableName() string {
	return "w_users"
}

// SetPassword 设置明文密码
func (u *User) SetPassword(password string) error {
	u.Password = password
	return nil
}

// SetEncryptedPassword 设置加密密码
func (u *User) SetEncryptedPassword(password string) error {
	if password == "" {
		u.Password = ""
		return nil
	}
	hashed, err := util.HashPassword(password)
	if err != nil {
		return err
	}
	u.Password = hashed
	return nil
}

// IsPasswordEncrypted 检查密码是否已加密
func (u *User) IsPasswordEncrypted() bool {
	return strings.HasPrefix(u.Password, "$2a$") || strings.HasPrefix(u.Password, "$2b$") || strings.HasPrefix(u.Password, "$2y$")
}

// CheckPassword 验证密码是否匹配
func (u *User) CheckPassword(password string) bool {
	if u.Password == "" || password == "" {
		return false
	}
	if u.IsPasswordEncrypted() {
		return util.CheckPasswordHash(u.Password, password)
	}
	return u.Password == password
}

// GetByID 根据 ID 查询用户
func (u *User) GetByID(tx *gorm.DB, id uint64) error {
	if err := tx.Where("id = ?", id).First(u).Error; err != nil {
		return err
	}
	return nil
}

// UpdateFromOAuthInfo 根据 OAuth 信息更新用户数据
func (u *User) UpdateFromOAuthInfo(oauthInfo *OAuthUserInfo) {
	u.Username = oauthInfo.Username
	u.Nickname = oauthInfo.Name
	u.Email = oauthInfo.Email
	u.AvatarURL = oauthInfo.AvatarURL
	u.IsActive = oauthInfo.Active
	u.LastLoginAt = time.Now()
}

// CheckActive 检查用户账户是否激活,未激活则返回错误
func (u *User) CheckActive() error {
	if !u.IsActive {
		return errors.New(common.BannedAccount)
	}
	return nil
}

func (u *User) assignIDIfMissing() error {
	if u.ID != 0 {
		return nil
	}
	u.ID = idgen.NextUint64ID()
	return nil
}

// CreateUser 创建新用户（用于 OAuth/OIDC 自动注册，含底层权限校验）
func (u *User) CreateUser(_ context.Context, tx *gorm.DB, oauthInfo *OAuthUserInfo) error {
	now := time.Now()
	userID := oauthInfo.GetID()
	newUser := User{
		ID:          userID,
		Username:    oauthInfo.Username,
		Nickname:    oauthInfo.Name,
		Email:       oauthInfo.Email,
		AvatarURL:   oauthInfo.AvatarURL,
		IsActive:    oauthInfo.Active,
		LastLoginAt: now,
		IsAdmin:     false,
	}
	if err := newUser.assignIDIfMissing(); err != nil {
		return err
	}
	if err := tx.Create(&newUser).Error; err != nil {
		return err
	}

	*u = newUser
	return nil
}

// RegisterUser 创建新用户并注册（用于本地密码注册，含全局开关和唯一性多重底层校验）
func (u *User) RegisterUser(_ context.Context, tx *gorm.DB) error {
	// 检查用户名冲突
	var count int64
	if err := tx.Model(&User{}).Where("username = ?", u.Username).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New(errUsernameExists)
	}

	// 检查邮箱冲突
	if u.Email != "" {
		var emailCount int64
		if err := tx.Model(&User{}).Where("email = ?", u.Email).Count(&emailCount).Error; err != nil {
			return err
		}
		if emailCount > 0 {
			return errors.New(errEmailAlreadyBound)
		}
	}

	if err := u.assignIDIfMissing(); err != nil {
		return err
	}
	if err := tx.Create(u).Error; err != nil {
		return err
	}
	return nil
}
