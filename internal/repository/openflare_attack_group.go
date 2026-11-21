// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrAttackGroupReferenced indicates deletion would leave a dangling WAF reference.
var ErrAttackGroupReferenced = errors.New("attack protection group is referenced by a WAF flow")

// DeleteOpenFlareAttackGroup checks all flows while holding the same group lock as graph writes.
func DeleteOpenFlareAttackGroup(ctx context.Context, id uint, referenced func(string) (bool, error)) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		var group model.OpenFlareAttackGroup
		if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).Select("id").Take(&group, id).Error; err != nil {
			return err
		}
		var flows []model.OpenFlareWAFRuleGroup
		if err := tx.Select("graph").Find(&flows).Error; err != nil {
			return err
		}
		for _, flow := range flows {
			inUse, err := referenced(flow.Graph)
			if err != nil {
				return err
			}
			if inUse {
				return ErrAttackGroupReferenced
			}
		}
		return tx.Delete(&group).Error
	})
}

// ListOpenFlareAttackGroups returns all groups or only the supplied IDs.
func ListOpenFlareAttackGroups(ctx context.Context, ids []uint) ([]model.OpenFlareAttackGroup, error) {
	conn, err := wafDB(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]model.OpenFlareAttackGroup, 0)
	if len(ids) != 0 {
		conn = conn.Where("id IN ?", ids)
	}
	err = conn.Order("id ASC").Find(&groups).Error
	return groups, err
}

// SaveOpenFlareAttackGroup creates a group or updates one at the supplied ID.
func SaveOpenFlareAttackGroup(ctx context.Context, group *model.OpenFlareAttackGroup) error {
	conn, err := wafDB(ctx)
	if err != nil {
		return err
	}
	if group.ID == 0 {
		if group.RuleOverrides == nil {
			group.RuleOverrides = map[string]string{}
		}
		return conn.Create(group).Error
	}
	fields := []string{"Name", "Levels", "UpdatedAt"}
	if group.RuleOverrides != nil {
		fields = append(fields, "RuleOverrides")
	}
	result := conn.Model(group).Clauses(clause.Returning{}).Select(fields).Updates(group)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
