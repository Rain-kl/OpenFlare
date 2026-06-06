// v10 升级内容：新增可配置认证源与第三方账号绑定，并迁移旧 GitHub 登录配置。
// 背景说明：登录体系从固定 GitHub OAuth 字段演进为通用认证源模型，需要创建 auth_sources、external_accounts，并把旧用户 GitHub 绑定迁移到新表。
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
	return ctx.EnsureDefaultGitHubAuthSource(db)
}

func validateV10(ctx Context, db *gorm.DB, backend string) error {
	return ctx.ValidateDatabaseSchemaVersion(db, backend, 10)
}
