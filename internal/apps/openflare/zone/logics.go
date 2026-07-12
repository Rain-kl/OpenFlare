// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package zone manages registered roots and their explicit hostnames.
package zone

import (
	"context"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"golang.org/x/net/publicsuffix"
	"gorm.io/gorm"
)

// Input is the mutable Zone payload.
type Input struct {
	Domain string `json:"domain"`
	Remark string `json:"remark"`
}

// DomainInput is the mutable Zone-domain payload.
type DomainInput struct {
	Domain string `json:"domain"`
	CertID *uint  `json:"cert_id"`
	Remark string `json:"remark"`
}

// Overview joins a Zone with its explicit domains.
type Overview struct {
	Zone    model.Zone         `json:"zone"`
	Domains []model.ZoneDomain `json:"domains"`
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
	zone := &model.Zone{Domain: domain, Remark: strings.TrimSpace(input.Remark)}
	if err := db.DB(ctx).Create(zone).Error; err != nil {
		if isUnique(err) {
			return nil, errors.New(errDomainExists)
		}
		return nil, err
	}
	return zone, nil
}

// Update replaces a Zone's mutable fields.
func Update(ctx context.Context, id uint, input Input) (*model.Zone, error) {
	var zone model.Zone
	if err := db.DB(ctx).First(&zone, id).Error; err != nil {
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
	zone.Domain, zone.Remark = domain, strings.TrimSpace(input.Remark)
	if err := db.DB(ctx).Save(&zone).Error; err != nil {
		if isUnique(err) {
			return nil, errors.New(errDomainExists)
		}
		return nil, err
	}
	return &zone, nil
}

// List returns all Zones in stable domain order.
func List(ctx context.Context) ([]model.Zone, error) {
	var zones []model.Zone
	err := db.DB(ctx).Order("domain asc").Find(&zones).Error
	return zones, err
}

// GetOverview returns a Zone and its domains.
func GetOverview(ctx context.Context, id uint) (*Overview, error) {
	var zone model.Zone
	if err := db.DB(ctx).First(&zone, id).Error; err != nil {
		return nil, err
	}
	var domains []model.ZoneDomain
	if err := db.DB(ctx).Where("zone_id = ?", id).Order("domain asc").Find(&domains).Error; err != nil {
		return nil, err
	}
	return &Overview{Zone: zone, Domains: domains}, nil
}

// CreateDomain adds a validated exact hostname to a Zone.
func CreateDomain(ctx context.Context, zoneID uint, input DomainInput) (*model.ZoneDomain, error) {
	var zone model.Zone
	if err := db.DB(ctx).First(&zone, zoneID).Error; err != nil {
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
		if _, err := model.GetTLSCertificateByID(ctx, *input.CertID); err != nil {
			return nil, errors.New(errCertificateNotFound)
		}
	}
	item := &model.ZoneDomain{ZoneID: zoneID, Domain: domain, CertID: input.CertID, Remark: strings.TrimSpace(input.Remark)}
	if err := db.DB(ctx).Create(item).Error; err != nil {
		if isUnique(err) {
			return nil, errors.New(errDomainExists)
		}
		return nil, err
	}
	return item, nil
}

// UpdateDomain replaces a Zone-domain's mutable fields.
func UpdateDomain(ctx context.Context, zoneID, id uint, input DomainInput) (*model.ZoneDomain, error) {
	var item model.ZoneDomain
	if err := db.DB(ctx).Where("id = ? AND zone_id = ?", id, zoneID).First(&item).Error; err != nil {
		return nil, err
	}
	domain, err := normalizeDomain(input.Domain)
	if err != nil {
		return nil, err
	}
	var zone model.Zone
	if err = db.DB(ctx).First(&zone, zoneID).Error; err != nil {
		return nil, err
	}
	root, err := zoneRoot(domain)
	if err != nil || root != zone.Domain {
		return nil, errors.New(errDomainOutsideZone)
	}
	if input.CertID != nil {
		if _, err = model.GetTLSCertificateByID(ctx, *input.CertID); err != nil {
			return nil, errors.New(errCertificateNotFound)
		}
	}
	item.Domain, item.CertID, item.Remark = domain, input.CertID, strings.TrimSpace(input.Remark)
	if err = db.DB(ctx).Save(&item).Error; err != nil {
		if isUnique(err) {
			return nil, errors.New(errDomainExists)
		}
		return nil, err
	}
	return &item, nil
}

func isUnique(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
