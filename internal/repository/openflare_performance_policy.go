// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	pingoraPolicyNodes = "engine = 'pingora' AND node_type = 'edge_node'"
	assignablePolicy   = "(preset_key IS NULL OR preset_key <> ?)"
)

// ErrInvalidPerformancePolicyNodes indicates that at least one target is not a Pingora edge node.
var ErrInvalidPerformancePolicyNodes = errors.New("all selected nodes must be Pingora edge nodes")

// ErrInheritedPerformancePolicy protects the system policy from deletion and explicit assignment.
var ErrInheritedPerformancePolicy = errors.New("the default performance policy is inherited automatically")

// ListPerformancePolicies returns the policies and eligible edge nodes.
func ListPerformancePolicies(ctx context.Context) ([]model.OpenFlarePerformancePolicy, []model.PerformancePolicyNode, error) {
	policies := make([]model.OpenFlarePerformancePolicy, 0)
	nodes := make([]model.PerformancePolicyNode, 0)
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Order("id").Find(&policies).Error; err != nil {
			return err
		}
		return tx.Model(&model.OpenFlareNode{}).
			Select("node_id, name, status, performance_policy_id AS policy_id").
			Where(pingoraPolicyNodes).Order("name, node_id").Find(&nodes).Error
	})
	return policies, nodes, err
}

// CreatePerformancePolicy stores a new policy.
func CreatePerformancePolicy(ctx context.Context, policy *model.OpenFlarePerformancePolicy) error {
	return db.DB(ctx).Create(policy).Error
}

// UpdatePerformancePolicy modifies an existing policy.
func UpdatePerformancePolicy(ctx context.Context, policy *model.OpenFlarePerformancePolicy) error {
	return performancePolicyWrite(db.DB(ctx).Model(policy).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "preset_key"}}}).
		Select("Name", "SettingsJSON", "UpdatedAt").Updates(policy))
}

// EditPerformanceSettings reads and updates one policy under its write lock.
func EditPerformanceSettings(ctx context.Context, id uint, edit func(string) (string, error)) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var policy model.OpenFlarePerformancePolicy
		if err := tx.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).First(&policy, id).Error; err != nil {
			return err
		}
		next, err := edit(policy.SettingsJSON)
		if err != nil {
			return err
		}
		return tx.Model(&policy).Update("settings_json", next).Error
	})
}

// DeletePerformancePolicy removes a policy and clears its node assignments.
func DeletePerformancePolicy(ctx context.Context, id uint) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where(assignablePolicy, model.DefaultPerformancePolicyKey).
			Delete(&model.OpenFlarePerformancePolicy{}, id)
		if err := explicitPolicyWrite(tx, id, result); err != nil {
			return err
		}
		return tx.Table((model.OpenFlareNode{}).TableName()).Where("performance_policy_id = ?", id).
			UpdateColumn("performance_policy_id", nil).Error
	})
}

// AssignPerformancePolicy replaces the set of nodes assigned to one policy atomically.
func AssignPerformancePolicy(ctx context.Context, id uint, nodeIDs []string) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		// Acquire the same write lock used by updates and deletion before touching assignments.
		policy := tx.Model(&model.OpenFlarePerformancePolicy{}).Where("id = ?", id).
			Where(assignablePolicy, model.DefaultPerformancePolicyKey)
		if err := explicitPolicyWrite(tx, id, policy.UpdateColumn("updated_at", gorm.Expr("updated_at"))); err != nil {
			return err
		}
		nodes := tx.Table((model.OpenFlareNode{}).TableName()).Where("performance_policy_id = ?", id)
		if len(nodeIDs) > 0 {
			nodes = nodes.Where("node_id NOT IN ?", nodeIDs)
		}
		if err := nodes.UpdateColumn("performance_policy_id", nil).Error; err != nil {
			return err
		}
		if len(nodeIDs) == 0 {
			return nil
		}
		result := tx.Table((model.OpenFlareNode{}).TableName()).Where(pingoraPolicyNodes).
			Where("node_id IN ?", nodeIDs).UpdateColumn("performance_policy_id", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(nodeIDs)) {
			return ErrInvalidPerformancePolicyNodes
		}
		return nil
	})
}

func performancePolicyWrite(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func explicitPolicyWrite(tx *gorm.DB, id uint, result *gorm.DB) error {
	if err := performancePolicyWrite(result); !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var policy model.OpenFlarePerformancePolicy
	if err := tx.Select("preset_key").Take(&policy, id).Error; err != nil {
		return err
	}
	if policy.IsDefault() {
		return ErrInheritedPerformancePolicy
	}
	return gorm.ErrRecordNotFound
}

// GetNodePerformancePolicy reads a node's explicit policy or the system default.
func GetNodePerformancePolicy(ctx context.Context, nodeID string) (string, error) {
	var policy model.OpenFlarePerformancePolicy
	err := db.DB(ctx).Table("of_nodes AS node").
		Select("COALESCE(assigned.settings_json, inherited.settings_json) AS settings_json").
		Joins("LEFT JOIN of_performance_policies AS assigned ON assigned.id = node.performance_policy_id").
		Joins("LEFT JOIN of_performance_policies AS inherited ON inherited.preset_key = ?", model.DefaultPerformancePolicyKey).
		Where("node.node_id = ? AND node.engine = 'pingora' AND node.node_type = 'edge_node'", nodeID).
		Where("assigned.id IS NOT NULL OR inherited.id IS NOT NULL").
		Take(&policy).Error
	return policy.SettingsJSON, err
}
