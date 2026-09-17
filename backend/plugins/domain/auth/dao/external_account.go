// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dao provides data access objects and caching for the auth domain plugin.
package dao

import (
	"Wavelet/plugins/domain/auth/model/entity"
	"context"
)

// FindExternalAccount 查询指定认证源的外部账号绑定
func (d *DAO) FindExternalAccount(ctx context.Context, authSourceID uint64, externalID string) (*entity.ExternalAccount, error) {
	var account entity.ExternalAccount
	if err := d.DB(ctx).Where("auth_source_id = ? AND external_id = ?", authSourceID, externalID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// BindExternalAccount 绑定外部账号
func (d *DAO) BindExternalAccount(ctx context.Context, account *entity.ExternalAccount) error {
	return d.DB(ctx).Create(account).Error
}

// ListExternalAccountsByUserID 获取用户绑定的所有外部账号
func (d *DAO) ListExternalAccountsByUserID(ctx context.Context, userID uint64) ([]entity.ExternalAccount, error) {
	var accounts []entity.ExternalAccount
	if err := d.DB(ctx).Where("user_id = ?", userID).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

// UnbindExternalAccount 解绑外部账号
func (d *DAO) UnbindExternalAccount(ctx context.Context, id, userID uint64) error {
	return d.DB(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&entity.ExternalAccount{}).Error
}
