// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import "time"

// ConnectionInput configures the global Cloudflare credential source.
type ConnectionInput struct {
	Source       string `json:"source"`
	DNSAccountID uint   `json:"dns_account_id"`
	APIToken     string `json:"api_token"`
}

// ConnectionView exposes connection state without credentials.
type ConnectionView struct {
	Configured   bool       `json:"configured"`
	Ready        bool       `json:"ready"`
	Source       string     `json:"source"`
	DNSAccountID *uint      `json:"dns_account_id"`
	Status       string     `json:"status"`
	VerifiedAt   *time.Time `json:"verified_at"`
}

// NodeOption is a selectable edge node.
type NodeOption struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// GroupInput creates or updates a pointing group.
type GroupInput struct {
	Name           string `json:"name"`
	PrimaryNodeID  uint   `json:"primary_node_id"`
	BackupNodeID   *uint  `json:"backup_node_id"`
	DefaultProxied bool   `json:"default_proxied"`
	Enabled        bool   `json:"enabled"`
}

// GroupItem is the admin-facing pointing group summary.
type GroupItem struct {
	ID             uint        `json:"id"`
	Name           string      `json:"name"`
	PrimaryNode    NodeOption  `json:"primary_node"`
	BackupNode     *NodeOption `json:"backup_node"`
	ActiveNode     NodeOption  `json:"active_node"`
	DefaultProxied bool        `json:"default_proxied"`
	Enabled        bool        `json:"enabled"`
	MemberCount    int64       `json:"member_count"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// MemberCreateInput adds a ZoneDomain to a group. Nil Proxied copies the group default.
type MemberCreateInput struct {
	ZoneDomainID uint  `json:"zone_domain_id"`
	Proxied      *bool `json:"proxied"`
}

// MemberUpdateInput updates the effective orange-cloud state.
type MemberUpdateInput struct {
	Proxied bool `json:"proxied"`
}

// MemberItem is the admin-facing member state.
type MemberItem struct {
	ID           uint       `json:"id"`
	GroupID      uint       `json:"group_id"`
	ZoneDomainID uint       `json:"zone_domain_id"`
	Domain       string     `json:"domain"`
	ZoneID       uint       `json:"zone_id"`
	Proxied      bool       `json:"proxied"`
	DesiredIP    string     `json:"desired_ip"`
	SyncStatus   string     `json:"sync_status"`
	LastError    string     `json:"last_error"`
	SyncedAt     *time.Time `json:"synced_at"`
}

// GroupDetail combines a group with its members.
type GroupDetail struct {
	Group   GroupItem    `json:"group"`
	Members []MemberItem `json:"members"`
}

// AvailableDomain is a ZoneDomain eligible for pointing.
type AvailableDomain struct {
	ID         uint   `json:"id"`
	ZoneID     uint   `json:"zone_id"`
	Domain     string `json:"domain"`
	ZoneDomain string `json:"zone_domain"`
}

// Overview summarizes Cloudflare pointing readiness and sync health.
type Overview struct {
	Connection   ConnectionView `json:"connection"`
	GroupCount   int            `json:"group_count"`
	MemberCount  int            `json:"member_count"`
	OKCount      int            `json:"ok_count"`
	PendingCount int            `json:"pending_count"`
	ErrorCount   int            `json:"error_count"`
}

// SyncReceipt identifies a queued asynchronous synchronization.
type SyncReceipt struct {
	TaskID string `json:"task_id"`
}
