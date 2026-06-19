// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"gorm.io/gorm"
)

// OpenFlareWAFRuleGroup stores a WAF rule group.
type OpenFlareWAFRuleGroup struct {
	ID                uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name              string    `json:"name" gorm:"size:255;not null"`
	Enabled           bool      `json:"enabled" gorm:"not null;default:true"`
	IsGlobal          bool      `json:"is_global" gorm:"not null;default:false;index"`
	BlockStatusCode   int       `json:"block_status_code" gorm:"not null;default:418"`
	BlockResponseBody string    `json:"block_response_body" gorm:"type:text;not null;default:''"`
	IPWhitelist       string    `json:"ip_whitelist" gorm:"type:text;not null;default:'[]'"`
	IPBlacklist       string    `json:"ip_blacklist" gorm:"type:text;not null;default:'[]'"`
	IPWhitelistGroups string    `json:"ip_whitelist_group_ids" gorm:"column:ip_whitelist_groups;type:text;not null;default:'[]'"`
	IPBlacklistGroups string    `json:"ip_blacklist_group_ids" gorm:"column:ip_blacklist_groups;type:text;not null;default:'[]'"`
	CountryWhitelist  string    `json:"country_whitelist" gorm:"type:text;not null;default:'[]'"`
	CountryBlacklist  string    `json:"country_blacklist" gorm:"type:text;not null;default:'[]'"`
	RegionWhitelist   string    `json:"region_whitelist" gorm:"type:text;not null;default:'[]'"`
	RegionBlacklist   string    `json:"region_blacklist" gorm:"type:text;not null;default:'[]'"`
	PoWEnabled        bool      `json:"pow_enabled" gorm:"column:pow_enabled;not null;default:false"`
	PoWConfig         string    `json:"pow_config" gorm:"column:pow_config;type:text;not null;default:'{}'"`
	Remark            string    `json:"remark" gorm:"size:255"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareWAFRuleGroup) TableName() string {
	return "of_waf_rule_groups"
}

// OpenFlareWAFIPGroup stores a WAF IP group.
type OpenFlareWAFIPGroup struct {
	ID                      uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name                    string     `json:"name" gorm:"size:255;not null"`
	Type                    string     `json:"type" gorm:"size:32;not null;index"`
	Enabled                 bool       `json:"enabled" gorm:"not null;default:true"`
	IPList                  string     `json:"ip_list" gorm:"type:text;not null;default:'[]'"`
	AutoConfig              string     `json:"auto_config" gorm:"type:text;not null;default:'{}'"`
	ExtIPs                  string     `json:"ext_ips" gorm:"type:text;not null;default:'[]'"`
	SubscriptionURL         string     `json:"subscription_url" gorm:"size:2048;not null;default:''"`
	SubscriptionFormat      string     `json:"subscription_format" gorm:"size:32;not null;default:'text'"`
	SubscriptionMappingRule string     `json:"subscription_mapping_rule" gorm:"size:255;not null;default:''"`
	SyncIntervalMinutes     int        `json:"sync_interval_minutes" gorm:"not null;default:1440"`
	LastSyncedAt            *time.Time `json:"last_synced_at"`
	NextSyncAt              *time.Time `json:"next_sync_at" gorm:"index"`
	LastSyncStatus          string     `json:"last_sync_status" gorm:"size:32;not null;default:''"`
	LastSyncMessage         string     `json:"last_sync_message" gorm:"type:text;not null;default:''"`
	Remark                  string     `json:"remark" gorm:"size:255"`
	CreatedAt               time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt               time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareWAFIPGroup) TableName() string {
	return "of_waf_ip_groups"
}

// OpenFlareWAFRuleGroupBinding binds a rule group to a proxy route.
type OpenFlareWAFRuleGroupBinding struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	RuleGroupID  uint      `json:"rule_group_id" gorm:"not null;uniqueIndex:idx_of_waf_group_route"`
	ProxyRouteID uint      `json:"proxy_route_id" gorm:"not null;uniqueIndex:idx_of_waf_group_route;index"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareWAFRuleGroupBinding) TableName() string {
	return "of_waf_rule_group_bindings"
}

func wafDB(ctx context.Context) (*gorm.DB, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	return conn, nil
}

// ListOpenFlareWAFRuleGroups returns all rule groups.
func ListOpenFlareWAFRuleGroups(ctx context.Context) ([]*OpenFlareWAFRuleGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var groups []*OpenFlareWAFRuleGroup
	if err = conn.Order("is_global desc").Order("id asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetOpenFlareWAFRuleGroupByID returns a rule group by id.
func GetOpenFlareWAFRuleGroupByID(ctx context.Context, id uint) (*OpenFlareWAFRuleGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var group OpenFlareWAFRuleGroup
	if err = conn.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// GetGlobalOpenFlareWAFRuleGroup returns the global rule group if present.
func GetGlobalOpenFlareWAFRuleGroup(ctx context.Context) (*OpenFlareWAFRuleGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var group OpenFlareWAFRuleGroup
	if err = conn.Where("is_global = ?", true).Order("id asc").First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// CreateOpenFlareWAFRuleGroup inserts a rule group.
func CreateOpenFlareWAFRuleGroup(ctx context.Context, group *OpenFlareWAFRuleGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Create(group).Error
}

// UpdateOpenFlareWAFRuleGroup updates mutable rule group fields.
func UpdateOpenFlareWAFRuleGroup(ctx context.Context, group *OpenFlareWAFRuleGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Model(&OpenFlareWAFRuleGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
		"name":                group.Name,
		"enabled":             group.Enabled,
		"is_global":           group.IsGlobal,
		"block_status_code":   group.BlockStatusCode,
		"block_response_body": group.BlockResponseBody,
		"ip_whitelist":        group.IPWhitelist,
		"ip_blacklist":        group.IPBlacklist,
		"ip_whitelist_groups": group.IPWhitelistGroups,
		"ip_blacklist_groups": group.IPBlacklistGroups,
		"country_whitelist":   group.CountryWhitelist,
		"country_blacklist":   group.CountryBlacklist,
		"region_whitelist":    group.RegionWhitelist,
		"region_blacklist":    group.RegionBlacklist,
		"pow_enabled":         group.PoWEnabled,
		"pow_config":          group.PoWConfig,
		"remark":              group.Remark,
	}).Error
}

// DeleteOpenFlareWAFRuleGroup removes a rule group.
func DeleteOpenFlareWAFRuleGroup(ctx context.Context, id uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Delete(&OpenFlareWAFRuleGroup{}, id).Error
}

// ListOpenFlareWAFIPGroups returns all IP groups.
func ListOpenFlareWAFIPGroups(ctx context.Context) ([]*OpenFlareWAFIPGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var groups []*OpenFlareWAFIPGroup
	if err = conn.Order("type asc").Order("id asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// ListOpenFlareWAFIPGroupsByIDs returns IP groups for the given ids.
func ListOpenFlareWAFIPGroupsByIDs(ctx context.Context, ids []uint) ([]*OpenFlareWAFIPGroup, error) {
	if len(ids) == 0 {
		return []*OpenFlareWAFIPGroup{}, nil
	}
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var groups []*OpenFlareWAFIPGroup
	if err = conn.Where("id IN ?", ids).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetOpenFlareWAFIPGroupByID returns an IP group by id.
func GetOpenFlareWAFIPGroupByID(ctx context.Context, id uint) (*OpenFlareWAFIPGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var group OpenFlareWAFIPGroup
	if err = conn.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// CreateOpenFlareWAFIPGroup inserts an IP group.
func CreateOpenFlareWAFIPGroup(ctx context.Context, group *OpenFlareWAFIPGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Create(group).Error
}

// UpdateOpenFlareWAFIPGroup updates mutable IP group fields.
func UpdateOpenFlareWAFIPGroup(ctx context.Context, group *OpenFlareWAFIPGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Model(&OpenFlareWAFIPGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
		"name":                      group.Name,
		"type":                      group.Type,
		"enabled":                   group.Enabled,
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
		"remark":                    group.Remark,
	}).Error
}

// ListDueOpenFlareWAFIPGroups returns enabled automatic/subscription groups due for sync.
func ListDueOpenFlareWAFIPGroups(ctx context.Context, now time.Time) ([]*OpenFlareWAFIPGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var groups []*OpenFlareWAFIPGroup
	err = conn.Where(
		"enabled = ? AND (type = ? OR (type = ? AND subscription_url <> '')) AND (next_sync_at IS NULL OR next_sync_at <= ?)",
		true, "automatic", "subscription", now,
	).Order("id asc").Find(&groups).Error
	return groups, err
}

// UpdateOpenFlareWAFIPGroupSyncResult persists IP group sync outcome fields.
func UpdateOpenFlareWAFIPGroupSyncResult(ctx context.Context, group *OpenFlareWAFIPGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Model(&OpenFlareWAFIPGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
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
	return conn.Delete(&OpenFlareWAFIPGroup{}, id).Error
}

// ListOpenFlareWAFRuleGroupBindings returns all bindings.
func ListOpenFlareWAFRuleGroupBindings(ctx context.Context) ([]OpenFlareWAFRuleGroupBinding, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var bindings []OpenFlareWAFRuleGroupBinding
	if err = conn.Order("rule_group_id asc").Order("proxy_route_id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// ListOpenFlareWAFRuleGroupBindingsByRouteID returns bindings for a proxy route.
func ListOpenFlareWAFRuleGroupBindingsByRouteID(ctx context.Context, routeID uint) ([]OpenFlareWAFRuleGroupBinding, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var bindings []OpenFlareWAFRuleGroupBinding
	if err = conn.Where("proxy_route_id = ?", routeID).Order("rule_group_id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// ReplaceOpenFlareWAFRuleGroupBindings replaces bindings for a rule group.
func ReplaceOpenFlareWAFRuleGroupBindings(ctx context.Context, groupID uint, routeIDs []uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err = tx.Where("rule_group_id = ?", groupID).Delete(&OpenFlareWAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		for _, routeID := range routeIDs {
			binding := OpenFlareWAFRuleGroupBinding{RuleGroupID: groupID, ProxyRouteID: routeID}
			if err = tx.Create(&binding).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ReplaceOpenFlareWAFSiteRuleGroupBindings replaces bindings for a proxy route.
func ReplaceOpenFlareWAFSiteRuleGroupBindings(ctx context.Context, routeID uint, groupIDs []uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err = tx.Where("proxy_route_id = ?", routeID).Delete(&OpenFlareWAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		for _, groupID := range groupIDs {
			binding := OpenFlareWAFRuleGroupBinding{RuleGroupID: groupID, ProxyRouteID: routeID}
			if err = tx.Create(&binding).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteOpenFlareWAFRuleGroupBindingsByGroupID removes bindings for a rule group.
func DeleteOpenFlareWAFRuleGroupBindingsByGroupID(ctx context.Context, groupID uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Where("rule_group_id = ?", groupID).Delete(&OpenFlareWAFRuleGroupBinding{}).Error
}

// DeleteOpenFlareWAFRuleGroupWithBindings removes a rule group and its bindings.
func DeleteOpenFlareWAFRuleGroupWithBindings(ctx context.Context, groupID uint) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err = tx.Where("rule_group_id = ?", groupID).Delete(&OpenFlareWAFRuleGroupBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(&OpenFlareWAFRuleGroup{}, groupID).Error
	})
}

// GetOpenFlareProxyRouteByID returns a proxy route by id when the table exists.
func GetOpenFlareProxyRouteByID(ctx context.Context, id uint) (*OriginProxyRoute, error) {
	if !HasProxyRoutesTable(ctx) {
		return nil, gorm.ErrRecordNotFound
	}
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	var route OriginProxyRoute
	if err = conn.First(&route, id).Error; err != nil {
		return nil, err
	}
	return &route, nil
}
