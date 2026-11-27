// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// OpenFlareNodeTag defines a reusable routing classification tag for edge nodes.
type OpenFlareNodeTag struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string    `json:"name" gorm:"size:64;not null"`
	Slug          string    `json:"slug" gorm:"size:64;not null;uniqueIndex:uidx_of_node_tags_slug"`
	Color         string    `json:"color" gorm:"size:32;not null;default:'blue'"`
	Description   string    `json:"description" gorm:"size:255;not null;default:''"`
	DefaultWeight float64   `json:"default_weight" gorm:"not null;default:1.0"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareNodeTag) TableName() string {
	return "of_node_tags"
}

// OpenFlareNodeTagBinding assigns a tag to a node with an optional node-specific weight override.
type OpenFlareNodeTagBinding struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID         string    `json:"node_id" gorm:"size:64;not null;uniqueIndex:uidx_of_node_tag_bindings_pair"`
	TagID          uint      `json:"tag_id" gorm:"not null;uniqueIndex:uidx_of_node_tag_bindings_pair;index:idx_of_node_tag_bindings_tag"`
	WeightOverride *float64  `json:"weight_override"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the GORM table name.
func (OpenFlareNodeTagBinding) TableName() string {
	return "of_node_tag_bindings"
}

// NodeTagSummary represents a node's effective tag attachment and resolved weight.
type NodeTagSummary struct {
	TagID           uint     `json:"tag_id"`
	Slug            string   `json:"slug"`
	Name            string   `json:"name"`
	Color           string   `json:"color"`
	DefaultWeight   float64  `json:"default_weight"`
	WeightOverride  *float64 `json:"weight_override,omitempty"`
	EffectiveWeight float64  `json:"effective_weight"`
}
