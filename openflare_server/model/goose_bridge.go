package model

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

type schemaMigrationState int

const (
	schemaMigrationStateFresh schemaMigrationState = iota
	schemaMigrationStateLegacyOnly
	schemaMigrationStateGooseOnly
	schemaMigrationStateLegacyBootstrap
	schemaMigrationStateMixed
)

func detectLegacySchemaState(db *gorm.DB) (schemaMigrationState, error) {
	hasLegacyTable := db.Migrator().HasTable(&DatabaseSchemaVersion{})
	hasGooseTable := db.Migrator().HasTable("goose_db_version")

	switch {
	case hasLegacyTable && hasGooseTable:
		return schemaMigrationStateMixed, nil
	case hasLegacyTable:
		return schemaMigrationStateLegacyOnly, nil
	case hasGooseTable:
		return schemaMigrationStateGooseOnly, nil
	}

	empty, err := isDatabaseEmpty(db)
	if err != nil {
		return 0, err
	}
	if empty {
		return schemaMigrationStateFresh, nil
	}
	return schemaMigrationStateLegacyBootstrap, nil
}

func loadGooseDatabaseVersion(db *gorm.DB) (int, bool, error) {
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

func bootstrapLegacySchemaVersion(db *gorm.DB) error {
	if err := autoMigrateLegacySchemaMetadata(db); err != nil {
		return err
	}
	version, exists, err := loadLegacyDatabaseSchemaVersion(db)
	if err != nil {
		return err
	}
	if exists {
		if version > legacyMigrationTerminalVersion {
			return fmt.Errorf("legacy schema version %d is newer than supported terminal version %d", version, legacyMigrationTerminalVersion)
		}
		return nil
	}
	return saveLegacyDatabaseSchemaVersion(db, legacyDatabaseSchemaVersion)
}

func upgradeLegacyToLegacyTerminal(db *gorm.DB, backend string) error {
	if err := bootstrapLegacySchemaVersion(db); err != nil {
		return err
	}
	version, exists, err := loadLegacyDatabaseSchemaVersion(db)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("legacy schema version record is missing after bootstrap")
	}
	return upgradeLegacyDatabaseSchema(db, backend, version)
}

func validateGooseBridgeState(db *gorm.DB) error {
	version, exists, err := loadGooseDatabaseVersion(db)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if version < legacyMigrationTerminalVersion {
		return fmt.Errorf("goose schema version %d is below legacy bridge baseline %d", version, legacyMigrationTerminalVersion)
	}
	if int64(version) > currentGooseTargetVersion() {
		return fmt.Errorf("goose schema version %d is newer than application target version %d", version, currentGooseTargetVersion())
	}
	return nil
}

func finalizeLegacyToGooseBridge(db *gorm.DB) error {
	gooseVersion, exists, err := loadGooseDatabaseVersion(db)
	if err != nil {
		return err
	}
	if !exists || gooseVersion < legacyMigrationTerminalVersion {
		return nil
	}
	if !db.Migrator().HasTable(&DatabaseSchemaVersion{}) {
		return nil
	}
	if err := db.Migrator().DropTable(&DatabaseSchemaVersion{}); err != nil {
		return fmt.Errorf("drop legacy schema versions table failed: %w", err)
	}
	slog.Info("completed legacy-to-goose migration bridge", "goose_version", gooseVersion)
	return nil
}

func repairCurrentSchemaState(db *gorm.DB, backend string) error {
	if err := dropLegacyNodeColumns(db, backend); err != nil {
		return err
	}
	if err := ensureDefaultGitHubAuthSource(db); err != nil {
		return err
	}
	if err := ensureDefaultWAFRuleGroup(db); err != nil {
		return err
	}
	return nil
}

func ensureDatabaseSchemaUpToDate(db *gorm.DB, backend string) error {
	state, err := detectLegacySchemaState(db)
	if err != nil {
		return err
	}

	switch state {
	case schemaMigrationStateFresh:
		if err := initializeFreshDatabaseSchema(db, backend); err != nil {
			return err
		}
	case schemaMigrationStateLegacyOnly:
		if err := upgradeLegacyToLegacyTerminal(db, backend); err != nil {
			return err
		}
	case schemaMigrationStateGooseOnly:
		if err := validateGooseBridgeState(db); err != nil {
			return err
		}
	case schemaMigrationStateLegacyBootstrap:
		if err := upgradeLegacyToLegacyTerminal(db, backend); err != nil {
			return err
		}
	case schemaMigrationStateMixed:
		legacyVersion, exists, err := loadLegacyDatabaseSchemaVersion(db)
		if err != nil {
			return err
		}
		if exists && legacyVersion != legacyMigrationTerminalVersion {
			return fmt.Errorf("incomplete mixed migration state: legacy schema version %d does not match bridge terminal version %d", legacyVersion, legacyMigrationTerminalVersion)
		}
		if err := validateGooseBridgeState(db); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown schema migration state: %d", state)
	}

	if err := runGooseMigrations(db, backend); err != nil {
		return err
	}
	if err := finalizeLegacyToGooseBridge(db); err != nil {
		return err
	}
	if err := repairCurrentSchemaState(db, backend); err != nil {
		return err
	}
	return validateCurrentDatabaseSchema(db, backend)
}
