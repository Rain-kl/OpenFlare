package model

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type legacyProxyRouteV4 struct {
	ID            uint   `gorm:"primaryKey"`
	Domain        string `gorm:"uniqueIndex;size:255;not null"`
	OriginID      *uint  `gorm:"index"`
	OriginURL     string `gorm:"size:2048;not null"`
	OriginHost    string `gorm:"size:255"`
	Upstreams     string `gorm:"type:text;not null;default:'[]'"`
	Enabled       bool   `gorm:"not null;default:true"`
	EnableHTTPS   bool   `gorm:"column:enable_https;not null;default:false"`
	CertID        *uint
	RedirectHTTP  bool   `gorm:"not null;default:false"`
	CacheEnabled  bool   `gorm:"not null;default:false"`
	CachePolicy   string `gorm:"size:32;not null;default:''"`
	CacheRules    string `gorm:"type:text;not null;default:'[]'"`
	CustomHeaders string `gorm:"type:text;not null;default:'[]'"`
	Remark        string `gorm:"size:255"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (legacyProxyRouteV4) TableName() string {
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
	if version != currentDatabaseSchemaVersion {
		t.Fatalf("unexpected schema version: got %d want %d", version, currentDatabaseSchemaVersion)
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
	if version != currentDatabaseSchemaVersion {
		t.Fatalf("unexpected schema version: got %d want %d", version, currentDatabaseSchemaVersion)
	}
}

func TestEnsureDatabaseSchemaUpToDateMigratesObservabilityShardsToID(t *testing.T) {
	db := openBareTestSQLiteDB(t, "legacy-observability-shards.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateAll(db); err != nil {
		t.Fatalf("auto migrate db: %v", err)
	}
	if err := autoMigrateSchemaMetadata(db); err != nil {
		t.Fatalf("auto migrate schema metadata: %v", err)
	}

	now := time.Now().UTC()
	if err := db.Table("node_metric_snapshots_00").Create(&NodeMetricSnapshot{
		ID:               1,
		NodeID:           "node-a",
		CapturedAt:       now.Add(-2 * time.Minute),
		CPUUsagePercent:  22,
		MemoryUsedBytes:  2,
		MemoryTotalBytes: 8,
	}).Error; err != nil {
		t.Fatalf("seed metric snapshot shard 00: %v", err)
	}
	if err := db.Table("node_metric_snapshots_01").Create(&NodeMetricSnapshot{
		ID:               1,
		NodeID:           "node-b",
		CapturedAt:       now.Add(-time.Minute),
		CPUUsagePercent:  44,
		MemoryUsedBytes:  4,
		MemoryTotalBytes: 8,
	}).Error; err != nil {
		t.Fatalf("seed metric snapshot shard 01: %v", err)
	}
	if err := db.Table("node_request_reports_00").Create(&NodeRequestReport{
		ID:                 1,
		NodeID:             "node-a",
		WindowStartedAt:    now.Add(-3 * time.Minute),
		WindowEndedAt:      now.Add(-2 * time.Minute),
		RequestCount:       12,
		ErrorCount:         1,
		UniqueVisitorCount: 6,
	}).Error; err != nil {
		t.Fatalf("seed request report shard 00: %v", err)
	}
	if err := db.Table("node_request_reports_01").Create(&NodeRequestReport{
		ID:                 1,
		NodeID:             "node-b",
		WindowStartedAt:    now.Add(-2 * time.Minute),
		WindowEndedAt:      now.Add(-time.Minute),
		RequestCount:       21,
		ErrorCount:         2,
		UniqueVisitorCount: 9,
	}).Error; err != nil {
		t.Fatalf("seed request report shard 01: %v", err)
	}
	if err := db.Table("node_access_logs_00").Create(&NodeAccessLog{
		ID:         1,
		NodeID:     "node-a",
		LoggedAt:   now.Add(-90 * time.Second),
		RemoteAddr: "203.0.113.10",
		Host:       "a.example.com",
		Path:       "/alpha",
		StatusCode: 200,
	}).Error; err != nil {
		t.Fatalf("seed access log shard 00: %v", err)
	}
	if err := db.Table("node_access_logs_01").Create(&NodeAccessLog{
		ID:         1,
		NodeID:     "node-b",
		LoggedAt:   now.Add(-60 * time.Second),
		RemoteAddr: "203.0.113.11",
		Host:       "b.example.com",
		Path:       "/beta",
		StatusCode: 502,
	}).Error; err != nil {
		t.Fatalf("seed access log shard 01: %v", err)
	}
	if err := saveDatabaseSchemaVersion(db, 2); err != nil {
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

	version, exists, err := loadDatabaseSchemaVersion(db)
	if err != nil {
		t.Fatalf("loadDatabaseSchemaVersion: %v", err)
	}
	if !exists {
		t.Fatal("expected migrated database to keep schema version record")
	}
	if version != currentDatabaseSchemaVersion {
		t.Fatalf("unexpected schema version: got %d want %d", version, currentDatabaseSchemaVersion)
	}

	for _, baseTable := range shardedObservabilityBaseTables() {
		for _, table := range observabilityShardTables(baseTable) {
			legacyTable := legacyObservabilityShardTableName(table)
			if db.Migrator().HasTable(legacyTable) {
				t.Fatalf("expected legacy shard table %s to be removed", legacyTable)
			}
		}
	}

	snapshots, err := ListMetricSnapshotsSince(time.Time{})
	if err != nil {
		t.Fatalf("ListMetricSnapshotsSince failed: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 migrated metric snapshots, got %+v", snapshots)
	}
	reports, err := ListRequestReportsSince(time.Time{})
	if err != nil {
		t.Fatalf("ListRequestReportsSince failed: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 migrated request reports, got %+v", reports)
	}
	logs, err := ListNodeAccessLogs(NodeAccessLogQuery{Page: 0, PageSize: 10})
	if err != nil {
		t.Fatalf("ListNodeAccessLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 migrated access logs, got %+v", logs)
	}

	seenSnapshotIDs := make(map[uint]struct{}, len(snapshots))
	for _, item := range snapshots {
		if item == nil || item.ID == 0 {
			t.Fatalf("expected migrated metric snapshot to have a new non-zero id: %+v", item)
		}
		if _, exists := seenSnapshotIDs[item.ID]; exists {
			t.Fatalf("expected migrated metric snapshot ids to be unique, got duplicate %d", item.ID)
		}
		seenSnapshotIDs[item.ID] = struct{}{}
		targetTable := observabilityShardTableForID("node_metric_snapshots", item.ID)
		var count int64
		if err := db.Table(targetTable).Where("id = ?", item.ID).Count(&count).Error; err != nil {
			t.Fatalf("count migrated metric snapshot in target shard: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected migrated metric snapshot id %d to be stored in %s", item.ID, targetTable)
		}
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

func TestEnsureDatabaseSchemaUpToDateBackfillsProxyRouteSiteFields(t *testing.T) {
	db := openBareTestSQLiteDB(t, "legacy-proxy-route-sites.db")
	if err := registerSharding(db, "sqlite"); err != nil {
		t.Fatalf("register sharding: %v", err)
	}
	if err := autoMigrateSchemaMetadata(db); err != nil {
		t.Fatalf("auto migrate schema metadata: %v", err)
	}

	for _, item := range registeredModels() {
		if _, ok := item.(*ProxyRoute); ok {
			continue
		}
		if err := db.AutoMigrate(item); err != nil {
			t.Fatalf("auto migrate supporting table: %v", err)
		}
	}
	if err := db.AutoMigrate(&legacyProxyRouteV4{}); err != nil {
		t.Fatalf("auto migrate legacy proxy_routes: %v", err)
	}

	now := time.Now().UTC()
	if err := db.Create(&legacyProxyRouteV4{
		Domain:        "app.example.com",
		OriginURL:     "https://origin-a.internal:8443",
		Upstreams:     `["https://origin-a.internal:8443","https://origin-b.internal:8443"]`,
		Enabled:       true,
		EnableHTTPS:   false,
		RedirectHTTP:  false,
		CacheEnabled:  false,
		CachePolicy:   "",
		CacheRules:    `[]`,
		CustomHeaders: `[]`,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("seed legacy proxy route: %v", err)
	}
	if err := saveDatabaseSchemaVersion(db, 4); err != nil {
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
	if route.SiteName != "app.example.com" {
		t.Fatalf("unexpected site_name after migration: %s", route.SiteName)
	}
	if route.Domain != "app.example.com" {
		t.Fatalf("unexpected domain mirror after migration: %s", route.Domain)
	}

	var domains []string
	if err := json.Unmarshal([]byte(route.Domains), &domains); err != nil {
		t.Fatalf("decode migrated domains: %v", err)
	}
	if len(domains) != 1 || domains[0] != "app.example.com" {
		t.Fatalf("unexpected migrated domains: %#v", domains)
	}
}

func TestRunDatabaseSchemaMigrationDoesNotAdvanceVersionWhenValidationFails(t *testing.T) {
	db := openBareTestSQLiteDB(t, "failed-validation.db")

	err := runDatabaseSchemaMigration(db, "sqlite", databaseSchemaMigration{
		fromVersion: legacyDatabaseSchemaVersion,
		toVersion:   currentDatabaseSchemaVersion,
		migrate: func(tx *gorm.DB, backend string) error {
			return autoMigrateSchemaMetadata(tx)
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
