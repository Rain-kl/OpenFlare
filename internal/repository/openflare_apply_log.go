// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

// ListOpenFlareApplyLogs returns apply logs ordered by id desc with optional pagination.
func ListOpenFlareApplyLogs(ctx context.Context, query model.OpenFlareApplyLogQuery) ([]*model.OpenFlareApplyLog, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	dbQuery := conn.Model(&model.OpenFlareApplyLog{}).Order("id desc")
	if query.NodeID != "" {
		dbQuery = dbQuery.Where("node_id = ?", query.NodeID)
	}
	if query.PageSize > 0 {
		offset := 0
		if query.PageNo > 1 {
			offset = (query.PageNo - 1) * query.PageSize
		}
		dbQuery = dbQuery.Limit(query.PageSize).Offset(offset)
	}

	var logs []*model.OpenFlareApplyLog
	if err := dbQuery.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// CountOpenFlareApplyLogs returns total apply logs, optionally filtered by node_id.
func CountOpenFlareApplyLogs(ctx context.Context, nodeID string) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}

	query := conn.Model(&model.OpenFlareApplyLog{})
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// GetLatestOpenFlareApplyLogByNodeID returns the most recent apply log for a node.
func GetLatestOpenFlareApplyLogByNodeID(ctx context.Context, nodeID string) (*model.OpenFlareApplyLog, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, errors.New("node_id is required")
	}

	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var log model.OpenFlareApplyLog
	err := conn.Where("node_id = ?", nodeID).Order("id desc").First(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// GetLatestOpenFlareApplyLogsByNodeIDs returns the latest apply log per node id.
func GetLatestOpenFlareApplyLogsByNodeIDs(ctx context.Context, nodeIDs []string) (map[string]*model.OpenFlareApplyLog, error) {
	result := make(map[string]*model.OpenFlareApplyLog)
	if len(nodeIDs) == 0 {
		return result, nil
	}

	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var logs []*model.OpenFlareApplyLog
	subQuery := conn.Model(&model.OpenFlareApplyLog{}).
		Select("MAX(id) AS id").
		Where("node_id IN ?", nodeIDs).
		Group("node_id")
	if err := conn.Where("id IN (?)", subQuery).Find(&logs).Error; err != nil {
		return nil, err
	}
	for _, log := range logs {
		result[log.NodeID] = log
	}
	return result, nil
}

// CreateOpenFlareApplyLog inserts an apply log row.
func CreateOpenFlareApplyLog(ctx context.Context, log *model.OpenFlareApplyLog) error {
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	return conn.Create(log).Error
}

// CreateOpenFlareApplyLogAndUpdateNode creates an apply log and updates the node from the apply result in one transaction.
func CreateOpenFlareApplyLogAndUpdateNode(ctx context.Context, log *model.OpenFlareApplyLog, applyResult, version, message string) error {
	if log == nil {
		return errors.New("apply log is required")
	}
	conn := db.DB(ctx)
	if conn == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	now := log.CreatedAt
	if now.IsZero() {
		now = time.Now()
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		return updateOpenFlareNodeFromApplyResultTx(tx, log.NodeID, applyResult, version, message, now)
	})
}

// DeleteAllOpenFlareApplyLogs removes every apply log record.
func DeleteAllOpenFlareApplyLogs(ctx context.Context) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}

	result := conn.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.OpenFlareApplyLog{})
	return result.RowsAffected, result.Error
}

// DeleteOpenFlareApplyLogsBefore removes apply logs created before the cutoff time.
func DeleteOpenFlareApplyLogsBefore(ctx context.Context, before time.Time) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}

	result := conn.Where("created_at < ?", before).Delete(&model.OpenFlareApplyLog{})
	return result.RowsAffected, result.Error
}
