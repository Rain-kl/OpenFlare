// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/routeidentity"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// ImportReport describes the idempotent legacy migration result.
type ImportReport struct {
	Zones     int      `json:"zones"`
	Domains   int      `json:"domains"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// LogAndReturn decorates the import failure with its conflict count.
func (r ImportReport) LogAndReturn(err error) error {
	if err != nil {
		return fmt.Errorf("迁移 Zone 失败（%d 个冲突）: %w", len(r.Conflicts), err)
	}
	return nil
}

type legacyDomain struct {
	Domain string
	CertID *uint
	Remark string
}

// legacyRouteRow reads pre-cleanup of_proxy_routes columns via raw scan.
type legacyRouteRow struct {
	ID            uint   `gorm:"column:id"`
	Domain        string `gorm:"column:domain"`
	Domains       string `gorm:"column:domains"`
	DomainCertIDs string `gorm:"column:domain_cert_ids"`
	Remark        string `gorm:"column:remark"`
}

// legacyManagedRow reads of_managed_domains while the table still exists.
type legacyManagedRow struct {
	Domain string `gorm:"column:domain"`
	CertID *uint  `gorm:"column:cert_id"`
	Remark string `gorm:"column:remark"`
}

// ImportLegacy imports legacy proxy-route / managed-domain rows into Zone tables.
// After the phase-2 schema cleanup, missing legacy columns or tables are skipped.
//
//nolint:cyclop // the transactional importer intentionally validates every legacy source in one pass.
func ImportLegacy(ctx context.Context) (report ImportReport, resultErr error) {
	conn := db.DB(ctx)
	if conn == nil {
		return report, fmt.Errorf("database is not initialized")
	}
	resultErr = conn.Transaction(func(tx *gorm.DB) error {
		items := make([]legacyDomain, 0)
		hasRouteDomains := false

		if tx.Migrator().HasColumn("of_proxy_routes", "domain") &&
			tx.Migrator().HasColumn("of_proxy_routes", "domains") {
			var routes []legacyRouteRow
			if err := tx.Table("of_proxy_routes").
				Select("id, domain, domains, domain_cert_ids, remark").
				Find(&routes).Error; err != nil {
				return err
			}
			for _, route := range routes {
				domains, err := routeidentity.DecodeDomains(route.Domains, route.Domain)
				if err != nil {
					report.Conflicts = append(report.Conflicts, fmt.Sprintf("route %d: %v", route.ID, err))
					continue
				}
				if len(domains) > 0 {
					hasRouteDomains = true
				}
				certIDs := decodeLegacyCertIDs(route.DomainCertIDs, len(domains))
				for i, domain := range domains {
					var certID *uint
					if i < len(certIDs) && certIDs[i] > 0 {
						v := certIDs[i]
						certID = &v
					}
					items = append(items, legacyDomain{Domain: domain, CertID: certID, Remark: route.Remark})
				}
			}
		}

		if !hasRouteDomains && tx.Migrator().HasTable("of_managed_domains") {
			var legacy []legacyManagedRow
			if err := tx.Table("of_managed_domains").
				Select("domain, cert_id, remark").
				Find(&legacy).Error; err != nil {
				return err
			}
			for _, item := range legacy {
				items = append(items, legacyDomain(item))
			}
		}

		for _, item := range items {
			domain, err := normalizeDomain(item.Domain)
			if err != nil {
				report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %v", item.Domain, err))
				continue
			}
			root, err := zoneRoot(domain)
			if err != nil {
				report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %v", domain, err))
				continue
			}
			var existing model.ZoneDomain
			err = tx.Where("domain = ?", domain).First(&existing).Error
			if err == nil {
				var z model.Zone
				if tx.First(&z, existing.ZoneID).Error != nil || z.Domain != root {
					report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: global domain conflict", domain))
				}
				continue
			}
			if err != nil && !isNotFound(err) {
				return err
			}
			var zone model.Zone
			err = tx.Where("domain = ?", root).First(&zone).Error
			if isNotFound(err) {
				zone = model.Zone{Domain: root}
				if err = tx.Create(&zone).Error; err != nil {
					return err
				}
				report.Zones++
			} else if err != nil {
				return err
			}
			if item.CertID != nil {
				var cert model.TLSCertificate
				if err = tx.First(&cert, *item.CertID).Error; err != nil {
					report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %s", domain, errCertificateNotFound))
					continue
				}
			}
			if err = tx.Create(&model.ZoneDomain{
				ZoneID: zone.ID,
				Domain: domain,
				CertID: item.CertID,
				Remark: item.Remark,
			}).Error; err != nil {
				return err
			}
			report.Domains++
		}
		if len(report.Conflicts) > 0 {
			return fmt.Errorf("legacy data has conflicts")
		}
		return nil
	})
	return report, resultErr
}

func decodeLegacyCertIDs(raw string, count int) []uint {
	var values []uint
	if strings.TrimSpace(raw) == "" {
		return make([]uint, count)
	}
	if json.Unmarshal([]byte(raw), &values) != nil {
		return make([]uint, count)
	}
	return values
}

func isNotFound(err error) bool { return err == gorm.ErrRecordNotFound }
