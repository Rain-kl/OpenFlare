package model

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

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
