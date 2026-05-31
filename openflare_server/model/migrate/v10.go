package migrate

import "gorm.io/gorm"

func init() {
	Register(V10())
}

func V10() Migration {
	return Migration{
		FromVersion: 9,
		ToVersion:   10,
		Migrate:     migrateV10,
		Validate:    validateV10,
	}
}

func migrateV10(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	return ctx.EnsureDefaultGitHubAuthSource(db)
}

func validateV10(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 10)
}
