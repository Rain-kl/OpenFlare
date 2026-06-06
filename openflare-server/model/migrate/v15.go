// v15 升级内容：为 nodes 增加 ip_manual_override 字段。
// 背景说明：管理端手动指定节点 IP 后，Agent 心跳不应继续覆盖该值，因此需要在节点表中记录 IP 是否由管理端锁定。
package migrate

import (
	"fmt"

	"gorm.io/gorm"
)

type nodeV15 struct {
	IPManualOverride bool `gorm:"column:ip_manual_override;not null;default:false"`
}

func init() {
	Register(V15())
}

func V15() Migration {
	return Migration{
		FromVersion: 14,
		ToVersion:   15,
		Migrate:     migrateV15,
		Validate:    validateV15,
	}
}

func (nodeV15) TableName() string {
	return "nodes"
}

func migrateV15(ctx Context, db *gorm.DB, backend string) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if !db.Migrator().HasColumn(&nodeV15{}, "ip_manual_override") {
		if err := db.Migrator().AddColumn(&nodeV15{}, "IPManualOverride"); err != nil {
			return fmt.Errorf("add nodes.ip_manual_override: %w", err)
		}
	}
	return nil
}

func validateV15(ctx Context, db *gorm.DB, backend string) error {
	if err := ctx.ValidateDatabaseSchemaVersion(db, backend, 14); err != nil {
		return err
	}
	if db == nil || !db.Migrator().HasColumn(&nodeV15{}, "ip_manual_override") {
		return fmt.Errorf("column nodes.ip_manual_override is missing")
	}
	return nil
}
