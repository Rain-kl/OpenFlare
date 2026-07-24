// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// Zone domain binding sentinel errors (shared by proxy_route and zone binding helpers).
var (
	// ErrZoneDomainBoundToAnotherRoute is returned when a domain is already bound to a different route.
	ErrZoneDomainBoundToAnotherRoute = errors.New("zone domain is already bound to another proxy route")
	// ErrZoneDomainNotFound is returned when one or more requested domain IDs do not exist.
	ErrZoneDomainNotFound = errors.New("one or more zone domains do not exist")
)

// WithProxyRouteTx runs fn inside a database transaction for proxy-route multi-step work.
func WithProxyRouteTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return db.DB(ctx).Transaction(fn)
}

// ListProxyRoutes 列出全部代理规则。
func ListProxyRoutes(ctx context.Context) ([]*model.ProxyRoute, error) {
	var routes []*model.ProxyRoute
	if err := db.DB(ctx).Order("id desc").Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

// GetProxyRouteByID 按 ID 查询代理规则。
func GetProxyRouteByID(ctx context.Context, id uint) (*model.ProxyRoute, error) {
	var route model.ProxyRoute
	if err := db.DB(ctx).First(&route, id).Error; err != nil {
		return nil, err
	}
	return &route, nil
}

// CreateProxyRouteRecord 创建代理规则。
func CreateProxyRouteRecord(ctx context.Context, route *model.ProxyRoute) error {
	return CreateProxyRouteRecordTx(db.DB(ctx), route)
}

// CreateProxyRouteRecordTx creates a proxy route within an existing transaction.
func CreateProxyRouteRecordTx(tx *gorm.DB, route *model.ProxyRoute) error {
	return tx.Create(route).Error
}

// UpdateProxyRouteRecord 更新代理规则。
func UpdateProxyRouteRecord(ctx context.Context, route *model.ProxyRoute) error {
	return UpdateProxyRouteRecordTx(db.DB(ctx), route)
}

// UpdateProxyRouteRecordTx updates a proxy route within an existing transaction.
func UpdateProxyRouteRecordTx(tx *gorm.DB, route *model.ProxyRoute) error {
	return tx.Model(&model.ProxyRoute{}).Where("id = ?", route.ID).Updates(proxyRouteUpdateMap(route)).Error
}

func proxyRouteUpdateMap(route *model.ProxyRoute) map[string]any {
	return map[string]any{
		"site_name":              route.SiteName,
		"origin_id":              route.OriginID,
		"origin_url":             route.OriginURL,
		"origin_host":            route.OriginHost,
		"upstreams":              route.Upstreams,
		colEnabled:               route.Enabled,
		"enable_https":           route.EnableHTTPS,
		"redirect_http":          route.RedirectHTTP,
		"limit_conn_per_server":  route.LimitConnPerServer,
		"limit_conn_per_ip":      route.LimitConnPerIP,
		"limit_rate":             route.LimitRate,
		"limit_req_per_ip":       route.LimitReqPerIP,
		"cache_enabled":          route.CacheEnabled,
		"cache_policy":           route.CachePolicy,
		"cache_rules":            route.CacheRules,
		"custom_headers":         route.CustomHeaders,
		"basic_auth_enabled":     route.BasicAuthEnabled,
		"basic_auth_username":    route.BasicAuthUsername,
		"basic_auth_password":    route.BasicAuthPassword,
		"upstream_type":          route.UpstreamType,
		"tunnel_node_id":         route.TunnelNodeID,
		"tunnel_target_addr":     route.TunnelTargetAddr,
		"tunnel_target_protocol": route.TunnelTargetProtocol,
		"pages_project_id":       route.PagesProjectID,
	}
}

// DeleteProxyRouteRecord 删除代理规则。
func DeleteProxyRouteRecord(ctx context.Context, id uint) error {
	return DeleteProxyRouteRecordTx(db.DB(ctx), id)
}

// DeleteProxyRouteRecordTx deletes a proxy route within an existing transaction.
func DeleteProxyRouteRecordTx(tx *gorm.DB, id uint) error {
	return tx.Delete(&model.ProxyRoute{}, id).Error
}

// ClearZoneDomainProxyRouteBindingsTx unbinds every zone domain from a proxy route.
func ClearZoneDomainProxyRouteBindingsTx(tx *gorm.DB, routeID uint) error {
	return tx.Model(&model.ZoneDomain{}).Where("proxy_route_id = ?", routeID).Update("proxy_route_id", nil).Error
}

// ReplaceZoneDomainRouteBindingsTx replaces every ZoneDomain binding for a proxy route
// inside the caller's transaction (with row locks on requested domains).
func ReplaceZoneDomainRouteBindingsTx(tx *gorm.DB, routeID uint, domainIDs []uint) error {
	var requested []model.ZoneDomain
	if len(domainIDs) > 0 {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ?", domainIDs).
			Find(&requested).Error; err != nil {
			return err
		}
		if len(requested) != len(uniqueZoneDomainIDs(domainIDs)) {
			return ErrZoneDomainNotFound
		}
		for _, domain := range requested {
			if domain.ProxyRouteID != nil && *domain.ProxyRouteID != routeID {
				return ErrZoneDomainBoundToAnotherRoute
			}
		}
	}

	current := tx.Model(&model.ZoneDomain{}).Where("proxy_route_id = ?", routeID)
	if len(domainIDs) > 0 {
		current = current.Where("id NOT IN ?", domainIDs)
	}
	if err := current.Update("proxy_route_id", nil).Error; err != nil {
		return err
	}

	if len(domainIDs) == 0 {
		return nil
	}
	return tx.Model(&model.ZoneDomain{}).Where("id IN ?", domainIDs).Update("proxy_route_id", routeID).Error
}

// DeleteProxyRouteAndUnbind clears domain bindings then deletes the proxy route in one transaction.
func DeleteProxyRouteAndUnbind(ctx context.Context, id uint) error {
	return WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		if err := ClearZoneDomainProxyRouteBindingsTx(tx, id); err != nil {
			return err
		}
		return DeleteProxyRouteRecordTx(tx, id)
	})
}
