// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package zone manages registered roots and their explicit hostnames.
package zone

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"golang.org/x/net/publicsuffix"
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

// ListItem is a Zone list row with denormalized domain count for the UI.
type ListItem struct {
	ID          uint      `json:"id"`
	Domain      string    `json:"domain"`
	DomainCount int64     `json:"domain_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func zoneRoot(domain string) (string, error) {
	return publicsuffix.EffectiveTLDPlusOne(strings.ToLower(strings.TrimSpace(domain)))
}

func normalizeDomain(raw string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(raw))
	if domain == "" {
		return "", errors.New(errZoneDomainRequired)
	}
	if strings.Contains(domain, "*") {
		return "", errors.New(errDomainWildcardUnsupported)
	}
	if strings.Contains(domain, "://") || strings.Contains(domain, "/") || strings.Contains(domain, "?") || strings.Contains(domain, "#") || strings.Contains(domain, "@") {
		return "", errors.New(errDomainInvalid)
	}
	if _, err := zoneRoot(domain); err != nil {
		return "", errors.New(errDomainInvalid)
	}
	return domain, nil
}

// Create persists a validated registered root.
func Create(ctx context.Context, input Input) (*model.Zone, error) {
	domain, err := normalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	root, err := zoneRoot(domain)
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
	domain, err := normalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	root, err := zoneRoot(domain)
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

// CreateDomain adds a validated exact hostname to a Zone.
func CreateDomain(ctx context.Context, zoneID uint, input DomainInput) (*model.ZoneDomain, error) {
	zone, err := repository.GetZoneByID(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	domain, err := normalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	root, err := zoneRoot(domain)
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
	domain, err := normalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	zone, err := repository.GetZoneByID(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	root, err := zoneRoot(domain)
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

// DeleteDomain removes a Zone domain that is not bound to a proxy route.
func DeleteDomain(ctx context.Context, zoneID, id uint) error {
	item, err := repository.GetZoneDomainByZoneAndID(ctx, zoneID, id)
	if err != nil {
		return err
	}
	if item.ProxyRouteID != nil {
		return errors.New(errDomainBoundToRoute)
	}
	return repository.DeleteZoneDomain(ctx, item)
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
