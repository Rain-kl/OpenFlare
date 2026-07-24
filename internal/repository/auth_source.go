// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// GetAuthSources 获取所有认证源（已脱敏）
func GetAuthSources(ctx context.Context) ([]model.AuthSource, error) {
	var sources []model.AuthSource
	if err := db.DB(ctx).Order("id asc").Find(&sources).Error; err != nil {
		return nil, err
	}
	for i := range sources {
		sources[i].Sanitize()
	}
	return sources, nil
}

// GetActiveAuthSources 获取所有已启用的认证源（已脱敏）
func GetActiveAuthSources(ctx context.Context) ([]model.AuthSource, error) {
	var sources []model.AuthSource
	if err := db.DB(ctx).Where("is_active = ?", true).Order("id asc").Find(&sources).Error; err != nil {
		return nil, err
	}
	for i := range sources {
		sources[i].Sanitize()
	}
	return sources, nil
}

// GetAuthSourceByID 根据 ID 获取认证源
func GetAuthSourceByID(ctx context.Context, id uint64) (*model.AuthSource, error) {
	if id == 0 {
		return nil, errors.New(errAuthSourceIDRequired)
	}
	var source model.AuthSource
	if err := db.DB(ctx).First(&source, "id = ?", id).Error; err != nil {
		return nil, err
	}
	source.ClientSecretConfigured = source.ClientSecret != ""
	return &source, nil
}

// GetAuthSourceByName 根据名称获取认证源（名称比较不区分大小写）
func GetAuthSourceByName(ctx context.Context, name string) (*model.AuthSource, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New(errAuthSourceNameRequired)
	}
	var source model.AuthSource
	if err := db.DB(ctx).First(&source, "LOWER(name) = LOWER(?)", name).Error; err != nil {
		return nil, err
	}
	source.ClientSecretConfigured = source.ClientSecret != ""
	return &source, nil
}

// CreateAuthSource 创建认证源
func CreateAuthSource(ctx context.Context, source *model.AuthSource) error {
	if err := source.Validate(); err != nil {
		return err
	}
	return db.DB(ctx).Create(source).Error
}

// UpdateAuthSource 更新认证源，keepSecret 为 true 时保留原密钥
func UpdateAuthSource(ctx context.Context, source *model.AuthSource, keepSecret bool) error {
	if source.ID == 0 {
		return errors.New(errAuthSourceIDRequired)
	}
	var current model.AuthSource
	if err := db.DB(ctx).First(&current, "id = ?", source.ID).Error; err != nil {
		return err
	}
	if keepSecret {
		source.ClientSecret = current.ClientSecret
	}
	if err := source.Validate(); err != nil {
		return err
	}
	return db.DB(ctx).Model(&current).Updates(map[string]any{
		colName:                source.Name,
		"type":                 source.Type,
		"display_name":         source.DisplayName,
		"is_active":            source.IsActive,
		"client_id":            source.ClientID,
		"client_secret":        source.ClientSecret,
		"openid_discovery_url": source.OpenIDDiscoveryURL,
		"scopes":               source.Scopes,
		"icon_url":             source.IconURL,
	}).Error
}

// ToggleAuthSource 切换认证源启用状态
func ToggleAuthSource(ctx context.Context, id uint64, isActive bool) error {
	source, err := GetAuthSourceByID(ctx, id)
	if err != nil {
		return err
	}
	source.IsActive = isActive
	if err := source.Validate(); err != nil {
		return err
	}
	return db.DB(ctx).Model(&model.AuthSource{}).Where("id = ?", id).Update("is_active", isActive).Error
}

// DeleteAuthSource 删除认证源及其关联的外部帐号绑定
func DeleteAuthSource(ctx context.Context, id uint64) error {
	if id == 0 {
		return errors.New(errAuthSourceIDRequired)
	}
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("auth_source_id = ?", id).Delete(&model.ExternalAccount{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.AuthSource{}, "id = ?", id).Error
	})
}

// FindExternalAccount 查找外部帐号绑定记录
func FindExternalAccount(ctx context.Context, sourceID uint64, externalID string) (*model.ExternalAccount, error) {
	var account model.ExternalAccount
	if err := db.DB(ctx).Where("auth_source_id = ? AND external_id = ?", sourceID, externalID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// BindExternalAccount 绑定外部帐号（已存在时更新用户名和邮箱）
func BindExternalAccount(ctx context.Context, account *model.ExternalAccount) error {
	if account.UserID == 0 || strings.TrimSpace(account.ExternalID) == "" {
		return errors.New(errExternalAccountBindingIncomplete)
	}
	account.ExternalID = strings.TrimSpace(account.ExternalID)
	account.ExternalUsername = strings.TrimSpace(account.ExternalUsername)
	account.Email = strings.TrimSpace(account.Email)

	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.ExternalAccount
		err := tx.Where("auth_source_id = ? AND external_id = ?", account.AuthSourceID, account.ExternalID).First(&current).Error
		if err == nil {
			if current.UserID != account.UserID {
				return errors.New(errExternalAccountAlreadyBoundToAnother)
			}
			return tx.Model(&current).Updates(map[string]any{
				"external_username": account.ExternalUsername,
				"email":             account.Email,
			}).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(account).Error
	})
}

// ListExternalAccountsByUserID 获取指定用户的所有外部帐号绑定视图
func ListExternalAccountsByUserID(ctx context.Context, userID uint64) ([]model.ExternalAccountView, error) {
	if userID == 0 {
		return nil, errors.New(errUserIDRequired)
	}
	var accounts []model.ExternalAccount
	if err := db.DB(ctx).Where("user_id = ?", userID).Order("id asc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	views := make([]model.ExternalAccountView, 0, len(accounts))
	for _, account := range accounts {
		var name, sourceType, label string
		if account.AuthSourceID == 0 {
			name = "default"
			sourceType = "oidc"
			label = "历史认证源"
		} else {
			source, err := GetAuthSourceByID(ctx, account.AuthSourceID)
			if err != nil {
				continue
			}
			name = source.Name
			sourceType = source.Type
			label = source.DisplayName
			if label == "" {
				label = source.Name
			}
		}
		views = append(views, model.ExternalAccountView{
			ID:               account.ID,
			AuthSourceID:     account.AuthSourceID,
			AuthSourceName:   name,
			AuthSourceType:   sourceType,
			AuthSourceLabel:  label,
			ExternalUsername: account.ExternalUsername,
			Email:            account.Email,
			CreatedAt:        account.CreatedAt,
		})
	}
	return views, nil
}

// DeleteExternalAccountForUser 删除指定用户的外部帐号绑定
func DeleteExternalAccountForUser(ctx context.Context, id uint64, userID uint64) error {
	if id == 0 || userID == 0 {
		return errors.New(errExternalAccountBindingIDRequired)
	}
	return db.DB(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.ExternalAccount{}).Error
}
