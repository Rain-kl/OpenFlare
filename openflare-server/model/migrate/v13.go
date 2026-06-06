// v13 升级内容：新增 WAF 规则组与站点绑定表，并创建默认全局规则组。
// 背景说明：WAF 配置从零散站点字段演进为可复用规则组，需要全局规则组作为默认入口，并支持站点与规则组绑定。
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
	return ctx.EnsureDefaultWAFRuleGroup(db)
}

func validateV13(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 13)
}
