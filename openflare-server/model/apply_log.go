package model

import (
	"time"

	"gorm.io/gorm"
)

type ApplyLogQuery struct {
	NodeID   string
	PageNo   int
	PageSize int
}

type ApplyLog struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	NodeID              string    `json:"node_id" gorm:"index;size:64;not null"`
	Version             string    `json:"version" gorm:"size:32;not null"`
	Result              string    `json:"result" gorm:"size:32;not null"`
	Message             string    `json:"message" gorm:"type:text"`
	Checksum            string    `json:"checksum" gorm:"size:64;not null;default:''"`
	MainConfigChecksum  string    `json:"main_config_checksum" gorm:"size:64;not null;default:''"`
	RouteConfigChecksum string    `json:"route_config_checksum" gorm:"size:64;not null;default:''"`
	SupportFileCount    int       `json:"support_file_count" gorm:"not null;default:0"`
	CreatedAt           time.Time `json:"created_at"`
}

func ListApplyLogs(query ApplyLogQuery) (logs []*ApplyLog, err error) {
	db := DB.Order("id desc")
	if query.NodeID != "" {
		db = db.Where("node_id = ?", query.NodeID)
	}
	if query.PageSize > 0 {
		offset := 0
		if query.PageNo > 1 {
			offset = (query.PageNo - 1) * query.PageSize
		}
		db = db.Limit(query.PageSize).Offset(offset)
	}
	err = db.Find(&logs).Error
	return logs, err
}

func CountApplyLogs(nodeID string) (total int64, err error) {
	query := DB.Model(&ApplyLog{})
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	err = query.Count(&total).Error
	return total, err
}

func GetLatestApplyLogsByNodeIDs(nodeIDs []string) (map[string]*ApplyLog, error) {
	result := make(map[string]*ApplyLog)
	if len(nodeIDs) == 0 {
		return result, nil
	}

	var logs []*ApplyLog
	subQuery := DB.Model(&ApplyLog{}).
		Select("MAX(id) AS id").
		Where("node_id IN ?", nodeIDs).
		Group("node_id")
	if err := DB.Where("id IN (?)", subQuery).Find(&logs).Error; err != nil {
		return nil, err
	}
	for _, log := range logs {
		result[log.NodeID] = log
	}
	return result, nil
}

func DeleteAllApplyLogs() (deleted int64, err error) {
	result := DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ApplyLog{})
	return result.RowsAffected, result.Error
}

func DeleteApplyLogsBefore(before time.Time) (deleted int64, err error) {
	result := DB.Where("created_at < ?", before).Delete(&ApplyLog{})
	return result.RowsAffected, result.Error
}
