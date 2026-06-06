package goose

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/glebarez/sqlite"
	presslygoose "github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Context interface {
	ApplyCurrentSchema(db *gorm.DB, backend string) error
	RegisterSharding(db *gorm.DB, backend string) error
}

func dialectForBackend(backend string) (presslygoose.Dialect, error) {
	switch backend {
	case "postgres":
		return presslygoose.DialectPostgres, nil
	case "sqlite":
		return presslygoose.DialectSQLite3, nil
	default:
		return "", fmt.Errorf("unsupported database backend: %s", backend)
	}
}

func openGORMDB(ctx Context, db *sql.DB, backend string) (*gorm.DB, error) {
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
	if err := ctx.RegisterSharding(gormDB, backend); err != nil {
		return nil, err
	}
	return gormDB, nil
}

func buildProvider(db *gorm.DB, backend string, ctx Context) (*presslygoose.Provider, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	dialect, err := dialectForBackend(backend)
	if err != nil {
		return nil, err
	}
	return presslygoose.NewProvider(
		dialect,
		sqlDB,
		nil,
		presslygoose.WithDisableGlobalRegistry(true),
		presslygoose.WithGoMigrations(buildMigrations(backend, ctx)...),
	)
}

func runMigrations(db *gorm.DB, backend string, ctx Context) error {
	provider, err := buildProvider(db, backend, ctx)
	if err != nil {
		return fmt.Errorf("build goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("goose up failed: %w", err)
	}
	return nil
}
