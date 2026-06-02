package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func gooseDialectForBackend(backend string) (goose.Dialect, error) {
	switch backend {
	case "postgres":
		return goose.DialectPostgres, nil
	case "sqlite":
		return goose.DialectSQLite3, nil
	default:
		return "", fmt.Errorf("unsupported database backend: %s", backend)
	}
}

func getGORMDBFromSQLDB(db *sql.DB, backend string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch backend {
	case "postgres":
		dialector = postgres.New(postgres.Config{Conn: db})
	case "sqlite":
		dialector = &sqlite.Dialector{Conn: db}
	default:
		return nil, fmt.Errorf("unsupported database backend: %s", backend)
	}

	gormDB, err := gorm.Open(dialector, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{},
	})
	if err != nil {
		return nil, err
	}
	if err := registerSharding(gormDB, backend); err != nil {
		return nil, err
	}
	return gormDB, nil
}

func buildGooseProvider(db *gorm.DB, backend string) (*goose.Provider, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	dialect, err := gooseDialectForBackend(backend)
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(
		dialect,
		sqlDB,
		nil,
		goose.WithDisableGlobalRegistry(true),
		goose.WithGoMigrations(buildGooseMigrations(backend)...),
	)
}

func runGooseMigrations(db *gorm.DB, backend string) error {
	provider, err := buildGooseProvider(db, backend)
	if err != nil {
		return fmt.Errorf("build goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("goose up failed: %w", err)
	}
	return nil
}
