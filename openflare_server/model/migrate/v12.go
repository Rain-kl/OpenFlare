package migrate

import "gorm.io/gorm"

func init() {
	Register(V12())
}

func V12() Migration {
	return Migration{
		FromVersion: 11,
		ToVersion:   12,
		Migrate:     migrateV12,
		Validate:    validateV12,
	}
}

func migrateV12(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ApplyCurrentSchema(db, backend)
}

func validateV12(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 12)
}
