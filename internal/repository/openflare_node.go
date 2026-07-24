// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	openFlareNodeStatusOnline   = "online"
	openFlareApplyResultSuccess = "success"
)

// ListOpenFlareNodes returns all nodes ordered by id desc.
func ListOpenFlareNodes(ctx context.Context) ([]model.OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var nodes []model.OpenFlareNode
	if err := conn.Order("id desc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// ListOpenFlareNodesByNodeIDs returns nodes matching the given node ids.
func ListOpenFlareNodesByNodeIDs(ctx context.Context, nodeIDs []string) ([]model.OpenFlareNode, error) {
	if len(nodeIDs) == 0 {
		return []model.OpenFlareNode{}, nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var nodes []model.OpenFlareNode
	if err := conn.Where("node_id IN ?", nodeIDs).Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// GetOpenFlareNodeByID returns a node by primary key.
func GetOpenFlareNodeByID(ctx context.Context, id uint) (*model.OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var node model.OpenFlareNode
	if err := conn.First(&node, id).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// GetOpenFlareNodeByNodeID returns a node by node_id.
func GetOpenFlareNodeByNodeID(ctx context.Context, nodeID string) (*model.OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var node model.OpenFlareNode
	if err := conn.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// GetOpenFlareNodeByAccessToken returns a node by access token.
func GetOpenFlareNodeByAccessToken(ctx context.Context, token string) (*model.OpenFlareNode, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}
	var node model.OpenFlareNode
	if err := conn.Where("access_token = ?", token).First(&node).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// CreateOpenFlareNode inserts a new node.
func CreateOpenFlareNode(ctx context.Context, node *model.OpenFlareNode) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(node).Error
}

// SaveOpenFlareNode persists node changes.
func SaveOpenFlareNode(ctx context.Context, node *model.OpenFlareNode) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Save(node).Error
}

// UpdateOpenFlareNodeFields updates selected columns for a node.
func UpdateOpenFlareNodeFields(ctx context.Context, node *model.OpenFlareNode, fields ...string) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	if len(fields) == 0 {
		return conn.Save(node).Error
	}
	return conn.Model(node).Select(fields).Updates(node).Error
}

// UpdateOpenFlareNodeColumns updates node columns from a map of column values.
// Empty maps are no-ops.
func UpdateOpenFlareNodeColumns(ctx context.Context, node *model.OpenFlareNode, changes map[string]any) error {
	if node == nil || len(changes) == 0 {
		return nil
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Model(node).Updates(changes).Error
}

// UpdateOpenFlareNodeFromApplyResult updates node status, last_seen, version and last_error after an apply report.
// When applyResult is "success", current_version is set and last_error is cleared; otherwise last_error is set to message.
func UpdateOpenFlareNodeFromApplyResult(ctx context.Context, nodeID, applyResult, version, message string, now time.Time) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return updateOpenFlareNodeFromApplyResultTx(conn, nodeID, applyResult, version, message, now)
}

func updateOpenFlareNodeFromApplyResultTx(tx *gorm.DB, nodeID, applyResult, version, message string, now time.Time) error {
	record := &model.OpenFlareNode{}
	if err := tx.Where("node_id = ?", nodeID).First(record).Error; err != nil {
		return err
	}
	record.Status = openFlareNodeStatusOnline
	lastSeen := now
	record.LastSeenAt = &lastSeen
	if applyResult == openFlareApplyResultSuccess {
		record.CurrentVersion = version
		record.LastError = ""
	} else {
		record.LastError = message
	}
	return tx.Model(record).Select("status", "last_seen_at", "current_version", "last_error").Updates(record).Error
}

// DeleteOpenFlareNode removes a node by primary key.
func DeleteOpenFlareNode(ctx context.Context, id uint) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Delete(&model.OpenFlareNode{}, id).Error
}
