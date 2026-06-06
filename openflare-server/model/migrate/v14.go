// v14 升级内容：为 WAF 规则组增加 PoW 策略字段。
// 背景说明：PoW 能力从站点路由侧沉淀到 WAF 规则组中，便于统一按规则组管理人机挑战策略。
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
	return ctx.EnsureDefaultWAFRuleGroup(db)
}

func validateV14(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 14)
}
