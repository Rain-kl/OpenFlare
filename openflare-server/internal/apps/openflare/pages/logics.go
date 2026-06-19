// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	uploadstorage "github.com/Rain-kl/Wavelet/internal/apps/upload/storage"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/storage"
	"gorm.io/gorm"
)

// Input Pages 项目创建/更新请求。
type Input struct {
	Name               string `json:"name"`
	Slug               string `json:"slug"`
	Description        string `json:"description"`
	Enabled            bool   `json:"enabled"`
	SPAFallbackEnabled bool   `json:"spa_fallback_enabled"`
	SPAFallbackPath    string `json:"spa_fallback_path"`
	APIProxyEnabled    bool   `json:"api_proxy_enabled"`
	APIProxyPath       string `json:"api_proxy_path"`
	APIProxyPass       string `json:"api_proxy_pass"`
	APIProxyRewrite    string `json:"api_proxy_rewrite"`
	RootDir            string `json:"root_dir"`
	EntryFile          string `json:"entry_file"`
}

// DeploymentView Pages 部署视图。
type DeploymentView struct {
	ID               uint       `json:"id"`
	ProjectID        uint       `json:"project_id"`
	DeploymentNumber int        `json:"deployment_number"`
	Checksum         string     `json:"checksum"`
	Status           string     `json:"status"`
	FileCount        int        `json:"file_count"`
	TotalSize        int64      `json:"total_size"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	ActivatedAt      *time.Time `json:"activated_at"`
}

// DeploymentFileView Pages 部署文件视图。
type DeploymentFileView struct {
	ID           uint      `json:"id"`
	DeploymentID uint      `json:"deployment_id"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Checksum     string    `json:"checksum"`
	CreatedAt    time.Time `json:"created_at"`
}

// View Pages 项目视图。
type View struct {
	ID                 uint            `json:"id"`
	Name               string          `json:"name"`
	Slug               string          `json:"slug"`
	Description        string          `json:"description"`
	Enabled            bool            `json:"enabled"`
	SPAFallbackEnabled bool            `json:"spa_fallback_enabled"`
	SPAFallbackPath    string          `json:"spa_fallback_path"`
	APIProxyEnabled    bool            `json:"api_proxy_enabled"`
	APIProxyPath       string          `json:"api_proxy_path"`
	APIProxyPass       string          `json:"api_proxy_pass"`
	APIProxyRewrite    string          `json:"api_proxy_rewrite"`
	RootDir            string          `json:"root_dir"`
	EntryFile          string          `json:"entry_file"`
	ActiveDeploymentID *uint           `json:"active_deployment_id"`
	ActiveDeployment   *DeploymentView `json:"active_deployment,omitempty"`
	DeploymentCount    int64           `json:"deployment_count"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// ListProjects 列出全部 Pages 项目。
func ListProjects(ctx context.Context) ([]View, error) {
	projects, err := model.ListPagesProjects(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]View, 0, len(projects))
	for _, project := range projects {
		view, err := buildProjectView(ctx, &project)
		if err != nil {
			return nil, err
		}
		views = append(views, *view)
	}
	return views, nil
}

// GetProject 获取 Pages 项目详情。
func GetProject(ctx context.Context, id uint) (*View, error) {
	project, err := model.GetPagesProjectByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildProjectView(ctx, project)
}

// CreateProject 创建 Pages 项目。
func CreateProject(ctx context.Context, input Input) (*View, error) {
	project, err := buildProject(nil, input)
	if err != nil {
		return nil, err
	}
	if err = model.CreatePagesProjectRecord(ctx, project); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errPagesSlugExists)
		}
		return nil, err
	}
	return buildProjectView(ctx, project)
}

// UpdateProject 更新 Pages 项目。
func UpdateProject(ctx context.Context, id uint, input Input) (*View, error) {
	project, err := model.GetPagesProjectByID(ctx, id)
	if err != nil {
		return nil, err
	}
	project, err = buildProject(project, input)
	if err != nil {
		return nil, err
	}
	if err = db.DB(ctx).Model(project).Updates(map[string]any{
		"name":                 project.Name,
		"slug":                 project.Slug,
		"description":          project.Description,
		"enabled":              project.Enabled,
		"spa_fallback_enabled": project.SPAFallbackEnabled,
		"spa_fallback_path":    project.SPAFallbackPath,
		"api_proxy_enabled":    project.APIProxyEnabled,
		"api_proxy_path":       project.APIProxyPath,
		"api_proxy_pass":       project.APIProxyPass,
		"api_proxy_rewrite":    project.APIProxyRewrite,
		"root_dir":             project.RootDir,
		"entry_file":           project.EntryFile,
	}).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errPagesSlugExists)
		}
		return nil, err
	}
	return buildProjectView(ctx, project)
}

// DeleteProject 删除 Pages 项目。
func DeleteProject(ctx context.Context, id uint) error {
	project, err := model.GetPagesProjectByID(ctx, id)
	if err != nil {
		return err
	}
	routeCount, err := model.CountProxyRoutesByPagesProjectID(ctx, project.ID)
	if err != nil {
		return err
	}
	if routeCount > 0 {
		return errors.New(errPagesDeleteReferenced)
	}
	deployments, err := model.ListPagesDeployments(ctx, project.ID)
	if err != nil {
		return err
	}
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"deployment_id IN (?)",
			tx.Model(&model.PagesDeployment{}).Select("id").Where("project_id = ?", project.ID),
		).Delete(&model.PagesDeploymentFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", project.ID).Delete(&model.PagesDeployment{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(project).Error; err != nil {
			return err
		}
		for index := range deployments {
			removeDeploymentArtifact(ctx, &deployments[index])
		}
		return nil
	})
}

// ListProjectDeployments 列出项目的全部部署。
func ListProjectDeployments(ctx context.Context, projectID uint) ([]DeploymentView, error) {
	if _, err := model.GetPagesProjectByID(ctx, projectID); err != nil {
		return nil, err
	}
	deployments, err := model.ListPagesDeployments(ctx, projectID)
	if err != nil {
		return nil, err
	}
	views := make([]DeploymentView, 0, len(deployments))
	for _, deployment := range deployments {
		views = append(views, buildDeploymentView(&deployment))
	}
	return views, nil
}

// ListDeploymentFiles 列出部署文件清单。
func ListDeploymentFiles(ctx context.Context, deploymentID uint) ([]DeploymentFileView, error) {
	if _, err := model.GetPagesDeploymentByID(ctx, deploymentID); err != nil {
		return nil, err
	}
	files, err := model.ListPagesDeploymentFiles(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	views := make([]DeploymentFileView, 0, len(files))
	for _, file := range files {
		views = append(views, DeploymentFileView{
			ID:           file.ID,
			DeploymentID: file.DeploymentID,
			Path:         file.Path,
			Size:         file.Size,
			Checksum:     file.Checksum,
			CreatedAt:    file.CreatedAt,
		})
	}
	return views, nil
}

// UploadDeployment 上传 Pages 部署包。
func UploadDeployment(ctx context.Context, projectID uint, fileHeader *multipart.FileHeader, createdBy string) (*DeploymentView, error) {
	project, err := model.GetPagesProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if fileHeader == nil {
		return nil, errors.New(errPagesPackageMissing)
	}
	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".zip") {
		return nil, errors.New(errPagesPackageNotZip)
	}
	rootDir, err := validateAndNormalizePagesRootDir(project.RootDir)
	if err != nil {
		return nil, err
	}
	entryFile := normalizePagesEntryFile(project.EntryFile)
	tempPath, checksum, packageSize, err := persistPagesUploadTemp(fileHeader)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempPath)
	manifest, err := inspectPagesZip(tempPath, rootDir, entryFile)
	if err != nil {
		return nil, err
	}
	ingestResult, err := ingestPagesDeploymentPackage(
		ctx,
		tempPath,
		checksum,
		packageSize,
		project.Slug,
		fileHeader.Filename,
	)
	if err != nil {
		return nil, err
	}
	ingestCommitted := false
	defer func() {
		if !ingestCommitted && ingestResult.Created {
			_, _ = upload.Remove(ctx, ingestResult.Upload.ID)
		}
	}()
	deployment := &model.PagesDeployment{}
	err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var maxNumber int
		if err := tx.Model(&model.PagesDeployment{}).
			Where("project_id = ?", project.ID).
			Select("COALESCE(MAX(deployment_number), 0)").
			Scan(&maxNumber).Error; err != nil {
			return err
		}
		deployment = &model.PagesDeployment{
			ProjectID:        project.ID,
			DeploymentNumber: maxNumber + 1,
			Checksum:         checksum,
			Status:           model.PagesDeploymentStatusUploaded,
			UploadID:         ingestResult.Upload.ID,
			FileCount:        manifest.FileCount,
			TotalSize:        manifest.TotalSize,
			CreatedBy:        strings.TrimSpace(createdBy),
		}
		if err := tx.Create(deployment).Error; err != nil {
			return err
		}
		for index := range manifest.Files {
			manifest.Files[index].DeploymentID = deployment.ID
		}
		if len(manifest.Files) > 0 {
			if err := tx.Create(&manifest.Files).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	ingestCommitted = true
	view := buildDeploymentView(deployment)
	return &view, nil
}

// ActivateDeployment 激活 Pages 部署。
func ActivateDeployment(ctx context.Context, projectID uint, deploymentID uint) (*View, error) {
	project, err := model.GetPagesProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	deployment, err := model.GetPagesDeploymentByID(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment.ProjectID != project.ID {
		return nil, errors.New(errPagesDeploymentMismatch)
	}
	now := time.Now()
	if err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PagesDeployment{}).
			Where("project_id = ?", project.ID).
			Update("status", model.PagesDeploymentStatusUploaded).Error; err != nil {
			return err
		}
		if err := tx.Model(deployment).Updates(map[string]any{
			"status":       model.PagesDeploymentStatusActive,
			"activated_at": &now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(project).Updates(map[string]any{
			"active_deployment_id": deployment.ID,
		}).Error
	}); err != nil {
		return nil, err
	}
	return GetProject(ctx, project.ID)
}

// OpenDeploymentPackage opens the deployment artifact from the upload storage framework.
func OpenDeploymentPackage(ctx context.Context, deploymentID uint) (*storage.Object, string, error) {
	deployment, err := model.GetPagesDeploymentByID(ctx, deploymentID)
	if err != nil {
		return nil, "", err
	}
	if err = ensureDeploymentInActiveSnapshot(ctx, deployment.ID); err != nil {
		return nil, "", err
	}
	fileName := fmt.Sprintf("pages-deployment-%d.zip", deployment.ID)
	if deployment.UploadID > 0 {
		uploadRecord, err := repository.GetActiveUploadByID(ctx, deployment.UploadID)
		if err != nil {
			return nil, "", errors.New(errPagesPackageUploadMissing)
		}
		obj, err := uploadstorage.OpenStoredObject(ctx, &uploadRecord)
		if err != nil {
			return nil, "", fmt.Errorf("Pages 部署包不存在: %w", err)
		}
		if obj.ContentType == "" {
			obj.ContentType = "application/zip"
		}
		return obj, fileName, nil
	}
	if strings.TrimSpace(deployment.ArtifactPath) == "" {
		return nil, "", errors.New(errPagesPackagePathEmpty)
	}
	file, err := os.Open(deployment.ArtifactPath)
	if err != nil {
		return nil, "", fmt.Errorf("Pages 部署包不存在: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, "", fmt.Errorf("Pages 部署包不存在: %w", err)
	}
	return &storage.Object{
		Body:          file,
		ContentLength: info.Size(),
		ContentType:   "application/zip",
	}, fileName, nil
}

func ensureDeploymentInActiveSnapshot(ctx context.Context, deploymentID uint) error {
	version, err := model.GetActiveConfigVersion(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errPagesPackageNotInActiveConfig)
		}
		return err
	}
	routes, err := parseSnapshotRoutes(version.SnapshotJSON)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if route.UpstreamType != "pages" || route.PagesDeployment == nil {
			continue
		}
		if route.PagesDeployment.DeploymentID == deploymentID {
			return nil
		}
	}
	return errors.New(errPagesPackageNotInActiveConfig)
}

type snapshotPagesDeployment struct {
	DeploymentID uint `json:"deployment_id"`
}

type snapshotRouteRef struct {
	UpstreamType    string                   `json:"upstream_type"`
	PagesDeployment *snapshotPagesDeployment `json:"pages_deployment"`
}

func parseSnapshotRoutes(snapshotJSON string) ([]snapshotRouteRef, error) {
	text := strings.TrimSpace(snapshotJSON)
	if text == "" {
		return []snapshotRouteRef{}, nil
	}
	if strings.HasPrefix(text, "[") {
		var routes []snapshotRouteRef
		if err := json.Unmarshal([]byte(text), &routes); err != nil {
			return nil, errors.New(errPagesInvalidSnapshotFormat)
		}
		return routes, nil
	}
	var snapshot struct {
		Routes []snapshotRouteRef `json:"routes"`
	}
	if err := json.Unmarshal([]byte(text), &snapshot); err != nil {
		return nil, errors.New(errPagesInvalidSnapshotFormat)
	}
	return snapshot.Routes, nil
}

// DeleteDeployment 删除 Pages 部署。
func DeleteDeployment(ctx context.Context, projectID uint, deploymentID uint) error {
	project, err := model.GetPagesProjectByID(ctx, projectID)
	if err != nil {
		return err
	}
	deployment, err := model.GetPagesDeploymentByID(ctx, deploymentID)
	if err != nil {
		return err
	}
	if deployment.ProjectID != project.ID {
		return errors.New(errPagesDeploymentMismatch)
	}
	if project.ActiveDeploymentID != nil && *project.ActiveDeploymentID == deployment.ID {
		return errors.New(errPagesDeleteActiveDeploy)
	}
	return db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("deployment_id = ?", deployment.ID).Delete(&model.PagesDeploymentFile{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(deployment).Error; err != nil {
			return err
		}
		removeDeploymentArtifact(ctx, deployment)
		return nil
	})
}

func buildProject(existing *model.PagesProject, input Input) (*model.PagesProject, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New(errPagesNameRequired)
	}
	slug := normalizePagesSlug(input.Slug)
	if slug == "" {
		slug = normalizePagesSlug(name)
	}
	if !pagesSlugPattern.MatchString(slug) {
		return nil, errors.New(errPagesSlugInvalid)
	}
	if existing == nil {
		existing = &model.PagesProject{}
	}
	existing.Name = name
	existing.Slug = slug
	existing.Description = strings.TrimSpace(input.Description)
	existing.Enabled = input.Enabled
	existing.SPAFallbackEnabled = input.SPAFallbackEnabled
	fallbackPath, err := normalizePagesFallbackPath(input.SPAFallbackPath)
	if err != nil {
		return nil, err
	}
	existing.SPAFallbackPath = fallbackPath

	existing.APIProxyEnabled = input.APIProxyEnabled
	apiProxyPath := strings.TrimSpace(input.APIProxyPath)
	apiProxyPass := strings.TrimSpace(input.APIProxyPass)
	apiProxyRewrite := strings.TrimSpace(input.APIProxyRewrite)

	if existing.APIProxyEnabled {
		if apiProxyPath == "" {
			return nil, errors.New(errPagesAPIProxyPathRequired)
		}
		if !strings.HasPrefix(apiProxyPath, "/") {
			return nil, errors.New(errPagesAPIProxyPathPrefix)
		}
		if apiProxyPass == "" {
			return nil, errors.New(errPagesAPIProxyPassRequired)
		}
		parsedURL, err := url.Parse(apiProxyPass)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
			return nil, errors.New(errPagesAPIProxyPassInvalid)
		}
	}
	existing.APIProxyPath = apiProxyPath
	existing.APIProxyPass = apiProxyPass
	existing.APIProxyRewrite = apiProxyRewrite

	rootDir, err := validateAndNormalizePagesRootDir(input.RootDir)
	if err != nil {
		return nil, err
	}
	existing.RootDir = rootDir
	existing.EntryFile = normalizePagesEntryFile(input.EntryFile)

	return existing, nil
}

func buildProjectView(ctx context.Context, project *model.PagesProject) (*View, error) {
	if project == nil {
		return nil, errors.New(errPagesProjectNotFound)
	}
	view := &View{
		ID:                 project.ID,
		Name:               project.Name,
		Slug:               project.Slug,
		Description:        project.Description,
		Enabled:            project.Enabled,
		SPAFallbackEnabled: project.SPAFallbackEnabled,
		SPAFallbackPath:    normalizeStoredPagesFallbackPath(project.SPAFallbackPath),
		APIProxyEnabled:    project.APIProxyEnabled,
		APIProxyPath:       project.APIProxyPath,
		APIProxyPass:       project.APIProxyPass,
		APIProxyRewrite:    project.APIProxyRewrite,
		RootDir:            project.RootDir,
		EntryFile:          project.EntryFile,
		ActiveDeploymentID: project.ActiveDeploymentID,
		CreatedAt:          project.CreatedAt,
		UpdatedAt:          project.UpdatedAt,
	}
	count, err := model.CountPagesDeploymentsByProjectID(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	view.DeploymentCount = count
	if project.ActiveDeploymentID != nil && *project.ActiveDeploymentID != 0 {
		deployment, err := model.GetPagesDeploymentByID(ctx, *project.ActiveDeploymentID)
		if err == nil {
			active := buildDeploymentView(deployment)
			view.ActiveDeployment = &active
		}
	}
	return view, nil
}

func buildDeploymentView(deployment *model.PagesDeployment) DeploymentView {
	if deployment == nil {
		return DeploymentView{}
	}
	return DeploymentView{
		ID:               deployment.ID,
		ProjectID:        deployment.ProjectID,
		DeploymentNumber: deployment.DeploymentNumber,
		Checksum:         deployment.Checksum,
		Status:           deployment.Status,
		FileCount:        deployment.FileCount,
		TotalSize:        deployment.TotalSize,
		CreatedBy:        deployment.CreatedBy,
		CreatedAt:        deployment.CreatedAt,
		ActivatedAt:      deployment.ActivatedAt,
	}
}
