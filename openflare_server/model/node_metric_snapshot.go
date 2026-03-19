package model

import (
	"sort"
	"time"

	"gorm.io/gorm"
)

type NodeMetricSnapshot struct {
	ID                   uint      `json:"id" gorm:"primaryKey"`
	NodeID               string    `json:"node_id" gorm:"index;size:64;not null"`
	CapturedAt           time.Time `json:"captured_at" gorm:"index"`
	CPUUsagePercent      float64   `json:"cpu_usage_percent"`
	MemoryUsedBytes      int64     `json:"memory_used_bytes"`
	MemoryTotalBytes     int64     `json:"memory_total_bytes"`
	StorageUsedBytes     int64     `json:"storage_used_bytes"`
	StorageTotalBytes    int64     `json:"storage_total_bytes"`
	DiskReadBytes        int64     `json:"disk_read_bytes"`
	DiskWriteBytes       int64     `json:"disk_write_bytes"`
	NetworkRxBytes       int64     `json:"network_rx_bytes"`
	NetworkTxBytes       int64     `json:"network_tx_bytes"`
	OpenrestyRxBytes     int64     `json:"openresty_rx_bytes"`
	OpenrestyTxBytes     int64     `json:"openresty_tx_bytes"`
	OpenrestyConnections int64     `json:"openresty_connections"`
	CreatedAt            time.Time `json:"created_at"`
}

func (snapshot *NodeMetricSnapshot) BeforeCreate(tx *gorm.DB) error {
	return assignObservabilityID(&snapshot.ID)
}

func (snapshot *NodeMetricSnapshot) Insert() error {
	return DB.Create(snapshot).Error
}

func ListNodeMetricSnapshots(nodeID string, since time.Time, limit int) (snapshots []*NodeMetricSnapshot, err error) {
	rows, err := queryAcrossShards("node_metric_snapshots", func(tx *gorm.DB) ([]*NodeMetricSnapshot, error) {
		var shardRows []*NodeMetricSnapshot
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
	sort.Slice(rows, func(i int, j int) bool {
		if rows[i].CapturedAt.Equal(rows[j].CapturedAt) {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].CapturedAt.After(rows[j].CapturedAt)
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func ListMetricSnapshotsSince(since time.Time) (snapshots []*NodeMetricSnapshot, err error) {
	rows, err := queryAcrossShards("node_metric_snapshots", func(tx *gorm.DB) ([]*NodeMetricSnapshot, error) {
		var shardRows []*NodeMetricSnapshot
		query := tx.Order("captured_at desc")
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
	sort.Slice(rows, func(i int, j int) bool {
		if rows[i].CapturedAt.Equal(rows[j].CapturedAt) {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].CapturedAt.After(rows[j].CapturedAt)
	})
	return rows, nil
}

func NodeMetricSnapshotExists(db *gorm.DB, nodeID string, capturedAt time.Time) (bool, error) {
	db = normalizeShardedDB(db)
	for _, table := range observabilityShardTables("node_metric_snapshots") {
		var count int64
		if err := db.Table(table).
			Where("node_id = ? AND captured_at = ?", nodeID, capturedAt).
			Limit(1).
			Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func DeleteNodeMetricSnapshotsBefore(db *gorm.DB, before time.Time) (int64, error) {
	return deleteAcrossShards(db, "node_metric_snapshots", &NodeMetricSnapshot{}, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("captured_at < ?", before)
	})
}

func DeleteAllNodeMetricSnapshots(db *gorm.DB) (int64, error) {
	return deleteAcrossShards(db, "node_metric_snapshots", &NodeMetricSnapshot{}, nil)
}
