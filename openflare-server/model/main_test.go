package model

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type legacyProxyRouteV7 struct {
	ID                 uint   `gorm:"primaryKey"`
	SiteName           string `gorm:"size:255;not null;default:''"`
	Domain             string `gorm:"uniqueIndex;size:255;not null"`
	Domains            string `gorm:"type:text;not null;default:'[]'"`
	OriginID           *uint  `gorm:"index"`
	OriginURL          string `gorm:"size:2048;not null"`
	OriginHost         string `gorm:"size:255"`
	Upstreams          string `gorm:"type:text;not null;default:'[]'"`
	Enabled            bool   `gorm:"not null;default:true"`
	EnableHTTPS        bool   `gorm:"column:enable_https;not null;default:false"`
	CertID             *uint
	CertIDs            string `gorm:"type:text;not null;default:'[]'"`
	RedirectHTTP       bool   `gorm:"not null;default:false"`
	LimitConnPerServer int    `gorm:"not null;default:0"`
	LimitConnPerIP     int    `gorm:"not null;default:0"`
	LimitRate          string `gorm:"size:32;not null;default:''"`
	CacheEnabled       bool   `gorm:"not null;default:false"`
	CachePolicy        string `gorm:"size:32;not null;default:''"`
	CacheRules         string `gorm:"type:text;not null;default:'[]'"`
	CustomHeaders      string `gorm:"type:text;not null;default:'[]'"`
	Remark             string `gorm:"size:255"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (legacyProxyRouteV7) TableName() string {
	return "proxy_routes"
}

func openBareTestSQLiteDB(t *testing.T, name string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

func openTestSQLiteDB(t *testing.T, name string) *gorm.DB {
	t.Helper()

	db := openBareTestSQLiteDB(t, name)
	if err := autoMigrateAll(db); err != nil {
		t.Fatalf("auto migrate db: %v", err)
	}
	return db
}

func findDBModelByTableName(t *testing.T, tableName string) dbModel {
	t.Helper()

	models, err := buildDBModels()
	if err != nil {
		t.Fatalf("build db models: %v", err)
	}
	for _, item := range models {
		if item.tableName == tableName {
			return item
		}
	}
	t.Fatalf("db model not found for table %s", tableName)
	return dbModel{}
}

func expectedCurrentDatabaseVersion() int {
	return int(currentGooseTargetVersion())
}

func TestIsDatabaseEmpty(t *testing.T) {
	db := openTestSQLiteDB(t, "empty.db")

	empty, err := isDatabaseEmpty(db)
	if err != nil {
		t.Fatalf("isDatabaseEmpty returned error: %v", err)
	}
	if !empty {
		t.Fatal("expected database to be empty")
	}

	if err := db.Create(&User{
		Username:    "alice",
		Password:    "secret",
		DisplayName: "Alice",
		Role:        1,
		Status:      1,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	empty, err = isDatabaseEmpty(db)
	if err != nil {
		t.Fatalf("isDatabaseEmpty after seed returned error: %v", err)
	}
	if empty {
		t.Fatal("expected database to be non-empty")
	}
}

func TestMigrateTableDataCopiesRows(t *testing.T) {
	source := openTestSQLiteDB(t, "source.db")
	target := openTestSQLiteDB(t, "target.db")

	user := User{
		Id:          1,
		Username:    "root",
		Password:    "hashed",
		DisplayName: "Root User",
		Role:        100,
		Status:      1,
	}
	option := Option{
		Key:   "AgentHeartbeatInterval",
		Value: "10000",
	}

	if err := source.Create(&user).Error; err != nil {
		t.Fatalf("seed source user: %v", err)
	}
	if err := source.Create(&option).Error; err != nil {
		t.Fatalf("seed source option: %v", err)
	}

	if err := migrateTableData(source, target, findDBModelByTableName(t, "users")); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	if err := migrateTableData(source, target, findDBModelByTableName(t, "options")); err != nil {
		t.Fatalf("migrate options: %v", err)
	}

	var gotUser User
	if err := target.First(&gotUser, 1).Error; err != nil {
		t.Fatalf("query migrated user: %v", err)
	}
	if gotUser.Username != user.Username || gotUser.DisplayName != user.DisplayName {
		t.Fatalf("unexpected migrated user: %+v", gotUser)
	}

	var gotOption Option
	if err := target.First(&gotOption, "key = ?", option.Key).Error; err != nil {
		t.Fatalf("query migrated option: %v", err)
	}
	if gotOption.Value != option.Value {
		t.Fatalf("unexpected migrated option value: %s", gotOption.Value)
	}
}

func TestRegisterShardingAutoMigratesShardTables(t *testing.T) {
	db := openBareTestSQLiteDB(t, "sharded.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateAll(db); err != nil {
		t.Fatalf("auto migrate db: %v", err)
	}

	for _, table := range []string{
		"node_metric_snapshots_00",
		"node_metric_snapshots_09",
		"node_request_reports_00",
		"node_request_reports_09",
		"node_access_logs_00",
		"node_access_logs_09",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected sharded table %s to exist", table)
		}
	}
}

func TestUpgradeDatabaseSchemaV15ToV16AppliesCompressedReleaseSchema(t *testing.T) {
	db := openBareTestSQLiteDB(t, "v16.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateLegacySchemaMetadata(db); err != nil {
		t.Fatalf("auto migrate legacy schema metadata: %v", err)
	}
	if err := applyCurrentSchema(db, "sqlite"); err != nil {
		t.Fatalf("apply current schema: %v", err)
	}
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_enabled BOOLEAN NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("failed to add legacy pow_enabled: %v", err)
	}
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_config TEXT NOT NULL DEFAULT '{}'").Error; err != nil {
		t.Fatalf("failed to add legacy pow_config: %v", err)
	}
	if err := ensureDefaultWAFRuleGroup(db); err != nil {
		t.Fatalf("ensure default waf rule group: %v", err)
	}
	if err := saveDatabaseSchemaVersion(db, 15); err != nil {
		t.Fatalf("save schema version: %v", err)
	}
	if err := upgradeDatabaseSchema(db, "sqlite", 15); err != nil {
		t.Fatalf("upgrade schema: %v", err)
	}
	if !db.Migrator().HasTable(&WAFIPGroup{}) {
		t.Fatal("expected waf_ip_groups table")
	}
	if !db.Migrator().HasColumn(&WAFRuleGroup{}, "ip_whitelist_groups") {
		t.Fatal("expected waf_rule_groups.ip_whitelist_groups column")
	}
	if !db.Migrator().HasColumn(&Node{}, "access_token") {
		t.Fatal("expected nodes.access_token column")
	}
	if !db.Migrator().HasColumn(&Node{}, "version") {
		t.Fatal("expected nodes.version column")
	}
	if !db.Migrator().HasColumn(&Node{}, "ext_version") {
		t.Fatal("expected nodes.ext_version column")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "tunnel_node_id") {
		t.Fatal("expected proxy_routes.tunnel_node_id column")
	}
	if db.Migrator().HasTable("tunnels") {
		t.Fatal("expected pre-release tunnels table to be absent")
	}
	version, ok, err := loadDatabaseSchemaVersion(db)
	if err != nil {
		t.Fatalf("load schema version: %v", err)
	}
	if !ok || version != currentDatabaseSchemaVersion {
		t.Fatalf("unexpected schema version: got %d ok=%v want %d", version, ok, currentDatabaseSchemaVersion)
	}
}

func TestMigrateObservabilityLegacyColumnsBackfillsHealthEventMetadata(t *testing.T) {
	db := openTestSQLiteDB(t, "legacy-health-events.db")

	if err := db.Exec("ALTER TABLE node_health_events ADD COLUMN raw_json TEXT").Error; err != nil {
		t.Fatalf("add raw_json column: %v", err)
	}
	rawJSON, err := json.Marshal(map[string]any{
		"event_type": "sync_error",
		"metadata": map[string]string{
			"reason": "checksum_mismatch",
			"scope":  "routes",
		},
	})
	if err != nil {
		t.Fatalf("marshal raw json: %v", err)
	}
	event := &NodeHealthEvent{
		NodeID:           "node-legacy",
		EventType:        "sync_error",
		Severity:         "warning",
		Status:           "active",
		Message:          "checksum mismatch",
		FirstTriggeredAt: time.Now().Add(-time.Minute),
		LastTriggeredAt:  time.Now(),
		ReportedAt:       time.Now(),
	}
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("create health event: %v", err)
	}
	if err := db.Exec("UPDATE node_health_events SET raw_json = ? WHERE id = ?", string(rawJSON), event.ID).Error; err != nil {
		t.Fatalf("seed legacy raw_json: %v", err)
	}

	if err := migrateObservabilityLegacyColumns(db); err != nil {
		t.Fatalf("migrateObservabilityLegacyColumns: %v", err)
	}

	var got NodeHealthEvent
	if err := db.First(&got, event.ID).Error; err != nil {
		t.Fatalf("query health event: %v", err)
	}
	if got.MetadataJSON == "" {
		t.Fatal("expected metadata_json to be backfilled")
	}
}

func TestEnsureDatabaseSchemaUpToDateInitializesFreshDatabase(t *testing.T) {
	db := openBareTestSQLiteDB(t, "fresh-schema.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("ensureDatabaseSchemaUpToDate: %v", err)
	}

	version, exists, err := loadDatabaseSchemaVersion(db)
	if err != nil {
		t.Fatalf("loadDatabaseSchemaVersion: %v", err)
	}
	if !exists {
		t.Fatal("expected database schema version to be recorded")
	}
	if version != expectedCurrentDatabaseVersion() {
		t.Fatalf("unexpected schema version: got %d want %d", version, expectedCurrentDatabaseVersion())
	}
	if db.Migrator().HasTable(&DatabaseSchemaVersion{}) {
		t.Fatal("expected fresh database to avoid legacy database_schema_versions table")
	}
	if !db.Migrator().HasTable("goose_db_version") {
		t.Fatal("expected fresh database to initialize goose_db_version")
	}
	if !db.Migrator().HasColumn(&Node{}, "capabilities_json") {
		t.Fatal("expected fresh database to apply goose migration nodes.capabilities_json")
	}
}

func TestEnsureDatabaseSchemaUpToDateUpgradesLegacyDatabase(t *testing.T) {
	db := openBareTestSQLiteDB(t, "legacy-schema.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateAll(db); err != nil {
		t.Fatalf("auto migrate db: %v", err)
	}
	// Add legacy PoW columns manually to proxy_routes table to simulate legacy schema v9-v17 state
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_enabled BOOLEAN NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("failed to add legacy pow_enabled: %v", err)
	}
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_config TEXT NOT NULL DEFAULT '{}'").Error; err != nil {
		t.Fatalf("failed to add legacy pow_config: %v", err)
	}
	if err := db.Create(&User{
		Username:    "legacy",
		Password:    "secret",
		DisplayName: "Legacy User",
		Role:        1,
		Status:      1,
	}).Error; err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("ensureDatabaseSchemaUpToDate: %v", err)
	}

	version, exists, err := loadDatabaseSchemaVersion(db)
	if err != nil {
		t.Fatalf("loadDatabaseSchemaVersion: %v", err)
	}
	if !exists {
		t.Fatal("expected legacy database to gain a schema version record")
	}
	if version != expectedCurrentDatabaseVersion() {
		t.Fatalf("unexpected schema version: got %d want %d", version, expectedCurrentDatabaseVersion())
	}
	if db.Migrator().HasTable(&DatabaseSchemaVersion{}) {
		t.Fatal("expected legacy database_schema_versions table to be removed after bridging to goose")
	}
	if !db.Migrator().HasTable("goose_db_version") {
		t.Fatal("expected legacy upgrade to initialize goose_db_version")
	}
	if !db.Migrator().HasColumn(&Node{}, "capabilities_json") {
		t.Fatal("expected legacy upgrade to apply goose migration nodes.capabilities_json")
	}
}

func TestMigrateOriginsSchemaBackfillsOrigins(t *testing.T) {
	db := openBareTestSQLiteDB(t, "legacy-origins.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := applyCurrentSchema(db, "sqlite"); err != nil {
		t.Fatalf("applyCurrentSchema: %v", err)
	}
	now := time.Now().UTC()
	route := &ProxyRoute{
		Domain:    "app.example.com",
		OriginURL: "https://origin-a.internal:8443/api",
		Upstreams: `["https://origin-a.internal:8443/api"]`,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(route).Error; err != nil {
		t.Fatalf("seed proxy route: %v", err)
	}
	if err := db.Exec(`DELETE FROM origins`).Error; err != nil {
		t.Fatalf("clear origins: %v", err)
	}
	if err := db.Model(&ProxyRoute{}).Where("id = ?", route.ID).Update("origin_id", nil).Error; err != nil {
		t.Fatalf("clear route origin_id: %v", err)
	}

	if err := backfillOriginsFromProxyRoutes(db); err != nil {
		t.Fatalf("backfillOriginsFromProxyRoutes: %v", err)
	}

	if !db.Migrator().HasTable(&Origin{}) {
		t.Fatal("expected origins table to exist")
	}
	if !db.Migrator().HasColumn(&ProxyRoute{}, "origin_id") {
		t.Fatal("expected proxy_routes.origin_id column to exist")
	}

	reloadedRoute := &ProxyRoute{}
	if err := db.First(reloadedRoute, route.ID).Error; err != nil {
		t.Fatalf("query proxy route: %v", err)
	}
	if reloadedRoute.OriginID == nil || *reloadedRoute.OriginID == 0 {
		t.Fatal("expected migrated route to be linked to a backfilled origin")
	}

	origin := &Origin{}
	if err := db.First(origin, *reloadedRoute.OriginID).Error; err != nil {
		t.Fatalf("query origin: %v", err)
	}
	if origin.Address != "origin-a.internal" {
		t.Fatalf("unexpected backfilled origin address: %s", origin.Address)
	}
}

func TestEnsureDatabaseSchemaUpToDateAddsProxyRouteDomainCertificateFields(t *testing.T) {
	db := openBareTestSQLiteDB(t, "legacy-proxy-route-domain-cert-ids.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateLegacySchemaMetadata(db); err != nil {
		t.Fatalf("auto migrate legacy schema metadata: %v", err)
	}

	for _, item := range registeredModels() {
		if _, ok := item.(*ProxyRoute); ok {
			continue
		}
		if err := db.AutoMigrate(item); err != nil {
			t.Fatalf("auto migrate supporting table: %v", err)
		}
	}
	if err := db.AutoMigrate(&legacyProxyRouteV7{}); err != nil {
		t.Fatalf("auto migrate legacy proxy_routes v7: %v", err)
	}
	// Add legacy PoW columns manually to proxy_routes table to simulate legacy schema v9-v17 state
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_enabled BOOLEAN NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("failed to add legacy pow_enabled: %v", err)
	}
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_config TEXT NOT NULL DEFAULT '{}'").Error; err != nil {
		t.Fatalf("failed to add legacy pow_config: %v", err)
	}

	now := time.Now().UTC()
	certID := uint(9)
	if err := db.Create(&legacyProxyRouteV7{
		SiteName:           "secure-site",
		Domain:             "secure.example.com",
		Domains:            `["secure.example.com","www.secure.example.com"]`,
		OriginURL:          "https://origin-secure.internal:8443",
		Upstreams:          `["https://origin-secure.internal:8443"]`,
		Enabled:            true,
		EnableHTTPS:        true,
		CertID:             &certID,
		CertIDs:            `[9]`,
		RedirectHTTP:       true,
		LimitConnPerServer: 120,
		LimitConnPerIP:     12,
		LimitRate:          "512k",
		CacheEnabled:       false,
		CachePolicy:        "",
		CacheRules:         `[]`,
		CustomHeaders:      `[]`,
		CreatedAt:          now,
		UpdatedAt:          now,
	}).Error; err != nil {
		t.Fatalf("seed legacy proxy route v7: %v", err)
	}
	if err := saveDatabaseSchemaVersion(db, 7); err != nil {
		t.Fatalf("save schema version: %v", err)
	}

	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
	})

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("ensureDatabaseSchemaUpToDate: %v", err)
	}

	var route ProxyRoute
	if err := db.First(&route).Error; err != nil {
		t.Fatalf("query migrated proxy route: %v", err)
	}

	var domainCertIDs []uint
	if err := json.Unmarshal([]byte(route.DomainCertIDs), &domainCertIDs); err != nil {
		t.Fatalf("decode migrated domain_cert_ids: %v", err)
	}
	if len(domainCertIDs) != 2 || domainCertIDs[0] != certID || domainCertIDs[1] != certID {
		t.Fatalf("unexpected migrated domain_cert_ids: %#v", domainCertIDs)
	}
}

func TestRunDatabaseSchemaMigrationDoesNotAdvanceVersionWhenValidationFails(t *testing.T) {
	db := openBareTestSQLiteDB(t, "failed-validation.db")

	err := runDatabaseSchemaMigration(db, "sqlite", databaseSchemaMigration{
		fromVersion: legacyDatabaseSchemaVersion,
		toVersion:   11,
		migrate: func(tx *gorm.DB, backend string) error {
			return autoMigrateLegacySchemaMetadata(tx)
		},
		validate: func(tx *gorm.DB, backend string) error {
			return gorm.ErrInvalidDB
		},
	})
	if err == nil {
		t.Fatal("expected migration validation to fail")
	}

	_, exists, loadErr := loadDatabaseSchemaVersion(db)
	if loadErr != nil {
		t.Fatalf("loadDatabaseSchemaVersion: %v", loadErr)
	}
	if exists {
		t.Fatal("expected schema version to remain unset after failed validation")
	}
}

func TestEnsureDatabaseSchemaUpToDateAddsNodeIPManualOverride(t *testing.T) {
	db := openBareTestSQLiteDB(t, "node-ip-manual-override-migration.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := applyCurrentSchema(db, "sqlite"); err != nil {
		t.Fatalf("apply current schema: %v", err)
	}
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_enabled BOOLEAN NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("failed to add legacy pow_enabled: %v", err)
	}
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_config TEXT NOT NULL DEFAULT '{}'").Error; err != nil {
		t.Fatalf("failed to add legacy pow_config: %v", err)
	}
	if err := ensureDefaultWAFRuleGroup(db); err != nil {
		t.Fatalf("ensure default waf rule group: %v", err)
	}
	if err := db.Migrator().DropColumn(&Node{}, "ip_manual_override"); err != nil {
		t.Fatalf("drop ip_manual_override column: %v", err)
	}
	if db.Migrator().HasColumn(&Node{}, "ip_manual_override") {
		t.Fatal("expected test database to simulate schema v14 without ip_manual_override")
	}
	if err := saveDatabaseSchemaVersion(db, 14); err != nil {
		t.Fatalf("save schema version: %v", err)
	}

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("ensureDatabaseSchemaUpToDate: %v", err)
	}

	if !db.Migrator().HasColumn(&Node{}, "ip_manual_override") {
		t.Fatal("expected migration to add nodes.ip_manual_override")
	}
	version, exists, err := loadDatabaseSchemaVersion(db)
	if err != nil {
		t.Fatalf("loadDatabaseSchemaVersion: %v", err)
	}
	if !exists {
		t.Fatal("expected schema version record to exist")
	}
	if version != expectedCurrentDatabaseVersion() {
		t.Fatalf("unexpected schema version: got %d want %d", version, expectedCurrentDatabaseVersion())
	}
	if !db.Migrator().HasColumn(&Node{}, "capabilities_json") {
		t.Fatal("expected migration chain to include nodes.capabilities_json")
	}
}

func TestEnsureDatabaseSchemaUpToDateV16BackfillsNodeColumnsWhenNewColumnsAlreadyExist(t *testing.T) {
	db := openBareTestSQLiteDB(t, "node-v16-existing-target-columns.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := applyCurrentSchema(db, "sqlite"); err != nil {
		t.Fatalf("apply current schema: %v", err)
	}
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_enabled BOOLEAN NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("failed to add legacy pow_enabled: %v", err)
	}
	if err := db.Exec("ALTER TABLE proxy_routes ADD COLUMN pow_config TEXT NOT NULL DEFAULT '{}'").Error; err != nil {
		t.Fatalf("failed to add legacy pow_config: %v", err)
	}
	if err := ensureDefaultWAFRuleGroup(db); err != nil {
		t.Fatalf("ensure default waf rule group: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE nodes ADD COLUMN agent_token text`,
		`ALTER TABLE nodes ADD COLUMN agent_version text`,
		`ALTER TABLE nodes ADD COLUMN nginx_version text`,
		`ALTER TABLE nodes ADD COLUMN relay_version text`,
		`ALTER TABLE nodes ADD COLUMN relay_frp_version text`,
		`ALTER TABLE nodes ADD COLUMN relay_frps_connections integer`,
		`ALTER TABLE nodes ADD COLUMN relay_frps_proxy_count integer`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("prepare legacy node column with %q: %v", stmt, err)
		}
	}
	now := time.Now()
	if err := db.Exec(`
		INSERT INTO nodes (
			node_id, name, ip, access_token, version, ext_version,
			agent_token, agent_version, nginx_version,
			status, last_seen_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "node-v16", "Node v16", "127.0.0.1", "", "", "", "legacy-token", "v2.0.0", "openresty/1.25.3", "offline", now, now, now).Error; err != nil {
		t.Fatalf("seed node with legacy columns: %v", err)
	}
	if err := saveDatabaseSchemaVersion(db, 15); err != nil {
		t.Fatalf("save schema version: %v", err)
	}

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("ensureDatabaseSchemaUpToDate: %v", err)
	}

	var node Node
	if err := db.Where("node_id = ?", "node-v16").First(&node).Error; err != nil {
		t.Fatalf("query migrated node: %v", err)
	}
	if node.AccessToken != "legacy-token" {
		t.Fatalf("unexpected access_token: got %q", node.AccessToken)
	}
	if node.Version != "v2.0.0" {
		t.Fatalf("unexpected version: got %q", node.Version)
	}
	if node.ExtVersion != "openresty/1.25.3" {
		t.Fatalf("unexpected ext_version: got %q", node.ExtVersion)
	}
	for _, column := range []string{
		"agent_token",
		"agent_version",
		"nginx_version",
		"relay_version",
		"relay_frp_version",
		"relay_frps_connections",
		"relay_frps_proxy_count",
	} {
		exists, err := databaseColumnExists(db, "nodes", column)
		if err != nil {
			t.Fatalf("inspect legacy nodes.%s: %v", column, err)
		}
		if exists {
			t.Fatalf("expected migration to drop legacy nodes.%s column", column)
		}
	}
	version, exists, err := loadDatabaseSchemaVersion(db)
	if err != nil {
		t.Fatalf("loadDatabaseSchemaVersion: %v", err)
	}
	if !exists {
		t.Fatal("expected schema version record to exist")
	}
	if version != expectedCurrentDatabaseVersion() {
		t.Fatalf("unexpected schema version: got %d want %d", version, expectedCurrentDatabaseVersion())
	}
	if !db.Migrator().HasColumn(&Node{}, "capabilities_json") {
		t.Fatal("expected v16 upgrade path to apply goose migration nodes.capabilities_json")
	}
}

func TestEnsureDatabaseSchemaUpToDateV16DropsLegacyNodeColumnsWhenAlreadyCurrent(t *testing.T) {
	db := openBareTestSQLiteDB(t, "node-v16-current-legacy-columns.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := applyCurrentSchema(db, "sqlite"); err != nil {
		t.Fatalf("apply current schema: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE nodes ADD COLUMN agent_token text`,
		`ALTER TABLE nodes ADD COLUMN agent_version text`,
		`ALTER TABLE nodes ADD COLUMN nginx_version text`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("prepare legacy node column with %q: %v", stmt, err)
		}
	}
	if err := saveDatabaseSchemaVersion(db, currentDatabaseSchemaVersion); err != nil {
		t.Fatalf("save schema version: %v", err)
	}

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("ensureDatabaseSchemaUpToDate: %v", err)
	}

	for _, column := range []string{"agent_token", "agent_version", "nginx_version"} {
		exists, err := databaseColumnExists(db, "nodes", column)
		if err != nil {
			t.Fatalf("inspect legacy nodes.%s: %v", column, err)
		}
		if exists {
			t.Fatalf("expected current-schema cleanup to drop legacy nodes.%s column", column)
		}
	}
	if db.Migrator().HasTable(&DatabaseSchemaVersion{}) {
		t.Fatal("expected current-schema legacy version table to be removed after goose bridge")
	}
	if !db.Migrator().HasTable("goose_db_version") {
		t.Fatal("expected current-schema goose_db_version table to exist")
	}
	if !db.Migrator().HasColumn(&Node{}, "capabilities_json") {
		t.Fatal("expected current-schema repair to preserve goose column nodes.capabilities_json")
	}
}

func TestEnsureDatabaseSchemaUpToDateKeepsGooseOnlyDatabaseOnReentry(t *testing.T) {
	db := openBareTestSQLiteDB(t, "goose-only-reentry.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("first ensureDatabaseSchemaUpToDate: %v", err)
	}
	if db.Migrator().HasTable(&DatabaseSchemaVersion{}) {
		t.Fatal("expected first initialization to avoid legacy table")
	}

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("second ensureDatabaseSchemaUpToDate: %v", err)
	}

	if db.Migrator().HasTable(&DatabaseSchemaVersion{}) {
		t.Fatal("expected goose-only database to remain free of legacy version table")
	}
	version, exists, err := loadGooseDatabaseVersion(db)
	if err != nil {
		t.Fatalf("loadGooseDatabaseVersion: %v", err)
	}
	if !exists {
		t.Fatal("expected goose-only database to keep goose version record")
	}
	if version != expectedCurrentDatabaseVersion() {
		t.Fatalf("unexpected goose version: got %d want %d", version, expectedCurrentDatabaseVersion())
	}
	if !db.Migrator().HasColumn(&Node{}, "capabilities_json") {
		t.Fatal("expected goose-only database to keep nodes.capabilities_json")
	}
}

func TestAllRegisteredMigrationsHaveValidationDefined(t *testing.T) {
	ctx := databaseSchemaMigrationContext{}
	for _, migration := range databaseSchemaMigrations() {
		err := ctx.ValidateDatabaseSchemaVersion(nil, "sqlite", migration.toVersion)
		if err != nil && strings.Contains(err.Error(), "is not defined") {
			t.Fatalf("Validation is not defined in migrations.go for registered migration version v%d: %v", migration.toVersion, err)
		}
	}
}

func TestAllGORMModelsAreRegistered(t *testing.T) {
	// 1. Gather all registered model names
	registeredNames := make(map[string]bool)
	for _, item := range registeredModels() {
		name := reflect.TypeOf(item).Elem().Name()
		registeredNames[name] = true
	}
	for _, item := range schemaMetadataModels() {
		name := reflect.TypeOf(item).Elem().Name()
		registeredNames[name] = true
	}

	// 2. Parse all .go files in model/ package
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(info os.FileInfo) bool {
		// Only parse .go files, exclude _test.go files and subdirectories
		return !info.IsDir() && strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("failed to parse directory: %v", err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					structType, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					// Verify if this struct has any field with a `gorm:"..."` tag
					isGORMModel := false
					for _, field := range structType.Fields.List {
						if field.Tag != nil && strings.Contains(field.Tag.Value, "gorm:") {
							isGORMModel = true
							break
						}
					}

					if isGORMModel {
						structName := typeSpec.Name.Name
						if !registeredNames[structName] {
							t.Errorf("Model struct %q is defined with GORM tags but is NOT registered in registeredModels() or schemaMetadataModels() in model/main.go!", structName)
						}
					}
				}
			}
		}
	}
}

func TestEnsureDatabaseSchemaUpToDateDropsPagesDeploymentUnusedFields(t *testing.T) {
	db := openBareTestSQLiteDB(t, "drop-pages-deployment-unused-fields.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}

	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("first ensureDatabaseSchemaUpToDate: %v", err)
	}

	// Verify columns do not exist
	if db.Migrator().HasColumn("pages_deployments", "root_dir") {
		t.Fatal("expected root_dir column to be absent initially")
	}
	if db.Migrator().HasColumn("pages_deployments", "entry_file") {
		t.Fatal("expected entry_file column to be absent initially")
	}

	// Manually add columns to simulate old state
	if err := db.Exec("ALTER TABLE pages_deployments ADD COLUMN root_dir TEXT").Error; err != nil {
		t.Fatalf("failed to add root_dir column: %v", err)
	}
	if err := db.Exec("ALTER TABLE pages_deployments ADD COLUMN entry_file TEXT").Error; err != nil {
		t.Fatalf("failed to add entry_file column: %v", err)
	}

	// Verify columns were added
	if !db.Migrator().HasColumn("pages_deployments", "root_dir") {
		t.Fatal("expected root_dir column to be present after manual add")
	}
	if !db.Migrator().HasColumn("pages_deployments", "entry_file") {
		t.Fatal("expected entry_file column to be present after manual add")
	}

	// Remove the migration record from goose_db_version table
	const versionToRerun = 202606040004
	if err := db.Exec("DELETE FROM goose_db_version WHERE version_id = ?", versionToRerun).Error; err != nil {
		t.Fatalf("failed to delete migration record: %v", err)
	}

	// Run migration again
	if err := ensureDatabaseSchemaUpToDate(db, "sqlite"); err != nil {
		t.Fatalf("second ensureDatabaseSchemaUpToDate: %v", err)
	}

	// Verify columns were dropped successfully
	if db.Migrator().HasColumn("pages_deployments", "root_dir") {
		t.Fatal("expected root_dir column to be dropped after migration rerun")
	}
	if db.Migrator().HasColumn("pages_deployments", "entry_file") {
		t.Fatal("expected entry_file column to be dropped after migration rerun")
	}
}
