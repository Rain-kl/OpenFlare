package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"log/slog"
	"openflare/common"
	"openflare/utils/security"
	"os"
	"reflect"
	"sync"
)

var DB *gorm.DB

type dbModel struct {
	value     any
	tableName string
	hasIDPK   bool
}

type databaseSchemaMigration struct {
	fromVersion int
	toVersion   int
	migrate     func(db *gorm.DB, backend string) error
	validate    func(db *gorm.DB, backend string) error
}

func registeredModels() []any {
	return []any{
		&File{},
		&User{},
		&Option{},
		&ProxyRoute{},
		&ConfigVersion{},
		&Node{},
		&NodeSystemProfile{},
		&ApplyLog{},
		&NodeMetricSnapshot{},
		&NodeRequestReport{},
		&NodeAccessLog{},
		&NodeHealthEvent{},
		&TLSCertificate{},
		&ManagedDomain{},
	}
}

func schemaMetadataModels() []any {
	return []any{
		&DatabaseSchemaVersion{},
	}
}

func buildDBModels() ([]dbModel, error) {
	models := registeredModels()
	result := make([]dbModel, 0, len(models))
	namer := schema.NamingStrategy{}
	cache := &sync.Map{}
	for _, item := range models {
		parsed, err := schema.Parse(item, cache, namer)
		if err != nil {
			return nil, err
		}
		hasIDPK := len(parsed.PrimaryFields) == 1 && parsed.PrimaryFields[0].DBName == "id"
		result = append(result, dbModel{
			value:     item,
			tableName: parsed.Table,
			hasIDPK:   hasIDPK,
		})
	}
	return result, nil
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

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		slog.Info("no user exists, create a root user", "username", "root")
		hashedPassword, err := security.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CountTable(tableName string) (num int64) {
	DB.Table(tableName).Count(&num)
	return
}

func openDatabase() (*gorm.DB, string, error) {
	if common.SQLDSN != "" {
		db, err := gorm.Open(postgres.Open(common.SQLDSN), &gorm.Config{})
		if err != nil {
			return nil, "", err
		}
		return db, "postgres", nil
	}
	db, err := gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{})
	if err != nil {
		return nil, "", err
	}
	slog.Info("database DSN not set, using SQLite as database", "sqlite_path", common.SQLitePath)
	return db, "sqlite", nil
}

func autoMigrateAll(db *gorm.DB) error {
	for _, item := range registeredModels() {
		if err := db.AutoMigrate(item); err != nil {
			return err
		}
	}
	return nil
}

func autoMigrateSchemaMetadata(db *gorm.DB) error {
	for _, item := range schemaMetadataModels() {
		if err := db.AutoMigrate(item); err != nil {
			return err
		}
	}
	return nil
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

func migrateObservabilityShardsToID(db *gorm.DB, backend string) error {
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

func databaseSchemaMigrations() []databaseSchemaMigration {
	return []databaseSchemaMigration{
		{
			fromVersion: 1,
			toVersion:   2,
			migrate:     applyCurrentSchema,
			validate:    validateDatabaseSchemaV2,
		},
		{
			fromVersion: 2,
			toVersion:   3,
			migrate:     migrateObservabilityShardsToID,
			validate:    validateDatabaseSchemaV3,
		},
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
	if err := validateDatabaseSchemaV3(db, backend); err != nil {
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

func isDatabaseEmpty(db *gorm.DB) (bool, error) {
	models, err := buildDBModels()
	if err != nil {
		return false, err
	}
	for _, item := range models {
		if isShardedObservabilityTable(item.tableName) {
			for _, table := range observabilityShardTables(item.tableName) {
				if !db.Migrator().HasTable(table) {
					continue
				}
				var count int64
				if err := db.Table(table).Limit(1).Count(&count).Error; err != nil {
					return false, err
				}
				if count > 0 {
					return false, nil
				}
			}
			continue
		}
		if !db.Migrator().HasTable(item.value) {
			continue
		}
		var count int64
		if err := db.Model(item.value).Limit(1).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return false, nil
		}
	}
	return true, nil
}

func sqliteSourceExists() bool {
	info, err := os.Stat(common.SQLitePath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func migrateSQLiteDataIfNeeded(target *gorm.DB, backend string) error {
	if backend != "postgres" {
		return nil
	}
	empty, err := isDatabaseEmpty(target)
	if err != nil {
		return err
	}
	if !empty {
		slog.Info("skip sqlite migration because target database already has data", "backend", backend)
		return nil
	}
	if !sqliteSourceExists() {
		slog.Info("skip sqlite migration because sqlite source file was not found", "sqlite_path", common.SQLitePath)
		return nil
	}

	source, err := gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
		PrepareStmt: true,
	})
	if err != nil {
		return fmt.Errorf("open sqlite source database failed: %w", err)
	}
	sourceSQLDB, err := source.DB()
	if err != nil {
		return fmt.Errorf("get sqlite source database handle failed: %w", err)
	}
	defer func() {
		_ = sourceSQLDB.Close()
	}()

	models, err := buildDBModels()
	if err != nil {
		return err
	}

	slog.Info("starting sqlite to postgres database migration", "sqlite_path", common.SQLitePath)
	err = target.Transaction(func(tx *gorm.DB) error {
		for _, item := range models {
			if err := migrateTableData(source, tx, item); err != nil {
				return err
			}
			if item.hasIDPK {
				if err := resetPostgresSequence(tx, item.tableName); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	slog.Info("sqlite to postgres database migration completed", "sqlite_path", common.SQLitePath)
	return nil
}

func migrateTableData(source *gorm.DB, target *gorm.DB, item dbModel) error {
	if !source.Migrator().HasTable(item.value) {
		slog.Info("database migration progress", "table", item.tableName, "migrated", 0, "total", 0, "status", "skipped_missing_source_table")
		return nil
	}
	var total int64
	if err := source.Model(item.value).Count(&total).Error; err != nil {
		return fmt.Errorf("count sqlite table %s failed: %w", item.tableName, err)
	}
	slog.Info("database migration progress", "table", item.tableName, "migrated", 0, "total", total, "status", "starting")
	if total == 0 {
		slog.Info("database migration progress", "table", item.tableName, "migrated", 0, "total", total, "status", "completed")
		return nil
	}

	modelType := reflect.TypeOf(item.value).Elem()
	sliceType := reflect.SliceOf(modelType)
	migrated := int64(0)
	offset := 0
	const batchSize = 200

	for {
		batchPtr := reflect.New(sliceType)
		query := source.Model(item.value).Limit(batchSize).Offset(offset)
		if item.hasIDPK {
			query = query.Order("id ASC")
		}
		if err := query.Find(batchPtr.Interface()).Error; err != nil {
			return fmt.Errorf("read sqlite table %s failed: %w", item.tableName, err)
		}
		batchLen := batchPtr.Elem().Len()
		if batchLen == 0 {
			break
		}
		if isShardedObservabilityTable(item.tableName) {
			for index := 0; index < batchLen; index++ {
				record := batchPtr.Elem().Index(index)
				if err := target.Create(record.Addr().Interface()).Error; err != nil {
					return fmt.Errorf("write target sharded table %s failed: %w", item.tableName, err)
				}
			}
		} else {
			if err := target.Create(batchPtr.Interface()).Error; err != nil {
				return fmt.Errorf("write target table %s failed: %w", item.tableName, err)
			}
		}
		migrated += int64(batchLen)
		offset += batchLen
		slog.Info("database migration progress", "table", item.tableName, "migrated", migrated, "total", total, "status", "running")
	}

	slog.Info("database migration progress", "table", item.tableName, "migrated", migrated, "total", total, "status", "completed")
	return nil
}

func resetPostgresSequence(db *gorm.DB, tableName string) error {
	sql := fmt.Sprintf(
		"SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE(MAX(id), 1), MAX(id) IS NOT NULL) FROM \"%s\"",
		tableName,
		tableName,
	)
	return db.Exec(sql).Error
}

func InitDB() (err error) {
	db, backend, err := openDatabase()
	if err != nil {
		slog.Error("open database failed", "error", err)
		os.Exit(1)
	}
	DB = db
	if err = registerSharding(db, backend); err != nil {
		return err
	}
	if err = ensureDatabaseSchemaUpToDate(db, backend); err != nil {
		return err
	}
	return createRootAccountIfNeed()
}

func CloseDB() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}
