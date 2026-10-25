// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_version

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
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
		return "", nil, nil, nil, errors.New("invalid Pages route configuration")
	}
	if !repository.HasPagesProjectsTable(ctx) {
		return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: Pages module is unavailable", route.SiteName)
	}
	if route.PagesProjectID == nil || *route.PagesProjectID == 0 {
		return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: no Pages project is bound", route.SiteName)
	}
	project, err := repository.GetPagesProjectByID(ctx, *route.PagesProjectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: Pages project was not found", route.SiteName)
		}
		return "", nil, nil, nil, err
	}
	if !project.Enabled {
		return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: Pages project is disabled", route.SiteName)
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID == 0 {
		return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: Pages project has no active deployment", route.SiteName)
	}
	activeDeployment, err := repository.GetPagesDeploymentByID(ctx, *project.ActiveDeploymentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: active Pages deployment was not found", route.SiteName)
		}
		return "", nil, nil, nil, err
	}
	if activeDeployment.ProjectID != project.ID {
		return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: active Pages deployment does not match", route.SiteName)
	}
	if strings.TrimSpace(activeDeployment.Checksum) == "" {
		return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: Pages deployment checksum is missing", route.SiteName)
	}

	pagesProjectID = route.PagesProjectID
	deployment, err = buildSnapshotPagesDeployment(project, activeDeployment)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("invalid Pages configuration for route %s: %w", route.SiteName, err)
	}
	originURL = fmt.Sprintf("openflare-pages://project/%d", project.ID)
	return originURL, []string{originURL}, pagesProjectID, deployment, nil
}

func buildSnapshotPagesDeployment(
	project *model.PagesProject,
	activeDeployment *model.PagesDeployment,
) (*openrestyrender.PagesDeployment, error) {
	if project == nil || activeDeployment == nil {
		return nil, errors.New("pages project or deployment is empty")
	}
	rootDir, err := pagesarchive.NormalizeLogicalPath(strings.TrimSpace(project.RootDir), true)
	if err != nil {
		return nil, fmt.Errorf("invalid Pages root directory: %w", err)
	}
	entryFile := strings.TrimSpace(project.EntryFile)
	if entryFile == "" {
		entryFile = defaultPagesSnapshotEntryFile
	}
	entryFile, err = pagesarchive.NormalizeLogicalPath(entryFile, false)
	if err != nil {
		return nil, fmt.Errorf("invalid Pages entry file: %w", err)
	}
	fallbackPath := strings.TrimSpace(project.SPAFallbackPath)
	if fallbackPath == "" {
		fallbackPath = defaultPagesSnapshotFallbackPath
	}
	localRoot := openrestyrender.PagesProjectLocalRoot(project.ID)
	if rootDir != "" {
		localRoot = path.Join(localRoot, rootDir)
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
		LocalRoot: localRoot,
	}, nil
}
