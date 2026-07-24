// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
)

const pagesRowLockStrength = "UPDATE"

// WithPagesTx runs fn inside a database transaction for Pages multi-step work.
func WithPagesTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return db.DB(ctx).Transaction(fn)
}

// GetPagesProjectSourceByID loads a project source by primary key.
func GetPagesProjectSourceByID(ctx context.Context, id uint) (*model.PagesProjectSource, error) {
	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("id = ?", id).First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// GetPagesProjectSourceByProjectID loads the unique source for a project.
func GetPagesProjectSourceByProjectID(ctx context.Context, projectID uint) (*model.PagesProjectSource, error) {
	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("project_id = ?", projectID).First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// GetPagesProjectSourceByIDAndConfigVersion loads a source matching both id and config version.
func GetPagesProjectSourceByIDAndConfigVersion(
	ctx context.Context,
	id uint,
	configVersion int,
) (*model.PagesProjectSource, error) {
	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("id = ? AND config_version = ?", id, configVersion).First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// GetPagesProjectSourceRuntimeBySourceID loads runtime for a source.
func GetPagesProjectSourceRuntimeBySourceID(
	ctx context.Context,
	sourceID uint,
) (*model.PagesProjectSourceRuntime, error) {
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", sourceID).First(&runtime).Error; err != nil {
		return nil, err
	}
	return &runtime, nil
}

// GetPagesProjectSourceAndRuntimeByProjectID loads source and its runtime for a project.
func GetPagesProjectSourceAndRuntimeByProjectID(
	ctx context.Context,
	projectID uint,
) (*model.PagesProjectSource, *model.PagesProjectSourceRuntime, error) {
	source, err := GetPagesProjectSourceByProjectID(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	runtime, err := GetPagesProjectSourceRuntimeBySourceID(ctx, source.ID)
	if err != nil {
		return nil, nil, err
	}
	return source, runtime, nil
}

// CreatePagesProjectSourceTx creates a source row inside an existing transaction.
func CreatePagesProjectSourceTx(tx *gorm.DB, source *model.PagesProjectSource) error {
	return tx.Create(source).Error
}

// CreatePagesProjectSourceRuntimeTx creates a runtime row inside an existing transaction.
func CreatePagesProjectSourceRuntimeTx(tx *gorm.DB, runtime *model.PagesProjectSourceRuntime) error {
	return tx.Create(runtime).Error
}

// UpdatePagesProjectSourceTx applies partial updates to a source inside a transaction.
func UpdatePagesProjectSourceTx(tx *gorm.DB, source *model.PagesProjectSource, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return tx.Model(source).Updates(updates).Error
}

// UpdatePagesProjectSourceRuntimeTx applies partial updates to a runtime inside a transaction.
func UpdatePagesProjectSourceRuntimeTx(
	tx *gorm.DB,
	runtime *model.PagesProjectSourceRuntime,
	updates map[string]any,
) error {
	if len(updates) == 0 {
		return nil
	}
	return tx.Model(runtime).Updates(updates).Error
}

// UpdatePagesProjectSourceRuntimeFieldTx updates a single column on a runtime row.
func UpdatePagesProjectSourceRuntimeFieldTx(
	tx *gorm.DB,
	runtime *model.PagesProjectSourceRuntime,
	column string,
	value any,
) error {
	return tx.Model(runtime).Update(column, value).Error
}

// DeletePagesProjectSourceRuntimeBySourceIDTx deletes runtime rows for a source.
func DeletePagesProjectSourceRuntimeBySourceIDTx(tx *gorm.DB, sourceID uint) error {
	return tx.Where("source_id = ?", sourceID).Delete(&model.PagesProjectSourceRuntime{}).Error
}

// DeletePagesProjectSourceTx deletes a source row inside a transaction.
func DeletePagesProjectSourceTx(tx *gorm.DB, source *model.PagesProjectSource) error {
	return tx.Delete(source).Error
}

// LockPagesProjectByIDTx locks a project row for update.
func LockPagesProjectByIDTx(tx *gorm.DB, id uint) (*model.PagesProject, error) {
	var project model.PagesProject
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).First(&project, id).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

// LockPagesProjectSourceByProjectIDTx locks the source for a project.
func LockPagesProjectSourceByProjectIDTx(tx *gorm.DB, projectID uint) (*model.PagesProjectSource, error) {
	var source model.PagesProjectSource
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("project_id = ?", projectID).
		First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// LockPagesProjectSourceByIDTx locks a source by id.
func LockPagesProjectSourceByIDTx(tx *gorm.DB, sourceID uint) (*model.PagesProjectSource, error) {
	var source model.PagesProjectSource
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("id = ?", sourceID).
		First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// LockPagesProjectSourceByIDAndProjectIDTx locks a source matching both identifiers.
func LockPagesProjectSourceByIDAndProjectIDTx(
	tx *gorm.DB,
	sourceID uint,
	projectID uint,
) (*model.PagesProjectSource, error) {
	var source model.PagesProjectSource
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("id = ? AND project_id = ?", sourceID, projectID).
		First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// LockPagesProjectSourceRuntimeBySourceIDTx locks runtime for a source.
func LockPagesProjectSourceRuntimeBySourceIDTx(
	tx *gorm.DB,
	sourceID uint,
) (*model.PagesProjectSourceRuntime, error) {
	var runtime model.PagesProjectSourceRuntime
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("source_id = ?", sourceID).
		First(&runtime).Error; err != nil {
		return nil, err
	}
	return &runtime, nil
}

// GetPagesProjectSourceByIDTx loads a source by id without locking.
func GetPagesProjectSourceByIDTx(tx *gorm.DB, sourceID uint) (*model.PagesProjectSource, error) {
	var source model.PagesProjectSource
	if err := tx.Where("id = ?", sourceID).First(&source).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

// TryAcquirePagesSourceRuntimeLease conditionally claims an idle/expired lease when config matches.
func TryAcquirePagesSourceRuntimeLease(
	ctx context.Context,
	sourceID uint,
	expectedConfigVersion int,
	now time.Time,
	updates map[string]any,
) (int64, error) {
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", sourceID).
		Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now).
		Where(
			"EXISTS (SELECT 1 FROM of_pages_project_sources source WHERE source.id = ? AND source.config_version = ?)",
			sourceID,
			expectedConfigVersion,
		).
		Updates(updates)
	return result.RowsAffected, result.Error
}

// RenewPagesSourceRuntimeLease extends an active lease held by the given token.
func RenewPagesSourceRuntimeLease(
	ctx context.Context,
	sourceID uint,
	token string,
	now time.Time,
	expiresAt time.Time,
) (int64, error) {
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ? AND lease_token = ? AND lease_expires_at > ?", sourceID, token, now).
		Updates(map[string]any{"lease_expires_at": expiresAt})
	return result.RowsAffected, result.Error
}

// UpdatePagesSourceRuntimeByActiveLease updates runtime while the caller still owns the lease.
func UpdatePagesSourceRuntimeByActiveLease(
	ctx context.Context,
	sourceID uint,
	token string,
	now time.Time,
	updates map[string]any,
) (int64, error) {
	return UpdatePagesSourceRuntimeByActiveLeaseTx(db.DB(ctx), sourceID, token, now, updates)
}

// UpdatePagesSourceRuntimeByActiveLeaseTx updates runtime under an active lease inside a transaction.
func UpdatePagesSourceRuntimeByActiveLeaseTx(
	tx *gorm.DB,
	sourceID uint,
	token string,
	now time.Time,
	updates map[string]any,
) (int64, error) {
	result := tx.Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ? AND lease_token = ? AND lease_expires_at > ?", sourceID, token, now).
		Updates(updates)
	return result.RowsAffected, result.Error
}

// RecoverExpiredPagesSourceRuntimeLease clears one exact expired lease owner.
func RecoverExpiredPagesSourceRuntimeLease(
	ctx context.Context,
	sourceID uint,
	token string,
	expiresAt time.Time,
	status string,
	now time.Time,
	updates map[string]any,
) (int64, error) {
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", sourceID).
		Where("lease_token = ?", token).
		Where("lease_expires_at = ? AND lease_expires_at <= ?", expiresAt, now).
		Where("sync_status = ?", status).
		Updates(updates)
	return result.RowsAffected, result.Error
}

// MarkPagesSourceInitialCheckDispatchFailed marks runtime failed when config still matches and lease is free.
func MarkPagesSourceInitialCheckDispatchFailed(
	ctx context.Context,
	sourceID uint,
	configVersion int,
	now time.Time,
	updates map[string]any,
) (int64, error) {
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", sourceID).
		Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now).
		Where(
			"EXISTS (SELECT 1 FROM of_pages_project_sources source WHERE source.id = ? AND source.config_version = ?)",
			sourceID,
			configVersion,
		).
		Updates(updates)
	return result.RowsAffected, result.Error
}

// RecordPagesSourceAutoDispatchFailure records a failed auto-sync dispatch while status still matches.
func RecordPagesSourceAutoDispatchFailure(
	ctx context.Context,
	sourceID uint,
	configVersion int,
	sourceType string,
	releaseSelector string,
	revision string,
	updateAvailableStatus string,
	now time.Time,
	updates map[string]any,
) (int64, error) {
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", sourceID).
		Where("sync_status = ? AND last_seen_revision = ?", updateAvailableStatus, revision).
		Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now).
		Where(`EXISTS (
			SELECT 1 FROM of_pages_project_sources AS source
			WHERE source.id = ? AND source.config_version = ?
				AND source.source_type = ? AND source.release_selector = ?
				AND source.auto_update_enabled = ?
		)`,
			sourceID,
			configVersion,
			sourceType,
			releaseSelector,
			true,
		).
		Updates(updates)
	return result.RowsAffected, result.Error
}

// ListExpiredPagesSourceLeaseCandidates returns expired checking/syncing leases for recovery.
func ListExpiredPagesSourceLeaseCandidates(
	ctx context.Context,
	now time.Time,
	syncStatuses []string,
) ([]model.PagesExpiredSourceLeaseCandidate, error) {
	var candidates []model.PagesExpiredSourceLeaseCandidate
	err := db.DB(ctx).
		Table("of_pages_project_source_runtime AS runtime").
		Select(`runtime.source_id, runtime.lease_token, runtime.lease_expires_at,
			runtime.sync_status, source.source_type, source.release_selector`).
		Joins("JOIN of_pages_project_sources AS source ON source.id = runtime.source_id").
		Where("runtime.lease_token <> ''").
		Where("runtime.lease_expires_at IS NOT NULL AND runtime.lease_expires_at <= ?", now).
		Where("runtime.sync_status IN ?", syncStatuses).
		Order("runtime.source_id ASC").
		Scan(&candidates).Error
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

// CountDueGitHubPagesSourceChecks counts due latest GitHub sources.
func CountDueGitHubPagesSourceChecks(
	ctx context.Context,
	now time.Time,
	sourceType string,
	releaseSelector string,
) (int64, error) {
	var count int64
	err := dueGitHubPagesSourceQuery(ctx, now, sourceType, releaseSelector).Count(&count).Error
	return count, err
}

// ListDueGitHubPagesSourceChecks lists a batch of due latest GitHub sources in stable order.
func ListDueGitHubPagesSourceChecks(
	ctx context.Context,
	now time.Time,
	sourceType string,
	releaseSelector string,
	limit int,
) ([]model.PagesDueGitHubSourceCandidate, error) {
	var candidates []model.PagesDueGitHubSourceCandidate
	err := dueGitHubPagesSourceQuery(ctx, now, sourceType, releaseSelector).
		Select("source.id AS source_id, source.config_version").
		Order("runtime.next_check_at ASC").
		Order("source.id ASC").
		Limit(limit).
		Scan(&candidates).Error
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

func dueGitHubPagesSourceQuery(
	ctx context.Context,
	now time.Time,
	sourceType string,
	releaseSelector string,
) *gorm.DB {
	return db.DB(ctx).
		Table("of_pages_project_source_runtime AS runtime").
		Joins("JOIN of_pages_project_sources AS source ON source.id = runtime.source_id").
		Where("source.source_type = ?", sourceType).
		Where("source.release_selector = ?", releaseSelector).
		Where("runtime.next_check_at IS NOT NULL AND runtime.next_check_at <= ?", now)
}

// GetPagesDeploymentBySourceRevision loads a deployment by project source identity and revision.
func GetPagesDeploymentBySourceRevision(
	ctx context.Context,
	projectID uint,
	sourceIdentity string,
	revision string,
) (*model.PagesDeployment, error) {
	var deployment model.PagesDeployment
	err := db.DB(ctx).
		Where("project_id = ? AND source_identity = ? AND source_revision = ?", projectID, sourceIdentity, revision).
		First(&deployment).Error
	if err != nil {
		return nil, err
	}
	return &deployment, nil
}
