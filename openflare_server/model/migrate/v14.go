package migrate

import "gorm.io/gorm"

func init() {
	Register(V14())
}

func V14() Migration {
	return Migration{
		FromVersion: 13,
		ToVersion:   14,
		Migrate:     migrateV14,
		Validate:    validateV14,
	}
}

func migrateV14(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	return ctx.EnsureDefaultWAFRuleGroup(db)
}

func validateV14(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 14)
}
