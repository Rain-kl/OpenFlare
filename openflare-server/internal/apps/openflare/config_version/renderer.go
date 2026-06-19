// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	openrestyrender "github.com/rain-kl/openflare/pkg/render/openresty"
)

// SupportFile is a rendered configuration support artifact.
type SupportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func renderSnapshotConfig(sourceJSON string, certificateFiles []SupportFile) (*openrestyrender.Result, error) {
	return openrestyrender.RenderJSON(sourceJSON, toOpenRestySupportFiles(certificateFiles))
}

func toOpenRestySupportFiles(files []SupportFile) []openrestyrender.SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]openrestyrender.SupportFile, 0, len(files))
	for _, file := range files {
		result = append(result, openrestyrender.SupportFile{
			Path:    file.Path,
			Content: file.Content,
		})
	}
	return result
}

func fromOpenRestySupportFiles(files []openrestyrender.SupportFile) []SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]SupportFile, 0, len(files))
	for _, file := range files {
		result = append(result, SupportFile{
			Path:    file.Path,
			Content: file.Content,
		})
	}
	return result
}

func renderPlaceholderConfig(snapshotJSON string) (mainConfig, routeConfig, checksum string) {
	mainConfig = `{"placeholder":"main_config"}`
	routeConfig = snapshotJSON
	checksum = openrestyrender.ChecksumBundle(mainConfig, routeConfig, nil)
	return mainConfig, routeConfig, checksum
}
