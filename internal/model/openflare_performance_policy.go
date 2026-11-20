// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// DefaultPerformancePolicyKey identifies the policy inherited by unassigned Pingora nodes.
const DefaultPerformancePolicyKey = "default"

// OpenFlarePerformancePolicy is a named collection of Pingora parameters.
type OpenFlarePerformancePolicy struct {
	ID           uint    `gorm:"primaryKey;autoIncrement"`
	Name         string  `gorm:"size:100;not null"`
	PresetKey    *string `gorm:"size:16;unique"`
	SettingsJSON string  `gorm:"type:text;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TableName returns the database table for performance policies.
func (OpenFlarePerformancePolicy) TableName() string {
	return "of_performance_policies"
}

// IsDefault reports whether the policy is inherited by nodes without an assignment.
func (p OpenFlarePerformancePolicy) IsDefault() bool {
	return p.PresetKey != nil && *p.PresetKey == DefaultPerformancePolicyKey
}

// PerformancePolicyNode is the assignment view used by the performance page.
type PerformancePolicyNode struct {
	NodeID   string `json:"node_id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	PolicyID *uint  `json:"policy_id"`
}
