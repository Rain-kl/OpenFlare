// v8 升级内容：为 proxy_routes 增加域名级证书绑定字段 domain_cert_ids，并回填已有站点的证书映射。
// 背景说明：v1-v7 已作为历史初始基线合并；v8 是当前保留逐版本升级链的起点，用于把早期站点级证书列表扩展为每个域名可独立绑定证书。
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
