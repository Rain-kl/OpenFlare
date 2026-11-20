// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// OpenFlareAttackGroup stores protection levels selected by an attack protection node.
type OpenFlareAttackGroup struct {
	ID        uint              `json:"id" gorm:"primaryKey"`
	Name      string            `json:"name"`
	Levels    map[string]string `json:"levels" gorm:"serializer:json;type:text"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// TableName returns the attack group configuration table.
func (OpenFlareAttackGroup) TableName() string { return "of_attack_groups" }
