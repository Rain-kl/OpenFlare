// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
	"gorm.io/gorm"
)

// RebindSnapshotPagesToCurrentActive rewrites pages_deployment fields so every
// pages route points at the project's current active deployment.
//
// Main config versions and Pages deployments are independent. Rolling back a
// main config version must not require old Pages packages; Agents always follow
// the live active deployment for each referenced project.
//
// Returns the original JSON unchanged when there are no pages routes. Does not
// mutate stored config_versions rows. Non-pages route fields are preserved.
func RebindSnapshotPagesToCurrentActive(ctx context.Context, snapshotJSON string) (string, error) {
	text := strings.TrimSpace(snapshotJSON)
	if text == "" {
		return snapshotJSON, nil
	}

	if strings.HasPrefix(text, "[") {
		var routes []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(text), &routes); err != nil {
			return "", fmt.Errorf("parse pages snapshot routes: %w", err)
		}
		changed, err := rebindPagesRouteMaps(ctx, routes)
		if err != nil {
			return "", err
		}
		if !changed {
			return snapshotJSON, nil
		}
		encoded, err := json.Marshal(routes)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return "", fmt.Errorf("parse pages snapshot document: %w", err)
	}
	routesRaw, ok := raw["routes"]
	if !ok || len(routesRaw) == 0 {
		return snapshotJSON, nil
	}
	var routes []map[string]json.RawMessage
	if err := json.Unmarshal(routesRaw, &routes); err != nil {
		return "", fmt.Errorf("parse pages snapshot routes: %w", err)
	}
	changed, err := rebindPagesRouteMaps(ctx, routes)
	if err != nil {
		return "", err
	}
	if !changed {
		return snapshotJSON, nil
	}
	encodedRoutes, err := json.Marshal(routes)
	if err != nil {
		return "", err
	}
	raw["routes"] = encodedRoutes
	encoded, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func rebindPagesRouteMaps(ctx context.Context, routes []map[string]json.RawMessage) (bool, error) {
	changed := false
	for index := range routes {
		route := routes[index]
		if route == nil {
			continue
		}
		upstreamType, _ := rawJSONString(route["upstream_type"])
		if !strings.EqualFold(strings.TrimSpace(upstreamType), "pages") {
			continue
		}
		siteName, _ := rawJSONString(route["site_name"])
		projectID, err := resolveProjectIDFromRouteMap(route)
		if err != nil {
			if siteName == "" {
				siteName = "pages"
			}
			return false, fmt.Errorf("路由 %s %w", siteName, err)
		}
		project, activeDeployment, err := loadActivePagesProject(ctx, projectID, siteName)
		if err != nil {
			return false, err
		}
		deployment := buildLivePagesDeployment(project, activeDeployment)
		projectIDCopy := project.ID
		originURL := fmt.Sprintf("openflare-pages://project/%d", project.ID)

		if err := putJSON(route, "pages_project_id", projectIDCopy); err != nil {
			return false, err
		}
		if err := putJSON(route, "pages_deployment", deployment); err != nil {
			return false, err
		}
		if err := putJSON(route, "origin_url", originURL); err != nil {
			return false, err
		}
		if err := putJSON(route, "upstreams", []string{originURL}); err != nil {
			return false, err
		}
		routes[index] = route
		changed = true
	}
	return changed, nil
}

const jsonNullLiteral = "null"

func isPresentJSON(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != jsonNullLiteral
}

func resolveProjectIDFromRouteMap(route map[string]json.RawMessage) (uint, error) {
	if raw, ok := route["pages_project_id"]; ok && isPresentJSON(raw) {
		var projectID uint
		if err := json.Unmarshal(raw, &projectID); err == nil && projectID != 0 {
			return projectID, nil
		}
	}
	if raw, ok := route["pages_deployment"]; ok && isPresentJSON(raw) {
		var deployment struct {
			ProjectID uint `json:"project_id"`
		}
		if err := json.Unmarshal(raw, &deployment); err == nil && deployment.ProjectID != 0 {
			return deployment.ProjectID, nil
		}
	}
	return 0, fmt.Errorf("pages 配置无效: 缺少 pages_project_id")
}

func loadActivePagesProject(ctx context.Context, projectID uint, siteName string) (*model.PagesProject, *model.PagesDeployment, error) {
	if siteName == "" {
		siteName = "pages"
	}
	project, err := model.GetPagesProjectByID(ctx, projectID)
	if err != nil {
		if errorsIsNotFound(err) {
			return nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 项目不存在", siteName)
		}
		return nil, nil, err
	}
	if !project.Enabled {
		return nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 项目未启用", siteName)
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID == 0 {
		return nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 项目没有激活部署", siteName)
	}
	activeDeployment, err := model.GetPagesDeploymentByID(ctx, *project.ActiveDeploymentID)
	if err != nil {
		if errorsIsNotFound(err) {
			return nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 激活部署不存在", siteName)
		}
		return nil, nil, err
	}
	if activeDeployment.ProjectID != project.ID {
		return nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 激活部署不匹配", siteName)
	}
	if strings.TrimSpace(activeDeployment.Checksum) == "" {
		return nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 部署校验和缺失", siteName)
	}
	return project, activeDeployment, nil
}

func buildLivePagesDeployment(project *model.PagesProject, active *model.PagesDeployment) *openrestyrender.PagesDeployment {
	entryFile := strings.TrimSpace(project.EntryFile)
	if entryFile == "" {
		entryFile = defaultPagesEntryFile
	}
	fallbackPath := strings.TrimSpace(project.SPAFallbackPath)
	if fallbackPath == "" {
		fallbackPath = defaultPagesFallbackPath
	}
	return &openrestyrender.PagesDeployment{
		ProjectID:          project.ID,
		ProjectSlug:        strings.TrimSpace(project.Slug),
		DeploymentID:       active.ID,
		DeploymentNumber:   active.DeploymentNumber,
		Checksum:           strings.TrimSpace(active.Checksum),
		EntryFile:          entryFile,
		SPAFallbackEnabled: project.SPAFallbackEnabled,
		SPAFallbackPath:    fallbackPath,
		APIProxyEnabled:    project.APIProxyEnabled,
		APIProxyPath:       strings.TrimSpace(project.APIProxyPath),
		APIProxyPass:       strings.TrimSpace(project.APIProxyPass),
		APIProxyRewrite:    strings.TrimSpace(project.APIProxyRewrite),
		LocalRoot:          openrestyrender.PagesProjectLocalRoot(project.ID),
	}
}

func rawJSONString(raw json.RawMessage) (string, bool) {
	if !isPresentJSON(raw) {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func putJSON(route map[string]json.RawMessage, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	route[key] = encoded
	return nil
}

func errorsIsNotFound(err error) bool {
	return err != nil && (err == gorm.ErrRecordNotFound || strings.Contains(strings.ToLower(err.Error()), "record not found"))
}
