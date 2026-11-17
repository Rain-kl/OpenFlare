// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// OpenFlarePoWPolicy stores the independent PoW policy for one site.
type OpenFlarePoWPolicy struct {
	RouteID uint   `json:"route_id" gorm:"primaryKey;autoIncrement:false"`
	Enabled bool   `json:"enabled" gorm:"not null;default:false"`
	Config  string `json:"config" gorm:"type:text;not null;default:'{}'"`
}

// TableName returns the policy table name.
func (OpenFlarePoWPolicy) TableName() string { return "of_pow_policies" }
