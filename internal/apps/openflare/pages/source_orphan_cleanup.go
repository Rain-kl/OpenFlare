// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const pagesOrphanUploadIsolation = 2 * time.Hour

// PagesOrphanCleanupSummary describes one bounded delayed compensation pass.
// Every candidate is counted exactly once in one outcome field.
//
//nolint:revive // Keep the domain-qualified exported name for scanner/task result clarity.
type PagesOrphanCleanupSummary struct {
	Candidates    int `json:"candidates"`
	Reconciled    int `json:"reconciled"`
	Referenced    int `json:"referenced"`
	LeaseBusy     int `json:"lease_busy"`
	InvalidMarker int `json:"invalid_marker"`
	Skipped       int `json:"skipped"`
	Failed        int `json:"failed"`
}

type pagesOrphanMarker struct {
	ProjectID uint
	SourceID  *uint
}

type pagesOrphanCleanupOutcome uint8

const (
	pagesOrphanCleanupSkipped pagesOrphanCleanupOutcome = iota
	pagesOrphanCleanupReconciled
	pagesOrphanCleanupReferenced
	pagesOrphanCleanupLeaseBusy
	pagesOrphanCleanupInvalidMarker
)

// ReconcilePagesOrphanUploads performs one bounded delayed compensation pass.
// Individual candidate failures are counted and logged so they do not prevent
// the scanner from continuing with source checks.
func ReconcilePagesOrphanUploads(
	ctx context.Context,
	now time.Time,
) (PagesOrphanCleanupSummary, error) {
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.UTC().Add(-pagesOrphanUploadIsolation)
	systemUser := repository.GetSystemUser(ctx)
	candidates, err := model.ListPagesOrphanUploadCandidates(ctx, model.PagesOrphanUploadCandidateQuery{
		SystemUserID:  systemUser.ID,
		UploadType:    upload.ReservedPagesDeploymentType,
		Marker:        pagesIngestMarkerV2,
		CreatedBefore: cutoff,
	})
	if err != nil {
		return PagesOrphanCleanupSummary{}, err
	}

	summary := PagesOrphanCleanupSummary{Candidates: len(candidates)}
	for index := range candidates {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		candidate := &candidates[index]
		marker, err := parsePagesOrphanMarker(candidate.Metadata)
		if err != nil {
			summary.InvalidMarker++
			logger.WarnF(ctx, "[PagesSource] orphan upload marker invalid: upload_id=%d error=%v", candidate.ID, err)
			continue
		}

		outcome, err := reconcilePagesOrphanUploadCandidate(
			ctx,
			candidate,
			marker,
			systemUser.ID,
			cutoff,
		)
		if err != nil {
			summary.Failed++
			logger.WarnF(ctx, "[PagesSource] orphan upload reconciliation failed: upload_id=%d error=%v", candidate.ID, err)
			continue
		}
		summary.add(outcome)
	}
	return summary, nil
}

func (summary *PagesOrphanCleanupSummary) add(outcome pagesOrphanCleanupOutcome) {
	switch outcome {
	case pagesOrphanCleanupReconciled:
		summary.Reconciled++
	case pagesOrphanCleanupReferenced:
		summary.Referenced++
	case pagesOrphanCleanupLeaseBusy:
		summary.LeaseBusy++
	case pagesOrphanCleanupInvalidMarker:
		summary.InvalidMarker++
	default:
		summary.Skipped++
	}
}

func reconcilePagesOrphanUploadCandidate(
	ctx context.Context,
	candidate *model.Upload,
	marker pagesOrphanMarker,
	systemUserID uint64,
	cutoff time.Time,
) (pagesOrphanCleanupOutcome, error) {
	if candidate == nil || candidate.ID == 0 {
		return pagesOrphanCleanupSkipped, nil
	}

	outcome := pagesOrphanCleanupSkipped
	uploadLocked := false
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		scopeOutcome, proceed, err := lockPagesOrphanCleanupScope(ctx, tx, candidate.ID, marker)
		if err != nil {
			return err
		}
		if !proceed {
			outcome = scopeOutcome
			return nil
		}
		lockedOutcome, locked, err := reconcileLockedPagesOrphanUpload(
			ctx,
			tx,
			candidate.ID,
			marker,
			systemUserID,
			cutoff,
		)
		if err != nil {
			return err
		}
		outcome = lockedOutcome
		uploadLocked = locked
		return nil
	})
	if err != nil {
		return pagesOrphanCleanupSkipped, err
	}
	if uploadLocked {
		// Also heal a prior post-commit cache invalidation interruption when the
		// status transition was an idempotent no-op.
		upload.InvalidateUploadMetaCache(ctx, candidate.ID)
	}
	return outcome, nil
}

func lockPagesOrphanCleanupScope(
	ctx context.Context,
	tx *gorm.DB,
	uploadID uint64,
	marker pagesOrphanMarker,
) (pagesOrphanCleanupOutcome, bool, error) {
	var project model.PagesProject
	if _, err := lockOptionalPagesCleanupRecord(tx, &project, "id = ?", marker.ProjectID); err != nil {
		return pagesOrphanCleanupSkipped, false, err
	}
	if marker.SourceID == nil {
		return pagesOrphanCleanupSkipped, true, nil
	}

	var source model.PagesProjectSource
	sourceExists, err := lockOptionalPagesCleanupRecord(tx, &source, "id = ?", *marker.SourceID)
	if err != nil {
		return pagesOrphanCleanupSkipped, false, err
	}
	if !sourceExists {
		return pagesOrphanCleanupSkipped, true, nil
	}
	if source.ProjectID != marker.ProjectID {
		logger.WarnF(ctx,
			"[PagesSource] orphan upload source ownership mismatch: upload_id=%d project_id=%d source_id=%d source_project_id=%d",
			uploadID,
			marker.ProjectID,
			*marker.SourceID,
			source.ProjectID,
		)
		return pagesOrphanCleanupInvalidMarker, false, nil
	}

	var runtime model.PagesProjectSourceRuntime
	runtimeExists, err := lockOptionalPagesCleanupRecord(tx, &runtime, "source_id = ?", source.ID)
	if err != nil {
		return pagesOrphanCleanupSkipped, false, err
	}
	// Read the real clock only after obtaining the runtime row lock. The scanner
	// snapshot time is only an isolation cutoff and may be stale after lock wait.
	leaseCheckedAt := time.Now()
	if runtimeExists && runtime.LeaseExpiresAt != nil && runtime.LeaseExpiresAt.After(leaseCheckedAt) {
		return pagesOrphanCleanupLeaseBusy, false, nil
	}
	return pagesOrphanCleanupSkipped, true, nil
}

func reconcileLockedPagesOrphanUpload(
	ctx context.Context,
	tx *gorm.DB,
	uploadID uint64,
	marker pagesOrphanMarker,
	systemUserID uint64,
	cutoff time.Time,
) (pagesOrphanCleanupOutcome, bool, error) {
	var lockedUpload model.Upload
	found, err := lockOptionalPagesCleanupRecord(tx, &lockedUpload, "id = ?", uploadID)
	if err != nil || !found {
		return pagesOrphanCleanupSkipped, false, err
	}

	lockedMarker, err := parsePagesOrphanMarker(lockedUpload.Metadata)
	if err != nil {
		logger.WarnF(ctx, "[PagesSource] orphan upload marker changed or invalid: upload_id=%d error=%v", uploadID, err)
		return pagesOrphanCleanupInvalidMarker, true, nil
	}
	if lockedUpload.Status != model.UploadStatusUsed ||
		lockedUpload.UserID != systemUserID ||
		lockedUpload.Type != upload.ReservedPagesDeploymentType ||
		!lockedUpload.CreatedAt.Before(cutoff) {
		return pagesOrphanCleanupSkipped, true, nil
	}
	if lockedMarker.ProjectID != marker.ProjectID || !sameOptionalPagesSourceID(lockedMarker.SourceID, marker.SourceID) {
		logger.WarnF(ctx, "[PagesSource] orphan upload marker changed during reconciliation: upload_id=%d", uploadID)
		return pagesOrphanCleanupInvalidMarker, true, nil
	}

	var references int64
	if err := tx.Model(&model.PagesDeployment{}).
		Where("upload_id = ?", lockedUpload.ID).
		Count(&references).Error; err != nil {
		return pagesOrphanCleanupSkipped, true, err
	}
	if references > 0 {
		return pagesOrphanCleanupReferenced, true, nil
	}

	transitioned, err := upload.RemoveLockedTx(tx, &lockedUpload)
	if err != nil {
		return pagesOrphanCleanupSkipped, true, err
	}
	if transitioned {
		return pagesOrphanCleanupReconciled, true, nil
	}
	return pagesOrphanCleanupSkipped, true, nil
}

func lockOptionalPagesCleanupRecord(
	tx *gorm.DB,
	value any,
	query string,
	args ...any,
) (bool, error) {
	err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where(query, args...).
		First(value).Error
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func parsePagesOrphanMarker(metadata model.UploadMetadata) (pagesOrphanMarker, error) {
	if metadata.Extra == nil {
		return pagesOrphanMarker{}, errors.New("pages marker metadata missing")
	}
	marker, ok := metadata.Extra[pagesIngestMarkerKey].(string)
	if !ok || marker != pagesIngestMarkerV2 {
		return pagesOrphanMarker{}, errors.New("pages marker version invalid")
	}
	projectID, err := parsePagesOrphanMetadataID(metadata.Extra, pagesProjectIDMetadataKey)
	if err != nil {
		return pagesOrphanMarker{}, err
	}
	result := pagesOrphanMarker{ProjectID: projectID}
	if _, exists := metadata.Extra[pagesSourceIDMetadataKey]; exists {
		sourceID, err := parsePagesOrphanMetadataID(metadata.Extra, pagesSourceIDMetadataKey)
		if err != nil {
			return pagesOrphanMarker{}, err
		}
		result.SourceID = &sourceID
	}
	return result, nil
}

func parsePagesOrphanMetadataID(extra map[string]any, key string) (uint, error) {
	raw, exists := extra[key]
	if !exists {
		return 0, fmt.Errorf("pages marker %s missing", key)
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return 0, fmt.Errorf("pages marker %s must be a decimal string", key)
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	maxModelID := uint64(^uint(0) >> 1)
	if err != nil || parsed == 0 || parsed > maxModelID || strconv.FormatUint(parsed, 10) != value {
		return 0, fmt.Errorf("pages marker %s is not a canonical non-zero decimal ID", key)
	}
	return uint(parsed), nil
}

func sameOptionalPagesSourceID(left, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
