// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"slices"
	"time"
)

// EdgeUpdateCapability identifies nodes that support managed binary updates.
const EdgeUpdateCapability = "edge_self_update_v1"

// OpenFlareNode stores an edge, relay, or tunnel client node.
type OpenFlareNode struct {
	PerformancePolicyID       *uint      `json:"-" gorm:"->;index:idx_of_nodes_performance_policy_id"`
	ID                        uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID                    string     `json:"node_id" gorm:"uniqueIndex;size:64;not null"`
	Name                      string     `json:"name" gorm:"size:128;not null"`
	IP                        string     `json:"ip" gorm:"size:64;not null;default:''"`
	IPManualOverride          bool       `json:"ip_manual_override" gorm:"not null;default:false"`
	GeoName                   string     `json:"geo_name" gorm:"size:128;not null;default:''"`
	GeoLatitude               *float64   `json:"geo_latitude"`
	GeoLongitude              *float64   `json:"geo_longitude"`
	GeoManualOverride         bool       `json:"geo_manual_override" gorm:"not null;default:false"`
	GeoSource                 string     `json:"geo_source" gorm:"column:geo_source;size:16;not null;default:''"`
	NetworkGroup              string     `json:"network_group" gorm:"column:network_group;size:64;not null;default:''"`
	ASN                       uint32     `json:"asn" gorm:"column:asn;not null;default:0;index:idx_of_nodes_asn"`
	ASNOrg                    string     `json:"asn_org" gorm:"column:asn_org;size:255;not null;default:''"`
	AccessToken               string     `json:"-" gorm:"column:access_token;size:128;index"`
	AutoUpdateEnabled         bool       `json:"auto_update_enabled" gorm:"not null;default:false"`
	UpdateRequested           bool       `json:"update_requested" gorm:"not null;default:false"`
	UpdateChannel             string     `json:"update_channel" gorm:"size:16;not null;default:'stable'"`
	UpdateTag                 string     `json:"update_tag" gorm:"size:64;not null;default:''"`
	RestartOpenrestyRequested bool       `json:"restart_openresty_requested" gorm:"not null;default:false"`
	Version                   string     `json:"version" gorm:"size:64;not null;default:''"`
	ExtVersion                string     `json:"ext_version" gorm:"size:64;not null;default:''"`
	OpenrestyStatus           string     `json:"openresty_status" gorm:"size:16;not null;default:'unknown'"`
	OpenrestyMessage          string     `json:"openresty_message" gorm:"type:text"`
	OpenrestyConnections      int64      `json:"openresty_connections" gorm:"column:openresty_connections;not null;default:0"`
	Engine                    string     `json:"engine" gorm:"size:32;not null;default:'pingora';index"`
	ContractVersion           int        `json:"contract_version" gorm:"not null;default:0"`
	Status                    string     `json:"status" gorm:"size:16;not null;default:'offline'"`
	CurrentVersion            string     `json:"current_version" gorm:"size:32;not null;default:''"`
	LastSeenAt                *time.Time `json:"last_seen_at"`
	LastError                 string     `json:"last_error" gorm:"type:text"`
	CreatedAt                 time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                 time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	NodeType                  string     `json:"node_type" gorm:"size:32;not null;default:'edge_node'"`
	CapabilitiesJSON          string     `json:"capabilities_json" gorm:"type:text;not null;default:'[]'"`
	QuarantinedAt             *time.Time `json:"quarantined_at" gorm:"index:idx_of_nodes_quarantined_at"`
	QuarantineReason          string     `json:"quarantine_reason" gorm:"size:255;not null;default:''"`
}

// TableName returns the GORM table name.
func (OpenFlareNode) TableName() string {
	return "of_nodes"
}

// IsQuarantined reports whether the node has been emergency-quarantined.
func (node *OpenFlareNode) IsQuarantined() bool {
	return node != nil && node.QuarantinedAt != nil && !node.QuarantinedAt.IsZero()
}

// HasCapability reports whether the node advertised a runtime capability.
func (node *OpenFlareNode) HasCapability(name string) bool {
	var capabilities []string
	return json.Unmarshal([]byte(node.CapabilitiesJSON), &capabilities) == nil && slices.Contains(capabilities, name)
}

// OpenFlareNodeListQuery describes a bounded node query.
type OpenFlareNodeListQuery struct {
	Page     int
	PageSize int
	Search   string
	Status   string
	NodeType string
	Now      time.Time
}
