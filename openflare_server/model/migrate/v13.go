package migrate

import "gorm.io/gorm"

func init() {
	Register(V13())
}

func V13() Migration {
	return Migration{
		FromVersion: 12,
		ToVersion:   13,
		Migrate:     migrateV13,
		Validate:    validateV13,
	}
}

func migrateV13(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	return ctx.EnsureDefaultWAFRuleGroup(db)
}

func validateV13(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 13)
}
