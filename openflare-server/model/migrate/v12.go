// v12 升级内容：为 proxy_routes 增加 Basic Auth 相关字段。
// 背景说明：站点级访问控制需要支持基础认证，因此在代理路由配置中持久化 Basic Auth 开关与凭据配置。
package migrate

import "gorm.io/gorm"

func init() {
	Register(V12())
}

func V12() Migration {
	return Migration{
		FromVersion: 11,
		ToVersion:   12,
		Migrate:     migrateV12,
		Validate:    validateV12,
	}
}

func migrateV12(ctx Context, db *gorm.DB, backend string) error {
	return nil
}

func validateV12(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 12)
}
