// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

const (
	// CFConnectionSourceDNSAccount imports credentials from an existing DNS account.
	CFConnectionSourceDNSAccount = "dns_account"
	// CFConnectionSourceStandalone stores an independent API token.
	CFConnectionSourceStandalone = "standalone"

	// CFConnectionStatusReady indicates the credential passed verification.
	CFConnectionStatusReady = "ready"
	// CFConnectionStatusError indicates the latest verification failed.
	CFConnectionStatusError = "error"

	// CFMemberSyncPending indicates synchronization is queued or required.
	CFMemberSyncPending = "pending"
	// CFMemberSyncing indicates a worker is reconciling the record.
	CFMemberSyncing = "syncing"
	// CFMemberSyncOK indicates the remote record matches the desired state.
	CFMemberSyncOK = "ok"
	// CFMemberSyncError indicates the latest reconciliation failed.
	CFMemberSyncError = "error"
)

// CFConnection stores the single Cloudflare API credential source.
type CFConnection struct {
	ID            uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	Source        string     `json:"source" gorm:"size:32;not null;default:''"`
	DNSAccountID  *uint      `json:"dns_account_id" gorm:"index:idx_of_cf_connections_dns_account_id"`
	Authorization string     `json:"-" gorm:"type:text;not null;default:''"`
	Status        string     `json:"status" gorm:"size:16;not null;default:''"`
	VerifiedAt    *time.Time `json:"verified_at"`
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the Cloudflare connection table name.
func (CFConnection) TableName() string { return "of_cf_connections" }

// CFPointingGroup stores a reusable node target for DNS records.
type CFPointingGroup struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name           string    `json:"name" gorm:"size:128;not null"`
	PrimaryNodeID  uint      `json:"primary_node_id" gorm:"not null;index:idx_of_cf_pointing_groups_primary_node_id"`
	BackupNodeID   *uint     `json:"backup_node_id" gorm:"index:idx_of_cf_pointing_groups_backup_node_id"`
	ActiveNodeID   uint      `json:"active_node_id" gorm:"not null;index:idx_of_cf_pointing_groups_active_node_id"`
	DefaultProxied bool      `json:"default_proxied" gorm:"not null;default:false"`
	Enabled        bool      `json:"enabled" gorm:"not null;default:false"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the Cloudflare pointing group table name.
func (CFPointingGroup) TableName() string { return "of_cf_pointing_groups" }

// CFPointingMember stores one managed ZoneDomain A record.
type CFPointingMember struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	GroupID      uint       `json:"group_id" gorm:"not null;index:idx_of_cf_pointing_members_group_id"`
	ZoneDomainID uint       `json:"zone_domain_id" gorm:"not null;uniqueIndex:idx_of_cf_pointing_members_zone_domain_id"`
	Proxied      bool       `json:"proxied" gorm:"not null;default:false"`
	CFZoneID     string     `json:"cf_zone_id" gorm:"size:64;not null;default:''"`
	CFRecordID   string     `json:"cf_record_id" gorm:"size:64;not null;default:''"`
	DesiredIP    string     `json:"desired_ip" gorm:"size:64;not null;default:''"`
	SyncStatus   string     `json:"sync_status" gorm:"size:16;not null;default:'pending'"`
	LastError    string     `json:"last_error" gorm:"type:text;not null;default:''"`
	SyncedAt     *time.Time `json:"synced_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the Cloudflare pointing member table name.
func (CFPointingMember) TableName() string { return "of_cf_pointing_members" }
