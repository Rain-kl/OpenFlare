package model

import (
	"time"

	"github.com/rain-kl/openflare/openflare-server/utils"

	"gorm.io/gorm"
)

type NodeObservationFrps struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	NodeID          string    `json:"node_id" gorm:"index;size:64;not null"`
	CapturedAt      time.Time `json:"captured_at" gorm:"index"`
	FrpsConnections int       `json:"frps_connections"`
	FrpsProxyCount  int       `json:"frps_proxy_count"`
	FrpsClientCount int       `json:"frps_client_count"`
	FrpsProxies     string    `json:"frps_proxies" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at"`
}

func (obs *NodeObservationFrps) GetID() uint {
	return obs.ID
}

func (obs *NodeObservationFrps) GetTime() time.Time {
	return obs.CapturedAt
}

func (obs *NodeObservationFrps) BeforeCreate(tx *gorm.DB) error {
	return assignObservabilityID(&obs.ID)
}

func (obs *NodeObservationFrps) Insert() error {
	return DB.Create(obs).Error
}

func ListNodeObservationFrps(nodeID string, since time.Time, limit int) (observations []*NodeObservationFrps, err error) {
	rows, err := queryAcrossShards("node_observation_frps", func(tx *gorm.DB) ([]*NodeObservationFrps, error) {
		var shardRows []*NodeObservationFrps
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

func DeleteNodeObservationFrpsBefore(db *gorm.DB, before time.Time) (int64, error) {
	return deleteAcrossShards(db, "node_observation_frps", &NodeObservationFrps{}, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("captured_at < ?", before)
	})
}
