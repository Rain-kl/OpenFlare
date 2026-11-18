// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/openflare/routeidentity"
	"golang.org/x/net/idna"
)

const (
	errLegacyDomainInvalid  = "invalid domain name format"
	errLegacyCertNotFound   = "selected certificate was not found"
	legacySQLRebindCapacity = 4
)

// ZoneImportReport describes the idempotent legacy zone-domain migration result.
type ZoneImportReport struct {
	Zones     int      `json:"zones"`
	Domains   int      `json:"domains"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// LogAndReturn decorates the import failure with its conflict count.
func (r ZoneImportReport) LogAndReturn(err error) error {
	if err != nil {
		return fmt.Errorf("failed to migrate zone (%d conflicts): %w", len(r.Conflicts), err)
	}
	return nil
}

type legacyZoneDomain struct {
	Domain       string
	CertID       *uint
	ProxyRouteID *uint
}

// ImportZoneLegacyTx imports legacy proxy-route / managed-domain rows into Zone tables
// within an existing SQL transaction (goose runs this on Server upgrade).
// postgres selects $n placeholders; otherwise SQLite-style ? is used.
// Missing legacy columns or tables are skipped so re-runs after phase-2 cleanup are no-ops.
func ImportZoneLegacyTx(ctx context.Context, tx *sql.Tx, postgres bool) (report ZoneImportReport, err error) {
	if tx == nil {
		return report, errors.New("transaction is required")
	}
	q := func(sqlText string) string { return rebindSQL(sqlText, postgres) }

	items, conflicts, err := collectAllLegacyDomains(ctx, tx, q, postgres)
	if err != nil {
		return report, err
	}
	report.Conflicts = append(report.Conflicts, conflicts...)

	for _, item := range items {
		if err := importSingleLegacyDomain(ctx, tx, q, item, &report); err != nil {
			return report, err
		}
	}

	if len(report.Conflicts) > 0 {
		return report, errors.New("legacy data has conflicts")
	}
	return report, nil
}

func collectAllLegacyDomains(
	ctx context.Context,
	tx *sql.Tx,
	q func(string) string,
	postgres bool,
) ([]legacyZoneDomain, []string, error) {
	hasDomainCol, err := hasTableColumn(ctx, tx, q, postgres, "of_proxy_routes", "domain")
	if err != nil {
		return nil, nil, err
	}
	hasDomainsCol, err := hasTableColumn(ctx, tx, q, postgres, "of_proxy_routes", "domains")
	if err != nil {
		return nil, nil, err
	}

	if hasDomainCol && hasDomainsCol {
		items, hasRouteDomains, conflicts, collectErr := collectLegacyRouteDomainsImpl(ctx, tx, q)
		if collectErr != nil {
			return nil, nil, collectErr
		}
		if hasRouteDomains {
			return items, conflicts, nil
		}
		managed, managedErr := collectManagedIfTableExists(ctx, tx, q, postgres)
		if managedErr != nil {
			return nil, nil, managedErr
		}
		return append(items, managed...), conflicts, nil
	}

	managed, managedErr := collectManagedIfTableExists(ctx, tx, q, postgres)
	if managedErr != nil {
		return nil, nil, managedErr
	}
	return managed, nil, nil
}

func collectManagedIfTableExists(
	ctx context.Context,
	tx *sql.Tx,
	q func(string) string,
	postgres bool,
) ([]legacyZoneDomain, error) {
	exists, tableErr := hasTable(ctx, tx, q, postgres, "of_managed_domains")
	if tableErr != nil {
		return nil, tableErr
	}
	if !exists {
		return nil, nil
	}
	return collectLegacyManagedDomains(ctx, tx, q)
}

func importSingleLegacyDomain(
	ctx context.Context,
	tx *sql.Tx,
	q func(string) string,
	item legacyZoneDomain,
	report *ZoneImportReport,
) error {
	domain, normErr := normalizeLegacyDomain(item.Domain)
	if normErr != nil {
		report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %v", item.Domain, normErr))
		return nil
	}
	root, rootErr := routeidentity.ZoneRoot(domain)
	if rootErr != nil {
		report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %v", domain, rootErr))
		return nil
	}

	var existingID uint
	var existingZoneDomain string
	var existingRouteID, existingCertID sql.Null[uint]
	scanErr := tx.QueryRowContext(ctx, q(`
		SELECT zd.id, z.domain, zd.proxy_route_id, zd.cert_id
		FROM of_zone_domains zd
		JOIN of_zones z ON z.id = zd.zone_id
		WHERE zd.domain = ?
	`), domain).Scan(&existingID, &existingZoneDomain, &existingRouteID, &existingCertID)
	if scanErr == nil {
		return reconcileExistingZoneDomain(ctx, tx, q, item, domain, root, existingID, existingZoneDomain, existingRouteID, existingCertID, report)
	}
	if !errors.Is(scanErr, sql.ErrNoRows) {
		return scanErr
	}

	zoneID, zoneErr := ensureZone(ctx, tx, q, root, report)
	if zoneErr != nil {
		return zoneErr
	}

	if item.CertID != nil {
		var certID uint
		if certErr := tx.QueryRowContext(ctx, q(`SELECT id FROM of_tls_certificates WHERE id = ?`), *item.CertID).
			Scan(&certID); certErr != nil {
			if errors.Is(certErr, sql.ErrNoRows) {
				report.Conflicts = append(report.Conflicts, fmt.Sprintf("%s: %s", domain, errLegacyCertNotFound))
				return nil
			}
			return certErr
		}
	}

	if _, insErr := tx.ExecContext(ctx, q(`
		INSERT INTO of_zone_domains (zone_id, proxy_route_id, domain, cert_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`), zoneID, nullableUint(item.ProxyRouteID), domain, nullableUint(item.CertID)); insErr != nil {
		return insErr
	}
	report.Domains++
	return nil
}

func reconcileExistingZoneDomain(
	ctx context.Context,
	tx *sql.Tx,
	q func(string) string,
	item legacyZoneDomain,
	domain, root string,
	existingID uint,
	existingZoneDomain string,
	existingRouteID, existingCertID sql.Null[uint],
	report *ZoneImportReport,
) error {
	if (item.ProxyRouteID != nil && existingRouteID.Valid && existingRouteID.V != *item.ProxyRouteID) ||
		(item.CertID != nil && existingCertID.Valid && existingCertID.V != *item.CertID) {
		report.Conflicts = append(report.Conflicts, domain+": canonical domain binding conflict")
		return nil
	}
	if existingZoneDomain != root {
		report.Conflicts = append(report.Conflicts, domain+": global domain conflict")
		return nil
	}
	if item.ProxyRouteID == nil && item.CertID == nil {
		return nil
	}
	if _, bindErr := tx.ExecContext(ctx, q(`
		UPDATE of_zone_domains
		SET proxy_route_id = COALESCE(proxy_route_id, ?),
		    cert_id = COALESCE(cert_id, ?)
		WHERE id = ?
	`), nullableUint(item.ProxyRouteID), nullableUint(item.CertID), existingID); bindErr != nil {
		return bindErr
	}
	return nil
}

// normalizeLegacyDomain canonicalizes historical IDNs for DNS identity and rendering.
// Certificate associations and route bindings remain attached to the imported row.
func normalizeLegacyDomain(raw string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(raw))
	host := strings.TrimPrefix(domain, "*.")
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", errors.New(errLegacyDomainInvalid)
	}
	if host != domain {
		ascii = "*." + ascii
	}
	return routeidentity.NormalizeDomain(ascii)
}

func ensureZone(
	ctx context.Context,
	tx *sql.Tx,
	q func(string) string,
	root string,
	report *ZoneImportReport,
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
		INSERT INTO of_zones (domain, created_at, updated_at)
		VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
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
) (items []legacyZoneDomain, hasRouteDomains bool, conflicts []string, err error) {
	// Probe domain_cert_ids: if SELECT fails, fall back without it.
	queryWithCert := q(`SELECT id, domain, domains, COALESCE(domain_cert_ids, '[]') FROM of_proxy_routes`)
	rows, err := tx.QueryContext(ctx, queryWithCert)
	useCert := true
	if err != nil {
		useCert = false
		rows, err = tx.QueryContext(ctx, q(`SELECT id, domain, domains FROM of_proxy_routes`))
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
		)
		if useCert {
			if err := rows.Scan(&id, &domain, &domains, &certIDs); err != nil {
				return nil, false, nil, err
			}
		} else {
			if err := rows.Scan(&id, &domain, &domains); err != nil {
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
			items = append(items, legacyZoneDomain{
				Domain:       d,
				CertID:       certID,
				ProxyRouteID: &routeID,
			})
		}
	}
	return items, hasRouteDomains, conflicts, rows.Err()
}

func collectLegacyManagedDomains(ctx context.Context, tx *sql.Tx, q func(string) string) ([]legacyZoneDomain, error) {
	rows, err := tx.QueryContext(ctx, q(`SELECT domain, cert_id FROM of_managed_domains`))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]legacyZoneDomain, 0)
	for rows.Next() {
		var (
			domain string
			certID sql.NullInt64
		)
		if err := rows.Scan(&domain, &certID); err != nil {
			return nil, err
		}
		item := legacyZoneDomain{Domain: domain}
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
	b.Grow(len(query) + len(query)/legacySQLRebindCapacity)
	n := 0
	for i := range len(query) {
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
