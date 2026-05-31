package migrate

import "gorm.io/gorm"

func init() {
	Register(V9())
}

func V9() Migration {
	return Migration{
		FromVersion: 8,
		ToVersion:   9,
		Migrate:     migrateV9,
		Validate:    validateV9,
	}
}

func migrateV9(ctx Context, db *gorm.DB, backend string) error {
	if err := migrateV8(ctx, db, backend); err != nil {
		return err
	}
	return nil
}

func validateV9(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 9)
}
