package goose

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"gorm.io/gorm"
)

type BridgeContext interface {
	Context
	AutoMigrateLegacySchemaMetadata(db *gorm.DB) error
	InitializeFreshDatabaseSchema(db *gorm.DB, backend string) error
	IsDatabaseEmpty(db *gorm.DB) (bool, error)
	RepairCurrentSchemaState(db *gorm.DB, backend string) error
	SaveLegacyDatabaseSchemaVersion(db *gorm.DB, version int) error
	UpgradeLegacyDatabaseSchema(db *gorm.DB, backend string, version int) error
	ValidateCurrentDatabaseSchema(db *gorm.DB, backend string) error
}

type schemaMigrationState int

const (
	schemaMigrationStateFresh schemaMigrationState = iota
	schemaMigrationStateLegacyOnly
	schemaMigrationStateGooseOnly
	schemaMigrationStateLegacyBootstrap
	schemaMigrationStateMixed
)

func detectSchemaState(db *gorm.DB, ctx BridgeContext) (schemaMigrationState, error) {
	hasLegacyTable := db.Migrator().HasTable("database_schema_versions")
	hasGooseTable := db.Migrator().HasTable("goose_db_version")

	switch {
	case hasLegacyTable && hasGooseTable:
		return schemaMigrationStateMixed, nil
	case hasLegacyTable:
		return schemaMigrationStateLegacyOnly, nil
	case hasGooseTable:
		return schemaMigrationStateGooseOnly, nil
	}

	empty, err := ctx.IsDatabaseEmpty(db)
	if err != nil {
		return 0, err
	}
	if empty {
		return schemaMigrationStateFresh, nil
	}
	return schemaMigrationStateLegacyBootstrap, nil
}

func LoadDatabaseVersion(db *gorm.DB) (int, bool, error) {
	if db == nil || !db.Migrator().HasTable("goose_db_version") {
		return 0, false, nil
	}

	var version int64
	err := db.Table("goose_db_version").
		Where("is_applied = ?", true).
		Order("version_id DESC").
		Select("version_id").
		Limit(1).
		Row().
		Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return int(version), true, nil
}

func loadLegacyDatabaseSchemaVersion(db *gorm.DB) (int, bool, error) {
	if db == nil || !db.Migrator().HasTable("database_schema_versions") {
		return 0, false, nil
	}

	var version int
	err := db.Table("database_schema_versions").
		Where("id = ?", 1).
		Select("version").
		Limit(1).
		Row().
		Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return version, true, nil
}

func bootstrapLegacySchemaVersion(db *gorm.DB, ctx BridgeContext) error {
	if err := ctx.AutoMigrateLegacySchemaMetadata(db); err != nil {
		return err
	}
	version, exists, err := loadLegacyDatabaseSchemaVersion(db)
	if err != nil {
		return err
	}
	if exists {
		if int64(version) > LegacyBridgeVersion {
			return fmt.Errorf("legacy schema version %d is newer than supported terminal version %d", version, LegacyBridgeVersion)
		}
		return nil
	}
	return ctx.SaveLegacyDatabaseSchemaVersion(db, 7)
}

func upgradeLegacyToTerminal(db *gorm.DB, backend string, ctx BridgeContext) error {
	if err := bootstrapLegacySchemaVersion(db, ctx); err != nil {
		return err
	}
	version, exists, err := loadLegacyDatabaseSchemaVersion(db)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("legacy schema version record is missing after bootstrap")
	}
	return ctx.UpgradeLegacyDatabaseSchema(db, backend, version)
}

func validateGooseBridgeState(db *gorm.DB) error {
	version, exists, err := LoadDatabaseVersion(db)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if int64(version) < LegacyBridgeVersion {
		return fmt.Errorf("goose schema version %d is below legacy bridge baseline %d", version, LegacyBridgeVersion)
	}
	if int64(version) > CurrentTargetVersion() {
		return fmt.Errorf("goose schema version %d is newer than application target version %d", version, CurrentTargetVersion())
	}
	return nil
}

func finalizeLegacyToGooseBridge(db *gorm.DB) error {
	gooseVersion, exists, err := LoadDatabaseVersion(db)
	if err != nil {
		return err
	}
	if !exists || int64(gooseVersion) < LegacyBridgeVersion {
		return nil
	}
	if !db.Migrator().HasTable("database_schema_versions") {
		return nil
	}
	if err := db.Exec("DROP TABLE IF EXISTS database_schema_versions").Error; err != nil {
		return fmt.Errorf("drop legacy schema versions table failed: %w", err)
	}
	slog.Info("completed legacy-to-goose migration bridge", "goose_version", gooseVersion)
	return nil
}

func ValidateRegisteredSchema(db *gorm.DB) error {
	if err := validateNodeCapabilitiesJSON(db); err != nil {
		return err
	}
	return nil
}

func EnsureDatabaseSchemaUpToDate(db *gorm.DB, backend string, ctx BridgeContext) (returnedErr error) {
	if backend == "sqlite" {
		backupPath, restore, err := backupSQLiteDatabase(db)
		if err != nil {
			slog.Warn("failed to backup sqlite database before migration", "error", err)
		} else if backupPath != "" {
			defer func() {
				if returnedErr != nil {
					restore()
				} else {
					os.Remove(backupPath)
				}
			}()
		}
	}

	var startDesc string
	legacyVer, hasLegacy, _ := loadLegacyDatabaseSchemaVersion(db)
	gooseVer, hasGoose, _ := LoadDatabaseVersion(db)
	if hasGoose {
		startDesc = fmt.Sprintf("goose version %d", gooseVer)
	} else if hasLegacy {
		startDesc = fmt.Sprintf("legacy version %d", legacyVer)
	} else {
		startDesc = "none (fresh database)"
	}

	state, err := detectSchemaState(db, ctx)
	if err != nil {
		return err
	}

	switch state {
	case schemaMigrationStateFresh:
		if err := ctx.InitializeFreshDatabaseSchema(db, backend); err != nil {
			return err
		}
	case schemaMigrationStateLegacyOnly:
		if err := upgradeLegacyToTerminal(db, backend, ctx); err != nil {
			return err
		}
	case schemaMigrationStateGooseOnly:
		if err := validateGooseBridgeState(db); err != nil {
			return err
		}
	case schemaMigrationStateLegacyBootstrap:
		if err := upgradeLegacyToTerminal(db, backend, ctx); err != nil {
			return err
		}
	case schemaMigrationStateMixed:
		legacyVersion, exists, err := loadLegacyDatabaseSchemaVersion(db)
		if err != nil {
			return err
		}
		if exists && int64(legacyVersion) != LegacyBridgeVersion {
			return fmt.Errorf("incomplete mixed migration state: legacy schema version %d does not match bridge terminal version %d", legacyVersion, LegacyBridgeVersion)
		}
		if err := validateGooseBridgeState(db); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown schema migration state: %d", state)
	}

	if err := runMigrations(db, backend, ctx); err != nil {
		return err
	}
	if err := finalizeLegacyToGooseBridge(db); err != nil {
		return err
	}
	if err := ctx.RepairCurrentSchemaState(db, backend); err != nil {
		return err
	}
	if err := ctx.ValidateCurrentDatabaseSchema(db, backend); err != nil {
		return err
	}
	if err := ValidateRegisteredSchema(db); err != nil {
		return err
	}

	endVer, _, _ := LoadDatabaseVersion(db)
	if hasGoose && int64(gooseVer) == int64(endVer) {
		slog.Info("database schema is already up to date", "version", endVer)
	} else {
		slog.Info("database migration completed successfully", "from", startDesc, "to", fmt.Sprintf("goose version %d", endVer))
	}
	return nil
}

func backupSQLiteDatabase(db *gorm.DB) (string, func(), error) {
	var dbList []struct {
		Seq  int
		Name string
		File string
	}
	if err := db.Raw("PRAGMA database_list").Scan(&dbList).Error; err != nil {
		return "", nil, err
	}
	var dbPath string
	for _, item := range dbList {
		if item.Name == "main" && item.File != "" {
			dbPath = item.File
			break
		}
	}
	if dbPath == "" {
		return "", nil, nil
	}

	backupPath := dbPath + ".bak"

	src, err := os.Open(dbPath)
	if err != nil {
		return "", nil, err
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", nil, err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", nil, err
	}
	dst.Sync()

	restoreFunc := func() {
		src, err := os.Open(backupPath)
		if err != nil {
			slog.Error("failed to open sqlite backup for restore", "error", err)
			return
		}
		defer src.Close()

		dst, err := os.OpenFile(dbPath, os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			slog.Error("failed to open sqlite db for restore", "error", err)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			slog.Error("failed to restore sqlite backup", "error", err)
		} else {
			dst.Sync()
			slog.Warn("restored sqlite database from backup due to migration failure")
		}
	}

	return backupPath, restoreFunc, nil
}
