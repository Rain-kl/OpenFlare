package goose

import (
	"context"
	"database/sql"
	"fmt"

	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

const LegacyBridgeVersion int64 = 17

type migrationFunc func(ctx Context, db *gorm.DB, backend string) error

func newBaselineMigration() *presslygoose.Migration {
	migration := presslygoose.NewGoMigration(LegacyBridgeVersion, nil, nil)
	migration.Source = fmt.Sprintf("%05d_legacy_terminal_baseline.go", LegacyBridgeVersion)
	return migration
}

func newGORMMigration(version int64, source string, backend string, ctx Context, up migrationFunc) *presslygoose.Migration {
	migration := presslygoose.NewGoMigration(version, &presslygoose.GoFunc{
		RunDB: func(_ context.Context, sqlDB *sql.DB) error {
			gormDB, err := openGORMDB(ctx, sqlDB, backend)
			if err != nil {
				return err
			}
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

func registeredMigrations(backend string, ctx Context) []*presslygoose.Migration {
	return []*presslygoose.Migration{
		migration202606020001(backend, ctx),
		migration202606030001(backend, ctx),
		migration202606030002(backend, ctx),
		migration202606030003(backend, ctx),
		migration202606040004(backend, ctx),
	}
}

func buildMigrations(backend string, ctx Context) []*presslygoose.Migration {
	migrations := []*presslygoose.Migration{newBaselineMigration()}
	migrations = append(migrations, registeredMigrations(backend, ctx)...)
	return migrations
}

func CurrentTargetVersion() int64 {
	var maxVersion int64 = LegacyBridgeVersion
	for _, migration := range buildMigrations("sqlite", noopContext{}) {
		if migration.Version > maxVersion {
			maxVersion = migration.Version
		}
	}
	return maxVersion
}

type noopContext struct{}

func (noopContext) ApplyCurrentSchema(db *gorm.DB, backend string) error {
	return nil
}

func (noopContext) RegisterSharding(db *gorm.DB, backend string) error {
	return nil
}
