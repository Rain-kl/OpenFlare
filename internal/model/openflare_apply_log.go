// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"errors"
	"strings"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"gorm.io/gorm"
)

// OpenFlareApplyLogQuery filters apply logs for list queries.
type OpenFlareApplyLogQuery struct {
	NodeID   string
	PageNo   int
	PageSize int
}

// OpenFlareApplyLog stores node configuration apply results.
type OpenFlareApplyLog struct {
	ID                  uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID              string    `json:"node_id" gorm:"index;size:64;not null"`
	Version             string    `json:"version" gorm:"size:32;not null"`
	Result              string    `json:"result" gorm:"size:32;not null"`
	Message             string    `json:"message" gorm:"type:text"`
	Checksum            string    `json:"checksum" gorm:"size:64;not null;default:''"`
	MainConfigChecksum  string    `json:"main_config_checksum" gorm:"size:64;not null;default:''"`
	RouteConfigChecksum string    `json:"route_config_checksum" gorm:"size:64;not null;default:''"`
	SupportFileCount    int       `json:"support_file_count" gorm:"not null;default:0"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

// TableName returns the GORM table name.
func (OpenFlareApplyLog) TableName() string {
	return "of_apply_logs"
}

// ListOpenFlareApplyLogs returns apply logs ordered by id desc with optional pagination.
func ListOpenFlareApplyLogs(ctx context.Context, query OpenFlareApplyLogQuery) ([]*OpenFlareApplyLog, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	dbQuery := conn.Model(&OpenFlareApplyLog{}).Order("id desc")
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

	var logs []*OpenFlareApplyLog
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

	query := conn.Model(&OpenFlareApplyLog{})
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
func GetLatestOpenFlareApplyLogByNodeID(ctx context.Context, nodeID string) (*OpenFlareApplyLog, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, errors.New("node_id is required")
	}

	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var log OpenFlareApplyLog
	err := conn.Where("node_id = ?", nodeID).Order("id desc").First(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// IsRepeatSuccessApplyLog reports whether the payload repeats an already-recorded success entry.
func IsRepeatSuccessApplyLog(latest *OpenFlareApplyLog, version, checksum, result string) bool {
	if latest == nil || result != "success" {
		return false
	}
	return latest.Result == "success" &&
		strings.TrimSpace(latest.Version) == strings.TrimSpace(version) &&
		strings.TrimSpace(latest.Checksum) == strings.TrimSpace(checksum)
}

// GetLatestOpenFlareApplyLogsByNodeIDs returns the latest apply log per node id.
func GetLatestOpenFlareApplyLogsByNodeIDs(ctx context.Context, nodeIDs []string) (map[string]*OpenFlareApplyLog, error) {
	result := make(map[string]*OpenFlareApplyLog)
	if len(nodeIDs) == 0 {
		return result, nil
	}

	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var logs []*OpenFlareApplyLog
	subQuery := conn.Model(&OpenFlareApplyLog{}).
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

// DeleteAllOpenFlareApplyLogs removes every apply log record.
func DeleteAllOpenFlareApplyLogs(ctx context.Context) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}

	result := conn.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&OpenFlareApplyLog{})
	return result.RowsAffected, result.Error
}

// DeleteOpenFlareApplyLogsBefore removes apply logs created before the cutoff time.
func DeleteOpenFlareApplyLogsBefore(ctx context.Context, before time.Time) (int64, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return 0, errors.New(errDatabaseNotInitialized)
	}

	result := conn.Where("created_at < ?", before).Delete(&OpenFlareApplyLog{})
	return result.RowsAffected, result.Error
}
