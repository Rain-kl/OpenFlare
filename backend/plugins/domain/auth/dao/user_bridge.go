// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dao provides data access objects and caching for the auth domain plugin.
package dao

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/auth/model/do"
	"context"
	"time"
)

// GetAccessTokenByHash 按令牌哈希读取访问令牌记录（仅取鉴权所需字段）
func (d *DAO) GetAccessTokenByHash(ctx context.Context, tokenHash string) (*do.CachedToken, error) {
	var row struct {
		ID      uint64
		UserID  uint64
		IsAdmin bool
	}
	if err := d.DB(ctx).Table("w_access_tokens").Where("token_hash = ?", tokenHash).First(&row).Error; err != nil {
		return nil, err
	}
	return &do.CachedToken{
		ID:      row.ID,
		UserID:  row.UserID,
		IsAdmin: row.IsAdmin,
	}, nil
}

// GetActiveUserByID 读取仍处于启用状态的用户
func (d *DAO) GetActiveUserByID(ctx context.Context, userID uint64) (*contracts.UserDTO, error) {
	var user contracts.UserDTO
	if err := d.DB(ctx).Table("w_users").Where("id = ? AND is_active = ?", userID, true).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID 按 ID 读取用户（不限制启用状态）
func (d *DAO) GetUserByID(ctx context.Context, userID uint64) (*contracts.UserDTO, error) {
	var user contracts.UserDTO
	if err := d.DB(ctx).Table("w_users").Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// InsertUser 新建用户记录
func (d *DAO) InsertUser(ctx context.Context, user *contracts.UserDTO) error {
	return d.DB(ctx).Table("w_users").Create(user).Error
}

// TouchUserLastLogin 刷新用户最后登录时间
func (d *DAO) TouchUserLastLogin(ctx context.Context, userID uint64, at time.Time) error {
	return d.DB(ctx).Table("w_users").Where("id = ?", userID).Update("last_login_at", at).Error
}

// ListSimilarUsernames 查询与基础用户名相同或带 `-序号` 后缀的用户名（用于用户名去重）
func (d *DAO) ListSimilarUsernames(ctx context.Context, base string) ([]string, error) {
	var existingUsernames []string
	if err := d.DB(ctx).Table("w_users").
		Where("username = ? OR username LIKE ? ESCAPE '\\'", base, util.EscapeLike(base)+"-%").
		Pluck("username", &existingUsernames).Error; err != nil {
		return nil, err
	}
	return existingUsernames, nil
}

// GetSystemConfigValue 读取系统配置项原始值
func (d *DAO) GetSystemConfigValue(ctx context.Context, key string) (string, error) {
	var val string
	if err := d.DB(ctx).Table("w_system_configs").Where("key = ?", key).Pluck("value", &val).Error; err != nil {
		return "", err
	}
	return val, nil
}

// ListSystemConfigsByKeys 按键批量读取系统配置项
func (d *DAO) ListSystemConfigsByKeys(ctx context.Context, keys []string) ([]do.CapConfigRecord, error) {
	var records []do.CapConfigRecord
	db := d.DB(ctx)
	if db == nil {
		return nil, nil
	}
	if err := db.Table("w_system_configs").Where("key IN ?", keys).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
