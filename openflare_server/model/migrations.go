package model

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"gorm.io/gorm"
)

type databaseSchemaMigration struct {
	fromVersion int
	toVersion   int
	migrate     func(db *gorm.DB, backend string) error
	validate    func(db *gorm.DB, backend string) error
}

func autoMigrateSchemaMetadata(db *gorm.DB) error {
	for _, item := range schemaMetadataModels() {
		if err := db.AutoMigrate(item); err != nil {
			return err
		}
	}
	return nil
}

func migrateProxyRouteEnableHTTPSColumn(db *gorm.DB) error {
	if !db.Migrator().HasTable(&ProxyRoute{}) {
		return nil
	}
	if db.Migrator().HasColumn(&ProxyRoute{}, "enable_https") || !db.Migrator().HasColumn(&ProxyRoute{}, "enable_http_s") {
		return nil
	}
	return db.Migrator().RenameColumn(&ProxyRoute{}, "enable_http_s", "enable_https")
}

func migrateTextColumns(db *gorm.DB, backend string) error {
	if backend != "postgres" {
		return nil
	}
	type textColumn struct {
		model  any
		table  string
		column string
	}
	columns := []textColumn{
		{model: &Node{}, table: "nodes", column: "openresty_message"},
		{model: &Node{}, table: "nodes", column: "last_error"},
		{model: &ApplyLog{}, table: "apply_logs", column: "message"},
		{model: &NodeHealthEvent{}, table: "node_health_events", column: "message"},
	}
	for _, item := range columns {
		if !db.Migrator().HasTable(item.model) || !db.Migrator().HasColumn(item.model, item.column) {
			continue
		}
		sql := fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" TYPE text`, item.table, item.column)
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("migrate column %s.%s to text failed: %w", item.table, item.column, err)
		}
	}
	return nil
}

func migrateObservabilityLegacyColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&NodeHealthEvent{}) || !db.Migrator().HasColumn(&NodeHealthEvent{}, "raw_json") {
		return nil
	}
	type legacyHealthEventRaw struct {
		ID           uint
		RawJSON      string
		MetadataJSON string
	}
	type legacyHealthEventPayload struct {
		Metadata map[string]string `json:"metadata"`
	}

	var rows []legacyHealthEventRaw
	if err := db.Model(&NodeHealthEvent{}).
		Select("id, raw_json, metadata_json").
		Where("raw_json <> '' AND (metadata_json IS NULL OR metadata_json = '')").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("query legacy node health event raw_json failed: %w", err)
	}
	for _, row := range rows {
		var payload legacyHealthEventPayload
		if err := json.Unmarshal([]byte(row.RawJSON), &payload); err != nil {
			continue
		}
		if len(payload.Metadata) == 0 {
			continue
		}
		metadataJSON, err := json.Marshal(payload.Metadata)
		if err != nil {
			continue
		}
		if err := db.Model(&NodeHealthEvent{}).
			Where("id = ?", row.ID).
			Update("metadata_json", string(metadataJSON)).Error; err != nil {
			return fmt.Errorf("migrate node health event metadata_json failed: %w", err)
		}
	}
	return nil
}

func applyCurrentSchema(db *gorm.DB, backend string) error {
	if err := autoMigrateSchemaMetadata(db); err != nil {
		return err
	}
	if err := migrateProxyRouteEnableHTTPSColumn(db); err != nil {
		return err
	}
	if err := autoMigrateAll(db); err != nil {
		return err
	}
	if err := migrateTextColumns(db, backend); err != nil {
		return err
	}
	if err := migrateObservabilityLegacyColumns(db); err != nil {
		return err
	}
	return nil
}

func loadDatabaseSchemaVersion(db *gorm.DB) (int, bool, error) {
	if db == nil {
		return 0, false, nil
	}
	if !db.Migrator().HasTable(&DatabaseSchemaVersion{}) {
		return 0, false, nil
	}
	var state DatabaseSchemaVersion
	err := db.Where("id = ?", databaseSchemaVersionRowID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return state.Version, true, nil
}

func saveDatabaseSchemaVersion(db *gorm.DB, version int) error {
	return db.Save(&DatabaseSchemaVersion{
		ID:      databaseSchemaVersionRowID,
		Version: version,
	}).Error
}

func validateDatabaseSchemaV2(db *gorm.DB, backend string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasTable(&DatabaseSchemaVersion{}) {
		return fmt.Errorf("table %s is missing", (&DatabaseSchemaVersion{}).TableName())
	}
	models, err := buildDBModels()
	if err != nil {
		return err
	}
	for _, item := range models {
		if isShardedObservabilityTable(item.tableName) {
			for _, table := range observabilityShardTables(item.tableName) {
				if !db.Migrator().HasTable(table) {
					return fmt.Errorf("sharded table %s is missing", table)
				}
			}
			continue
		}
		if !db.Migrator().HasTable(item.value) {
			return fmt.Errorf("table %s is missing", item.tableName)
		}
	}
	if !db.Migrator().HasColumn(&NodeHealthEvent{}, "metadata_json") {
		return fmt.Errorf("column node_health_events.metadata_json is missing")
	}
	_ = backend
	return nil
}

func validateDatabaseSchemaV3(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV2(db, backend); err != nil {
		return err
	}
	for _, baseTable := range shardedObservabilityBaseTables() {
		for _, table := range observabilityShardTables(baseTable) {
			legacyTable := legacyObservabilityShardTableName(table)
			if db.Migrator().HasTable(legacyTable) {
				return fmt.Errorf("legacy sharded table %s still exists", legacyTable)
			}
		}
	}
	return nil
}

func validateDatabaseSchemaV4(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV3(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasTable(&Origin{}) {
		return fmt.Errorf("table origins is missing")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "origin_id") {
		return fmt.Errorf("column proxy_routes.origin_id is missing")
	}
	return nil
}

func normalizeProxyRouteDomainForMigration(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeProxyRouteSiteNameForMigration(raw string, primaryDomain string) string {
	siteName := strings.TrimSpace(raw)
	if siteName != "" {
		return siteName
	}
	return primaryDomain
}

func decodeProxyRouteDomainsForMigration(raw string, fallbackDomain string) ([]string, error) {
	primaryDomain := normalizeProxyRouteDomainForMigration(fallbackDomain)
	text := strings.TrimSpace(raw)
	if text == "" {
		if primaryDomain == "" {
			return nil, fmt.Errorf("proxy route primary domain is empty")
		}
		return []string{primaryDomain}, nil
	}

	var domains []string
	if err := json.Unmarshal([]byte(text), &domains); err != nil {
		return nil, fmt.Errorf("decode proxy route domains failed: %w", err)
	}

	normalized := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		item := normalizeProxyRouteDomainForMigration(domain)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		if primaryDomain == "" {
			return nil, fmt.Errorf("proxy route domains are empty")
		}
		return []string{primaryDomain}, nil
	}
	if primaryDomain == "" {
		primaryDomain = normalized[0]
	}
	if normalized[0] != primaryDomain {
		rest := make([]string, 0, len(normalized))
		for _, domain := range normalized {
			if domain == primaryDomain {
				continue
			}
			rest = append(rest, domain)
		}
		normalized = append([]string{primaryDomain}, rest...)
	}
	return normalized, nil
}

func backfillProxyRouteSiteFields(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasTable(&ProxyRoute{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "site_name") || !db.Migrator().HasColumn(&ProxyRoute{}, "domains") {
		return nil
	}

	var routes []ProxyRoute
	if err := db.Order("id asc").Find(&routes).Error; err != nil {
		return fmt.Errorf("list proxy routes for site field backfill failed: %w", err)
	}
	for _, route := range routes {
		domains, err := decodeProxyRouteDomainsForMigration(route.Domains, route.Domain)
		if err != nil {
			return fmt.Errorf("normalize proxy route %d domains failed: %w", route.ID, err)
		}
		domainsJSON, err := json.Marshal(domains)
		if err != nil {
			return fmt.Errorf("encode proxy route %d domains failed: %w", route.ID, err)
		}

		primaryDomain := domains[0]
		siteName := normalizeProxyRouteSiteNameForMigration(route.SiteName, primaryDomain)
		updates := make(map[string]any, 3)
		if route.Domain != primaryDomain {
			updates["domain"] = primaryDomain
		}
		if route.SiteName != siteName {
			updates["site_name"] = siteName
		}
		if strings.TrimSpace(route.Domains) != string(domainsJSON) {
			updates["domains"] = string(domainsJSON)
		}
		if len(updates) == 0 {
			continue
		}
		if err := db.Model(&ProxyRoute{}).Where("id = ?", route.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update proxy route %d site fields failed: %w", route.ID, err)
		}
	}
	return nil
}

func ensureProxyRouteSiteNameUniqueIndex(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasTable(&ProxyRoute{}) || !db.Migrator().HasColumn(&ProxyRoute{}, "site_name") {
		return nil
	}
	return db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_proxy_routes_site_name ON proxy_routes(site_name)`).Error
}

func decodeProxyRouteCertIDsForMigration(raw string, fallbackCertID *uint) ([]uint, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		if fallbackCertID == nil || *fallbackCertID == 0 {
			return []uint{}, nil
		}
		return []uint{*fallbackCertID}, nil
	}

	var certIDs []uint
	if err := json.Unmarshal([]byte(text), &certIDs); err != nil {
		return nil, fmt.Errorf("decode proxy route cert_ids failed: %w", err)
	}

	normalized := make([]uint, 0, len(certIDs))
	seen := make(map[uint]struct{}, len(certIDs))
	for _, certID := range certIDs {
		if certID == 0 {
			continue
		}
		if _, ok := seen[certID]; ok {
			continue
		}
		seen[certID] = struct{}{}
		normalized = append(normalized, certID)
	}
	if len(normalized) == 0 && fallbackCertID != nil && *fallbackCertID != 0 {
		return []uint{*fallbackCertID}, nil
	}
	return normalized, nil
}

func backfillProxyRouteCertificateFields(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasTable(&ProxyRoute{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "cert_ids") {
		return nil
	}

	var routes []ProxyRoute
	if err := db.Order("id asc").Find(&routes).Error; err != nil {
		return fmt.Errorf("list proxy routes for certificate field backfill failed: %w", err)
	}
	for _, route := range routes {
		certIDs, err := decodeProxyRouteCertIDsForMigration(route.CertIDs, route.CertID)
		if err != nil {
			return fmt.Errorf("normalize proxy route %d cert_ids failed: %w", route.ID, err)
		}
		certIDsJSON, err := json.Marshal(certIDs)
		if err != nil {
			return fmt.Errorf("encode proxy route %d cert_ids failed: %w", route.ID, err)
		}

		var primaryCertID *uint
		if len(certIDs) > 0 {
			primaryCertID = &certIDs[0]
		}

		updates := make(map[string]any, 2)
		if strings.TrimSpace(route.CertIDs) != string(certIDsJSON) {
			updates["cert_ids"] = string(certIDsJSON)
		}
		if (route.CertID == nil) != (primaryCertID == nil) || (route.CertID != nil && primaryCertID != nil && *route.CertID != *primaryCertID) {
			updates["cert_id"] = primaryCertID
		}
		if len(updates) == 0 {
			continue
		}
		if err := db.Model(&ProxyRoute{}).Where("id = ?", route.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update proxy route %d certificate fields failed: %w", route.ID, err)
		}
	}
	return nil
}

func decodeProxyRouteDomainCertIDsForMigration(
	raw string,
	domainCount int,
) ([]uint, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []uint{}, nil
	}

	var domainCertIDs []uint
	if err := json.Unmarshal([]byte(text), &domainCertIDs); err != nil {
		return nil, fmt.Errorf("decode proxy route domain_cert_ids failed: %w", err)
	}
	if len(domainCertIDs) == 0 {
		return []uint{}, nil
	}
	if domainCount > 0 && len(domainCertIDs) != domainCount {
		return nil, fmt.Errorf("proxy route domain_cert_ids length does not match domains")
	}

	normalized := make([]uint, len(domainCertIDs))
	copy(normalized, domainCertIDs)
	return normalized, nil
}

func parseLeafCertificateForMigration(certPEM string) (*x509.Certificate, error) {
	var firstErr error
	rest := []byte(certPEM)
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			return certificate, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fmt.Errorf("parse certificate pem failed")
}

func deriveProxyRouteDomainCertIDsForMigration(
	db *gorm.DB,
	domains []string,
	certIDs []uint,
) ([]uint, error) {
	if len(certIDs) == 0 {
		return []uint{}, nil
	}
	if len(certIDs) == 1 {
		result := make([]uint, len(domains))
		for index := range result {
			result[index] = certIDs[0]
		}
		return result, nil
	}
	if len(certIDs) == len(domains) {
		result := make([]uint, len(certIDs))
		copy(result, certIDs)
		return result, nil
	}

	var certificates []TLSCertificate
	if err := db.Where("id IN ?", certIDs).Find(&certificates).Error; err != nil {
		return nil, fmt.Errorf("load certificates for proxy route migration failed: %w", err)
	}
	certificateByID := make(map[uint]*x509.Certificate, len(certificates))
	for index := range certificates {
		leaf, err := parseLeafCertificateForMigration(certificates[index].CertPEM)
		if err != nil {
			return nil, fmt.Errorf("parse certificate %d for proxy route migration failed: %w", certificates[index].ID, err)
		}
		certificateByID[certificates[index].ID] = leaf
	}

	result := make([]uint, len(domains))
	for domainIndex, domain := range domains {
		if domainIndex < len(certIDs) {
			certificate := certificateByID[certIDs[domainIndex]]
			if certificate != nil && certificate.VerifyHostname(domain) == nil {
				result[domainIndex] = certIDs[domainIndex]
				continue
			}
		}

		assigned := uint(0)
		for _, certID := range certIDs {
			certificate := certificateByID[certID]
			if certificate != nil && certificate.VerifyHostname(domain) == nil {
				assigned = certID
				break
			}
		}
		if assigned == 0 {
			return nil, fmt.Errorf("no certificate covers domain %s", domain)
		}
		result[domainIndex] = assigned
	}
	return result, nil
}

func uniqueProxyRouteCertIDsFromDomainAssignments(domainCertIDs []uint) []uint {
	unique := make([]uint, 0, len(domainCertIDs))
	seen := make(map[uint]struct{}, len(domainCertIDs))
	for _, certID := range domainCertIDs {
		if certID == 0 {
			continue
		}
		if _, ok := seen[certID]; ok {
			continue
		}
		seen[certID] = struct{}{}
		unique = append(unique, certID)
	}
	return unique
}

func backfillProxyRouteDomainCertificateFields(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasTable(&ProxyRoute{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "domain_cert_ids") {
		return nil
	}

	var routes []ProxyRoute
	if err := db.Order("id asc").Find(&routes).Error; err != nil {
		return fmt.Errorf("list proxy routes for domain certificate field backfill failed: %w", err)
	}
	for _, route := range routes {
		domains, err := decodeProxyRouteDomainsForMigration(route.Domains, route.Domain)
		if err != nil {
			return fmt.Errorf("normalize proxy route %d domains failed: %w", route.ID, err)
		}
		certIDs, err := decodeProxyRouteCertIDsForMigration(route.CertIDs, route.CertID)
		if err != nil {
			return fmt.Errorf("normalize proxy route %d cert_ids failed: %w", route.ID, err)
		}

		domainCertIDs, err := decodeProxyRouteDomainCertIDsForMigration(
			route.DomainCertIDs,
			len(domains),
		)
		if err != nil {
			return fmt.Errorf("normalize proxy route %d domain_cert_ids failed: %w", route.ID, err)
		}
		if len(domainCertIDs) == 0 && len(certIDs) > 0 {
			domainCertIDs, err = deriveProxyRouteDomainCertIDsForMigration(
				db,
				domains,
				certIDs,
			)
			if err != nil {
				return fmt.Errorf("derive proxy route %d domain_cert_ids failed: %w", route.ID, err)
			}
		}
		if !route.EnableHTTPS {
			domainCertIDs = []uint{}
			certIDs = []uint{}
		}

		domainCertIDsJSON, err := json.Marshal(domainCertIDs)
		if err != nil {
			return fmt.Errorf("encode proxy route %d domain_cert_ids failed: %w", route.ID, err)
		}
		normalizedCertIDs := uniqueProxyRouteCertIDsFromDomainAssignments(domainCertIDs)
		if len(domainCertIDs) == 0 {
			normalizedCertIDs = []uint{}
		}
		certIDsJSON, err := json.Marshal(normalizedCertIDs)
		if err != nil {
			return fmt.Errorf("encode proxy route %d cert_ids failed: %w", route.ID, err)
		}

		var primaryCertID *uint
		if len(normalizedCertIDs) > 0 {
			primaryCertID = &normalizedCertIDs[0]
		}

		updates := make(map[string]any, 3)
		if strings.TrimSpace(route.DomainCertIDs) != string(domainCertIDsJSON) {
			updates["domain_cert_ids"] = string(domainCertIDsJSON)
		}
		if strings.TrimSpace(route.CertIDs) != string(certIDsJSON) {
			updates["cert_ids"] = string(certIDsJSON)
		}
		if (route.CertID == nil) != (primaryCertID == nil) || (route.CertID != nil && primaryCertID != nil && *route.CertID != *primaryCertID) {
			updates["cert_id"] = primaryCertID
		}
		if len(updates) == 0 {
			continue
		}
		if err := db.Model(&ProxyRoute{}).Where("id = ?", route.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update proxy route %d domain certificate fields failed: %w", route.ID, err)
		}
	}
	return nil
}

func validateDatabaseSchemaV5(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV4(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "site_name") {
		return fmt.Errorf("column proxy_routes.site_name is missing")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "domains") {
		return fmt.Errorf("column proxy_routes.domains is missing")
	}

	var routes []ProxyRoute
	if err := db.Order("id asc").Find(&routes).Error; err != nil {
		return fmt.Errorf("list proxy routes for validation failed: %w", err)
	}

	siteNames := make(map[string]uint, len(routes))
	domainOwners := make(map[string]uint, len(routes))
	for _, route := range routes {
		domains, err := decodeProxyRouteDomainsForMigration(route.Domains, route.Domain)
		if err != nil {
			return fmt.Errorf("proxy route %d domains are invalid: %w", route.ID, err)
		}
		if len(domains) == 0 {
			return fmt.Errorf("proxy route %d domains are empty", route.ID)
		}
		if route.Domain != domains[0] {
			return fmt.Errorf("proxy route %d primary domain mirror is invalid", route.ID)
		}

		siteName := normalizeProxyRouteSiteNameForMigration(route.SiteName, domains[0])
		if siteName == "" {
			return fmt.Errorf("proxy route %d site_name is empty", route.ID)
		}
		if existingID, ok := siteNames[siteName]; ok && existingID != route.ID {
			return fmt.Errorf("proxy route site_name %s is duplicated", siteName)
		}
		siteNames[siteName] = route.ID

		localSeen := make(map[string]struct{}, len(domains))
		for _, domain := range domains {
			if _, ok := localSeen[domain]; ok {
				return fmt.Errorf("proxy route %d contains duplicated domain %s", route.ID, domain)
			}
			localSeen[domain] = struct{}{}
			if existingID, ok := domainOwners[domain]; ok && existingID != route.ID {
				return fmt.Errorf("proxy route domain %s is duplicated", domain)
			}
			domainOwners[domain] = route.ID
		}
	}
	return nil
}

func validateDatabaseSchemaV6(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV5(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "limit_conn_per_server") {
		return fmt.Errorf("column proxy_routes.limit_conn_per_server is missing")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "limit_conn_per_ip") {
		return fmt.Errorf("column proxy_routes.limit_conn_per_ip is missing")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "limit_rate") {
		return fmt.Errorf("column proxy_routes.limit_rate is missing")
	}
	return nil
}

func validateDatabaseSchemaV7(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV6(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "cert_ids") {
		return fmt.Errorf("column proxy_routes.cert_ids is missing")
	}

	var routes []ProxyRoute
	if err := db.Order("id asc").Find(&routes).Error; err != nil {
		return fmt.Errorf("list proxy routes for certificate validation failed: %w", err)
	}
	for _, route := range routes {
		certIDs, err := decodeProxyRouteCertIDsForMigration(route.CertIDs, route.CertID)
		if err != nil {
			return fmt.Errorf("proxy route %d cert_ids are invalid: %w", route.ID, err)
		}
		if route.EnableHTTPS && len(certIDs) == 0 {
			return fmt.Errorf("proxy route %d has https enabled without cert_ids", route.ID)
		}
		if !route.EnableHTTPS && route.RedirectHTTP {
			return fmt.Errorf("proxy route %d enables redirect_http without https", route.ID)
		}
		if len(certIDs) == 0 {
			if route.CertID != nil {
				return fmt.Errorf("proxy route %d primary cert_id mirror is invalid", route.ID)
			}
			continue
		}
		if route.CertID == nil || *route.CertID != certIDs[0] {
			return fmt.Errorf("proxy route %d primary cert_id mirror is invalid", route.ID)
		}
	}
	return nil
}

func validateDatabaseSchemaV8(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV7(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "domain_cert_ids") {
		return fmt.Errorf("column proxy_routes.domain_cert_ids is missing")
	}

	var routes []ProxyRoute
	if err := db.Order("id asc").Find(&routes).Error; err != nil {
		return fmt.Errorf("list proxy routes for domain certificate validation failed: %w", err)
	}
	for _, route := range routes {
		domains, err := decodeProxyRouteDomainsForMigration(route.Domains, route.Domain)
		if err != nil {
			return fmt.Errorf("proxy route %d domains are invalid: %w", route.ID, err)
		}
		domainCertIDs, err := decodeProxyRouteDomainCertIDsForMigration(route.DomainCertIDs, len(domains))
		if err != nil {
			return fmt.Errorf("proxy route %d domain_cert_ids are invalid: %w", route.ID, err)
		}
		certIDs, err := decodeProxyRouteCertIDsForMigration(route.CertIDs, route.CertID)
		if err != nil {
			return fmt.Errorf("proxy route %d cert_ids are invalid: %w", route.ID, err)
		}
		if !route.EnableHTTPS {
			if len(domainCertIDs) != 0 {
				return fmt.Errorf("proxy route %d has domain_cert_ids while https is disabled", route.ID)
			}
			continue
		}
		if len(domainCertIDs) != len(domains) {
			return fmt.Errorf("proxy route %d domain_cert_ids length is invalid", route.ID)
		}
		normalizedCertIDs := uniqueProxyRouteCertIDsFromDomainAssignments(domainCertIDs)
		if len(normalizedCertIDs) == 0 {
			return fmt.Errorf("proxy route %d has https enabled without domain certificate assignments", route.ID)
		}
		if !uintSlicesEqualForMigration(certIDs, normalizedCertIDs) {
			return fmt.Errorf("proxy route %d cert_ids mirror is invalid", route.ID)
		}
		if route.CertID == nil || *route.CertID != normalizedCertIDs[0] {
			return fmt.Errorf("proxy route %d primary cert_id mirror is invalid", route.ID)
		}
	}
	return nil
}

func uintSlicesEqualForMigration(left []uint, right []uint) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func renameLegacyObservabilityShardTables(db *gorm.DB) error {
	for _, baseTable := range shardedObservabilityBaseTables() {
		for _, table := range observabilityShardTables(baseTable) {
			legacyTable := legacyObservabilityShardTableName(table)
			if db.Migrator().HasTable(legacyTable) {
				return fmt.Errorf("legacy sharded table %s already exists", legacyTable)
			}
			if !db.Migrator().HasTable(table) {
				continue
			}
			if err := db.Migrator().RenameTable(table, legacyTable); err != nil {
				return fmt.Errorf("rename sharded table %s to %s failed: %w", table, legacyTable, err)
			}
			if err := dropLegacyObservabilitySecondaryIndexes(db, legacyTable); err != nil {
				return err
			}
		}
	}
	return nil
}

func dropLegacyObservabilitySecondaryIndexes(db *gorm.DB, table string) error {
	db = sessionIgnoringSharding(db)
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	backend := baseDialector(db).Name()
	indexes := make([]string, 0)
	switch backend {
	case "sqlite":
		if err := db.Raw(
			`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND name LIKE 'idx_%'`,
			table,
		).Scan(&indexes).Error; err != nil {
			return fmt.Errorf("list indexes for %s failed: %w", table, err)
		}
	case "postgres":
		if err := db.Raw(
			`SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = ? AND indexname LIKE 'idx_%'`,
			table,
		).Scan(&indexes).Error; err != nil {
			return fmt.Errorf("list indexes for %s failed: %w", table, err)
		}
	default:
		return fmt.Errorf("unsupported database backend %s", backend)
	}
	for _, indexName := range indexes {
		if err := db.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, indexName)).Error; err != nil {
			return fmt.Errorf("drop legacy index %s failed: %w", indexName, err)
		}
	}
	return nil
}

func autoMigrateObservabilityShardTables(db *gorm.DB) error {
	db = sessionIgnoringSharding(db)
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	dialector := baseDialector(db)
	if dialector == nil {
		return fmt.Errorf("database dialector is nil")
	}
	type shardedTable struct {
		model any
		base  string
	}
	tables := []shardedTable{
		{model: &NodeMetricSnapshot{}, base: "node_metric_snapshots"},
		{model: &NodeRequestReport{}, base: "node_request_reports"},
		{model: &NodeAccessLog{}, base: "node_access_logs"},
	}
	for _, item := range tables {
		for _, table := range observabilityShardTables(item.base) {
			tx := db.Table(table)
			if err := dialector.Migrator(tx).AutoMigrate(item.model); err != nil {
				return fmt.Errorf("auto migrate sharded table %s failed: %w", table, err)
			}
		}
	}
	return nil
}

func dropLegacyObservabilityShardTables(db *gorm.DB) error {
	db = sessionIgnoringSharding(db)
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	for _, baseTable := range shardedObservabilityBaseTables() {
		for _, table := range observabilityShardTables(baseTable) {
			legacyTable := legacyObservabilityShardTableName(table)
			if !db.Migrator().HasTable(legacyTable) {
				continue
			}
			if err := db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, legacyTable)).Error; err != nil {
				return fmt.Errorf("drop legacy sharded table %s failed: %w", legacyTable, err)
			}
		}
	}
	return nil
}

func migrateLegacyNodeMetricSnapshots(db *gorm.DB) error {
	for _, table := range observabilityShardTables("node_metric_snapshots") {
		legacyTable := legacyObservabilityShardTableName(table)
		if !db.Migrator().HasTable(legacyTable) {
			continue
		}
		var lastSeenID uint
		for {
			var rows []NodeMetricSnapshot
			query := db.Table(legacyTable).Order("id ASC").Limit(500)
			if lastSeenID > 0 {
				query = query.Where("id > ?", lastSeenID)
			}
			if err := query.Find(&rows).Error; err != nil {
				return fmt.Errorf("query legacy sharded table %s failed: %w", legacyTable, err)
			}
			if len(rows) == 0 {
				break
			}
			lastSeenID = rows[len(rows)-1].ID
			grouped := make(map[string][]NodeMetricSnapshot, observabilityShardCount)
			for index := range rows {
				rows[index].ID = 0
				if err := assignObservabilityID(&rows[index].ID); err != nil {
					return err
				}
				targetTable := observabilityShardTableForID("node_metric_snapshots", rows[index].ID)
				grouped[targetTable] = append(grouped[targetTable], rows[index])
			}
			for targetTable, batch := range grouped {
				if err := db.Table(targetTable).Create(&batch).Error; err != nil {
					return fmt.Errorf("write migrated rows into %s failed: %w", targetTable, err)
				}
			}
		}
	}
	return nil
}

func migrateLegacyNodeRequestReports(db *gorm.DB) error {
	for _, table := range observabilityShardTables("node_request_reports") {
		legacyTable := legacyObservabilityShardTableName(table)
		if !db.Migrator().HasTable(legacyTable) {
			continue
		}
		var lastSeenID uint
		for {
			var rows []NodeRequestReport
			query := db.Table(legacyTable).Order("id ASC").Limit(500)
			if lastSeenID > 0 {
				query = query.Where("id > ?", lastSeenID)
			}
			if err := query.Find(&rows).Error; err != nil {
				return fmt.Errorf("query legacy sharded table %s failed: %w", legacyTable, err)
			}
			if len(rows) == 0 {
				break
			}
			lastSeenID = rows[len(rows)-1].ID
			grouped := make(map[string][]NodeRequestReport, observabilityShardCount)
			for index := range rows {
				rows[index].ID = 0
				if err := assignObservabilityID(&rows[index].ID); err != nil {
					return err
				}
				targetTable := observabilityShardTableForID("node_request_reports", rows[index].ID)
				grouped[targetTable] = append(grouped[targetTable], rows[index])
			}
			for targetTable, batch := range grouped {
				if err := db.Table(targetTable).Create(&batch).Error; err != nil {
					return fmt.Errorf("write migrated rows into %s failed: %w", targetTable, err)
				}
			}
		}
	}
	return nil
}

func migrateLegacyNodeAccessLogs(db *gorm.DB) error {
	for _, table := range observabilityShardTables("node_access_logs") {
		legacyTable := legacyObservabilityShardTableName(table)
		if !db.Migrator().HasTable(legacyTable) {
			continue
		}
		var lastSeenID uint
		for {
			var rows []NodeAccessLog
			query := db.Table(legacyTable).Order("id ASC").Limit(500)
			if lastSeenID > 0 {
				query = query.Where("id > ?", lastSeenID)
			}
			if err := query.Find(&rows).Error; err != nil {
				return fmt.Errorf("query legacy sharded table %s failed: %w", legacyTable, err)
			}
			if len(rows) == 0 {
				break
			}
			lastSeenID = rows[len(rows)-1].ID
			grouped := make(map[string][]NodeAccessLog, observabilityShardCount)
			for index := range rows {
				rows[index].ID = 0
				if err := assignObservabilityID(&rows[index].ID); err != nil {
					return err
				}
				targetTable := observabilityShardTableForID("node_access_logs", rows[index].ID)
				grouped[targetTable] = append(grouped[targetTable], rows[index])
			}
			for targetTable, batch := range grouped {
				if err := db.Table(targetTable).Create(&batch).Error; err != nil {
					return fmt.Errorf("write migrated rows into %s failed: %w", targetTable, err)
				}
			}
		}
	}
	return nil
}

func normalizeOriginAddressForMigration(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func extractOriginAddressForMigration(rawURL string) string {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return normalizeOriginAddressForMigration(parsed.Hostname())
}

func backfillOriginsFromProxyRoutes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasTable(&Origin{}) || !db.Migrator().HasTable(&ProxyRoute{}) {
		return nil
	}

	var routes []ProxyRoute
	if err := db.Order("id asc").Find(&routes).Error; err != nil {
		return fmt.Errorf("list proxy routes for origin backfill failed: %w", err)
	}

	type originSeed struct {
		ID      uint
		Address string
	}

	originByAddress := make(map[string]originSeed)
	var origins []Origin
	if err := db.Order("id asc").Find(&origins).Error; err != nil {
		return fmt.Errorf("list origins for backfill failed: %w", err)
	}
	for _, origin := range origins {
		address := normalizeOriginAddressForMigration(origin.Address)
		if address == "" {
			continue
		}
		originByAddress[address] = originSeed{ID: origin.ID, Address: address}
	}

	for _, route := range routes {
		address := extractOriginAddressForMigration(route.OriginURL)
		if address == "" {
			continue
		}
		origin, ok := originByAddress[address]
		if !ok {
			name := address
			if ip := net.ParseIP(address); ip != nil {
				name = ip.String()
			}
			record := Origin{
				Name:    name,
				Address: address,
				Remark:  "",
			}
			if err := db.Create(&record).Error; err != nil {
				return fmt.Errorf("create origin for address %s failed: %w", address, err)
			}
			origin = originSeed{ID: record.ID, Address: address}
			originByAddress[address] = origin
		}
		if route.OriginID != nil && *route.OriginID == origin.ID {
			continue
		}
		if err := db.Model(&ProxyRoute{}).
			Where("id = ?", route.ID).
			Update("origin_id", origin.ID).Error; err != nil {
			return fmt.Errorf("backfill proxy route %d origin_id failed: %w", route.ID, err)
		}
	}

	return nil
}

// migrateV2 upgrades the legacy schema to the first versioned schema by
// creating schema metadata, applying the current tables, and backfilling
// compatibility columns.
func migrateV2(db *gorm.DB, backend string) error {
	return applyCurrentSchema(db, backend)
}

// migrateV3 upgrades observability shard tables from legacy ID layout to the
// current ID-sharded layout and migrates existing shard data into the new tables.
func migrateV3(db *gorm.DB, backend string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	_ = backend
	if err := renameLegacyObservabilityShardTables(db); err != nil {
		return err
	}
	if err := autoMigrateObservabilityShardTables(db); err != nil {
		return err
	}
	if err := migrateLegacyNodeMetricSnapshots(db); err != nil {
		return err
	}
	if err := migrateLegacyNodeRequestReports(db); err != nil {
		return err
	}
	if err := migrateLegacyNodeAccessLogs(db); err != nil {
		return err
	}
	return dropLegacyObservabilityShardTables(db)
}

// migrateV4 introduces the origins schema and backfills proxy route origin
// references from existing origin_url values.
func migrateV4(db *gorm.DB, backend string) error {
	if err := applyCurrentSchema(db, backend); err != nil {
		return err
	}
	return backfillOriginsFromProxyRoutes(db)
}

// migrateV5 upgrades proxy_routes to website-level identity fields by
// backfilling site_name and domains while keeping domain as the primary-domain
// compatibility mirror.
func migrateV5(db *gorm.DB, backend string) error {
	if err := applyCurrentSchema(db, backend); err != nil {
		return err
	}
	if err := backfillOriginsFromProxyRoutes(db); err != nil {
		return err
	}
	if err := backfillProxyRouteSiteFields(db); err != nil {
		return err
	}
	return ensureProxyRouteSiteNameUniqueIndex(db)
}

// migrateV6 adds structured website-level rate limit fields to proxy_routes.
func migrateV6(db *gorm.DB, backend string) error {
	if err := applyCurrentSchema(db, backend); err != nil {
		return err
	}
	if err := backfillOriginsFromProxyRoutes(db); err != nil {
		return err
	}
	if err := backfillProxyRouteSiteFields(db); err != nil {
		return err
	}
	return ensureProxyRouteSiteNameUniqueIndex(db)
}

// migrateV7 adds structured website-level certificate lists to proxy_routes
// while keeping cert_id as the primary certificate compatibility mirror.
func migrateV7(db *gorm.DB, backend string) error {
	if err := applyCurrentSchema(db, backend); err != nil {
		return err
	}
	if err := backfillOriginsFromProxyRoutes(db); err != nil {
		return err
	}
	if err := backfillProxyRouteSiteFields(db); err != nil {
		return err
	}
	if err := ensureProxyRouteSiteNameUniqueIndex(db); err != nil {
		return err
	}
	return backfillProxyRouteCertificateFields(db)
}

// migrateV8 adds per-domain certificate assignments to proxy_routes while
// keeping cert_ids as the website-level compatibility mirror.
func migrateV8(db *gorm.DB, backend string) error {
	if err := applyCurrentSchema(db, backend); err != nil {
		return err
	}
	if err := backfillOriginsFromProxyRoutes(db); err != nil {
		return err
	}
	if err := backfillProxyRouteSiteFields(db); err != nil {
		return err
	}
	if err := ensureProxyRouteSiteNameUniqueIndex(db); err != nil {
		return err
	}
	if err := backfillProxyRouteCertificateFields(db); err != nil {
		return err
	}
	return backfillProxyRouteDomainCertificateFields(db)
}

func databaseSchemaMigrations() []databaseSchemaMigration {
	return []databaseSchemaMigration{
		{fromVersion: 1, toVersion: 2, migrate: migrateV2, validate: validateDatabaseSchemaV2},
		{fromVersion: 2, toVersion: 3, migrate: migrateV3, validate: validateDatabaseSchemaV3},
		{fromVersion: 3, toVersion: 4, migrate: migrateV4, validate: validateDatabaseSchemaV4},
		{fromVersion: 4, toVersion: 5, migrate: migrateV5, validate: validateDatabaseSchemaV5},
		{fromVersion: 5, toVersion: 6, migrate: migrateV6, validate: validateDatabaseSchemaV6},
		{fromVersion: 6, toVersion: 7, migrate: migrateV7, validate: validateDatabaseSchemaV7},
		{fromVersion: 7, toVersion: 8, migrate: migrateV8, validate: validateDatabaseSchemaV8},
	}
}

func databaseSchemaMigrationMap() map[int]databaseSchemaMigration {
	migrations := make(map[int]databaseSchemaMigration, len(databaseSchemaMigrations()))
	for _, item := range databaseSchemaMigrations() {
		migrations[item.fromVersion] = item
	}
	return migrations
}

func runDatabaseSchemaMigration(db *gorm.DB, backend string, migration databaseSchemaMigration) error {
	if backend == "sqlite" {
		if err := migration.migrate(db, backend); err != nil {
			return fmt.Errorf("migrate database schema from v%d to v%d failed: %w", migration.fromVersion, migration.toVersion, err)
		}
		if err := migration.validate(db, backend); err != nil {
			return fmt.Errorf("validate database schema v%d failed: %w", migration.toVersion, err)
		}
		if err := saveDatabaseSchemaVersion(db, migration.toVersion); err != nil {
			return fmt.Errorf("persist database schema version v%d failed: %w", migration.toVersion, err)
		}
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := migration.migrate(tx, backend); err != nil {
			return fmt.Errorf("migrate database schema from v%d to v%d failed: %w", migration.fromVersion, migration.toVersion, err)
		}
		if err := migration.validate(tx, backend); err != nil {
			return fmt.Errorf("validate database schema v%d failed: %w", migration.toVersion, err)
		}
		if err := saveDatabaseSchemaVersion(tx, migration.toVersion); err != nil {
			return fmt.Errorf("persist database schema version v%d failed: %w", migration.toVersion, err)
		}
		return nil
	})
}

func upgradeDatabaseSchema(db *gorm.DB, backend string, version int) error {
	if version > currentDatabaseSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than application version %d", version, currentDatabaseSchemaVersion)
	}
	if version == currentDatabaseSchemaVersion {
		return nil
	}
	migrationMap := databaseSchemaMigrationMap()
	for version < currentDatabaseSchemaVersion {
		migration, ok := migrationMap[version]
		if !ok {
			return fmt.Errorf("database schema migration from v%d is not defined", version)
		}
		if err := runDatabaseSchemaMigration(db, backend, migration); err != nil {
			return err
		}
		version = migration.toVersion
	}
	return nil
}

func initializeFreshDatabaseSchema(db *gorm.DB, backend string) error {
	if err := applyCurrentSchema(db, backend); err != nil {
		return err
	}
	if err := migrateSQLiteDataIfNeeded(db, backend); err != nil {
		return err
	}
	if err := backfillOriginsFromProxyRoutes(db); err != nil {
		return err
	}
	if err := backfillProxyRouteSiteFields(db); err != nil {
		return err
	}
	if err := ensureProxyRouteSiteNameUniqueIndex(db); err != nil {
		return err
	}
	if err := backfillProxyRouteCertificateFields(db); err != nil {
		return err
	}
	if err := backfillProxyRouteDomainCertificateFields(db); err != nil {
		return err
	}
	if err := validateDatabaseSchemaV8(db, backend); err != nil {
		return err
	}
	return saveDatabaseSchemaVersion(db, currentDatabaseSchemaVersion)
}

func ensureDatabaseSchemaUpToDate(db *gorm.DB, backend string) error {
	version, exists, err := loadDatabaseSchemaVersion(db)
	if err != nil {
		return err
	}
	if exists {
		return upgradeDatabaseSchema(db, backend, version)
	}
	empty, err := isDatabaseEmpty(db)
	if err != nil {
		return err
	}
	if empty {
		return initializeFreshDatabaseSchema(db, backend)
	}
	if err := autoMigrateSchemaMetadata(db); err != nil {
		return err
	}
	return upgradeDatabaseSchema(db, backend, legacyDatabaseSchemaVersion)
}
