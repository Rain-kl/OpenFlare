// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

func wafDB(ctx context.Context) (*gorm.DB, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	return conn, nil
}

// ListOpenFlareWAFRuleGroups returns all rule groups.
func ListOpenFlareWAFRuleGroups(ctx context.Context) ([]*model.OpenFlareWAFRuleGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var groups []*model.OpenFlareWAFRuleGroup
	if err = conn.Order("is_global desc").Order("id asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetOpenFlareWAFRuleGroupByID returns a rule group by id.
func GetOpenFlareWAFRuleGroupByID(ctx context.Context, id uint) (*model.OpenFlareWAFRuleGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var group model.OpenFlareWAFRuleGroup
	if err = conn.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// GetGlobalOpenFlareWAFRuleGroup returns the global rule group if present.
func GetGlobalOpenFlareWAFRuleGroup(ctx context.Context) (*model.OpenFlareWAFRuleGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var group model.OpenFlareWAFRuleGroup
	if err = conn.Where("is_global = ?", true).Order("id asc").First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// CreateOpenFlareWAFRuleGroup inserts a rule group.
func CreateOpenFlareWAFRuleGroup(ctx context.Context, group *model.OpenFlareWAFRuleGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Create(group).Error
}

// UpdateOpenFlareWAFRuleGroup updates mutable rule group fields.
func UpdateOpenFlareWAFRuleGroup(ctx context.Context, group *model.OpenFlareWAFRuleGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Model(&model.OpenFlareWAFRuleGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
		"name":      group.Name,
		colEnabled:  group.Enabled,
		"is_global": group.IsGlobal,
	}).Error
}

// UpdateOpenFlareWAFRuleGraph atomically replaces a graph when revision is current.
func UpdateOpenFlareWAFRuleGraph(ctx context.Context, id uint, revision uint64, graph string, attackGroupIDs ...uint) (uint64, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return 0, err
	}
	err = conn.Transaction(func(tx *gorm.DB) error {
		for _, groupID := range attackGroupIDs {
			var group model.OpenFlareAttackGroup
			if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthShare}).Select("id").Take(&group, groupID).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&model.OpenFlareWAFRuleGroup{}).
			Where("id = ? AND revision = ?", id, revision).
			Updates(map[string]any{"graph": graph, "revision": gorm.Expr("revision + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrWAFRuleRevisionConflict
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return revision + 1, nil
}

// ErrWAFGroupGlobal identifies flows that cannot be managed through site bindings.
var ErrWAFGroupGlobal = errors.New("global WAF flow applies independently of site bindings")

// SetOpenFlareWAFSiteRuleGroup adds or removes one binding, preserving the order of other flows.
func SetOpenFlareWAFSiteRuleGroup(ctx context.Context, routeID, groupID uint, bind bool) error {
	return WithProxyRouteTx(ctx, func(tx *gorm.DB) error {
		if _, err := LockProxyRouteTx(tx, routeID); err != nil {
			return err
		}
		var group model.OpenFlareWAFRuleGroup
		if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Select("id", "is_global").Take(&group, groupID).Error; err != nil {
			return err
		}
		if group.IsGlobal {
			return ErrWAFGroupGlobal
		}
		if !bind {
			return tx.Where("proxy_route_id = ? AND rule_group_id = ?", routeID, groupID).Delete(&model.OpenFlareWAFRuleGroupBinding{}).Error
		}
		var bindings []model.OpenFlareWAFRuleGroupBinding
		if err := tx.Where("proxy_route_id = ?", routeID).Order("sequence DESC").Find(&bindings).Error; err != nil {
			return err
		}
		for _, binding := range bindings {
			if binding.RuleGroupID == groupID {
				return nil
			}
		}
		sequence := 0
		if len(bindings) > 0 {
			sequence = bindings[0].Sequence + 1
		}
		return insertOpenFlareWAFRuleGroupBindings(tx, []model.OpenFlareWAFRuleGroupBinding{{ProxyRouteID: routeID, RuleGroupID: groupID, Sequence: sequence}})
	})
}

// DeleteOpenFlareWAFRuleGroup removes a rule group.
func DeleteOpenFlareWAFRuleGroup(ctx context.Context, id uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Delete(&model.OpenFlareWAFRuleGroup{}, id).Error
}

// ListOpenFlareWAFIPGroups returns all IP groups.
func ListOpenFlareWAFIPGroups(ctx context.Context) ([]*model.OpenFlareWAFIPGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var groups []*model.OpenFlareWAFIPGroup
	if err = conn.Order("type asc").Order("id asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// ListOpenFlareWAFIPGroupsByIDs returns IP groups for the given ids.
func ListOpenFlareWAFIPGroupsByIDs(ctx context.Context, ids []uint) ([]*model.OpenFlareWAFIPGroup, error) {
	if len(ids) == 0 {
		return []*model.OpenFlareWAFIPGroup{}, nil
	}
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var groups []*model.OpenFlareWAFIPGroup
	if err = conn.Where("id IN ?", ids).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetOpenFlareWAFIPGroupByID returns an IP group by id.
func GetOpenFlareWAFIPGroupByID(ctx context.Context, id uint) (*model.OpenFlareWAFIPGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var group model.OpenFlareWAFIPGroup
	if err = conn.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// CreateOpenFlareWAFIPGroup inserts an IP group.
func CreateOpenFlareWAFIPGroup(ctx context.Context, group *model.OpenFlareWAFIPGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Create(group).Error
}

// UpdateOpenFlareWAFIPGroup updates mutable IP group fields.
func UpdateOpenFlareWAFIPGroup(ctx context.Context, group *model.OpenFlareWAFIPGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Model(&model.OpenFlareWAFIPGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
		"name":                      group.Name,
		"type":                      group.Type,
		colEnabled:                  group.Enabled,
		"ip_list":                   group.IPList,
		"auto_config":               group.AutoConfig,
		"ext_ips":                   group.ExtIPs,
		"subscription_url":          group.SubscriptionURL,
		"subscription_format":       group.SubscriptionFormat,
		"subscription_mapping_rule": group.SubscriptionMappingRule,
		"sync_interval_minutes":     group.SyncIntervalMinutes,
		"next_sync_at":              group.NextSyncAt,
		"last_sync_status":          group.LastSyncStatus,
		"last_sync_message":         group.LastSyncMessage,
	}).Error
}

// ListDueOpenFlareWAFIPGroups returns enabled automatic/subscription groups due for sync.
func ListDueOpenFlareWAFIPGroups(ctx context.Context, now time.Time) ([]*model.OpenFlareWAFIPGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var groups []*model.OpenFlareWAFIPGroup
	err = conn.Where(
		"enabled = ? AND (type = ? OR (type = ? AND subscription_url <> '')) AND (next_sync_at IS NULL OR next_sync_at <= ?)",
		true, "automatic", "subscription", now,
	).Order("id asc").Find(&groups).Error
	return groups, err
}

// UpdateOpenFlareWAFIPGroupSyncResult persists IP group sync outcome fields.
func UpdateOpenFlareWAFIPGroupSyncResult(ctx context.Context, group *model.OpenFlareWAFIPGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Model(&model.OpenFlareWAFIPGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
		"ip_list":             group.IPList,
		"ext_ips":             group.ExtIPs,
		"last_synced_at":      group.LastSyncedAt,
		"next_sync_at":        group.NextSyncAt,
		"last_sync_status":    group.LastSyncStatus,
		"last_sync_message":   group.LastSyncMessage,
		"subscription_format": group.SubscriptionFormat,
	}).Error
}

// DeleteOpenFlareWAFIPGroup removes an IP group.
func DeleteOpenFlareWAFIPGroup(ctx context.Context, id uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Delete(&model.OpenFlareWAFIPGroup{}, id).Error
}

// ListOpenFlareWAFRuleGroupBindings returns bindings to existing proxy routes.
func ListOpenFlareWAFRuleGroupBindings(ctx context.Context) ([]model.OpenFlareWAFRuleGroupBinding, error) {
	conn, err := wafDB(db.WithPrimaryReads(ctx))
	if err != nil {
		return nil, err
	}
	var bindings []model.OpenFlareWAFRuleGroupBinding
	routeIDs := conn.Session(&gorm.Session{}).Model(&model.ProxyRoute{}).Select("id")
	if err = conn.Session(&gorm.Session{}).Where("proxy_route_id IN (?)", routeIDs).
		Order("sequence asc").Order("id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// ListOpenFlareWAFRuleGroupSites returns proxy routes in the rule group's scope.
func ListOpenFlareWAFRuleGroupSites(ctx context.Context, groupID uint, global bool) ([]model.ProxyRoute, error) {
	conn, err := wafDB(db.WithPrimaryReads(ctx))
	if err != nil {
		return nil, err
	}
	query := conn.Session(&gorm.Session{}).Model(&model.ProxyRoute{}).Select("id", "site_name").
		Preload("ZoneDomains", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("proxy_route_id", "domain").Order("domain ASC")
		}).Order("site_name ASC").Order("id ASC")
	if !global {
		boundRouteIDs := conn.Session(&gorm.Session{}).Model(&model.OpenFlareWAFRuleGroupBinding{}).
			Select("proxy_route_id").Where("rule_group_id = ?", groupID)
		query = query.Where("id IN (?)", boundRouteIDs)
	}
	var routes []model.ProxyRoute
	if err = query.Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

// DeleteOpenFlareWAFRuleGroupSites removes one group's bindings to the selected routes.
func DeleteOpenFlareWAFRuleGroupSites(ctx context.Context, groupID uint, routeIDs []uint) (int64, error) {
	// Keep each statement below the database's bind-parameter limit.
	const batchSize = 500
	conn, err := wafDB(ctx)
	if err != nil {
		return 0, err
	}
	var removed int64
	err = conn.Transaction(func(tx *gorm.DB) error {
		for batch := range slices.Chunk(routeIDs, batchSize) {
			result := tx.Where("rule_group_id = ? AND proxy_route_id IN ?", groupID, batch).
				Delete(&model.OpenFlareWAFRuleGroupBinding{})
			if result.Error != nil {
				return result.Error
			}
			removed += result.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}

// ListOpenFlareWAFRuleGroupBindingsByRouteID returns bindings for a proxy route.
func ListOpenFlareWAFRuleGroupBindingsByRouteID(ctx context.Context, routeID uint) ([]model.OpenFlareWAFRuleGroupBinding, error) {
	conn, err := wafDB(db.WithPrimaryReads(ctx))
	if err != nil {
		return nil, err
	}
	var bindings []model.OpenFlareWAFRuleGroupBinding
	if err = conn.Where("proxy_route_id = ?", routeID).Order("sequence asc").Order("id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

func syncWAFBindingIDSequence(tx *gorm.DB) error {
	if tx == nil || tx.Dialector.Name() != "postgres" { //nolint:staticcheck // QF1008: keep explicit Dialector field access
		return nil
	}
	return tx.Exec(`
		SELECT setval(
			pg_get_serial_sequence('of_waf_rule_group_bindings', 'id'),
			GREATEST(COALESCE((SELECT MAX(id) FROM of_waf_rule_group_bindings), 0), 1),
			COALESCE((SELECT MAX(id) FROM of_waf_rule_group_bindings), 0) > 0
		)
	`).Error
}

func insertOpenFlareWAFRuleGroupBindings(tx *gorm.DB, bindings []model.OpenFlareWAFRuleGroupBinding) error {
	if len(bindings) == 0 {
		return nil
	}
	if err := syncWAFBindingIDSequence(tx); err != nil {
		return err
	}
	return tx.Create(&bindings).Error
}

// ReplaceOpenFlareWAFRuleGroupBindings replaces bindings for a rule group.
func ReplaceOpenFlareWAFRuleGroupBindings(ctx context.Context, groupID uint, routeIDs []uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err = tx.Where("rule_group_id = ?", groupID).Delete(&model.OpenFlareWAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		bindings := make([]model.OpenFlareWAFRuleGroupBinding, 0, len(routeIDs))
		for index, routeID := range routeIDs {
			bindings = append(bindings, model.OpenFlareWAFRuleGroupBinding{
				RuleGroupID:  groupID,
				ProxyRouteID: routeID,
				Sequence:     index,
			})
		}
		return insertOpenFlareWAFRuleGroupBindings(tx, bindings)
	})
}

// ReplaceOpenFlareWAFSiteRuleGroupBindings replaces bindings for a proxy route.
func ReplaceOpenFlareWAFSiteRuleGroupBindings(ctx context.Context, routeID uint, groupIDs []uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err = tx.Where("proxy_route_id = ?", routeID).Delete(&model.OpenFlareWAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		bindings := make([]model.OpenFlareWAFRuleGroupBinding, 0, len(groupIDs))
		for index, groupID := range groupIDs {
			bindings = append(bindings, model.OpenFlareWAFRuleGroupBinding{
				RuleGroupID:  groupID,
				ProxyRouteID: routeID,
				Sequence:     index,
			})
		}
		return insertOpenFlareWAFRuleGroupBindings(tx, bindings)
	})
}

// DeleteOpenFlareWAFRuleGroupBindingsByGroupID removes bindings for a rule group.
func DeleteOpenFlareWAFRuleGroupBindingsByGroupID(ctx context.Context, groupID uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Where("rule_group_id = ?", groupID).Delete(&model.OpenFlareWAFRuleGroupBinding{}).Error
}

// DeleteOpenFlareWAFRuleGroupWithBindings removes a rule group and its bindings.
func DeleteOpenFlareWAFRuleGroupWithBindings(ctx context.Context, groupID uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		var group model.OpenFlareWAFRuleGroup
		if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Select("id").Take(&group, groupID).Error; err != nil {
			return err
		}
		if err = tx.Where("rule_group_id = ?", groupID).Delete(&model.OpenFlareWAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.OpenFlareWAFRuleGroup{}, groupID).Error
	})
}

// GetOpenFlareProxyRouteByID returns a proxy route by id when the table exists.
func GetOpenFlareProxyRouteByID(ctx context.Context, id uint) (*model.OriginProxyRoute, error) {
	if !HasProxyRoutesTable(ctx) {
		return nil, gorm.ErrRecordNotFound
	}
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var route model.OriginProxyRoute
	if err = conn.First(&route, id).Error; err != nil {
		return nil, err
	}
	return &route, nil
}
