package service

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"openflare/common"
	"openflare/model"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	pagesMaxDeploymentFiles  = 1000
	pagesMaxDeploymentBytes  = 100 * 1024 * 1024
	defaultPagesEntryFile    = "index.html"
	defaultPagesFallbackPath = "/index.html"
)

var pagesSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,126}[a-z0-9]$|^[a-z0-9]$`)

type PagesProjectInput struct {
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

type PagesProjectView struct {
	ID                 uint                 `json:"id"`
	Name               string               `json:"name"`
	Slug               string               `json:"slug"`
	Description        string               `json:"description"`
	Enabled            bool                 `json:"enabled"`
	SPAFallbackEnabled bool                 `json:"spa_fallback_enabled"`
	SPAFallbackPath    string               `json:"spa_fallback_path"`
	APIProxyEnabled    bool                 `json:"api_proxy_enabled"`
	APIProxyPath       string               `json:"api_proxy_path"`
	APIProxyPass       string               `json:"api_proxy_pass"`
	APIProxyRewrite    string               `json:"api_proxy_rewrite"`
	RootDir            string               `json:"root_dir"`
	EntryFile          string               `json:"entry_file"`
	ActiveDeploymentID *uint                `json:"active_deployment_id"`
	ActiveDeployment   *PagesDeploymentView `json:"active_deployment,omitempty"`
	DeploymentCount    int64                `json:"deployment_count"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}

type PagesDeploymentView struct {
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

type PagesDeploymentFileView struct {
	ID           uint      `json:"id"`
	DeploymentID uint      `json:"deployment_id"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	Checksum     string    `json:"checksum"`
	CreatedAt    time.Time `json:"created_at"`
}

type pagesDeploymentManifest struct {
	Files     []model.PagesDeploymentFile
	FileCount int
	TotalSize int64
	EntryFile string
}

func ListPagesProjects() ([]*PagesProjectView, error) {
	projects, err := model.ListPagesProjects()
	if err != nil {
		return nil, err
	}
	views := make([]*PagesProjectView, 0, len(projects))
	for _, project := range projects {
		view, err := buildPagesProjectView(project)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func GetPagesProject(id uint) (*PagesProjectView, error) {
	project, err := model.GetPagesProjectByID(id)
	if err != nil {
		return nil, err
	}
	return buildPagesProjectView(project)
}

func CreatePagesProject(input PagesProjectInput) (*PagesProjectView, error) {
	project, err := buildPagesProject(nil, input)
	if err != nil {
		return nil, err
	}
	if err = model.DB.Create(project).Error; err != nil {
		if model.IsUniqueConstraintError(err) {
			return nil, errors.New("Pages 项目标识已存在")
		}
		return nil, err
	}
	return buildPagesProjectView(project)
}

func UpdatePagesProject(id uint, input PagesProjectInput) (*PagesProjectView, error) {
	project, err := model.GetPagesProjectByID(id)
	if err != nil {
		return nil, err
	}
	project, err = buildPagesProject(project, input)
	if err != nil {
		return nil, err
	}
	if err = model.DB.Model(project).Updates(map[string]any{
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
		if model.IsUniqueConstraintError(err) {
			return nil, errors.New("Pages 项目标识已存在")
		}
		return nil, err
	}
	return buildPagesProjectView(project)
}

func DeletePagesProject(id uint) error {
	project, err := model.GetPagesProjectByID(id)
	if err != nil {
		return err
	}
	var routeCount int64
	if err = model.DB.Model(&model.ProxyRoute{}).Where("pages_project_id = ?", project.ID).Count(&routeCount).Error; err != nil {
		return err
	}
	if routeCount > 0 {
		return errors.New("Pages 项目已被规则引用，不能删除")
	}
	deployments, err := model.ListPagesDeployments(project.ID)
	if err != nil {
		return err
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("deployment_id IN (?)", tx.Model(&model.PagesDeployment{}).Select("id").Where("project_id = ?", project.ID)).Delete(&model.PagesDeploymentFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", project.ID).Delete(&model.PagesDeployment{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(project).Error; err != nil {
			return err
		}
		for _, deployment := range deployments {
			_ = os.Remove(deployment.ArtifactPath)
		}
		return nil
	})
}

func ListPagesProjectDeployments(projectID uint) ([]*PagesDeploymentView, error) {
	if _, err := model.GetPagesProjectByID(projectID); err != nil {
		return nil, err
	}
	deployments, err := model.ListPagesDeployments(projectID)
	if err != nil {
		return nil, err
	}
	views := make([]*PagesDeploymentView, 0, len(deployments))
	for _, deployment := range deployments {
		views = append(views, buildPagesDeploymentView(deployment))
	}
	return views, nil
}

func ListPagesDeploymentFiles(deploymentID uint) ([]*PagesDeploymentFileView, error) {
	if _, err := model.GetPagesDeploymentByID(deploymentID); err != nil {
		return nil, err
	}
	files, err := model.ListPagesDeploymentFiles(deploymentID)
	if err != nil {
		return nil, err
	}
	views := make([]*PagesDeploymentFileView, 0, len(files))
	for _, file := range files {
		views = append(views, &PagesDeploymentFileView{
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

func UploadPagesDeployment(projectID uint, fileHeader *multipart.FileHeader, rootDir string, entryFile string, createdBy string) (*PagesDeploymentView, error) {
	project, err := model.GetPagesProjectByID(projectID)
	if err != nil {
		return nil, err
	}
	if fileHeader == nil {
		return nil, errors.New("缺少 Pages 部署包")
	}
	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".zip") {
		return nil, errors.New("Pages 部署包必须是 .zip 文件")
	}
	rootDir, err = validateAndNormalizePagesRootDir(project.RootDir)
	if err != nil {
		return nil, err
	}
	entryFile = normalizePagesEntryFile(project.EntryFile)
	tempPath, checksum, err := persistPagesUploadTemp(fileHeader)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempPath)
	manifest, err := inspectPagesZip(tempPath, rootDir, entryFile)
	if err != nil {
		return nil, err
	}
	artifactPath, err := pagesArtifactPath(project.Slug, checksum)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建 Pages 存储目录失败: %w", err)
	}
	if err = copyFile(tempPath, artifactPath); err != nil {
		return nil, err
	}
	deployment := &model.PagesDeployment{}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
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
			ArtifactPath:     artifactPath,
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
		_ = os.Remove(artifactPath)
		return nil, err
	}
	return buildPagesDeploymentView(deployment), nil
}

func validateAndNormalizePagesRootDir(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	if len(value) > 512 {
		return "", errors.New("Pages 根目录长度不能超过 512")
	}
	if strings.Contains(value, "\\") || strings.ContainsAny(value, "\"';") {
		return "", errors.New("Pages 根目录包含不支持的字符")
	}
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return "", errors.New("Pages 根目录不能包含空白或控制字符")
		}
	}
	cleaned := path.Clean(filepath.ToSlash(value))
	if cleaned == "." || cleaned == "/" {
		return "", nil
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("Pages 根目录不能包含 . 或 .. 路径段")
		}
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func ActivatePagesDeployment(projectID uint, deploymentID uint) (*PagesProjectView, error) {
	project, err := model.GetPagesProjectByID(projectID)
	if err != nil {
		return nil, err
	}
	deployment, err := model.GetPagesDeploymentByID(deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment.ProjectID != project.ID {
		return nil, errors.New("Pages 部署不属于该项目")
	}
	now := time.Now()
	if err = model.DB.Transaction(func(tx *gorm.DB) error {
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
	return GetPagesProject(project.ID)
}

func DeletePagesDeployment(projectID uint, deploymentID uint) error {
	project, err := model.GetPagesProjectByID(projectID)
	if err != nil {
		return err
	}
	deployment, err := model.GetPagesDeploymentByID(deploymentID)
	if err != nil {
		return err
	}
	if deployment.ProjectID != project.ID {
		return errors.New("Pages 部署不属于该项目")
	}
	if project.ActiveDeploymentID != nil && *project.ActiveDeploymentID == deployment.ID {
		return errors.New("不能删除当前激活的 Pages 部署")
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("deployment_id = ?", deployment.ID).Delete(&model.PagesDeploymentFile{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(deployment).Error; err != nil {
			return err
		}
		_ = os.Remove(deployment.ArtifactPath)
		return nil
	})
}

func GetPagesDeploymentPackagePath(deploymentID uint) (string, string, error) {
	deployment, err := model.GetPagesDeploymentByID(deploymentID)
	if err != nil {
		return "", "", err
	}
	if err = ensurePagesDeploymentInActiveSnapshot(deployment.ID); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(deployment.ArtifactPath) == "" {
		return "", "", errors.New("Pages 部署包路径为空")
	}
	if _, err = os.Stat(deployment.ArtifactPath); err != nil {
		return "", "", fmt.Errorf("Pages 部署包不存在: %w", err)
	}
	return deployment.ArtifactPath, fmt.Sprintf("pages-deployment-%d.zip", deployment.ID), nil
}

func ensurePagesDeploymentInActiveSnapshot(deploymentID uint) error {
	version, err := model.GetActiveConfigVersion()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("Pages 部署尚未进入激活配置")
		}
		return err
	}
	snapshot, err := parseSnapshotDocument(version.SnapshotJSON)
	if err != nil {
		return err
	}
	for _, route := range snapshot.Routes {
		if route.UpstreamType != "pages" || route.PagesDeployment == nil {
			continue
		}
		if route.PagesDeployment.DeploymentID == deploymentID {
			return nil
		}
	}
	return errors.New("Pages 部署尚未进入激活配置")
}

func buildPagesProject(project *model.PagesProject, input PagesProjectInput) (*model.PagesProject, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("Pages 项目名称不能为空")
	}
	slug := normalizePagesSlug(input.Slug)
	if slug == "" {
		slug = normalizePagesSlug(name)
	}
	if !pagesSlugPattern.MatchString(slug) {
		return nil, errors.New("Pages 项目标识只能包含小写字母、数字和连字符")
	}
	if project == nil {
		project = &model.PagesProject{}
	}
	project.Name = name
	project.Slug = slug
	project.Description = strings.TrimSpace(input.Description)
	project.Enabled = input.Enabled
	project.SPAFallbackEnabled = input.SPAFallbackEnabled
	fallbackPath, err := normalizePagesFallbackPath(input.SPAFallbackPath)
	if err != nil {
		return nil, err
	}
	project.SPAFallbackPath = fallbackPath

	project.APIProxyEnabled = input.APIProxyEnabled
	apiProxyPath := strings.TrimSpace(input.APIProxyPath)
	apiProxyPass := strings.TrimSpace(input.APIProxyPass)
	apiProxyRewrite := strings.TrimSpace(input.APIProxyRewrite)

	if project.APIProxyEnabled {
		if apiProxyPath == "" {
			return nil, errors.New("启用 API 反代时，匹配路径不能为空")
		}
		if !strings.HasPrefix(apiProxyPath, "/") {
			return nil, errors.New("API 反代匹配路径必须以 '/' 开头")
		}
		if apiProxyPass == "" {
			return nil, errors.New("启用 API 反代时，后端服务地址不能为空")
		}
		parsedURL, err := url.Parse(apiProxyPass)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
			return nil, errors.New("API 反代后端服务地址必须是有效的 HTTP/HTTPS URL")
		}
	}
	project.APIProxyPath = apiProxyPath
	project.APIProxyPass = apiProxyPass
	project.APIProxyRewrite = apiProxyRewrite

	rootDir, err := validateAndNormalizePagesRootDir(input.RootDir)
	if err != nil {
		return nil, err
	}
	project.RootDir = rootDir
	project.EntryFile = normalizePagesEntryFile(input.EntryFile)

	return project, nil
}

func buildPagesProjectView(project *model.PagesProject) (*PagesProjectView, error) {
	if project == nil {
		return nil, errors.New("Pages 项目为空")
	}
	view := &PagesProjectView{
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
	if err := model.DB.Model(&model.PagesDeployment{}).Where("project_id = ?", project.ID).Count(&view.DeploymentCount).Error; err != nil {
		return nil, err
	}
	if project.ActiveDeploymentID != nil && *project.ActiveDeploymentID != 0 {
		deployment, err := model.GetPagesDeploymentByID(*project.ActiveDeploymentID)
		if err == nil {
			view.ActiveDeployment = buildPagesDeploymentView(deployment)
		}
	}
	return view, nil
}

func buildPagesDeploymentView(deployment *model.PagesDeployment) *PagesDeploymentView {
	if deployment == nil {
		return nil
	}
	return &PagesDeploymentView{
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

func normalizePagesSlug(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func normalizePagesFallbackPath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = defaultPagesFallbackPath
	}
	if len(value) > 512 {
		return "", errors.New("SPA fallback 回退路径长度不能超过 512")
	}
	if !strings.HasPrefix(value, "/") {
		return "", errors.New("SPA fallback 回退路径必须以 / 开头")
	}
	if value == "/" || strings.HasSuffix(value, "/") {
		return "", errors.New("SPA fallback 回退路径必须指向具体文件")
	}
	if strings.Contains(value, "\\") || strings.ContainsAny(value, "\"';") {
		return "", errors.New("SPA fallback 回退路径包含不支持的字符")
	}
	for _, r := range value {
		if r <= 0x20 || r == 0x7f {
			return "", errors.New("SPA fallback 回退路径不能包含空白或控制字符")
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("SPA fallback 回退路径不能包含 . 或 .. 路径段")
		}
	}
	cleaned := path.Clean(value)
	if cleaned == "." || !strings.HasPrefix(cleaned, "/") {
		return "", errors.New("SPA fallback 回退路径不合法")
	}
	if cleaned == "/" || strings.HasSuffix(cleaned, "/") {
		return "", errors.New("SPA fallback 回退路径必须指向具体文件")
	}
	return cleaned, nil
}

func normalizeStoredPagesFallbackPath(value string) string {
	normalized, err := normalizePagesFallbackPath(value)
	if err != nil {
		return defaultPagesFallbackPath
	}
	return normalized
}

func normalizePagesEntryFile(raw string) string {
	value := path.Clean(strings.TrimSpace(filepath.ToSlash(raw)))
	if value == "." || value == "/" {
		return defaultPagesEntryFile
	}
	return strings.TrimPrefix(value, "/")
}

func persistPagesUploadTemp(fileHeader *multipart.FileHeader) (string, string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	temp, err := os.CreateTemp("", "openflare-pages-*.zip")
	if err != nil {
		return "", "", err
	}
	defer temp.Close()
	hash := sha256.New()
	limited := io.LimitReader(file, pagesMaxDeploymentBytes+1)
	written, err := io.Copy(io.MultiWriter(temp, hash), limited)
	if err != nil {
		_ = os.Remove(temp.Name())
		return "", "", err
	}
	if written > pagesMaxDeploymentBytes {
		_ = os.Remove(temp.Name())
		return "", "", fmt.Errorf("Pages 部署包不能超过 %d MiB", pagesMaxDeploymentBytes/1024/1024)
	}
	return temp.Name(), hex.EncodeToString(hash.Sum(nil)), nil
}

func findCommonRootPrefix(files []*zip.File) (string, error) {
	var firstFilePath string
	hasMultipleFiles := false
	for _, item := range files {
		normalizedPath, skip, err := normalizePagesZipPath(item.Name)
		if err != nil {
			return "", err
		}
		if skip {
			continue
		}
		if firstFilePath == "" {
			firstFilePath = normalizedPath
		} else {
			hasMultipleFiles = true
		}
	}
	if firstFilePath == "" {
		return "", nil
	}
	parts := strings.Split(firstFilePath, "/")
	if len(parts) <= 1 {
		return "", nil
	}
	commonPrefix := parts[0] + "/"
	if hasMultipleFiles {
		for _, item := range files {
			normalizedPath, skip, err := normalizePagesZipPath(item.Name)
			if err != nil {
				return "", err
			}
			if skip {
				continue
			}
			if !strings.HasPrefix(normalizedPath, commonPrefix) {
				return "", nil
			}
		}
	}
	return commonPrefix, nil
}

func inspectPagesZip(zipPath string, rootDir string, entryFile string) (*pagesDeploymentManifest, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, errors.New("Pages 部署包不是有效 zip 文件")
	}
	defer reader.Close()

	commonPrefix, err := findCommonRootPrefix(reader.File)
	if err != nil {
		return nil, err
	}

	manifest := &pagesDeploymentManifest{
		Files:     []model.PagesDeploymentFile{},
		EntryFile: entryFile,
	}
	targetEntryPath := entryFile
	if rootDir != "" {
		targetEntryPath = path.Join(rootDir, entryFile)
	}
	entrySeen := false
	for _, item := range reader.File {
		normalizedPath, skip, err := normalizePagesZipPath(item.Name)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}

		if commonPrefix != "" {
			normalizedPath = strings.TrimPrefix(normalizedPath, commonPrefix)
		}

		if item.FileInfo().Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("Pages 部署包不支持符号链接: %s", normalizedPath)
		}
		if item.UncompressedSize64 > pagesMaxDeploymentBytes {
			return nil, fmt.Errorf("Pages 文件过大: %s", normalizedPath)
		}
		manifest.FileCount++
		if manifest.FileCount > pagesMaxDeploymentFiles {
			return nil, fmt.Errorf("Pages 部署文件数不能超过 %d", pagesMaxDeploymentFiles)
		}
		manifest.TotalSize += int64(item.UncompressedSize64)
		if manifest.TotalSize > pagesMaxDeploymentBytes {
			return nil, fmt.Errorf("Pages 部署展开后不能超过 %d MiB", pagesMaxDeploymentBytes/1024/1024)
		}
		checksum, err := checksumZipFile(item)
		if err != nil {
			return nil, err
		}
		if normalizedPath == targetEntryPath {
			entrySeen = true
		}
		manifest.Files = append(manifest.Files, model.PagesDeploymentFile{
			Path:     normalizedPath,
			Size:     int64(item.UncompressedSize64),
			Checksum: checksum,
		})
	}
	if manifest.FileCount == 0 {
		return nil, errors.New("Pages 部署包不能为空")
	}
	if !entrySeen {
		return nil, fmt.Errorf("Pages 部署包缺少入口文件 %s", targetEntryPath)
	}
	return manifest, nil
}

func normalizePagesZipPath(raw string) (string, bool, error) {
	name := strings.TrimSpace(filepath.ToSlash(raw))
	if name == "" {
		return "", true, nil
	}
	if strings.HasSuffix(name, "/") {
		return "", true, nil
	}
	if strings.HasPrefix(name, "/") || path.IsAbs(name) {
		return "", false, fmt.Errorf("Pages 部署包不能包含绝对路径: %s", raw)
	}
	cleaned := path.Clean(name)
	if cleaned == "." {
		return "", true, nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", false, fmt.Errorf("Pages 部署包路径不能逃逸目录: %s", raw)
	}
	return cleaned, false, nil
}

func checksumZipFile(item *zip.File) (string, error) {
	file, err := item.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func pagesArtifactPath(projectSlug string, checksum string) (string, error) {
	root, err := pagesStorageRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "artifacts", projectSlug, checksum+".zip"), nil
}

func pagesStorageRoot() (string, error) {
	if common.SQLDSN != "" {
		return filepath.Abs(filepath.Join("data", "pages"))
	}
	dbPath := strings.TrimSpace(common.SQLitePath)
	if dbPath == "" {
		return filepath.Abs(filepath.Join("data", "pages"))
	}
	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		dir = "data"
	}
	return filepath.Abs(filepath.Join(dir, "pages"))
}

func copyFile(src string, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err = io.Copy(output, input); err != nil {
		return err
	}
	return output.Sync()
}
