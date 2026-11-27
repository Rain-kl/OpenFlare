// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// NodeTagBindingSpec defines a desired tag binding for a node.
type NodeTagBindingSpec struct {
	TagID          uint
	WeightOverride *float64
}

type nodeTagBindingSummaryRow struct {
	NodeID         string   `gorm:"column:node_id"`
	TagID          uint     `gorm:"column:tag_id"`
	Slug           string   `gorm:"column:slug"`
	Name           string   `gorm:"column:name"`
	Color          string   `gorm:"column:color"`
	DefaultWeight  float64  `gorm:"column:default_weight"`
	WeightOverride *float64 `gorm:"column:weight_override"`
}

// ListOpenFlareNodeTags returns all defined node routing tags ordered by ID.
func ListOpenFlareNodeTags(ctx context.Context) ([]model.OpenFlareNodeTag, error) {
	var tags []model.OpenFlareNodeTag
	if err := db.DB(ctx).Order("id ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// GetOpenFlareNodeTagByID returns a node tag by primary key.
func GetOpenFlareNodeTagByID(ctx context.Context, id uint) (*model.OpenFlareNodeTag, error) {
	var tag model.OpenFlareNodeTag
	if err := db.DB(ctx).First(&tag, id).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// GetOpenFlareNodeTagBySlug returns a node tag by its unique slug.
func GetOpenFlareNodeTagBySlug(ctx context.Context, slug string) (*model.OpenFlareNodeTag, error) {
	var tag model.OpenFlareNodeTag
	if err := db.DB(ctx).Where("slug = ?", strings.TrimSpace(slug)).First(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// ListOpenFlareNodeTagsBySlugs returns node tags matching the provided slugs.
func ListOpenFlareNodeTagsBySlugs(ctx context.Context, slugs []string) ([]model.OpenFlareNodeTag, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	var tags []model.OpenFlareNodeTag
	if err := db.DB(ctx).Where("slug IN ?", slugs).Order("id ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// CreateOpenFlareNodeTag creates a new node tag.
func CreateOpenFlareNodeTag(ctx context.Context, tag *model.OpenFlareNodeTag) error {
	return db.DB(ctx).Create(tag).Error
}

// SaveOpenFlareNodeTag persists changes to an existing node tag.
func SaveOpenFlareNodeTag(ctx context.Context, tag *model.OpenFlareNodeTag) error {
	return db.DB(ctx).Save(tag).Error
}

// DeleteOpenFlareNodeTag removes a node tag and all its node bindings atomically.
func DeleteOpenFlareNodeTag(ctx context.Context, id uint) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", id).Delete(&model.OpenFlareNodeTagBinding{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&model.OpenFlareNodeTag{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// ReplaceOpenFlareNodeTagBindings atomically replaces a node's tag bindings.
func ReplaceOpenFlareNodeTagBindings(ctx context.Context, nodeID string, specs []NodeTagBindingSpec) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("node_id = ?", nodeID).Delete(&model.OpenFlareNodeTagBinding{}).Error; err != nil {
			if len(specs) == 0 && isMissingTableError(err) {
				return nil
			}
			return err
		}
		if len(specs) == 0 {
			return nil
		}
		seen := make(map[uint]struct{}, len(specs))
		rows := make([]model.OpenFlareNodeTagBinding, 0, len(specs))
		for _, spec := range specs {
			if spec.TagID == 0 {
				continue
			}
			if _, ok := seen[spec.TagID]; ok {
				continue
			}
			seen[spec.TagID] = struct{}{}
			var override *float64
			if spec.WeightOverride != nil {
				v := *spec.WeightOverride
				override = &v
			}
			rows = append(rows, model.OpenFlareNodeTagBinding{
				NodeID:         nodeID,
				TagID:          spec.TagID,
				WeightOverride: override,
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

// ListNodeTagSummariesByNodeIDs returns resolved tag summaries grouped by node ID.
func ListNodeTagSummariesByNodeIDs(ctx context.Context, nodeIDs []string) (map[string][]model.NodeTagSummary, error) {
	result := make(map[string][]model.NodeTagSummary, len(nodeIDs))
	if len(nodeIDs) == 0 {
		return result, nil
	}

	var rows []nodeTagBindingSummaryRow
	err := db.DB(ctx).
		Table("of_node_tag_bindings AS b").
		Select("b.node_id, b.tag_id, t.slug, t.name, t.color, t.default_weight, b.weight_override").
		Joins("JOIN of_node_tags AS t ON t.id = b.tag_id").
		Where("b.node_id IN ?", nodeIDs).
		Order("b.id ASC").
		Scan(&rows).Error
	if err != nil {
		if isMissingTableError(err) {
			return result, nil
		}
		return nil, err
	}

	for _, row := range rows {
		effective := row.DefaultWeight
		if row.WeightOverride != nil && *row.WeightOverride > 0 {
			effective = *row.WeightOverride
		}
		if effective <= 0 {
			effective = 1.0
		}
		var override *float64
		if row.WeightOverride != nil {
			v := *row.WeightOverride
			override = &v
		}
		result[row.NodeID] = append(result[row.NodeID], model.NodeTagSummary{
			TagID:           row.TagID,
			Slug:            row.Slug,
			Name:            row.Name,
			Color:           row.Color,
			DefaultWeight:   row.DefaultWeight,
			WeightOverride:  override,
			EffectiveWeight: effective,
		})
	}
	return result, nil
}
