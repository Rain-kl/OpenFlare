package model

import "time"

type ApplyLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	NodeID    string    `json:"node_id" gorm:"index;size:64;not null"`
	Version   string    `json:"version" gorm:"size:32;not null"`
	Result    string    `json:"result" gorm:"size:32;not null"`
	Message   string    `json:"message" gorm:"size:1024"`
	CreatedAt time.Time `json:"created_at"`
}

func ListApplyLogs(nodeID string) (logs []*ApplyLog, err error) {
	query := DB.Order("id desc")
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	err = query.Find(&logs).Error
	return logs, err
}

func GetLatestApplyLog(nodeID string) (*ApplyLog, error) {
	log := &ApplyLog{}
	err := DB.Where("node_id = ?", nodeID).Order("id desc").First(log).Error
	return log, err
}
