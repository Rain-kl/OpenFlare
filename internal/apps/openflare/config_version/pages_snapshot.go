// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	openrestyrender "github.com/Rain-kl/Wavelet/pkg/render/openresty"
	"gorm.io/gorm"
)

const defaultPagesSnapshotEntryFile = "index.html"
const defaultPagesSnapshotFallbackPath = "/index.html"

func buildPagesRouteSnapshot(
	ctx context.Context,
	route *model.ProxyRoute,
) (originURL string, upstreams []string, pagesProjectID *uint, deployment *openrestyrender.PagesDeployment, err error) {
	if route == nil {
		return "", nil, nil, nil, errors.New("pages 路由配置无效")
	}
	if !model.HasPagesProjectsTable(ctx) {
		return "", nil, nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 模块不可用", route.SiteName)
	}
	if route.PagesProjectID == nil || *route.PagesProjectID == 0 {
		return "", nil, nil, nil, fmt.Errorf("路由 %s Pages 配置无效: 未绑定 Pages 项目", route.SiteName)
	}
	project, err := model.GetPagesProjectByID(ctx, *route.PagesProjectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 项目不存在", route.SiteName)
		}
		return "", nil, nil, nil, err
	}
	if !project.Enabled {
		return "", nil, nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 项目未启用", route.SiteName)
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID == 0 {
		return "", nil, nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 项目没有激活部署", route.SiteName)
	}
	activeDeployment, err := model.GetPagesDeploymentByID(ctx, *project.ActiveDeploymentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 激活部署不存在", route.SiteName)
		}
		return "", nil, nil, nil, err
	}
	if activeDeployment.ProjectID != project.ID {
		return "", nil, nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 激活部署不匹配", route.SiteName)
	}
	if strings.TrimSpace(activeDeployment.Checksum) == "" {
		return "", nil, nil, nil, fmt.Errorf("路由 %s Pages 配置无效: pages 部署校验和缺失", route.SiteName)
	}

	pagesProjectID = route.PagesProjectID
	deployment = buildSnapshotPagesDeployment(project, activeDeployment)
	originURL = fmt.Sprintf("openflare-pages://project/%d", project.ID)
	return originURL, []string{originURL}, pagesProjectID, deployment, nil
}

func buildSnapshotPagesDeployment(project *model.PagesProject, activeDeployment *model.PagesDeployment) *openrestyrender.PagesDeployment {
	if project == nil || activeDeployment == nil {
		return nil
	}
	entryFile := strings.TrimSpace(project.EntryFile)
	if entryFile == "" {
		entryFile = defaultPagesSnapshotEntryFile
	}
	fallbackPath := strings.TrimSpace(project.SPAFallbackPath)
	if fallbackPath == "" {
		fallbackPath = defaultPagesSnapshotFallbackPath
	}
	return &openrestyrender.PagesDeployment{
		ProjectID:          project.ID,
		ProjectSlug:        strings.TrimSpace(project.Slug),
		DeploymentID:       activeDeployment.ID,
		DeploymentNumber:   activeDeployment.DeploymentNumber,
		Checksum:           strings.TrimSpace(activeDeployment.Checksum),
		EntryFile:          entryFile,
		SPAFallbackEnabled: project.SPAFallbackEnabled,
		SPAFallbackPath:    fallbackPath,
		APIProxyEnabled:    project.APIProxyEnabled,
		APIProxyPath:       strings.TrimSpace(project.APIProxyPath),
		APIProxyPass:       strings.TrimSpace(project.APIProxyPass),
		APIProxyRewrite:    strings.TrimSpace(project.APIProxyRewrite),
		// Root is project-scoped so Agents can swap active packages without
		// re-publishing main config (nginx root stays stable).
		LocalRoot: openrestyrender.PagesProjectLocalRoot(project.ID),
	}
}
