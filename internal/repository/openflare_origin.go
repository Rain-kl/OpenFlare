// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// WithOriginTx runs fn inside a database transaction for origin multi-step work.
func WithOriginTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return db.DB(ctx).Transaction(fn)
}

// HasProxyRoutesTable 判断代理规则表是否已迁移。
func HasProxyRoutesTable(ctx context.Context) bool {
	return db.DB(ctx).Migrator().HasTable(&model.OriginProxyRoute{})
}

// ListOrigins 列出全部源站。
func ListOrigins(ctx context.Context) ([]model.Origin, error) {
	var origins []model.Origin
	if err := db.DB(ctx).Order("id desc").Find(&origins).Error; err != nil {
		return nil, err
	}
	return origins, nil
}

// GetOriginByID 按 ID 查询源站。
func GetOriginByID(ctx context.Context, id uint) (*model.Origin, error) {
	var origin model.Origin
	if err := db.DB(ctx).First(&origin, id).Error; err != nil {
		return nil, err
	}
	return &origin, nil
}

// GetOriginByAddress 按地址查询源站。
func GetOriginByAddress(ctx context.Context, address string) (*model.Origin, error) {
	var origin model.Origin
	if err := db.DB(ctx).Where("address = ?", address).First(&origin).Error; err != nil {
		return nil, err
	}
	return &origin, nil
}

// CreateOriginRecord 创建源站。
func CreateOriginRecord(ctx context.Context, origin *model.Origin) error {
	return db.DB(ctx).Create(origin).Error
}

// SaveOrigin 保存源站。
func SaveOrigin(ctx context.Context, origin *model.Origin) error {
	return SaveOriginTx(db.DB(ctx), origin)
}

// SaveOriginTx saves an origin within an existing transaction.
func SaveOriginTx(tx *gorm.DB, origin *model.Origin) error {
	return tx.Save(origin).Error
}

// DeleteOriginRecord 删除源站。
func DeleteOriginRecord(ctx context.Context, id uint) error {
	return db.DB(ctx).Delete(&model.Origin{}, id).Error
}

// ListOriginRouteCounts 统计各源站关联的代理规则数量。
func ListOriginRouteCounts(ctx context.Context) ([]model.OriginRouteCount, error) {
	if !HasProxyRoutesTable(ctx) {
		return nil, nil
	}
	result := make([]model.OriginRouteCount, 0)
	err := db.DB(ctx).Model(&model.OriginProxyRoute{}).
		Select("origin_id, COUNT(*) AS route_count").
		Where("origin_id IS NOT NULL").
		Group("origin_id").
		Scan(&result).Error
	return result, err
}

// ListProxyRoutesByOriginID 列出源站关联的代理规则。
func ListProxyRoutesByOriginID(ctx context.Context, originID uint) ([]model.OriginProxyRoute, error) {
	if !HasProxyRoutesTable(ctx) {
		return nil, nil
	}
	var routes []model.OriginProxyRoute
	if err := db.DB(ctx).Where("origin_id = ?", originID).Order("id desc").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

// ListProxyRoutesByOriginIDAscTx lists origin-linked proxy routes ordered by id asc within a transaction.
func ListProxyRoutesByOriginIDAscTx(tx *gorm.DB, originID uint) ([]model.OriginProxyRoute, error) {
	var routes []model.OriginProxyRoute
	if err := tx.Where("origin_id = ?", originID).Order("id asc").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

// UpdateProxyRouteOriginAddressTx updates a proxy route's origin_url and upstreams within a transaction.
func UpdateProxyRouteOriginAddressTx(tx *gorm.DB, routeID uint, originURL, upstreamsJSON string) error {
	return tx.Model(&model.OriginProxyRoute{}).
		Where("id = ?", routeID).
		Updates(map[string]any{
			"origin_url": originURL,
			"upstreams":  upstreamsJSON,
		}).Error
}

// CountProxyRoutesByOriginID 统计源站关联的代理规则数量。
func CountProxyRoutesByOriginID(ctx context.Context, originID uint) (int64, error) {
	if !HasProxyRoutesTable(ctx) {
		return 0, nil
	}
	var count int64
	if err := db.DB(ctx).Model(&model.OriginProxyRoute{}).Where("origin_id = ?", originID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
