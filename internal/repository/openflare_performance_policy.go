// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

const pingoraPolicyNodes = "engine = 'pingora' AND node_type = 'edge_node'"

// ErrInvalidPerformancePolicyNodes indicates that at least one target is not a Pingora edge node.
var ErrInvalidPerformancePolicyNodes = errors.New("all selected nodes must be Pingora edge nodes")

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
	return performancePolicyWrite(db.DB(ctx).Model(policy).Select("Name", "SettingsJSON", "UpdatedAt").Updates(policy))
}

// DeletePerformancePolicy removes a policy and clears its node assignments.
func DeletePerformancePolicy(ctx context.Context, id uint) error {
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := performancePolicyWrite(tx.Delete(&model.OpenFlarePerformancePolicy{}, id)); err != nil {
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
		policy := tx.Model(&model.OpenFlarePerformancePolicy{}).Where("id = ?", id)
		if err := performancePolicyWrite(policy.UpdateColumn("updated_at", gorm.Expr("updated_at"))); err != nil {
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

// GetNodePerformancePolicy reads the desired settings from the current assignment.
func GetNodePerformancePolicy(ctx context.Context, nodeID string) (string, error) {
	var policy model.OpenFlarePerformancePolicy
	err := db.DB(ctx).Model(&policy).Select("of_performance_policies.settings_json").
		Joins("JOIN of_nodes ON of_nodes.performance_policy_id = of_performance_policies.id").
		Where("of_nodes.node_id = ? AND of_nodes.engine = 'pingora' AND of_nodes.node_type = 'edge_node'", nodeID).
		Take(&policy).Error
	return policy.SettingsJSON, err
}
