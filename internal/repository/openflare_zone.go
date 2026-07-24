package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// ListZones returns all zones ordered by domain ascending.
func ListZones(ctx context.Context) ([]model.Zone, error) {
	var zones []model.Zone
	if err := db.DB(ctx).Order("domain asc").Find(&zones).Error; err != nil {
		return nil, err
	}
	return zones, nil
}

// GetZoneByID returns a zone by primary key.
func GetZoneByID(ctx context.Context, id uint) (*model.Zone, error) {
	var zone model.Zone
	if err := db.DB(ctx).First(&zone, id).Error; err != nil {
		return nil, err
	}
	return &zone, nil
}

// CreateZone creates a zone record.
func CreateZone(ctx context.Context, zone *model.Zone) error {
	return db.DB(ctx).Create(zone).Error
}

// SaveZone persists zone updates.
func SaveZone(ctx context.Context, zone *model.Zone) error {
	return db.DB(ctx).Save(zone).Error
}

// DeleteZone deletes a zone by primary key.
func DeleteZone(ctx context.Context, id uint) error {
	return db.DB(ctx).Delete(&model.Zone{}, id).Error
}

// ListZoneDomainCounts returns per-zone domain counts for list cards.
func ListZoneDomainCounts(ctx context.Context) ([]model.ZoneDomainCount, error) {
	var rows []model.ZoneDomainCount
	if err := db.DB(ctx).Model(&model.ZoneDomain{}).
		Select("zone_id, count(*) as count").
		Group("zone_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListZoneDomainsByZoneID returns domains under a zone ordered by domain ascending.
func ListZoneDomainsByZoneID(ctx context.Context, zoneID uint) ([]model.ZoneDomain, error) {
	var domains []model.ZoneDomain
	if err := db.DB(ctx).Where("zone_id = ?", zoneID).Order("domain asc").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

// CountZoneDomainsByZoneID counts domains under a zone.
func CountZoneDomainsByZoneID(ctx context.Context, zoneID uint) (int64, error) {
	var count int64
	if err := db.DB(ctx).Model(&model.ZoneDomain{}).Where("zone_id = ?", zoneID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetZoneDomainByZoneAndID returns a domain scoped to a zone.
func GetZoneDomainByZoneAndID(ctx context.Context, zoneID, id uint) (*model.ZoneDomain, error) {
	var item model.ZoneDomain
	if err := db.DB(ctx).Where("id = ? AND zone_id = ?", id, zoneID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateZoneDomain creates a zone domain record.
func CreateZoneDomain(ctx context.Context, domain *model.ZoneDomain) error {
	return db.DB(ctx).Create(domain).Error
}

// SaveZoneDomain persists zone domain updates.
func SaveZoneDomain(ctx context.Context, domain *model.ZoneDomain) error {
	return db.DB(ctx).Save(domain).Error
}

// DeleteZoneDomain deletes a zone domain record.
func DeleteZoneDomain(ctx context.Context, domain *model.ZoneDomain) error {
	return db.DB(ctx).Delete(domain).Error
}

// ListZoneDomainsByRouteID returns the domains bound to a proxy route.
func ListZoneDomainsByRouteID(ctx context.Context, routeID uint) ([]model.ZoneDomain, error) {
	var domains []model.ZoneDomain
	if err := db.DB(ctx).Where("proxy_route_id = ?", routeID).Order("id asc").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

// ListZoneDomainsByIDs returns explicit domains in the requested ID order.
func ListZoneDomainsByIDs(ctx context.Context, domainIDs []uint) ([]model.ZoneDomain, error) {
	if len(domainIDs) == 0 {
		return []model.ZoneDomain{}, nil
	}
	var domains []model.ZoneDomain
	if err := db.DB(ctx).Where("id IN ?", domainIDs).Find(&domains).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]model.ZoneDomain, len(domains))
	for _, domain := range domains {
		byID[domain.ID] = domain
	}
	ordered := make([]model.ZoneDomain, 0, len(domainIDs))
	for _, id := range domainIDs {
		domain, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("one or more zone domains do not exist")
		}
		ordered = append(ordered, domain)
	}
	return ordered, nil
}

// CountZoneDomainsByCertificateID reports whether a certificate is assigned to a model.Zone domain.
func CountZoneDomainsByCertificateID(ctx context.Context, certificateID uint) (int64, error) {
	var count int64
	err := db.DB(ctx).Model(&model.ZoneDomain{}).Where("cert_id = ?", certificateID).Count(&count).Error
	return count, err
}

// ReplaceZoneDomainRouteBindings replaces every model.ZoneDomain binding for a proxy route.
func ReplaceZoneDomainRouteBindings(ctx context.Context, routeID uint, domainIDs []uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}

	return conn.Transaction(func(tx *gorm.DB) error {
		return ReplaceZoneDomainRouteBindingsTx(tx, routeID, domainIDs)
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
