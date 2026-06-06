package model

import (
	"time"

	"gorm.io/gorm/clause"
)

type NodeSystemProfile struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	NodeID           string    `json:"node_id" gorm:"uniqueIndex;size:64;not null"`
	Hostname         string    `json:"hostname" gorm:"size:255"`
	OSName           string    `json:"os_name" gorm:"size:128"`
	OSVersion        string    `json:"os_version" gorm:"size:128"`
	KernelVersion    string    `json:"kernel_version" gorm:"size:128"`
	Architecture     string    `json:"architecture" gorm:"size:64"`
	CPUModel         string    `json:"cpu_model" gorm:"size:255"`
	CPUCores         int       `json:"cpu_cores"`
	TotalMemoryBytes int64     `json:"total_memory_bytes"`
	TotalDiskBytes   int64     `json:"total_disk_bytes"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	ReportedAt       time.Time `json:"reported_at" gorm:"index"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func GetNodeSystemProfile(nodeID string) (*NodeSystemProfile, error) {
	profile := &NodeSystemProfile{}
	err := DB.Where("node_id = ?", nodeID).First(profile).Error
	return profile, err
}

func UpsertNodeSystemProfile(profile *NodeSystemProfile) error {
	if profile == nil {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"hostname",
			"os_name",
			"os_version",
			"kernel_version",
			"architecture",
			"cpu_model",
			"cpu_cores",
			"total_memory_bytes",
			"total_disk_bytes",
			"uptime_seconds",
			"reported_at",
			"updated_at",
		}),
	}).Create(profile).Error
}
