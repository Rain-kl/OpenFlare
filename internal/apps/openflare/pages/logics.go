// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeploymentPackage is a streamable Pages deployment artifact for agent download.
type DeploymentPackage struct {
	FileName      string
	ContentType   string
	ContentLength int64
	Body          io.ReadCloser
}

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
	UploadID         uint64     `json:"upload_id,string"`
	FileCount        int        `json:"file_count"`
	TotalSize        int64      `json:"total_size"`
	CreatedBy        string     `json:"created_by"`
	SourceType       string     `json:"source_type"`
	SourceLabel      string     `json:"source_label"`
	TriggerType      string     `json:"trigger_type"`
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
	var project *model.PagesProject
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.PagesProject
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&existing, id).Error; err != nil {
			return err
		}
		updated := existing
		var err error
		project, err = buildProject(&updated, input)
		if err != nil {
			return err
		}
		contentConfigChanged := existing.RootDir != project.RootDir || existing.EntryFile != project.EntryFile
		if contentConfigChanged &&
			existing.ActiveDeploymentID != nil && *existing.ActiveDeploymentID != 0 {
			if err := ensureDeploymentEntry(tx, *existing.ActiveDeploymentID, project.RootDir, project.EntryFile); err != nil {
				return err
			}
		}
		updates := map[string]any{
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
		}
		if contentConfigChanged {
			updates["content_config_version"] = existing.ContentConfigVersion + 1
			var source model.PagesProjectSource
			sourceErr := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
				Where("project_id = ?", existing.ID).
				First(&source).Error
			if sourceErr != nil && !errors.Is(sourceErr, gorm.ErrRecordNotFound) {
				return sourceErr
			}
			if sourceErr == nil {
				if err := fenceAndNormalizeRuntime(tx, source.ID); err != nil {
					return err
				}
			}
		}
		return tx.Model(&existing).Updates(updates).Error
	})
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New(errPagesSlugExists)
		}
		return nil, err
	}
	return buildProjectView(ctx, project)
}

func ensureDeploymentEntry(conn *gorm.DB, deploymentID uint, rootDir, entryFile string) error {
	targetPath := entryFile
	if rootDir != "" {
		targetPath = path.Join(rootDir, entryFile)
	}
	var count int64
	if err := conn.Model(&model.PagesDeploymentFile{}).
		Where("deployment_id = ? AND path = ?", deploymentID, targetPath).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%s: %s", errPagesEntryFileMissing, targetPath)
	}
	return nil
}

// DeleteProject 删除 Pages 项目。
func DeleteProject(ctx context.Context, id uint) error {
	project, err := model.GetPagesProjectByID(ctx, id)
	if err != nil {
		return err
	}
	var deployments []model.PagesDeployment
	err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedProject model.PagesProject
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&lockedProject, project.ID).Error; err != nil {
			return err
		}
		if tx.Migrator().HasTable(&model.ProxyRoute{}) {
			var routeCount int64
			if err := tx.Model(&model.ProxyRoute{}).
				Where("pages_project_id = ?", project.ID).
				Count(&routeCount).Error; err != nil {
				return err
			}
			if routeCount > 0 {
				return errors.New(errPagesDeleteReferenced)
			}
		}
		var source model.PagesProjectSource
		sourceErr := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
			Where("project_id = ?", project.ID).
			First(&source).Error
		if sourceErr != nil && !errors.Is(sourceErr, gorm.ErrRecordNotFound) {
			return sourceErr
		}
		if sourceErr == nil {
			var runtime model.PagesProjectSourceRuntime
			runtimeErr := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
				Where("source_id = ?", source.ID).
				First(&runtime).Error
			if runtimeErr != nil && !errors.Is(runtimeErr, gorm.ErrRecordNotFound) {
				return runtimeErr
			}
			if err := tx.Where("source_id = ?", source.ID).Delete(&model.PagesProjectSourceRuntime{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&source).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("project_id = ?", project.ID).Find(&deployments).Error; err != nil {
			return err
		}
		if err := tx.Where(
			"deployment_id IN (?)",
			tx.Model(&model.PagesDeployment{}).Select("id").Where("project_id = ?", project.ID),
		).Delete(&model.PagesDeploymentFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", project.ID).Delete(&model.PagesDeployment{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&lockedProject).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	for index := range deployments {
		removeDeploymentArtifact(ctx, project.ID, &deployments[index])
	}
	return nil
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

// UploadDeployment 上传 Pages 部署包（本地 multipart 文件）。
func UploadDeployment(ctx context.Context, projectID uint, fileHeader *multipart.FileHeader, createdBy string) (*DeploymentView, error) {
	project, err := model.GetPagesProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if fileHeader == nil {
		return nil, errors.New(errPagesPackageMissing)
	}
	limits := resolvePagesLimits(ctx)
	tempPath, checksum, _, format, err := persistPagesUploadTemp(fileHeader, limits.PackageBytes)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tempPath) }()
	return createDeploymentFromTempPackage(
		ctx,
		project,
		tempPath,
		checksum,
		format,
		fileHeader.Filename,
		createdBy,
		"manual_upload",
		"manual_upload",
		limits,
	)
}

// UploadFromURLInput is the request body for downloading a deployment package from a remote URL.
type UploadFromURLInput struct {
	URL string `json:"url"`
}

// UploadDeploymentFromURL downloads a package from url and creates a deployment.
func UploadDeploymentFromURL(ctx context.Context, projectID uint, rawURL string, createdBy string) (*DeploymentView, error) {
	project, err := model.GetPagesProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	limits := resolvePagesLimits(ctx)
	tempPath, checksum, _, format, fileName, err := downloadPagesPackageFromURL(ctx, rawURL, limits.PackageBytes)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tempPath) }()
	return createDeploymentFromTempPackage(
		ctx,
		project,
		tempPath,
		checksum,
		format,
		fileName,
		createdBy,
		"manual_url",
		"manual_url",
		limits,
	)
}

func createDeploymentFromTempPackage(
	ctx context.Context,
	project *model.PagesProject,
	tempPath string,
	checksum string,
	format pagesarchive.Format,
	fileName string,
	createdBy string,
	sourceType string,
	triggerType string,
	limits pagesLimits,
) (*DeploymentView, error) {
	if project == nil {
		return nil, errors.New(errPagesProjectNotFound)
	}
	rootDir, err := validateAndNormalizePagesRootDir(project.RootDir)
	if err != nil {
		return nil, err
	}
	entryFile, err := validateAndNormalizePagesEntryFile(project.EntryFile)
	if err != nil {
		return nil, err
	}
	manifest, err := inspectPagesPackage(tempPath, format, rootDir, entryFile, limits)
	if err != nil {
		return nil, err
	}
	ingestResult, err := ingestPagesDeploymentPackage(
		ctx,
		tempPath,
		checksum,
		project.ID,
		fileName,
		format,
	)
	if err != nil {
		return nil, err
	}
	ingestCommitted := false
	defer func() {
		if !ingestCommitted && ingestResult.Created {
			if removeErr := removePagesUploadIfUnreferenced(ctx, project.ID, ingestResult.Upload.ID); removeErr != nil {
				logger.ErrorF(ctx,
					"[Pages] compensate deployment upload failed: project_id=%d upload_id=%d error=%v",
					project.ID, ingestResult.Upload.ID, removeErr,
				)
			}
		}
	}()
	deployment := &model.PagesDeployment{}
	err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedProject model.PagesProject
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&lockedProject, project.ID).Error; err != nil {
			return err
		}
		var uploadRecord model.Upload
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
			Where("id = ?", ingestResult.Upload.ID).
			First(&uploadRecord).Error; err != nil {
			return errors.New(errPagesPackageUploadMissing)
		}
		if uploadRecord.Status != model.UploadStatusUsed || uploadRecord.Type != upload.ReservedPagesDeploymentType {
			return errors.New(errPagesPackageUploadMissing)
		}
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
			SourceType:       sourceType,
			SourceLabel:      safeRemoteSourceLabel(fileName),
			TriggerType:      triggerType,
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

	if pruneErr := pruneProjectDeploymentHistory(ctx, project.ID, limits.HistoryCount, deployment.ID); pruneErr != nil {
		logger.ErrorF(ctx,
			"[Pages] prune deployment history failed: project_id=%d keep=%d error=%v",
			project.ID, limits.HistoryCount, pruneErr,
		)
	}

	view := buildDeploymentView(deployment)
	return &view, nil
}

// pruneProjectDeploymentHistory enforces the retention policy for one project.
//
// Policy (keepCount > 0):
//   - At most keepCount deployment rows remain for the project.
//   - The current active deployment is always retained (if any).
//   - Remaining slots are filled by newest deployments first (id desc).
//   - All other non-kept deployments are deleted with their file lists and artifacts.
//
// keepCount <= 0 means unlimited history.
//
// Concurrency: DB row deletes run in a single transaction after a consistent read
// of project + deployments. Concurrent uploads may briefly exceed keepCount; the
// next successful prune brings the project back within the limit (eventual).
func pruneProjectDeploymentHistory(ctx context.Context, projectID uint, keepCount int, preserveCandidateID uint) error {
	if keepCount <= 0 {
		return nil
	}

	// Two passes: first pass after upload, second pass heals a concurrent race
	// that inserted another deployment between our list and delete.
	var lastErr error
	for pass := 0; pass < 2; pass++ {
		deleted, err := pruneProjectDeploymentHistoryOnce(ctx, projectID, keepCount, preserveCandidateID)
		if err != nil {
			lastErr = err
			break
		}
		if deleted == 0 {
			break
		}
	}
	return lastErr
}

// pruneProjectDeploymentHistoryOnce performs one list → select → delete cycle.
// Returns the number of deployments deleted from the database.
func pruneProjectDeploymentHistoryOnce(ctx context.Context, projectID uint, keepCount int, preserveCandidateID uint) (int, error) {
	var deletedDeployments []model.PagesDeployment
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var project model.PagesProject
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&project, projectID).Error; err != nil {
			return fmt.Errorf("load pages project: %w", err)
		}
		var deployments []model.PagesDeployment
		if err := tx.Where("project_id = ?", projectID).Order("id desc").Find(&deployments).Error; err != nil {
			return fmt.Errorf("list pages deployments: %w", err)
		}
		var activeID uint
		if project.ActiveDeploymentID != nil {
			activeID = *project.ActiveDeploymentID
		}
		// Preserve mode is signaled by a non-zero candidate ID, but the lock-time
		// newest non-active deployment wins. This prevents concurrent upload A/B
		// prune passes from deleting each other's newer candidate.
		resolvedCandidateID := resolveLatestCandidateID(deployments, activeID, preserveCandidateID != 0)
		toDelete := selectDeploymentsToPrune(deployments, activeID, resolvedCandidateID, keepCount)
		for index := range toDelete {
			deployment := toDelete[index]
			if err := tx.Where("deployment_id = ?", deployment.ID).Delete(&model.PagesDeploymentFile{}).Error; err != nil {
				return fmt.Errorf("delete deployment files id=%d: %w", deployment.ID, err)
			}
			result := tx.Where("id = ? AND project_id = ?", deployment.ID, projectID).
				Delete(&model.PagesDeployment{})
			if result.Error != nil {
				return fmt.Errorf("delete deployment id=%d: %w", deployment.ID, result.Error)
			}
			if result.RowsAffected == 1 {
				deletedDeployments = append(deletedDeployments, deployment)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	for index := range deletedDeployments {
		deployment := deletedDeployments[index]
		removeDeploymentArtifact(ctx, projectID, &deployment)
	}

	logger.InfoF(ctx,
		"[Pages] pruned deployment history: project_id=%d keep=%d deleted=%d",
		projectID, keepCount, len(deletedDeployments),
	)
	return len(deletedDeployments), nil
}

func resolveLatestCandidateID(deployments []model.PagesDeployment, activeID uint, preserve bool) uint {
	if !preserve {
		return 0
	}
	for _, deployment := range deployments {
		if deployment.ID != activeID {
			return deployment.ID
		}
	}
	return 0
}

// selectDeploymentsToPrune returns deployments that should be removed under the
// "at most keepCount, always keep active, fill with newest" policy.
// deployments must be ordered newest-first (id desc).
func selectDeploymentsToPrune(
	deployments []model.PagesDeployment,
	activeID uint,
	preserveCandidateID uint,
	keepCount int,
) []model.PagesDeployment {
	if keepCount <= 0 || len(deployments) <= keepCount {
		return nil
	}

	keepIDs := make(map[uint]struct{}, keepCount)
	// 1) Active is always retained and occupies one slot when present.
	if activeID != 0 {
		// Only count active if it still exists in the list.
		for _, deployment := range deployments {
			if deployment.ID == activeID {
				keepIDs[activeID] = struct{}{}
				break
			}
		}
	}
	// A freshly uploaded manual candidate is temporarily protected in addition
	// to the active deployment. This intentionally permits two rows when the
	// configured history limit is one.
	if preserveCandidateID != 0 {
		for _, deployment := range deployments {
			if deployment.ID == preserveCandidateID {
				keepIDs[preserveCandidateID] = struct{}{}
				break
			}
		}
	}
	// 2) Fill remaining slots from newest to oldest.
	for _, deployment := range deployments {
		if len(keepIDs) >= keepCount {
			break
		}
		keepIDs[deployment.ID] = struct{}{}
	}

	toDelete := make([]model.PagesDeployment, 0, len(deployments)-len(keepIDs))
	for _, deployment := range deployments {
		if _, keep := keepIDs[deployment.ID]; keep {
			continue
		}
		// Safety: never mark active for deletion.
		if activeID != 0 && deployment.ID == activeID {
			continue
		}
		toDelete = append(toDelete, deployment)
	}
	return toDelete
}

// ActivateDeployment 激活 Pages 部署。
func ActivateDeployment(ctx context.Context, projectID uint, deploymentID uint) (*View, error) {
	return ActivateDeploymentAs(ctx, projectID, deploymentID, "system:pages-manual-activation")
}

// ActivateDeploymentAs activates a historical deployment and fences any
// configured source when the active deployment actually changes.
func ActivateDeploymentAs(ctx context.Context, projectID uint, deploymentID uint, actor string) (*View, error) {
	if err := ensureActivationDeploymentUpload(ctx, projectID, deploymentID); err != nil {
		return nil, err
	}
	audit, err := activateDeploymentTransaction(ctx, projectID, deploymentID, time.Now())
	if err != nil {
		return nil, err
	}
	if audit.Noop {
		return GetProject(ctx, projectID)
	}
	logger.InfoF(ctx,
		"[Pages] manual activation: actor=%s project_id=%d old_deployment_id=%d new_deployment_id=%d source_type=%s source_identity=%s auto_disabled=%t",
		strings.TrimSpace(actor), projectID, audit.OldDeploymentID, deploymentID,
		audit.SourceType, audit.SourceIdentity, audit.AutoDisabled,
	)
	limits := resolvePagesLimits(ctx)
	if pruneErr := pruneProjectDeploymentHistory(ctx, projectID, limits.HistoryCount, 0); pruneErr != nil {
		logger.ErrorF(ctx,
			"[Pages] strict prune after activation failed: project_id=%d keep=%d error=%v",
			projectID, limits.HistoryCount, pruneErr,
		)
	}
	return GetProject(ctx, projectID)
}

type deploymentActivationAudit struct {
	OldDeploymentID uint
	SourceType      string
	SourceIdentity  string
	AutoDisabled    bool
	Noop            bool
}

type deploymentActivationSource struct {
	Source  *model.PagesProjectSource
	Runtime *model.PagesProjectSourceRuntime
}

func ensureActivationDeploymentUpload(ctx context.Context, projectID uint, deploymentID uint) error {
	deployment, err := model.GetPagesDeploymentByID(ctx, deploymentID)
	if err != nil {
		return err
	}
	if deployment.ProjectID != projectID {
		return errors.New(errPagesDeploymentMismatch)
	}
	if deployment.UploadID != 0 {
		return nil
	}
	return ensureDeploymentUploadRecord(ctx, deployment)
}

func activateDeploymentTransaction(
	ctx context.Context,
	projectID uint,
	deploymentID uint,
	now time.Time,
) (deploymentActivationAudit, error) {
	audit := deploymentActivationAudit{}
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var project model.PagesProject
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&project, projectID).Error; err != nil {
			return err
		}
		if project.ActiveDeploymentID != nil {
			audit.OldDeploymentID = *project.ActiveDeploymentID
		}
		if audit.OldDeploymentID == deploymentID {
			audit.Noop = true
			return nil
		}

		sourceState, err := lockDeploymentActivationSource(tx, project.ID)
		if err != nil {
			return err
		}
		deployment, err := loadDeploymentActivationTarget(tx, &project, deploymentID)
		if err != nil {
			return err
		}
		if err := fenceDeploymentActivationSource(tx, sourceState, deployment, &audit); err != nil {
			return err
		}
		return switchActiveDeploymentTx(tx, &project, deployment, now)
	})
	return audit, err
}

func lockDeploymentActivationSource(tx *gorm.DB, projectID uint) (*deploymentActivationSource, error) {
	var source model.PagesProjectSource
	err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("project_id = ?", projectID).
		First(&source).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var runtime model.PagesProjectSourceRuntime
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("source_id = ?", source.ID).
		First(&runtime).Error; err != nil {
		return nil, err
	}
	return &deploymentActivationSource{Source: &source, Runtime: &runtime}, nil
}

func loadDeploymentActivationTarget(
	tx *gorm.DB,
	project *model.PagesProject,
	deploymentID uint,
) (*model.PagesDeployment, error) {
	var deployment model.PagesDeployment
	if err := tx.First(&deployment, deploymentID).Error; err != nil {
		return nil, err
	}
	if deployment.ProjectID != project.ID {
		return nil, errors.New(errPagesDeploymentMismatch)
	}
	rootDir, err := validateAndNormalizePagesRootDir(project.RootDir)
	if err != nil {
		return nil, err
	}
	entryFile, err := validateAndNormalizePagesEntryFile(project.EntryFile)
	if err != nil {
		return nil, err
	}
	if err := ensureDeploymentEntry(tx, deployment.ID, rootDir, entryFile); err != nil {
		return nil, err
	}
	var uploadRecord model.Upload
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("id = ?", deployment.UploadID).
		First(&uploadRecord).Error; err != nil {
		return nil, errors.New(errPagesPackageUploadMissing)
	}
	if uploadRecord.Status != model.UploadStatusUsed || uploadRecord.Type != upload.ReservedPagesDeploymentType {
		return nil, errors.New(errPagesPackageUploadMissing)
	}
	return &deployment, nil
}

func fenceDeploymentActivationSource(
	tx *gorm.DB,
	state *deploymentActivationSource,
	deployment *model.PagesDeployment,
	audit *deploymentActivationAudit,
) error {
	if state == nil {
		return nil
	}
	audit.SourceType = state.Source.SourceType
	audit.SourceIdentity = state.Source.SourceIdentity
	audit.AutoDisabled = state.Source.AutoUpdateEnabled
	if err := tx.Model(state.Source).Updates(map[string]any{
		sourceColumnConfigVersion:     state.Source.ConfigVersion + 1,
		sourceColumnAutoUpdateEnabled: false,
	}).Error; err != nil {
		return err
	}
	if deployment.SourceIdentity != nil && *deployment.SourceIdentity == state.Source.SourceIdentity &&
		deployment.SourceRevision != nil {
		state.Runtime.LastAppliedRevision = *deployment.SourceRevision
		state.Runtime.LastAppliedDetail = deployment.SourceMeta
	} else {
		state.Runtime.LastAppliedRevision = ""
		state.Runtime.LastAppliedDetail = ""
	}
	return tx.Model(state.Runtime).Updates(map[string]any{
		"last_applied_revision":           state.Runtime.LastAppliedRevision,
		"last_applied_detail":             state.Runtime.LastAppliedDetail,
		sourceRuntimeColumnSyncStatus:     normalizedSourceRuntimeStatus(state.Runtime),
		sourceRuntimeColumnLeaseToken:     "",
		sourceRuntimeColumnLeaseExpiresAt: nil,
	}).Error
}

func switchActiveDeploymentTx(
	tx *gorm.DB,
	project *model.PagesProject,
	deployment *model.PagesDeployment,
	now time.Time,
) error {
	if err := tx.Model(&model.PagesDeployment{}).
		Where("project_id = ?", project.ID).
		Update("status", model.PagesDeploymentStatusUploaded).Error; err != nil {
		return err
	}
	if err := tx.Model(deployment).Updates(map[string]any{
		pagesDeploymentColumnStatus: model.PagesDeploymentStatusActive,
		"activated_at":              &now,
	}).Error; err != nil {
		return err
	}
	return tx.Model(project).Update("active_deployment_id", deployment.ID).Error
}

// GetDeploymentPackageHash returns the upload SHA-256 hash of the deployment package.
// Prefer GetProjectLatestPackageHash for Agent latest-pointer pulls.
func GetDeploymentPackageHash(ctx context.Context, deploymentID uint) (string, error) {
	deployment, err := model.GetPagesDeploymentByID(ctx, deploymentID)
	if err != nil {
		return "", err
	}
	if err = ensureDeploymentInActiveSnapshot(ctx, deployment.ID); err != nil {
		return "", err
	}
	return deploymentPackageHash(ctx, deployment)
}

// GetProjectLatestPackageHash returns the package hash of a project's active deployment.
// This is the Agent "latest" pointer: callers pass project_id only.
func GetProjectLatestPackageHash(ctx context.Context, projectID uint) (uint, string, error) {
	deployment, err := resolveProjectActiveDeploymentForAgent(ctx, projectID)
	if err != nil {
		return 0, "", err
	}
	hash, err := deploymentPackageHash(ctx, deployment)
	if err != nil {
		return 0, "", err
	}
	return deployment.ID, hash, nil
}

// OpenDeploymentPackage opens the deployment artifact from the upload storage framework.
func OpenDeploymentPackage(ctx context.Context, deploymentID uint) (DeploymentPackage, error) {
	deployment, err := model.GetPagesDeploymentByID(ctx, deploymentID)
	if err != nil {
		return DeploymentPackage{}, err
	}
	if err = ensureDeploymentInActiveSnapshot(ctx, deployment.ID); err != nil {
		return DeploymentPackage{}, err
	}
	if deployment.UploadID == 0 {
		if err := ensureDeploymentUploadRecord(ctx, deployment); err != nil {
			return DeploymentPackage{}, err
		}
	}
	return openDeploymentPackageFromUpload(ctx, deployment.UploadID, deployment.ID)
}

// OpenProjectLatestPackage opens the currently active deployment package for a project.
func OpenProjectLatestPackage(ctx context.Context, projectID uint) (DeploymentPackage, error) {
	deployment, err := resolveProjectActiveDeploymentForAgent(ctx, projectID)
	if err != nil {
		return DeploymentPackage{}, err
	}
	if deployment.UploadID == 0 {
		if err := ensureDeploymentUploadRecord(ctx, deployment); err != nil {
			return DeploymentPackage{}, err
		}
	}
	return openDeploymentPackageFromUpload(ctx, deployment.UploadID, deployment.ID)
}

func resolveProjectActiveDeploymentForAgent(ctx context.Context, projectID uint) (*model.PagesDeployment, error) {
	if projectID == 0 {
		return nil, errors.New(errPagesProjectNotFound)
	}
	project, err := model.GetPagesProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if !project.Enabled {
		return nil, errors.New(errPagesPackageNotInActiveConfig)
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID == 0 {
		return nil, errors.New(errPagesPackageNotInActiveConfig)
	}
	if err := ensureProjectInActiveConfig(ctx, project.ID); err != nil {
		return nil, err
	}
	deployment, err := model.GetPagesDeploymentByID(ctx, *project.ActiveDeploymentID)
	if err != nil {
		return nil, err
	}
	if deployment.ProjectID != project.ID {
		return nil, errors.New(errPagesDeploymentMismatch)
	}
	return deployment, nil
}

func deploymentPackageHash(ctx context.Context, deployment *model.PagesDeployment) (string, error) {
	if deployment == nil {
		return "", errors.New(errPagesDeploymentNotFound)
	}
	if deployment.UploadID == 0 {
		if err := ensureDeploymentUploadRecord(ctx, deployment); err != nil {
			return "", err
		}
		reloaded, err := model.GetPagesDeploymentByID(ctx, deployment.ID)
		if err != nil {
			return "", err
		}
		deployment = reloaded
	}
	if deployment.UploadID == 0 {
		hash := strings.TrimSpace(deployment.Checksum)
		if hash == "" {
			return "", errors.New(errPagesDeploymentHashMissing)
		}
		return hash, nil
	}
	uploadRecord, err := upload.GetActiveUpload(ctx, deployment.UploadID)
	if err != nil {
		return "", fmt.Errorf("pages 部署包不存在: %w", err)
	}
	hash := strings.TrimSpace(uploadRecord.Hash)
	if hash == "" {
		hash = strings.TrimSpace(deployment.Checksum)
	}
	if hash == "" {
		return "", errors.New(errPagesDeploymentHashMissing)
	}
	return hash, nil
}

func openDeploymentPackageFromUpload(ctx context.Context, uploadID uint64, deploymentID uint) (DeploymentPackage, error) {
	opened, err := upload.OpenStoredUpload(ctx, uploadID)
	if err != nil {
		return DeploymentPackage{}, fmt.Errorf("pages 部署包不存在: %w", err)
	}
	contentType := opened.ContentType
	if contentType == "" {
		contentType = opened.Upload.MimeType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fileName := packageDownloadName(deploymentID, opened.Upload.FileName, contentType)
	return DeploymentPackage{
		FileName:      fileName,
		ContentType:   contentType,
		ContentLength: opened.ContentLength,
		Body:          opened.Body,
	}, nil
}

func ensureDeploymentUploadRecord(ctx context.Context, deployment *model.PagesDeployment) error {
	if deployment == nil {
		return errors.New(errPagesDeploymentNotFound)
	}
	if deployment.UploadID > 0 {
		return nil
	}
	project, err := model.GetPagesProjectByID(ctx, deployment.ProjectID)
	if err != nil {
		return err
	}
	if _, err := hydrateLegacyDeploymentUpload(ctx, deployment, project); err != nil {
		return err
	}
	return nil
}

func hydrateLegacyDeploymentUpload(
	ctx context.Context,
	deployment *model.PagesDeployment,
	project *model.PagesProject,
) (*model.Upload, error) {
	if deployment == nil || project == nil {
		return nil, errors.New(errPagesDeploymentNotFound)
	}
	if deployment.UploadID > 0 {
		uploadRecord, err := upload.GetActiveUpload(ctx, deployment.UploadID)
		if err == nil {
			return &uploadRecord, nil
		}
	}

	artifactPath, _, err := upload.ResolveLocalFile(ctx, upload.LocalFileCandidateRequest{
		StoredPath:    deployment.ArtifactPath,
		RelativePaths: pagesLegacyRelativeCandidates(project, deployment),
	})
	if err != nil {
		return nil, fmt.Errorf("pages 部署包不存在: %w", err)
	}

	ingestResult, err := ingestPagesDeploymentPackage(
		ctx,
		artifactPath,
		deployment.Checksum,
		project.ID,
		fmt.Sprintf("pages-deployment-%d.zip", deployment.ID),
		pagesarchive.FormatZip,
	)
	if err != nil {
		return nil, err
	}
	winnerUploadID, err := attachLegacyDeploymentUpload(
		ctx,
		project.ID,
		deployment.ID,
		ingestResult.Upload.ID,
	)
	if ingestResult.Created && (err != nil || winnerUploadID != ingestResult.Upload.ID) {
		if removeErr := removePagesUploadIfUnreferenced(ctx, project.ID, ingestResult.Upload.ID); removeErr != nil {
			logger.ErrorF(ctx,
				"[Pages] compensate legacy deployment upload failed: project_id=%d upload_id=%d error=%v",
				project.ID, ingestResult.Upload.ID, removeErr,
			)
		}
	}
	if err != nil {
		return nil, err
	}
	winner, err := upload.GetActiveUpload(ctx, winnerUploadID)
	if err != nil {
		return nil, err
	}
	deployment.UploadID = winnerUploadID
	deployment.ArtifactPath = ""
	return &winner, nil
}

func attachLegacyDeploymentUpload(
	ctx context.Context,
	projectID uint,
	deploymentID uint,
	uploadID uint64,
) (uint64, error) {
	winnerUploadID := uint64(0)
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		winnerUploadID, err = attachLegacyDeploymentUploadTx(tx, projectID, deploymentID, uploadID)
		return err
	})
	return winnerUploadID, err
}

func attachLegacyDeploymentUploadTx(
	tx *gorm.DB,
	projectID uint,
	deploymentID uint,
	uploadID uint64,
) (uint64, error) {
	var lockedProject model.PagesProject
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&lockedProject, projectID).Error; err != nil {
		return 0, err
	}
	var lockedDeployment model.PagesDeployment
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&lockedDeployment, deploymentID).Error; err != nil {
		return 0, err
	}
	if lockedDeployment.ProjectID != lockedProject.ID {
		return 0, errors.New(errPagesDeploymentMismatch)
	}
	if lockedDeployment.UploadID != 0 {
		return lockedDeployment.UploadID, nil
	}

	var uploadRecord model.Upload
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("id = ?", uploadID).
		First(&uploadRecord).Error; err != nil {
		return 0, err
	}
	if uploadRecord.Status != model.UploadStatusUsed || uploadRecord.Type != upload.ReservedPagesDeploymentType {
		return 0, errors.New(errPagesPackageUploadMissing)
	}
	result := tx.Model(&model.PagesDeployment{}).
		Where("id = ? AND project_id = ? AND upload_id = 0", lockedDeployment.ID, lockedProject.ID).
		Updates(map[string]any{
			"upload_id":     uploadRecord.ID,
			"artifact_path": "",
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, errors.New(errPagesPackageUploadMissing)
	}
	return uploadRecord.ID, nil
}

// ensureDeploymentInActiveSnapshot allows download of a specific deployment when
// it is the project's current active deployment and the project is used by the
// active main config.
func ensureDeploymentInActiveSnapshot(ctx context.Context, deploymentID uint) error {
	deployment, err := model.GetPagesDeploymentByID(ctx, deploymentID)
	if err != nil {
		return err
	}
	project, err := model.GetPagesProjectByID(ctx, deployment.ProjectID)
	if err != nil {
		return err
	}
	if project.ActiveDeploymentID == nil || *project.ActiveDeploymentID != deployment.ID {
		return errors.New(errPagesPackageNotInActiveConfig)
	}
	return ensureProjectInActiveConfig(ctx, project.ID)
}

// ensureProjectInActiveConfig checks that the Pages project is referenced by at
// least one pages route in the active main config snapshot.
func ensureProjectInActiveConfig(ctx context.Context, projectID uint) error {
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
		if !strings.EqualFold(strings.TrimSpace(route.UpstreamType), "pages") {
			continue
		}
		if route.PagesProjectID != nil && *route.PagesProjectID == projectID {
			return nil
		}
		if route.PagesDeployment == nil {
			continue
		}
		if route.PagesDeployment.ProjectID == projectID {
			return nil
		}
		if route.PagesDeployment.DeploymentID == 0 {
			continue
		}
		if route.PagesDeployment.DeploymentID != 0 {
			// Historical snapshot may only pin deployment_id.
			snapDeployment, snapErr := model.GetPagesDeploymentByID(ctx, route.PagesDeployment.DeploymentID)
			if snapErr != nil {
				continue
			}
			if snapDeployment.ProjectID == projectID {
				return nil
			}
		}
	}
	return errors.New(errPagesPackageNotInActiveConfig)
}

type snapshotPagesDeployment struct {
	DeploymentID uint `json:"deployment_id"`
	ProjectID    uint `json:"project_id"`
}

type snapshotRouteRef struct {
	UpstreamType    string                   `json:"upstream_type"`
	PagesProjectID  *uint                    `json:"pages_project_id"`
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
	var removed model.PagesDeployment
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var project model.PagesProject
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&project, projectID).Error; err != nil {
			return err
		}
		if err := tx.First(&removed, deploymentID).Error; err != nil {
			return err
		}
		if removed.ProjectID != project.ID {
			return errors.New(errPagesDeploymentMismatch)
		}
		if project.ActiveDeploymentID != nil && *project.ActiveDeploymentID == removed.ID {
			return errors.New(errPagesDeleteActiveDeploy)
		}
		if err := tx.Where("deployment_id = ?", removed.ID).Delete(&model.PagesDeploymentFile{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&removed).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	removeDeploymentArtifact(ctx, projectID, &removed)
	return nil
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
		if err != nil || (parsedURL.Scheme != remoteSourceSchemeHTTP && parsedURL.Scheme != remoteSourceSchemeHTTPS) || parsedURL.Host == "" {
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
	entryFile, err := validateAndNormalizePagesEntryFile(input.EntryFile)
	if err != nil {
		return nil, err
	}
	existing.EntryFile = entryFile

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
		UploadID:         deployment.UploadID,
		FileCount:        deployment.FileCount,
		TotalSize:        deployment.TotalSize,
		CreatedBy:        deployment.CreatedBy,
		SourceType:       deployment.SourceType,
		SourceLabel:      deployment.SourceLabel,
		TriggerType:      deployment.TriggerType,
		CreatedAt:        deployment.CreatedAt,
		ActivatedAt:      deployment.ActivatedAt,
	}
}
