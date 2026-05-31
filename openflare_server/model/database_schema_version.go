package model

import (
	"openflare/model/migrate"
	"time"
)

const (
	legacyDatabaseSchemaVersion = migrate.BaseDatabaseSchemaVersion
	databaseSchemaVersionRowID  = 1
)

var currentDatabaseSchemaVersion = migrate.CurrentVersion()

type DatabaseSchemaVersion struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Version   int       `json:"version" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DatabaseSchemaVersion) TableName() string {
	return "database_schema_versions"
}
