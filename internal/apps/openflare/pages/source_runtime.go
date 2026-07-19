// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	pagesSourceCheckLeaseDuration = 2 * time.Minute
	pagesSourceSyncLeaseDuration  = 15 * time.Minute
	sourceLeaseTokenBytes         = 32
	sourceRuntimeErrorMaxBytes    = 512
	sourceRevisionHexLength       = 64

	sourceRuntimeColumnSyncStatus     = "sync_status"
	sourceRuntimeColumnLastError      = "last_error"
	sourceRuntimeColumnLeaseToken     = "lease_token"
	sourceRuntimeColumnLeaseExpiresAt = "lease_expires_at"
)

type sourceLeaseOutcome string

const (
	sourceLeaseAcquired sourceLeaseOutcome = "acquired"
	sourceLeaseBusy     sourceLeaseOutcome = "busy"
	sourceLeaseStale    sourceLeaseOutcome = "stale"
)

// sourceExecutionSnapshot captures every mutable value that can affect archive
// validation or the atomic activation decision. The queued payload deliberately
// does not carry project content configuration.
type sourceExecutionSnapshot struct {
	ProjectID            uint
	SourceID             uint
	SourceConfigVersion  int
	ContentConfigVersion int
	SourceType           string
	SourceIdentity       string
	RemoteURL            string
	RemoteNetworkPolicy  string
	RootDir              string
	EntryFile            string
	LeaseToken           string
	LeaseExpiresAt       time.Time
}

func acquireSourceLease(
	ctx context.Context,
	sourceID uint,
	expectedConfigVersion int,
	action string,
) (*sourceExecutionSnapshot, sourceLeaseOutcome, error) {
	leaseDuration, status, err := sourceLeaseParameters(action)
	if err != nil {
		return nil, sourceLeaseStale, err
	}
	token, err := newSourceLeaseToken()
	if err != nil {
		return nil, sourceLeaseStale, err
	}
	now := time.Now()
	expiresAt := now.Add(leaseDuration)

	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", sourceID).
		Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now).
		Where(
			"EXISTS (SELECT 1 FROM of_pages_project_sources source WHERE source.id = ? AND source.config_version = ?)",
			sourceID,
			expectedConfigVersion,
		).
		Updates(map[string]any{
			sourceRuntimeColumnLeaseToken:     token,
			sourceRuntimeColumnLeaseExpiresAt: expiresAt,
			sourceRuntimeColumnSyncStatus:     status,
			sourceRuntimeColumnLastError:      "",
		})
	if result.Error != nil {
		return nil, sourceLeaseStale, result.Error
	}
	if result.RowsAffected == 0 {
		outcome, inspectErr := inspectSourceLeaseMiss(ctx, sourceID, expectedConfigVersion, now)
		return nil, outcome, inspectErr
	}

	snapshot, err := loadSourceExecutionSnapshot(ctx, sourceID, token)
	if err != nil {
		if errors.Is(err, errSourceLeaseSnapshotStale) {
			return nil, sourceLeaseStale, nil
		}
		return nil, sourceLeaseStale, err
	}
	return snapshot, sourceLeaseAcquired, nil
}

var errSourceLeaseSnapshotStale = errors.New("source lease snapshot stale")

func loadSourceExecutionSnapshot(
	ctx context.Context,
	sourceID uint,
	token string,
) (*sourceExecutionSnapshot, error) {
	var snapshot sourceExecutionSnapshot
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		var source model.PagesProjectSource
		if err := tx.Where("id = ?", sourceID).First(&source).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSourceLeaseSnapshotStale
			}
			return err
		}
		var project model.PagesProject
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
			First(&project, source.ProjectID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSourceLeaseSnapshotStale
			}
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
			Where("id = ?", sourceID).
			First(&source).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSourceLeaseSnapshotStale
			}
			return err
		}
		var runtime model.PagesProjectSourceRuntime
		if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
			Where("source_id = ?", source.ID).
			First(&runtime).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSourceLeaseSnapshotStale
			}
			return err
		}
		// 必须在 runtime 行锁拿到后重新取时间，避免锁等待跨过
		// lease expiry 时仍使用事务开始前的旧时间继续执行。
		now := time.Now()
		if runtime.LeaseToken != token || runtime.LeaseExpiresAt == nil || !runtime.LeaseExpiresAt.After(now) {
			return errSourceLeaseSnapshotStale
		}
		snapshot = sourceExecutionSnapshot{
			ProjectID:            project.ID,
			SourceID:             source.ID,
			SourceConfigVersion:  source.ConfigVersion,
			ContentConfigVersion: project.ContentConfigVersion,
			SourceType:           source.SourceType,
			SourceIdentity:       source.SourceIdentity,
			RemoteURL:            source.RemoteURL,
			RemoteNetworkPolicy:  source.RemoteNetworkPolicy,
			RootDir:              project.RootDir,
			EntryFile:            project.EntryFile,
			LeaseToken:           token,
			LeaseExpiresAt:       *runtime.LeaseExpiresAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func inspectSourceLeaseMiss(
	ctx context.Context,
	sourceID uint,
	expectedConfigVersion int,
	now time.Time,
) (sourceLeaseOutcome, error) {
	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("id = ?", sourceID).First(&source).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sourceLeaseStale, nil
		}
		return sourceLeaseStale, err
	}
	if source.ConfigVersion != expectedConfigVersion {
		return sourceLeaseStale, nil
	}
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", sourceID).First(&runtime).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sourceLeaseStale, nil
		}
		return sourceLeaseStale, err
	}
	if runtime.LeaseExpiresAt != nil && runtime.LeaseExpiresAt.After(now) {
		return sourceLeaseBusy, nil
	}
	return sourceLeaseStale, nil
}

func sourceLeaseParameters(action string) (time.Duration, string, error) {
	switch action {
	case sourceActionCheck:
		return pagesSourceCheckLeaseDuration, pagesSourceStatusChecking, nil
	case sourceActionSync:
		return pagesSourceSyncLeaseDuration, pagesSourceStatusSyncing, nil
	default:
		return 0, "", errors.New(errPagesSourceActionInvalid)
	}
}

func newSourceLeaseToken() (string, error) {
	value := make([]byte, sourceLeaseTokenBytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func renewSourceLease(ctx context.Context, snapshot *sourceExecutionSnapshot, duration time.Duration) (bool, error) {
	if snapshot == nil || snapshot.SourceID == 0 || snapshot.LeaseToken == "" || duration <= 0 {
		return false, errors.New(errPagesSourceLeaseLost)
	}
	now := time.Now()
	expiresAt := now.Add(duration)
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ? AND lease_token = ? AND lease_expires_at > ?", snapshot.SourceID, snapshot.LeaseToken, now).
		Updates(map[string]any{sourceRuntimeColumnLeaseExpiresAt: expiresAt})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	snapshot.LeaseExpiresAt = expiresAt
	return true, nil
}

func failSourceLease(ctx context.Context, snapshot *sourceExecutionSnapshot, message string) error {
	if snapshot == nil || snapshot.SourceID == 0 || snapshot.LeaseToken == "" {
		return nil
	}
	message = safeSourceRuntimeError(message)
	now := time.Now()
	return db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ? AND lease_token = ? AND lease_expires_at > ?", snapshot.SourceID, snapshot.LeaseToken, now).
		Updates(map[string]any{
			sourceRuntimeColumnSyncStatus:     pagesSourceStatusFailed,
			sourceRuntimeColumnLastError:      message,
			sourceRuntimeColumnLeaseToken:     "",
			sourceRuntimeColumnLeaseExpiresAt: nil,
		}).Error
}

func safeSourceRuntimeError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return errPagesSourceSyncFailed
	}
	if len(message) > sourceRuntimeErrorMaxBytes {
		message = message[:sourceRuntimeErrorMaxBytes]
	}
	return message
}

func sourceLeaseIsBusy(ctx context.Context, sourceID uint) (bool, error) {
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", sourceID).First(&runtime).Error; err != nil {
		return false, err
	}
	return runtime.LeaseExpiresAt != nil && runtime.LeaseExpiresAt.After(time.Now()), nil
}

// fenceAndNormalizeRuntime invalidates in-flight work while preserving safe
// seen/applied cursors. The caller must already hold the source row lock.
func fenceAndNormalizeRuntime(tx *gorm.DB, sourceID uint) error {
	var runtime model.PagesProjectSourceRuntime
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("source_id = ?", sourceID).
		First(&runtime).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return tx.Model(&runtime).Updates(map[string]any{
		sourceRuntimeColumnLeaseToken:     "",
		sourceRuntimeColumnLeaseExpiresAt: nil,
		sourceRuntimeColumnSyncStatus:     normalizedSourceRuntimeStatus(&runtime),
	}).Error
}

func sourceHasSameReleaseReplacement(runtime *model.PagesProjectSourceRuntime) bool {
	if runtime == nil || runtime.LastSeenRevision == "" || runtime.LastSeenRevision == runtime.LastAppliedRevision {
		return false
	}
	seen := sourceDetail{}
	applied := sourceDetail{}
	if unmarshalSourceDetail(runtime.LastSeenDetail, &seen) != nil ||
		unmarshalSourceDetail(runtime.LastAppliedDetail, &applied) != nil {
		return false
	}
	return seen.ReleaseID != "" && seen.ReleaseID == applied.ReleaseID
}
