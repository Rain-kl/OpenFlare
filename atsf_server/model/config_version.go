package model

import "time"

type ConfigVersion struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	Version          string    `json:"version" gorm:"uniqueIndex;size:32;not null"`
	SnapshotJSON     string    `json:"snapshot_json" gorm:"type:text;not null"`
	RenderedConfig   string    `json:"rendered_config" gorm:"type:text;not null"`
	SupportFilesJSON string    `json:"support_files_json" gorm:"type:text;not null;default:'[]'"`
	Checksum         string    `json:"checksum" gorm:"size:64;not null"`
	IsActive         bool      `json:"is_active" gorm:"not null;default:false;index"`
	CreatedBy        string    `json:"created_by" gorm:"size:64;not null"`
	CreatedAt        time.Time `json:"created_at"`
}

func ListConfigVersions() (versions []*ConfigVersion, err error) {
	err = DB.Order("id desc").Find(&versions).Error
	return versions, err
}

func GetConfigVersionByID(id uint) (*ConfigVersion, error) {
	version := &ConfigVersion{}
	err := DB.First(version, id).Error
	return version, err
}

func GetActiveConfigVersion() (*ConfigVersion, error) {
	version := &ConfigVersion{}
	err := DB.Where("is_active = ?", true).Order("id desc").First(version).Error
	return version, err
}
