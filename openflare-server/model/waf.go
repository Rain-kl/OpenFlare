package model

import "time"

type WAFRuleGroup struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	Name              string    `json:"name" gorm:"size:255;not null"`
	Enabled           bool      `json:"enabled" gorm:"not null;default:true"`
	IsGlobal          bool      `json:"is_global" gorm:"not null;default:false;index"`
	BlockStatusCode   int       `json:"block_status_code" gorm:"not null;default:418"`
	BlockResponseBody string    `json:"block_response_body" gorm:"type:text;not null;default:''"`
	IPWhitelist       string    `json:"ip_whitelist" gorm:"type:text;not null;default:'[]'"`
	IPBlacklist       string    `json:"ip_blacklist" gorm:"type:text;not null;default:'[]'"`
	IPWhitelistGroups string    `json:"ip_whitelist_group_ids" gorm:"type:text;not null;default:'[]'"`
	IPBlacklistGroups string    `json:"ip_blacklist_group_ids" gorm:"type:text;not null;default:'[]'"`
	CountryWhitelist  string    `json:"country_whitelist" gorm:"type:text;not null;default:'[]'"`
	CountryBlacklist  string    `json:"country_blacklist" gorm:"type:text;not null;default:'[]'"`
	RegionWhitelist   string    `json:"region_whitelist" gorm:"type:text;not null;default:'[]'"`
	RegionBlacklist   string    `json:"region_blacklist" gorm:"type:text;not null;default:'[]'"`
	PoWEnabled        bool      `json:"pow_enabled" gorm:"column:pow_enabled;not null;default:false"`
	PoWConfig         string    `json:"pow_config" gorm:"column:pow_config;type:text;not null;default:'{}'"`
	Remark            string    `json:"remark" gorm:"size:255"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type WAFIPGroup struct {
	ID                      uint       `json:"id" gorm:"primaryKey"`
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
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type WAFRuleGroupBinding struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	RuleGroupID  uint      `json:"rule_group_id" gorm:"not null;uniqueIndex:idx_waf_group_route"`
	ProxyRouteID uint      `json:"proxy_route_id" gorm:"not null;uniqueIndex:idx_waf_group_route;index"`
	CreatedAt    time.Time `json:"created_at"`
}

func ListWAFRuleGroups() ([]*WAFRuleGroup, error) {
	var groups []*WAFRuleGroup
	err := DB.Order("is_global desc").Order("id asc").Find(&groups).Error
	return groups, err
}

func GetWAFRuleGroupByID(id uint) (*WAFRuleGroup, error) {
	group := &WAFRuleGroup{}
	err := DB.First(group, id).Error
	return group, err
}

func GetGlobalWAFRuleGroup() (*WAFRuleGroup, error) {
	group := &WAFRuleGroup{}
	err := DB.Where("is_global = ?", true).Order("id asc").First(group).Error
	return group, err
}

func (group *WAFRuleGroup) Insert() error {
	return DB.Create(group).Error
}

func (group *WAFRuleGroup) Update() error {
	return DB.Model(&WAFRuleGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
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

func (group *WAFRuleGroup) Delete() error {
	return DB.Delete(group).Error
}

func ListWAFIPGroups() ([]*WAFIPGroup, error) {
	var groups []*WAFIPGroup
	err := DB.Order("type asc").Order("id asc").Find(&groups).Error
	return groups, err
}

func GetWAFIPGroupByID(id uint) (*WAFIPGroup, error) {
	group := &WAFIPGroup{}
	err := DB.First(group, id).Error
	return group, err
}

func ListWAFIPGroupsByIDs(ids []uint) ([]*WAFIPGroup, error) {
	if len(ids) == 0 {
		return []*WAFIPGroup{}, nil
	}
	var groups []*WAFIPGroup
	err := DB.Where("id IN ?", ids).Order("id asc").Find(&groups).Error
	return groups, err
}

func ListDueWAFIPGroups(now time.Time) ([]*WAFIPGroup, error) {
	var groups []*WAFIPGroup
	err := DB.Where("enabled = ? AND (type = ? OR (type = ? AND subscription_url <> '')) AND (next_sync_at IS NULL OR next_sync_at <= ?)", true, "automatic", "subscription", now).
		Order("id asc").
		Find(&groups).Error
	return groups, err
}

func (group *WAFIPGroup) Insert() error {
	return DB.Create(group).Error
}

func (group *WAFIPGroup) Update() error {
	return DB.Model(&WAFIPGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
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

func (group *WAFIPGroup) UpdateSyncResult() error {
	return DB.Model(&WAFIPGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
		"ip_list":             group.IPList,
		"ext_ips":             group.ExtIPs,
		"last_synced_at":      group.LastSyncedAt,
		"next_sync_at":        group.NextSyncAt,
		"last_sync_status":    group.LastSyncStatus,
		"last_sync_message":   group.LastSyncMessage,
		"subscription_format": group.SubscriptionFormat,
	}).Error
}

func (group *WAFIPGroup) Delete() error {
	return DB.Delete(group).Error
}
