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

// ImportLegacy imports legacy proxy-route names first, and managed domains only when routes contain no domains.
// ImportLegacy imports legacy records atomically after collecting validation conflicts.
//
//nolint:cyclop // the transactional importer intentionally validates every legacy source in one pass.
func ImportLegacy(ctx context.Context) (report ImportReport, resultErr error) {
	conn := db.DB(ctx)
	if conn == nil {
		return report, fmt.Errorf("database is not initialized")
	}
	resultErr = conn.Transaction(func(tx *gorm.DB) error {
		var routes []model.ProxyRoute
		if err := tx.Find(&routes).Error; err != nil {
			return err
		}
		items := make([]legacyDomain, 0)
		hasRouteDomains := false
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
		if !hasRouteDomains {
			var legacy []model.ManagedDomain
			if err := tx.Find(&legacy).Error; err != nil {
				return err
			}
			for _, item := range legacy {
				items = append(items, legacyDomain{Domain: item.Domain, CertID: item.CertID, Remark: item.Remark})
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
			if err = tx.Create(&model.ZoneDomain{ZoneID: zone.ID, Domain: domain, CertID: item.CertID, Remark: item.Remark}).Error; err != nil {
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
