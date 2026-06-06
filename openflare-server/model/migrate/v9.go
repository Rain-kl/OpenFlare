// v9 升级内容：为 proxy_routes 增加 PoW 防护配置字段。
// 背景说明：反向代理站点需要支持 Proof-of-Work 抗机器人能力，因此在路由配置中持久化 PoW 开关与策略，并沿用 v8 的证书与站点字段回填。
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
