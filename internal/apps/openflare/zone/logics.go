// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package zone manages registered roots and their exact or wildcard hostnames.
package zone

import (
	"context"
	"errors"
	"strings"
	"time"

	cf "github.com/Rain-kl/Wavelet/internal/apps/openflare/cloudflare"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/routeidentity"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

// Input is the mutable Zone payload.
type Input struct {
	Domain string `json:"domain"`
}

// DomainInput is the mutable Zone-domain payload.
type DomainInput struct {
	Domain string `json:"domain"`
	CertID *uint  `json:"cert_id"`
}

// Overview joins a Zone with its explicit domains.
type Overview struct {
	Zone    model.Zone         `json:"zone"`
	Domains []model.ZoneDomain `json:"domains"`
}

// CleanupUnlinkedDomainsResult reports how many unlinked domains were deleted from a Zone.
type CleanupUnlinkedDomainsResult struct {
	DeletedCount int64 `json:"deleted_count"`
}

// ListItem is a Zone list row with denormalized domain count for the UI.
type ListItem struct {
	ID          uint      `json:"id"`
	Domain      string    `json:"domain"`
	DomainCount int64     `json:"domain_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Create persists a validated registered root.
func Create(ctx context.Context, input Input) (*model.Zone, error) {
	domain, err := routeidentity.NormalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	root, err := routeidentity.ZoneRoot(domain)
	if err != nil || root != domain {
		return nil, errors.New(errZoneRootInvalid)
	}
	zone := &model.Zone{Domain: domain}
	if err := repository.CreateZone(ctx, zone); err != nil {
		if isUnique(err) {
			return nil, errors.New(errDomainExists)
		}
		return nil, err
	}
	return zone, nil
}

// Update replaces a Zone's mutable fields.
func Update(ctx context.Context, id uint, input Input) (*model.Zone, error) {
	zone, err := repository.GetZoneByID(ctx, id)
	if err != nil {
		return nil, err
	}
	domain, err := routeidentity.NormalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	root, err := routeidentity.ZoneRoot(domain)
	if err != nil || root != domain {
		return nil, errors.New(errZoneRootInvalid)
	}
	zone.Domain = domain
	if err := repository.SaveZone(ctx, zone); err != nil {
		if isUnique(err) {
			return nil, errors.New(errDomainExists)
		}
		return nil, err
	}
	return zone, nil
}

// List returns all Zones in stable domain order, with domain counts for list cards.
func List(ctx context.Context) ([]ListItem, error) {
	zones, err := repository.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := repository.ListZoneDomainCounts(ctx)
	if err != nil {
		return nil, err
	}
	counts := make(map[uint]int64, len(rows))
	for _, row := range rows {
		counts[row.ZoneID] = row.Count
	}

	items := make([]ListItem, 0, len(zones))
	for _, zone := range zones {
		items = append(items, ListItem{
			ID:          zone.ID,
			Domain:      zone.Domain,
			DomainCount: counts[zone.ID],
			CreatedAt:   zone.CreatedAt,
			UpdatedAt:   zone.UpdatedAt,
		})
	}
	return items, nil
}

// GetOverview returns a Zone and its domains.
func GetOverview(ctx context.Context, id uint) (*Overview, error) {
	zone, err := repository.GetZoneByID(ctx, id)
	if err != nil {
		return nil, err
	}
	domains, err := repository.ListZoneDomainsByZoneID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Overview{Zone: *zone, Domains: domains}, nil
}

// CreateDomain adds a validated exact or wildcard hostname to a Zone.
func CreateDomain(ctx context.Context, zoneID uint, input DomainInput) (*model.ZoneDomain, error) {
	zone, err := repository.GetZoneByID(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	domain, err := routeidentity.NormalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	root, err := routeidentity.ZoneRoot(domain)
	if err != nil || root != zone.Domain {
		return nil, errors.New(errDomainOutsideZone)
	}
	if input.CertID != nil {
		if _, err := repository.GetTLSCertificateByID(ctx, *input.CertID); err != nil {
			return nil, errors.New(errCertificateNotFound)
		}
	}
	item := &model.ZoneDomain{ZoneID: zoneID, Domain: domain, CertID: input.CertID}
	if err := repository.CreateZoneDomain(ctx, item); err != nil {
		if isUnique(err) {
			return nil, errors.New(errDomainExists)
		}
		return nil, err
	}
	return item, nil
}

// UpdateDomain replaces a Zone-domain's mutable fields.
func UpdateDomain(ctx context.Context, zoneID, id uint, input DomainInput) (*model.ZoneDomain, error) {
	item, err := repository.GetZoneDomainByZoneAndID(ctx, zoneID, id)
	if err != nil {
		return nil, err
	}
	domain, err := routeidentity.NormalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	zone, err := repository.GetZoneByID(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	root, err := routeidentity.ZoneRoot(domain)
	if err != nil || root != zone.Domain {
		return nil, errors.New(errDomainOutsideZone)
	}
	if input.CertID != nil {
		if _, err = repository.GetTLSCertificateByID(ctx, *input.CertID); err != nil {
			return nil, errors.New(errCertificateNotFound)
		}
	}
	item.Domain, item.CertID = domain, input.CertID
	if err = repository.SaveZoneDomain(ctx, item); err != nil {
		if isUnique(err) {
			return nil, errors.New(errDomainExists)
		}
		return nil, err
	}
	return item, nil
}

func deleteUnboundDomain(ctx context.Context, item *model.ZoneDomain, member *model.CFPointingMember) (bool, error) {
	if member != nil {
		if delErr := cf.DeleteManagedRecord(ctx, member.ID); delErr != nil {
			logger.WarnF(ctx, "[Zone] delete managed Cloudflare record failed for domain %s (member_id=%d): %v", item.Domain, member.ID, delErr)
		}
	}
	deleted, err := repository.DeleteUnlinkedZoneDomain(ctx, item.ZoneID, item.ID)
	if err != nil {
		logger.ErrorF(ctx, "[Zone] delete unlinked domain failed: zone_id=%d domain_id=%d error=%v", item.ZoneID, item.ID, err)
		return false, err
	}
	return deleted, nil
}

// DeleteDomain removes a Zone domain that is not bound to a proxy route.
func DeleteDomain(ctx context.Context, zoneID, id uint) error {
	item, err := repository.GetZoneDomainByZoneAndID(ctx, zoneID, id)
	if err != nil {
		return err
	}
	if item.ProxyRouteID != nil {
		return errors.New(errDomainBoundToRoute)
	}
	member, cfErr := repository.GetCFPointingMemberByZoneDomainID(ctx, item.ID)
	if cfErr != nil && !errors.Is(cfErr, gorm.ErrRecordNotFound) {
		return cfErr
	}
	deleted, err := deleteUnboundDomain(ctx, item, member)
	if err != nil {
		return err
	}
	if !deleted {
		return errors.New(errDomainBoundToRoute)
	}
	return nil
}

// CleanupUnlinkedDomains deletes every domain in a Zone that is not bound to a proxy route.
func CleanupUnlinkedDomains(ctx context.Context, zoneID uint) (*CleanupUnlinkedDomainsResult, error) {
	if _, err := repository.GetZoneByID(ctx, zoneID); err != nil {
		return nil, err
	}
	domains, err := repository.ListUnlinkedZoneDomainsByZoneID(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	domainIDs := make([]uint, len(domains))
	for i := range domains {
		domainIDs[i] = domains[i].ID
	}
	members, err := repository.ListCFPointingMembersByZoneDomainIDs(ctx, domainIDs)
	if err != nil {
		return nil, err
	}
	memberByDomainID := make(map[uint]*model.CFPointingMember, len(members))
	for i := range members {
		member := members[i]
		memberByDomainID[member.ZoneDomainID] = &member
	}
	var deletedCount int64
	for i := range domains {
		deleted, delErr := deleteUnboundDomain(ctx, &domains[i], memberByDomainID[domains[i].ID])
		if delErr != nil {
			return nil, delErr
		}
		if deleted {
			deletedCount++
		}
	}
	return &CleanupUnlinkedDomainsResult{DeletedCount: deletedCount}, nil
}

// Delete removes a Zone that has no remaining domains.
func Delete(ctx context.Context, id uint) error {
	if _, err := repository.GetZoneByID(ctx, id); err != nil {
		return err
	}
	count, err := repository.CountZoneDomainsByZoneID(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New(errZoneHasDomains)
	}
	return repository.DeleteZone(ctx, id)
}

func isUnique(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
