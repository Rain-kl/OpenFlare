package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func newGooseBaselineMigration() *goose.Migration {
	migration := goose.NewGoMigration(int64(legacyMigrationTerminalVersion), nil, nil)
	migration.Source = fmt.Sprintf("%05d_legacy_terminal_baseline.go", legacyMigrationTerminalVersion)
	return migration
}

func newGooseGORMMigration(
	version int64,
	source string,
	backend string,
	up func(ctx databaseSchemaMigrationContext, db *gorm.DB, backend string) error,
) *goose.Migration {
	migration := goose.NewGoMigration(version, &goose.GoFunc{
		RunDB: func(_ context.Context, sqlDB *sql.DB) error {
			gormDB, err := getGORMDBFromSQLDB(sqlDB, backend)
			if err != nil {
				return err
			}
			ctx := databaseSchemaMigrationContext{}
			if backend == "postgres" {
				return gormDB.Transaction(func(tx *gorm.DB) error {
					return up(ctx, tx, backend)
				})
			}
			return up(ctx, gormDB, backend)
		},
	}, nil)
	migration.Source = source
	return migration
}

func registeredGooseMigrations(backend string) []*goose.Migration {
	_ = backend
	return nil
}

func buildGooseMigrations(backend string) []*goose.Migration {
	migrations := []*goose.Migration{newGooseBaselineMigration()}
	migrations = append(migrations, registeredGooseMigrations(backend)...)
	return migrations
}

func currentGooseTargetVersion() int64 {
	var maxVersion int64 = legacyMigrationTerminalVersion
	for _, migration := range buildGooseMigrations("sqlite") {
		if migration.Version > maxVersion {
			maxVersion = migration.Version
		}
	}
	return maxVersion
}
