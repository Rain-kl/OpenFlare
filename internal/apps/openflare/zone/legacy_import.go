// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/routeidentity"
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
	Domain       string
	CertID       *uint
	Remark       string
	ProxyRouteID *uint
}

// ImportLegacyTx imports legacy proxy-route / managed-domain rows into Zone tables
// within an existing SQL transaction (goose runs this on Server upgrade).
// postgres selects $n placeholders; otherwise SQLite-style ? is used.
// Missing legacy columns or tables are skipped so re-runs after phase-2 cleanup are no-ops.
//
//nolint:cyclop,gocyclo // single-pass legacy importer validates every source before write.
func ImportLegacyTx(ctx context.Context, tx *sql.Tx, postgres bool) (report ImportReport, err error) {
	if tx == nil {
		return report, errors.New("transaction is required")
	}
	q := func(sqlText string) string { return rebindSQL(sqlText, postgres) }

	items := make([]legacyDomain, 0)
	hasRouteDomains := false

	hasDomainCol, err := hasTableColumn(ctx, tx, q, postgres, "of_proxy_routes", "domain")
	if err != nil {
		return report, err
	}
	hasDomainsCol, err := hasTableColumn(ctx, tx, q, postgres, "of_proxy_routes", "domains")
	if err != nil {
		return report, err
	}
	if hasDomainCol && hasDomainsCol {
		var collectErr error
		items, hasRouteDomains, report.Conflicts, collectErr = collectLegacyRouteDomainsImpl(ctx, tx, q)
		if collectErr != nil {
			return report, collectErr
		}
	}

	if !hasRouteDomains {
		exists, tableErr := hasTable(ctx, tx, q, postgres, "of_managed_domains")
		if tableErr != nil {
			return report, tableErr
		}
		if exists {
			managed, managedErr := collectLegacyManagedDomains(ctx, tx, q)
			if managedErr != nil {
				return report, managedErr
			}
			items = append(items, managed...)
		}
	}

	for _, item := range items {
		domain, normErr := normalizeDomain(item.Domain)
		if normErr != nil {
			report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %v", item.Domain, normErr))
			continue
		}
		root, rootErr := zoneRoot(domain)
		if rootErr != nil {
			report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %v", domain, rootErr))
			continue
		}

		var existingID uint
		var existingZoneDomain string
		scanErr := tx.QueryRowContext(ctx, q(`
			SELECT zd.id, z.domain
			FROM of_zone_domains zd
			JOIN of_zones z ON z.id = zd.zone_id
			WHERE zd.domain = ?
		`), domain).Scan(&existingID, &existingZoneDomain)
		if scanErr == nil {
			if existingZoneDomain != root {
				report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: global domain conflict", domain))
			} else if item.ProxyRouteID != nil {
				if _, bindErr := tx.ExecContext(ctx, q(`
					UPDATE of_zone_domains
					SET proxy_route_id = COALESCE(proxy_route_id, ?),
					    cert_id = COALESCE(cert_id, ?)
					WHERE id = ?
				`), *item.ProxyRouteID, nullableUint(item.CertID), existingID); bindErr != nil {
					return report, bindErr
				}
			}
			continue
		}
		if !errors.Is(scanErr, sql.ErrNoRows) {
			return report, scanErr
		}

		zoneID, zoneErr := ensureZone(ctx, tx, q, root, &report)
		if zoneErr != nil {
			return report, zoneErr
		}

		if item.CertID != nil {
			var certID uint
			if certErr := tx.QueryRowContext(ctx, q(`SELECT id FROM of_tls_certificates WHERE id = ?`), *item.CertID).
				Scan(&certID); certErr != nil {
				if errors.Is(certErr, sql.ErrNoRows) {
					report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %s", domain, errCertificateNotFound))
					continue
				}
				return report, certErr
			}
		}

		if _, insErr := tx.ExecContext(ctx, q(`
			INSERT INTO of_zone_domains (zone_id, proxy_route_id, domain, cert_id, remark, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`), zoneID, nullableUint(item.ProxyRouteID), domain, nullableUint(item.CertID), item.Remark); insErr != nil {
			return report, insErr
		}
		report.Domains++
	}

	if len(report.Conflicts) > 0 {
		return report, fmt.Errorf("legacy data has conflicts")
	}
	return report, nil
}

func ensureZone(
	ctx context.Context,
	tx *sql.Tx,
	q func(string) string,
	root string,
	report *ImportReport,
) (uint, error) {
	var zoneID uint
	err := tx.QueryRowContext(ctx, q(`SELECT id FROM of_zones WHERE domain = ?`), root).Scan(&zoneID)
	if err == nil {
		return zoneID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if _, execErr := tx.ExecContext(ctx, q(`
		INSERT INTO of_zones (domain, remark, created_at, updated_at)
		VALUES (?, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`), root); execErr != nil {
		return 0, execErr
	}
	if err := tx.QueryRowContext(ctx, q(`SELECT id FROM of_zones WHERE domain = ?`), root).Scan(&zoneID); err != nil {
		return 0, err
	}
	report.Zones++
	return zoneID, nil
}

func collectLegacyRouteDomainsImpl(
	ctx context.Context,
	tx *sql.Tx,
	q func(string) string,
) (items []legacyDomain, hasRouteDomains bool, conflicts []string, err error) {
	// Probe domain_cert_ids: if SELECT fails, fall back without it.
	queryWithCert := q(`SELECT id, domain, domains, COALESCE(domain_cert_ids, '[]'), remark FROM of_proxy_routes`)
	rows, err := tx.QueryContext(ctx, queryWithCert)
	useCert := true
	if err != nil {
		useCert = false
		rows, err = tx.QueryContext(ctx, q(`SELECT id, domain, domains, remark FROM of_proxy_routes`))
		if err != nil {
			return nil, false, nil, err
		}
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			id      uint
			domain  string
			domains string
			certIDs string
			remark  string
		)
		if useCert {
			if err := rows.Scan(&id, &domain, &domains, &certIDs, &remark); err != nil {
				return nil, false, nil, err
			}
		} else {
			if err := rows.Scan(&id, &domain, &domains, &remark); err != nil {
				return nil, false, nil, err
			}
			certIDs = "[]"
		}
		decoded, decodeErr := routeidentity.DecodeDomains(domains, domain)
		if decodeErr != nil {
			conflicts = append(conflicts, fmt.Sprintf("route %d: %v", id, decodeErr))
			continue
		}
		if len(decoded) > 0 {
			hasRouteDomains = true
		}
		ids := decodeLegacyCertIDs(certIDs, len(decoded))
		routeID := id
		for i, d := range decoded {
			var certID *uint
			if i < len(ids) && ids[i] > 0 {
				v := ids[i]
				certID = &v
			}
			items = append(items, legacyDomain{
				Domain:       d,
				CertID:       certID,
				Remark:       remark,
				ProxyRouteID: &routeID,
			})
		}
	}
	return items, hasRouteDomains, conflicts, rows.Err()
}

func collectLegacyManagedDomains(ctx context.Context, tx *sql.Tx, q func(string) string) ([]legacyDomain, error) {
	rows, err := tx.QueryContext(ctx, q(`SELECT domain, cert_id, remark FROM of_managed_domains`))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]legacyDomain, 0)
	for rows.Next() {
		var (
			domain string
			certID sql.NullInt64
			remark string
		)
		if err := rows.Scan(&domain, &certID, &remark); err != nil {
			return nil, err
		}
		item := legacyDomain{Domain: domain, Remark: remark}
		if certID.Valid && certID.Int64 > 0 {
			v := uint(certID.Int64)
			item.CertID = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
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

func nullableUint(v *uint) any {
	if v == nil {
		return nil
	}
	return *v
}

func rebindSQL(query string, postgres bool) string {
	if !postgres {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + len(query)/4)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

func hasTable(
	ctx context.Context,
	tx *sql.Tx,
	_ func(string) string,
	postgres bool,
	table string,
) (bool, error) {
	var count int
	var err error
	if postgres {
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		`, table).Scan(&count)
	} else {
		err = tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&count)
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func hasTableColumn(
	ctx context.Context,
	tx *sql.Tx,
	_ func(string) string,
	postgres bool,
	table, column string,
) (bool, error) {
	var count int
	var err error
	if postgres {
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
		`, table, column).Scan(&count)
	} else {
		err = tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column,
		).Scan(&count)
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
