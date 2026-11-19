// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pages"
	openrestyrender "github.com/Rain-kl/Wavelet/internal/openflare/openresty"
	"gorm.io/gorm"
)

func getActiveConfigMetaByEngine(ctx context.Context, engine string) (*protocol.ActiveConfigMeta, error) {
	version, err := repository.GetActiveConfigVersionByEngine(ctx, engine)
	if err != nil {
		return nil, err
	}
	return &protocol.ActiveConfigMeta{
		Version:         version.Version,
		Checksum:        version.Checksum,
		Engine:          version.Engine,
		ReleaseSequence: version.ReleaseSequence,
		PayloadChecksum: version.PayloadChecksum,
	}, nil
}

// GetActivePingoraConfig returns the dedicated active Pingora configuration.
func GetActivePingoraConfig(ctx context.Context) (*PingoraConfigResponse, error) {
	version, err := repository.GetActivePingoraConfigVersion(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(version.PayloadJSON) == "" || strings.TrimSpace(version.PayloadChecksum) == "" {
		return nil, errors.New(errNoActiveConfig)
	}
	return &PingoraConfigResponse{
		Version:         version.Version,
		Checksum:        version.Checksum,
		Engine:          version.Engine,
		ReleaseSequence: version.ReleaseSequence,
		PayloadJSON:     version.PayloadJSON,
		PayloadChecksum: version.PayloadChecksum,
		CreatedAt:       version.CreatedAt,
	}, nil
}

func getActiveConfigForAgent(ctx context.Context, siteAuthSupported bool) (*ConfigResponse, error) {
	version, err := repository.GetActiveConfigVersion(ctx)
	if err != nil {
		return nil, err
	}

	var supportFiles []protocol.SupportFile
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

	if !siteAuthSupported {
		sourceJSON, err = legacyAgentSiteAuthConfig(sourceJSON)
		if err != nil {
			return nil, err
		}
	}

	return &ConfigResponse{
		Version:          version.Version,
		Checksum:         version.Checksum,
		SourceConfigJSON: sourceJSON,
		SupportFiles:     sourceSupportFiles(supportFiles),
		CreatedAt:        version.CreatedAt,
	}, nil
}

// Preserve the site access restriction as Basic Auth on Agents that cannot
// render the HTML login mode. Legacy Pages API locations lack that guard.
func legacyAgentSiteAuthConfig(sourceJSON string) (string, error) {
	if !strings.Contains(sourceJSON, `"site_auth_ttl_hours"`) {
		return sourceJSON, nil
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(sourceJSON), &document); err != nil {
		return "", err
	}
	if document["routes"] == nil {
		return sourceJSON, nil
	}
	var routes []map[string]json.RawMessage
	if err := json.Unmarshal(document["routes"], &routes); err != nil {
		return "", err
	}
	for _, route := range routes {
		if route["site_auth_ttl_hours"] == nil {
			continue
		}
		route["basic_auth_enabled"] = json.RawMessage("true")
		route["redirect_http"] = json.RawMessage("true")
		delete(route, "site_auth_ttl_hours")
		delete(route, "site_auth_key")
		if err := disableLegacyPagesAPIProxy(route); err != nil {
			return "", err
		}
	}
	encodedRoutes, err := json.Marshal(routes)
	if err != nil {
		return "", err
	}
	document["routes"] = encodedRoutes
	encoded, err := json.Marshal(document)
	return string(encoded), err
}

func disableLegacyPagesAPIProxy(route map[string]json.RawMessage) error {
	if route["pages_deployment"] == nil {
		return nil
	}
	var deployment map[string]json.RawMessage
	if err := json.Unmarshal(route["pages_deployment"], &deployment); err != nil {
		return err
	}
	if deployment == nil {
		return nil
	}
	deployment["api_proxy_enabled"] = json.RawMessage("false")
	encoded, err := json.Marshal(deployment)
	if err != nil {
		return err
	}
	route["pages_deployment"] = encoded
	return nil
}

func sourceSupportFiles(files []protocol.SupportFile) []protocol.SupportFile {
	if len(files) == 0 {
		return nil
	}
	result := make([]protocol.SupportFile, 0, len(files))
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
