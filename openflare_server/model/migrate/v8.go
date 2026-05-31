package migrate

import "gorm.io/gorm"

func init() {
	Register(V8())
}

func V8() Migration {
	return Migration{
		FromVersion: 7,
		ToVersion:   8,
		Migrate:     migrateV8,
		Validate:    validateV8,
	}
}

func migrateV8(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ApplyCurrentSchema(db, backend); err != nil {
		return err
	}
	if err := ctx.BackfillOriginsFromProxyRoutes(db); err != nil {
		return err
	}
	if err := ctx.BackfillProxyRouteSiteFields(db); err != nil {
		return err
	}
	if err := ctx.EnsureProxyRouteSiteNameUniqueIndex(db); err != nil {
		return err
	}
	if err := ctx.BackfillProxyRouteCertificateFields(db); err != nil {
		return err
	}
	return ctx.BackfillProxyRouteDomainCertificateFields(db)
}

func validateV8(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 8)
}
