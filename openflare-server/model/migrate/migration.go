package migrate

import (
	"sort"

	"gorm.io/gorm"
)

const BaseDatabaseSchemaVersion = 7

type Context interface {
	ApplyCurrentSchema(db *gorm.DB, backend string) error
	ApplyCurrentSchemaExcept(db *gorm.DB, backend string, excludedTables ...string) error
	BackfillOriginsFromProxyRoutes(db *gorm.DB) error
	BackfillProxyRouteSiteFields(db *gorm.DB) error
	EnsureProxyRouteSiteNameUniqueIndex(db *gorm.DB) error
	BackfillProxyRouteCertificateFields(db *gorm.DB) error
	BackfillProxyRouteDomainCertificateFields(db *gorm.DB) error
	EnsureDefaultGitHubAuthSource(db *gorm.DB) error
	EnsureDefaultWAFRuleGroup(db *gorm.DB) error
	DropLegacyNodeColumns(db *gorm.DB, backend string) error
	ValidateDatabaseSchemaVersion(db *gorm.DB, backend string, version int) error
}

type Migration struct {
	FromVersion int
	ToVersion   int
	Migrate     func(ctx Context, db *gorm.DB, backend string) error
	Validate    func(ctx Context, db *gorm.DB, backend string) error
}

var registeredMigrations []Migration

func Register(migration Migration) {
	registeredMigrations = append(registeredMigrations, migration)
}

func Migrations() []Migration {
	migrations := append([]Migration{}, registeredMigrations...)
	sort.Slice(migrations, func(i int, j int) bool {
		return migrations[i].FromVersion < migrations[j].FromVersion
	})
	return migrations
}

func CurrentVersion() int {
	version := BaseDatabaseSchemaVersion
	for _, migration := range registeredMigrations {
		if migration.ToVersion > version {
			version = migration.ToVersion
		}
	}
	return version
}
