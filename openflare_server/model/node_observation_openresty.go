package model

import (
	"openflare/utils"
	"time"

	"gorm.io/gorm"
)

type NodeObservationOpenresty struct {
	ID                   uint      `json:"id" gorm:"primaryKey"`
	NodeID               string    `json:"node_id" gorm:"index;size:64;not null"`
	CapturedAt           time.Time `json:"captured_at" gorm:"index"`
	OpenrestyRxBytes     int64     `json:"openresty_rx_bytes"`
	OpenrestyTxBytes     int64     `json:"openresty_tx_bytes"`
	OpenrestyConnections int64     `json:"openresty_connections"`
	CreatedAt            time.Time `json:"created_at"`
}

func (obs *NodeObservationOpenresty) GetID() uint {
	return obs.ID
}

func (obs *NodeObservationOpenresty) GetTime() time.Time {
	return obs.CapturedAt
}

func (obs *NodeObservationOpenresty) BeforeCreate(tx *gorm.DB) error {
	return assignObservabilityID(&obs.ID)
}

func (obs *NodeObservationOpenresty) Insert() error {
	return DB.Create(obs).Error
}

func ListNodeObservationOpenresty(nodeID string, since time.Time, limit int) (observations []*NodeObservationOpenresty, err error) {
	rows, err := queryAcrossShards("node_observation_openresties", func(tx *gorm.DB) ([]*NodeObservationOpenresty, error) {
		var shardRows []*NodeObservationOpenresty
		query := tx.Order("captured_at desc, id desc")
		if nodeID != "" {
			query = query.Where("node_id = ?", nodeID)
		}
		if !since.IsZero() {
			query = query.Where("captured_at >= ?", since)
		}
		if err := query.Find(&shardRows).Error; err != nil {
			return nil, err
		}
		return shardRows, nil
	})
	if err != nil {
		return nil, err
	}
	return utils.SortAndLimitRecords(rows, limit), nil
}

func DeleteNodeObservationOpenrestiesBefore(db *gorm.DB, before time.Time) (int64, error) {
	return deleteAcrossShards(db, "node_observation_openresties", &NodeObservationOpenresty{}, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("captured_at < ?", before)
	})
}
