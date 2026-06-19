// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
)

// Origin OpenFlare 源站实体。
type Origin struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"size:255;not null"`
	Address   string    `json:"address" gorm:"uniqueIndex;size:255;not null"`
	Remark    string    `json:"remark" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (Origin) TableName() string {
	return "of_origins"
}

// OriginRouteCount 源站关联的代理规则数量。
type OriginRouteCount struct {
	OriginID   uint  `json:"origin_id"`
	RouteCount int64 `json:"route_count"`
}

// OriginProxyRoute 源站模块查询代理规则时使用的最小字段集。
type OriginProxyRoute struct {
	ID        uint      `gorm:"column:id;primaryKey"`
	OriginID  *uint     `gorm:"column:origin_id"`
	Domain    string    `gorm:"column:domain"`
	OriginURL string    `gorm:"column:origin_url"`
	Upstreams string    `gorm:"column:upstreams"`
	Enabled   bool      `gorm:"column:enabled"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName 表名。
func (OriginProxyRoute) TableName() string {
	return "of_proxy_routes"
}

// HasProxyRoutesTable 判断代理规则表是否已迁移。
func HasProxyRoutesTable(ctx context.Context) bool {
	return db.DB(ctx).Migrator().HasTable(&OriginProxyRoute{})
}

// ListOrigins 列出全部源站。
func ListOrigins(ctx context.Context) ([]Origin, error) {
	var origins []Origin
	if err := db.DB(ctx).Order("id desc").Find(&origins).Error; err != nil {
		return nil, err
	}
	return origins, nil
}

// GetOriginByID 按 ID 查询源站。
func GetOriginByID(ctx context.Context, id uint) (*Origin, error) {
	var origin Origin
	if err := db.DB(ctx).First(&origin, id).Error; err != nil {
		return nil, err
	}
	return &origin, nil
}

// GetOriginByAddress 按地址查询源站。
func GetOriginByAddress(ctx context.Context, address string) (*Origin, error) {
	var origin Origin
	if err := db.DB(ctx).Where("address = ?", address).First(&origin).Error; err != nil {
		return nil, err
	}
	return &origin, nil
}

// CreateOriginRecord 创建源站。
func CreateOriginRecord(ctx context.Context, origin *Origin) error {
	return db.DB(ctx).Create(origin).Error
}

// SaveOrigin 保存源站。
func SaveOrigin(ctx context.Context, origin *Origin) error {
	return db.DB(ctx).Save(origin).Error
}

// DeleteOriginRecord 删除源站。
func DeleteOriginRecord(ctx context.Context, id uint) error {
	return db.DB(ctx).Delete(&Origin{}, id).Error
}

// ListOriginRouteCounts 统计各源站关联的代理规则数量。
func ListOriginRouteCounts(ctx context.Context) ([]OriginRouteCount, error) {
	if !HasProxyRoutesTable(ctx) {
		return nil, nil
	}
	result := make([]OriginRouteCount, 0)
	err := db.DB(ctx).Model(&OriginProxyRoute{}).
		Select("origin_id, COUNT(*) AS route_count").
		Where("origin_id IS NOT NULL").
		Group("origin_id").
		Scan(&result).Error
	return result, err
}

// ListProxyRoutesByOriginID 列出源站关联的代理规则。
func ListProxyRoutesByOriginID(ctx context.Context, originID uint) ([]OriginProxyRoute, error) {
	if !HasProxyRoutesTable(ctx) {
		return nil, nil
	}
	var routes []OriginProxyRoute
	if err := db.DB(ctx).Where("origin_id = ?", originID).Order("id desc").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

// CountProxyRoutesByOriginID 统计源站关联的代理规则数量。
func CountProxyRoutesByOriginID(ctx context.Context, originID uint) (int64, error) {
	if !HasProxyRoutesTable(ctx) {
		return 0, nil
	}
	var count int64
	if err := db.DB(ctx).Model(&OriginProxyRoute{}).Where("origin_id = ?", originID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
