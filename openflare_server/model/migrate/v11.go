package migrate

import "gorm.io/gorm"

func init() {
	Register(V11())
}

func V11() Migration {
	return Migration{
		FromVersion: 10,
		ToVersion:   11,
		Migrate:     migrateV11,
		Validate:    validateV11,
	}
}

func migrateV11(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ApplyCurrentSchema(db, backend)
}

func validateV11(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 11)
}
