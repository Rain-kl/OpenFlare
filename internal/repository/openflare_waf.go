// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

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
func UpdateOpenFlareWAFRuleGraph(ctx context.Context, id uint, revision uint64, graph string) (uint64, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return 0, err
	}
	result := conn.Model(&model.OpenFlareWAFRuleGroup{}).
		Where("id = ? AND revision = ?", id, revision).
		Updates(map[string]any{"graph": graph, "revision": gorm.Expr("revision + 1")})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, model.ErrWAFRuleRevisionConflict
	}
	return revision + 1, nil
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

// ListOpenFlareWAFRuleGroupBindings returns all bindings.
func ListOpenFlareWAFRuleGroupBindings(ctx context.Context) ([]model.OpenFlareWAFRuleGroupBinding, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var bindings []model.OpenFlareWAFRuleGroupBinding
	if err = conn.Order("sequence asc").Order("id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// ListOpenFlareWAFRuleGroupBindingsByRouteID returns bindings for a proxy route.
func ListOpenFlareWAFRuleGroupBindingsByRouteID(ctx context.Context, routeID uint) ([]model.OpenFlareWAFRuleGroupBinding, error) {
	conn, err := wafDB(ctx)
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
