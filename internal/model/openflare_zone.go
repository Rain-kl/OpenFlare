// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	tableOfZones       = "of_zones"
	tableOfZoneDomains = "of_zone_domains"
)

var errZoneDomainBoundToAnotherRoute = errors.New("zone domain is already bound to another proxy route")

// Zone OpenFlare 注册根域实体。
type Zone struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Domain    string    `json:"domain" gorm:"uniqueIndex:idx_of_zones_domain;size:255;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (Zone) TableName() string {
	return tableOfZones
}

// ZoneDomain OpenFlare Zone 下的明确域名实体。
type ZoneDomain struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ZoneID       uint      `json:"zone_id" gorm:"not null;index:idx_of_zone_domains_zone_id"`
	ProxyRouteID *uint     `json:"proxy_route_id" gorm:"index:idx_of_zone_domains_proxy_route_id"`
	Domain       string    `json:"domain" gorm:"uniqueIndex:idx_of_zone_domains_domain;size:255;not null"`
	CertID       *uint     `json:"cert_id" gorm:"index:idx_of_zone_domains_cert_id"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名。
func (ZoneDomain) TableName() string {
	return tableOfZoneDomains
}

// ListZoneDomainsByRouteID returns the domains bound to a proxy route.
func ListZoneDomainsByRouteID(ctx context.Context, routeID uint) ([]ZoneDomain, error) {
	var domains []ZoneDomain
	if err := db.DB(ctx).Where("proxy_route_id = ?", routeID).Order("id asc").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

// ListZoneDomainsByIDs returns explicit domains in the requested ID order.
func ListZoneDomainsByIDs(ctx context.Context, domainIDs []uint) ([]ZoneDomain, error) {
	if len(domainIDs) == 0 {
		return []ZoneDomain{}, nil
	}
	var domains []ZoneDomain
	if err := db.DB(ctx).Where("id IN ?", domainIDs).Find(&domains).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]ZoneDomain, len(domains))
	for _, domain := range domains {
		byID[domain.ID] = domain
	}
	ordered := make([]ZoneDomain, 0, len(domainIDs))
	for _, id := range domainIDs {
		domain, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("one or more zone domains do not exist")
		}
		ordered = append(ordered, domain)
	}
	return ordered, nil
}

// CountZoneDomainsByCertificateID reports whether a certificate is assigned to a Zone domain.
func CountZoneDomainsByCertificateID(ctx context.Context, certificateID uint) (int64, error) {
	var count int64
	err := db.DB(ctx).Model(&ZoneDomain{}).Where("cert_id = ?", certificateID).Count(&count).Error
	return count, err
}

// ReplaceZoneDomainRouteBindings replaces every ZoneDomain binding for a proxy route.
func ReplaceZoneDomainRouteBindings(ctx context.Context, routeID uint, domainIDs []uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New("database is not initialized")
	}

	return conn.Transaction(func(tx *gorm.DB) error {
		var requested []ZoneDomain
		if len(domainIDs) > 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id IN ?", domainIDs).
				Find(&requested).Error; err != nil {
				return err
			}
			if len(requested) != len(uniqueZoneDomainIDs(domainIDs)) {
				return fmt.Errorf("one or more zone domains do not exist")
			}
			for _, domain := range requested {
				if domain.ProxyRouteID != nil && *domain.ProxyRouteID != routeID {
					return errZoneDomainBoundToAnotherRoute
				}
			}
		}

		current := tx.Model(&ZoneDomain{}).Where("proxy_route_id = ?", routeID)
		if len(domainIDs) > 0 {
			current = current.Where("id NOT IN ?", domainIDs)
		}
		if err := current.Update("proxy_route_id", nil).Error; err != nil {
			return err
		}

		if len(domainIDs) == 0 {
			return nil
		}
		return tx.Model(&ZoneDomain{}).Where("id IN ?", domainIDs).Update("proxy_route_id", routeID).Error
	})
}

func uniqueZoneDomainIDs(domainIDs []uint) []uint {
	seen := make(map[uint]struct{}, len(domainIDs))
	ids := make([]uint, 0, len(domainIDs))
	for _, id := range domainIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
