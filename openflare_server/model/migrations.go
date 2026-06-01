package model

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	schemamigrate "openflare/model/migrate"
	"strings"

	"gorm.io/gorm"
)

type databaseSchemaMigration struct {
	fromVersion int
	toVersion   int
	migrate     func(db *gorm.DB, backend string) error
	validate    func(db *gorm.DB, backend string) error
}

type databaseSchemaMigrationContext struct{}

func (databaseSchemaMigrationContext) ApplyCurrentSchema(db *gorm.DB, backend string) error {
	return applyCurrentSchema(db, backend)
}

func (databaseSchemaMigrationContext) BackfillOriginsFromProxyRoutes(db *gorm.DB) error {
	return backfillOriginsFromProxyRoutes(db)
}

func (databaseSchemaMigrationContext) BackfillProxyRouteSiteFields(db *gorm.DB) error {
	return backfillProxyRouteSiteFields(db)
}

func (databaseSchemaMigrationContext) EnsureProxyRouteSiteNameUniqueIndex(db *gorm.DB) error {
	return ensureProxyRouteSiteNameUniqueIndex(db)
}

func (databaseSchemaMigrationContext) BackfillProxyRouteCertificateFields(db *gorm.DB) error {
	return backfillProxyRouteCertificateFields(db)
}

func (databaseSchemaMigrationContext) BackfillProxyRouteDomainCertificateFields(db *gorm.DB) error {
	return backfillProxyRouteDomainCertificateFields(db)
}

func (databaseSchemaMigrationContext) EnsureDefaultGitHubAuthSource(db *gorm.DB) error {
	return ensureDefaultGitHubAuthSource(db)
}

func (databaseSchemaMigrationContext) EnsureDefaultWAFRuleGroup(db *gorm.DB) error {
	return ensureDefaultWAFRuleGroup(db)
}

func (databaseSchemaMigrationContext) ValidateDatabaseSchemaVersion(db *gorm.DB, backend string, version int) error {
	switch version {
	case 7:
		return validateDatabaseSchemaV7(db, backend)
	case 8:
		return validateDatabaseSchemaV8(db, backend)
	case 9:
		return validateDatabaseSchemaV9(db, backend)
	case 10:
		return validateDatabaseSchemaV10(db, backend)
	case 11:
		return validateDatabaseSchemaV11(db, backend)
	case 12:
		return validateDatabaseSchemaV12(db, backend)
	case 13:
		return validateDatabaseSchemaV13(db, backend)
	case 14:
		return validateDatabaseSchemaV14(db, backend)
	case 15:
		return validateDatabaseSchemaV15(db, backend)
	case 16:
		return validateDatabaseSchemaV16(db, backend)
	case 17:
		return validateDatabaseSchemaV17(db, backend)
	case 18:
		return validateDatabaseSchemaV18(db, backend)
	case 19:
		return validateDatabaseSchemaV19(db, backend)
	default:
		return fmt.Errorf("database schema validation for v%d is not defined", version)
	}
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

func ensureDefaultGitHubAuthSource(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&AuthSource{}) || !db.Migrator().HasTable(&ExternalAccount{}) {
		return nil
	}

	var githubUserCount int64
	if db.Migrator().HasColumn(&User{}, "github_id") {
		if err := db.Model(&User{}).Where("github_id <> ''").Count(&githubUserCount).Error; err != nil {
			return fmt.Errorf("count legacy github users failed: %w", err)
		}
	}

	optionMap := map[string]string{}
	if db.Migrator().HasTable(&Option{}) {
		var options []Option
		if err := db.Find(&options).Error; err != nil {
			return fmt.Errorf("query options for github auth source migration failed: %w", err)
		}
		for _, option := range options {
			optionMap[option.Key] = option.Value
		}
	}

	clientID := strings.TrimSpace(optionMap["GitHubClientId"])
	clientSecret := strings.TrimSpace(optionMap["GitHubClientSecret"])
	enabled := optionMap["GitHubOAuthEnabled"] == "true" && clientID != "" && clientSecret != ""
	if githubUserCount == 0 && clientID == "" && clientSecret == "" {
		return nil
	}

	source := AuthSource{}
	err := db.Where("type = ? AND name = ?", AuthSourceTypeGitHub, "GitHub").First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		source = AuthSource{
			Name:         "GitHub",
			Type:         AuthSourceTypeGitHub,
			DisplayName:  "GitHub",
			IsActive:     enabled,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       "user:email",
		}
		if err := db.Create(&source).Error; err != nil {
			return fmt.Errorf("create default github auth source failed: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("query default github auth source failed: %w", err)
	} else {
		updates := map[string]any{}
		if source.ClientID == "" && clientID != "" {
			updates["client_id"] = clientID
		}
		if source.ClientSecret == "" && clientSecret != "" {
			updates["client_secret"] = clientSecret
		}
		if source.Scopes == "" {
			updates["scopes"] = "user:email"
		}
		if enabled && !source.IsActive {
			updates["is_active"] = true
		}
		if len(updates) > 0 {
			if err := db.Model(&source).Updates(updates).Error; err != nil {
				return fmt.Errorf("update default github auth source failed: %w", err)
			}
		}
	}

	if githubUserCount == 0 {
		return nil
	}

	var users []User
	if err := db.Select("id", "github_id", "username", "email").Where("github_id <> ''").Find(&users).Error; err != nil {
		return fmt.Errorf("query legacy github users failed: %w", err)
	}
	for _, user := range users {
		account := ExternalAccount{
			AuthSourceID:     source.ID,
			UserID:           user.Id,
			ExternalID:       user.GitHubId,
			ExternalUsername: user.GitHubId,
			Email:            user.Email,
		}
		if err := db.Where(ExternalAccount{
			AuthSourceID: source.ID,
			ExternalID:   user.GitHubId,
		}).FirstOrCreate(&account).Error; err != nil {
			return fmt.Errorf("migrate github external account for user %d failed: %w", user.Id, err)
		}
	}
	return nil
}

func validateDatabaseSchemaV9(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV8(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "pow_enabled") {
		return fmt.Errorf("column proxy_routes.pow_enabled is missing")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "pow_config") {
		return fmt.Errorf("column proxy_routes.pow_config is missing")
	}
	return nil
}

func validateDatabaseSchemaV10(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV9(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasTable(&AuthSource{}) {
		return fmt.Errorf("table auth_sources is missing")
	}
	if !db.Migrator().HasTable(&ExternalAccount{}) {
		return fmt.Errorf("table external_accounts is missing")
	}
	return nil
}

func validateDatabaseSchemaV11(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV10(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasTable(&AcmeAccount{}) {
		return fmt.Errorf("table acme_accounts is missing")
	}
	if !db.Migrator().HasTable(&DnsAccount{}) {
		return fmt.Errorf("table dns_accounts is missing")
	}
	if !db.Migrator().HasColumn(&TLSCertificate{}, "provider") {
		return fmt.Errorf("column tls_certificates.provider is missing")
	}
	return nil
}

func validateDatabaseSchemaV12(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV11(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "basic_auth_enabled") {
		return fmt.Errorf("column proxy_routes.basic_auth_enabled is missing")
	}
	return nil
}

func ensureDefaultWAFRuleGroup(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasTable(&WAFRuleGroup{}) {
		return nil
	}
	var count int64
	if err := db.Model(&WAFRuleGroup{}).Where("is_global = ?", true).Count(&count).Error; err != nil {
		return fmt.Errorf("count global waf rule groups failed: %w", err)
	}
	if count > 0 {
		return nil
	}
	group := WAFRuleGroup{
		Name:              "全局规则组",
		Enabled:           true,
		IsGlobal:          true,
		BlockStatusCode:   418,
		IPWhitelist:       "[]",
		IPBlacklist:       "[]",
		IPWhitelistGroups: "[]",
		IPBlacklistGroups: "[]",
		CountryWhitelist:  "[]",
		CountryBlacklist:  "[]",
		RegionWhitelist:   "[]",
		RegionBlacklist:   "[]",
		PoWEnabled:        false,
		PoWConfig:         "{}",
		BlockResponseBody: "",
	}
	if err := db.Create(&group).Error; err != nil {
		return fmt.Errorf("create default waf rule group failed: %w", err)
	}
	return nil
}

func validateDatabaseSchemaV13(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV12(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasTable(&WAFRuleGroup{}) {
		return fmt.Errorf("table waf_rule_groups is missing")
	}
	if !db.Migrator().HasTable(&WAFRuleGroupBinding{}) {
		return fmt.Errorf("table waf_rule_group_bindings is missing")
	}
	var count int64
	if err := db.Model(&WAFRuleGroup{}).Where("is_global = ?", true).Count(&count).Error; err != nil {
		return fmt.Errorf("count global waf rule groups failed: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("expected exactly one global waf rule group, got %d", count)
	}
	return nil
}

func validateDatabaseSchemaV14(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV13(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&WAFRuleGroup{}, "pow_enabled") {
		return fmt.Errorf("column waf_rule_groups.pow_enabled is missing")
	}
	if !db.Migrator().HasColumn(&WAFRuleGroup{}, "pow_config") {
		return fmt.Errorf("column waf_rule_groups.pow_config is missing")
	}
	return nil
}

func validateDatabaseSchemaV15(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV14(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&Node{}, "ip_manual_override") {
		return fmt.Errorf("column nodes.ip_manual_override is missing")
	}
	return nil
}

func validateDatabaseSchemaV16(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV15(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "tunnel_node_id") {
		if !db.Migrator().HasTable("tunnels") {
			return fmt.Errorf("table tunnels is missing")
		}
		if !db.Migrator().HasColumn(&ProxyRoute{}, "tunnel_id") {
			return fmt.Errorf("column proxy_routes.tunnel_id is missing")
		}
	}
	if !db.Migrator().HasColumn(&Node{}, "node_type") {
		return fmt.Errorf("column nodes.node_type is missing")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "upstream_type") {
		return fmt.Errorf("column proxy_routes.upstream_type is missing")
	}
	return nil
}

func validateDatabaseSchemaV17(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV16(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasTable(&WAFIPGroup{}) {
		return fmt.Errorf("table waf_ip_groups is missing")
	}
	if !db.Migrator().HasColumn(&WAFRuleGroup{}, "ip_whitelist_groups") {
		return fmt.Errorf("column waf_rule_groups.ip_whitelist_groups is missing")
	}
	if !db.Migrator().HasColumn(&WAFRuleGroup{}, "ip_blacklist_groups") {
		return fmt.Errorf("column waf_rule_groups.ip_blacklist_groups is missing")
	}
	return nil
}

func validateDatabaseSchemaV18(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV17(db, backend); err != nil {
		return err
	}
	if !db.Migrator().HasColumn(&WAFIPGroup{}, "ext_ips") {
		return fmt.Errorf("column waf_ip_groups.ext_ips is missing")
	}
	return nil
}

func validateDatabaseSchemaV19(db *gorm.DB, backend string) error {
	if err := validateDatabaseSchemaV18(db, backend); err != nil {
		return err
	}
	if db.Migrator().HasTable("tunnels") {
		return fmt.Errorf("table tunnels should be dropped in v19")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "tunnel_node_id") {
		return fmt.Errorf("column proxy_routes.tunnel_node_id is missing")
	}
	if db.Migrator().HasColumn(&ProxyRoute{}, "tunnel_id") {
		return fmt.Errorf("column proxy_routes.tunnel_id should be dropped in v19")
	}
	return nil
}

func databaseSchemaMigrations() []databaseSchemaMigration {
	ctx := databaseSchemaMigrationContext{}
	migrations := []databaseSchemaMigration{}
	for _, item := range schemamigrate.Migrations() {
		external := item
		migrations = append(migrations, databaseSchemaMigration{
			fromVersion: external.FromVersion,
			toVersion:   external.ToVersion,
			migrate: func(db *gorm.DB, backend string) error {
				return external.Migrate(ctx, db, backend)
			},
			validate: func(db *gorm.DB, backend string) error {
				return validateExternalDatabaseSchema(ctx, db, backend, external.ToVersion)
			},
		})
	}
	return migrations
}

func validateExternalDatabaseSchema(ctx databaseSchemaMigrationContext, db *gorm.DB, backend string, targetVersion int) error {
	if targetVersion <= schemamigrate.BaseDatabaseSchemaVersion {
		return ctx.ValidateDatabaseSchemaVersion(db, backend, targetVersion)
	}
	for _, migration := range schemamigrate.Migrations() {
		if migration.ToVersion > targetVersion {
			continue
		}
		if err := migration.Validate(ctx, db, backend); err != nil {
			return err
		}
	}
	return nil
}

func validateCurrentDatabaseSchema(db *gorm.DB, backend string) error {
	return validateExternalDatabaseSchema(databaseSchemaMigrationContext{}, db, backend, currentDatabaseSchemaVersion)
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
	if version < legacyDatabaseSchemaVersion {
		slog.Warn("database schema version is below supported baseline; treating it as historical initial schema", "version", version, "baseline", legacyDatabaseSchemaVersion)
		version = legacyDatabaseSchemaVersion
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
	if err := ensureDefaultGitHubAuthSource(db); err != nil {
		return err
	}
	if err := ensureDefaultWAFRuleGroup(db); err != nil {
		return err
	}
	if err := validateCurrentDatabaseSchema(db, backend); err != nil {
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
