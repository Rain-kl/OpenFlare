// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"errors"
	"time"
)

// OpenFlareWAFRuleGroup stores a WAF rule group.
type OpenFlareWAFRuleGroup struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"size:255;not null"`
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	IsGlobal  bool      `json:"is_global" gorm:"not null;default:false;index"`
	Graph     string    `json:"graph" gorm:"type:text;not null;default:''"`
	Revision  uint64    `json:"revision" gorm:"not null;default:1"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
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
	Sequence     int       `json:"sequence" gorm:"not null;default:0"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// ErrWAFRuleRevisionConflict indicates that a rule graph was updated from a stale revision.
var ErrWAFRuleRevisionConflict = errors.New("waf rule revision conflict")

// TableName returns the GORM table name.
func (OpenFlareWAFRuleGroupBinding) TableName() string {
	return "of_waf_rule_group_bindings"
}
