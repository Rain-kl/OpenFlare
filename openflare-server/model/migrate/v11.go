// v11 升级内容：新增 ACME 账户、DNS 账户，并扩展证书 provider 字段。
// 背景说明：证书申请能力从单一手工导入扩展到自动签发，需要持久化 ACME/DNS 凭据，并标记证书来源。
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
	return nil
}

func validateV11(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 11)
}
