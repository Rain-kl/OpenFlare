package model

import (
	schemagoose "github.com/rain-kl/openflare/openflare-server/model/goose"

	"gorm.io/gorm"
)

func currentGooseTargetVersion() int64 {
	return schemagoose.CurrentTargetVersion()
}

func loadGooseDatabaseVersion(db *gorm.DB) (int, bool, error) {
	return schemagoose.LoadDatabaseVersion(db)
}

func ensureDatabaseSchemaUpToDate(db *gorm.DB, backend string) error {
	return schemagoose.EnsureDatabaseSchemaUpToDate(db, backend, databaseSchemaMigrationContext{})
}

func (databaseSchemaMigrationContext) RegisterSharding(db *gorm.DB, backend string) error {
	return registerSharding(db, backend)
}

func (databaseSchemaMigrationContext) AutoMigrateLegacySchemaMetadata(db *gorm.DB) error {
	return autoMigrateLegacySchemaMetadata(db)
}

func (databaseSchemaMigrationContext) InitializeFreshDatabaseSchema(db *gorm.DB, backend string) error {
	return initializeFreshDatabaseSchema(db, backend)
}

func (databaseSchemaMigrationContext) IsDatabaseEmpty(db *gorm.DB) (bool, error) {
	return isDatabaseEmpty(db)
}

func (databaseSchemaMigrationContext) RepairCurrentSchemaState(db *gorm.DB, backend string) error {
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

func (databaseSchemaMigrationContext) SaveLegacyDatabaseSchemaVersion(db *gorm.DB, version int) error {
	return saveLegacyDatabaseSchemaVersion(db, version)
}

func (databaseSchemaMigrationContext) UpgradeLegacyDatabaseSchema(db *gorm.DB, backend string, version int) error {
	return upgradeLegacyDatabaseSchema(db, backend, version)
}

func (databaseSchemaMigrationContext) ValidateCurrentDatabaseSchema(db *gorm.DB, backend string) error {
	return validateCurrentDatabaseSchema(db, backend)
}
