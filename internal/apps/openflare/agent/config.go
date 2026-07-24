// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pages"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
	"gorm.io/gorm"
)

func getActiveConfigMeta(ctx context.Context) (*ActiveConfigMeta, error) {
	version, err := repository.GetActiveConfigVersion(ctx)
	if err != nil {
		return nil, err
	}
	return &ActiveConfigMeta{
		Version:  version.Version,
		Checksum: version.Checksum,
	}, nil
}

func getActiveConfigForAgent(ctx context.Context) (*ConfigResponse, error) {
	version, err := repository.GetActiveConfigVersion(ctx)
	if err != nil {
		return nil, err
	}

	var supportFiles []SupportFile
	if strings.TrimSpace(version.SupportFilesJSON) != "" {
		if err = json.Unmarshal([]byte(version.SupportFilesJSON), &supportFiles); err != nil {
			return nil, err
		}
	}

	// Main config version history is independent of Pages deployment history.
	// Agents always receive pages routes bound to each project's current active
	// deployment so config rollback never depends on pruned packages.
	sourceJSON := version.SnapshotJSON
	if rebound, rebindErr := pages.RebindSnapshotPagesToCurrentActive(ctx, version.SnapshotJSON); rebindErr != nil {
		return nil, rebindErr
	} else if strings.TrimSpace(rebound) != "" {
		sourceJSON = rebound
	}

	return &ConfigResponse{
		Version:          version.Version,
		Checksum:         version.Checksum,
		SourceConfigJSON: sourceJSON,
		SupportFiles:     sourceSupportFiles(supportFiles),
		CreatedAt:        version.CreatedAt,
	}, nil
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
	case "pow_config.json", "waf_config.json", openrestyrender.SourceConfigFileName: // pow_config.json is legacy; waf_config.json is canonical
		return true
	default:
		return false
	}
}

func isActiveConfigNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
