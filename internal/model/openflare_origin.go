// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"
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
	return tableOfProxyRoutes
}
