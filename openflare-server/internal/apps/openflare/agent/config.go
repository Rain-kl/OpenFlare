// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	openrestyrender "github.com/rain-kl/openflare/pkg/render/openresty"
	"gorm.io/gorm"
)

type configVersionRecord struct {
	ID               uint      `gorm:"primaryKey"`
	Version          string    `gorm:"column:version"`
	SnapshotJSON     string    `gorm:"column:snapshot_json"`
	SupportFilesJSON string    `gorm:"column:support_files_json"`
	Checksum         string    `gorm:"column:checksum"`
	IsActive         bool      `gorm:"column:is_active"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (configVersionRecord) TableName() string {
	return "of_config_versions"
}

func getActiveConfigMeta(ctx context.Context) (*ActiveConfigMeta, error) {
	version, err := loadActiveConfigVersion(ctx)
	if err != nil {
		return nil, err
	}
	return &ActiveConfigMeta{
		Version:  version.Version,
		Checksum: version.Checksum,
	}, nil
}

func getActiveConfigForAgent(ctx context.Context) (*ConfigResponse, error) {
	version, err := loadActiveConfigVersion(ctx)
	if err != nil {
		return nil, err
	}

	var supportFiles []SupportFile
	if strings.TrimSpace(version.SupportFilesJSON) != "" {
		if err = json.Unmarshal([]byte(version.SupportFilesJSON), &supportFiles); err != nil {
			return nil, err
		}
	}

	return &ConfigResponse{
		Version:          version.Version,
		Checksum:         version.Checksum,
		SourceConfigJSON: version.SnapshotJSON,
		SupportFiles:     sourceSupportFiles(supportFiles),
		CreatedAt:        version.CreatedAt,
	}, nil
}

func loadActiveConfigVersion(ctx context.Context) (*configVersionRecord, error) {
	conn := db.DB(ctx)
	if conn == nil {
		return nil, errors.New("database not initialized")
	}
	version := &configVersionRecord{}
	err := conn.Where("is_active = ?", true).Order("id desc").First(version).Error
	if err != nil {
		return nil, err
	}
	return version, nil
}

func sourceSupportFiles(files []SupportFile) []SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]SupportFile, 0, len(files))
	for _, file := range files {
		if isRuntimeGeneratedSupportFile(file.Path) {
			continue
		}
		result = append(result, file)
	}
	return result
}

func isRuntimeGeneratedSupportFile(path string) bool {
	switch strings.TrimSpace(path) {
	case "pow_config.json", "waf_config.json", openrestyrender.SourceConfigFileName:
		return true
	default:
		return false
	}
}

func isActiveConfigNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
