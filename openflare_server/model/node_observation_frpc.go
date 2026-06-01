package model

import (
	"openflare/utils"
	"time"

	"gorm.io/gorm"
)

type NodeObservationFrpc struct {
	ID                   uint      `json:"id" gorm:"primaryKey"`
	NodeID               string    `json:"node_id" gorm:"index;size:64;not null"`
	CapturedAt           time.Time `json:"captured_at" gorm:"index"`
	TunnelStatus         string    `json:"tunnel_status" gorm:"size:16"`
	ConnectedRelaysCount int       `json:"connected_relays_count"`
	CreatedAt            time.Time `json:"created_at"`
}

func (obs *NodeObservationFrpc) GetID() uint {
	return obs.ID
}

func (obs *NodeObservationFrpc) GetTime() time.Time {
	return obs.CapturedAt
}

func (obs *NodeObservationFrpc) BeforeCreate(tx *gorm.DB) error {
	return assignObservabilityID(&obs.ID)
}

func (obs *NodeObservationFrpc) Insert() error {
	return DB.Create(obs).Error
}

func ListNodeObservationFrpcs(nodeID string, since time.Time, limit int) (observations []*NodeObservationFrpc, err error) {
	rows, err := queryAcrossShards("node_observation_frpcs", func(tx *gorm.DB) ([]*NodeObservationFrpc, error) {
		var shardRows []*NodeObservationFrpc
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

func DeleteNodeObservationFrpcsBefore(db *gorm.DB, before time.Time) (int64, error) {
	return deleteAcrossShards(db, "node_observation_frpcs", &NodeObservationFrpc{}, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("captured_at < ?", before)
	})
}
