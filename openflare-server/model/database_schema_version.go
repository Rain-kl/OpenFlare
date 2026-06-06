package model

import (
	"time"

	"github.com/rain-kl/openflare/openflare-server/model/migrate"
)

const (
	legacyDatabaseSchemaVersion    = migrate.BaseDatabaseSchemaVersion
	legacyMigrationTerminalVersion = 17
	databaseSchemaVersionRowID     = 1
)

// currentDatabaseSchemaVersion tracks the current physical schema validated by the
// legacy validator set. Goose owns only post-v17 migrations, and none exist yet.
var currentDatabaseSchemaVersion = legacyMigrationTerminalVersion

type DatabaseSchemaVersion struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Version   int       `json:"version" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DatabaseSchemaVersion) TableName() string {
	return "database_schema_versions"
}
