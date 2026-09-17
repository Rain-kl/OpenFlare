// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dao provides data access objects and caching for the auth domain plugin.
package dao

import (
	"Wavelet/plugins/domain/auth/model/entity"
	"context"
)

// ListAllAuthSources 获取全部认证源（含未启用），按 ID 升序
func (d *DAO) ListAllAuthSources(ctx context.Context) ([]entity.AuthSource, error) {
	var sources []entity.AuthSource
	if err := d.DB(ctx).Order("id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// GetAuthSourceByID 根据 ID 获取认证源
func (d *DAO) GetAuthSourceByID(ctx context.Context, id uint64) (*entity.AuthSource, error) {
	var src entity.AuthSource
	if err := d.DB(ctx).First(&src, id).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

// GetAuthSourceByName 根据名称获取认证源
func (d *DAO) GetAuthSourceByName(ctx context.Context, name string) (*entity.AuthSource, error) {
	var src entity.AuthSource
	if err := d.DB(ctx).Where("name = ?", name).First(&src).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

// ListActiveAuthSources 获取所有启用的认证源
func (d *DAO) ListActiveAuthSources(ctx context.Context) ([]entity.AuthSource, error) {
	var sources []entity.AuthSource
	if err := d.DB(ctx).Where("is_active = ?", true).Order("id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// CreateAuthSource 新建认证源记录
func (d *DAO) CreateAuthSource(ctx context.Context, source *entity.AuthSource) error {
	return d.DB(ctx).Create(source).Error
}

// SaveAuthSource 全量保存认证源记录
func (d *DAO) SaveAuthSource(ctx context.Context, source *entity.AuthSource) error {
	return d.DB(ctx).Save(source).Error
}

// DeleteAuthSource 删除认证源记录
func (d *DAO) DeleteAuthSource(ctx context.Context, source *entity.AuthSource) error {
	return d.DB(ctx).Delete(source).Error
}
