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

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const (
	pagesSourceCheckLeaseDuration = 2 * time.Minute
	pagesSourceSyncLeaseDuration  = 15 * time.Minute
	sourceLeaseTokenBytes         = 32
	sourceRuntimeErrorMaxBytes    = 512
	sourceRevisionHexLength       = 64

	sourceColumnAutoUpdateEnabled     = "auto_update_enabled"
	sourceColumnConfigVersion         = "config_version"
	sourceRuntimeColumnSyncStatus     = "sync_status"
	sourceRuntimeColumnLastError      = "last_error"
	sourceRuntimeColumnLastCheckedAt  = "last_checked_at"
	sourceRuntimeColumnNextCheckAt    = "next_check_at"
	sourceRuntimeColumnLeaseToken     = "lease_token"
	sourceRuntimeColumnLeaseExpiresAt = "lease_expires_at"
	pagesDeploymentColumnStatus       = "status"
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
	AllowInsecure        bool
	GitHubRepository     string
	ReleaseSelector      string
	ReleaseTag           string
	AssetName            string
	AutoUpdateEnabled    bool
	CheckIntervalMinutes int
	ETag                 string
	LastSeenRevision     string
	LastSeenDetail       string
	LastAppliedRevision  string
	LastAppliedDetail    string
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

	rows, err := repository.TryAcquirePagesSourceRuntimeLease(ctx, sourceID, expectedConfigVersion, now, map[string]any{
		sourceRuntimeColumnLeaseToken:     token,
		sourceRuntimeColumnLeaseExpiresAt: expiresAt,
		sourceRuntimeColumnSyncStatus:     status,
		sourceRuntimeColumnLastError:      "",
	})
	if err != nil {
		return nil, sourceLeaseStale, err
	}
	if rows == 0 {
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
	err := repository.WithPagesTx(ctx, func(tx *gorm.DB) error {
		source, err := repository.GetPagesProjectSourceByIDTx(tx, sourceID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSourceLeaseSnapshotStale
			}
			return err
		}
		project, err := repository.LockPagesProjectByIDTx(tx, source.ProjectID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSourceLeaseSnapshotStale
			}
			return err
		}
		source, err = repository.LockPagesProjectSourceByIDTx(tx, sourceID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errSourceLeaseSnapshotStale
			}
			return err
		}
		runtime, err := repository.LockPagesProjectSourceRuntimeBySourceIDTx(tx, source.ID)
		if err != nil {
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
			AllowInsecure:        source.AllowInsecure,
			GitHubRepository:     source.GitHubRepository,
			ReleaseSelector:      source.ReleaseSelector,
			ReleaseTag:           source.ReleaseTag,
			AssetName:            source.AssetName,
			AutoUpdateEnabled:    source.AutoUpdateEnabled,
			CheckIntervalMinutes: source.CheckIntervalMinutes,
			ETag:                 runtime.ETag,
			LastSeenRevision:     runtime.LastSeenRevision,
			LastSeenDetail:       runtime.LastSeenDetail,
			LastAppliedRevision:  runtime.LastAppliedRevision,
			LastAppliedDetail:    runtime.LastAppliedDetail,
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
	source, err := repository.GetPagesProjectSourceByID(ctx, sourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sourceLeaseStale, nil
		}
		return sourceLeaseStale, err
	}
	if source.ConfigVersion != expectedConfigVersion {
		return sourceLeaseStale, nil
	}
	runtime, err := repository.GetPagesProjectSourceRuntimeBySourceID(ctx, sourceID)
	if err != nil {
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
	rows, err := repository.RenewPagesSourceRuntimeLease(ctx, snapshot.SourceID, snapshot.LeaseToken, now, expiresAt)
	if err != nil {
		return false, err
	}
	if rows == 0 {
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
	_, err := repository.UpdatePagesSourceRuntimeByActiveLease(
		ctx,
		snapshot.SourceID,
		snapshot.LeaseToken,
		now,
		map[string]any{
			sourceRuntimeColumnSyncStatus:     pagesSourceStatusFailed,
			sourceRuntimeColumnLastError:      message,
			sourceRuntimeColumnLeaseToken:     "",
			sourceRuntimeColumnLeaseExpiresAt: nil,
		},
	)
	return err
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
	runtime, err := repository.GetPagesProjectSourceRuntimeBySourceID(ctx, sourceID)
	if err != nil {
		return false, err
	}
	return runtime.LeaseExpiresAt != nil && runtime.LeaseExpiresAt.After(time.Now()), nil
}

// recoverExpiredSourceLease clears one exact expired lease owner. Matching the
// token, observed expiry and status prevents a scanner from overwriting a
// worker that renewed or was replaced after the candidate query.
func recoverExpiredSourceLease(
	ctx context.Context,
	sourceID uint,
	token string,
	expiresAt time.Time,
	status string,
	now time.Time,
	nextCheckAt *time.Time,
) (bool, error) {
	if sourceID == 0 || token == "" ||
		(status != pagesSourceStatusChecking && status != pagesSourceStatusSyncing) {
		return false, nil
	}
	updates := map[string]any{
		sourceRuntimeColumnSyncStatus:     pagesSourceStatusFailed,
		sourceRuntimeColumnLastError:      errPagesSourceLeaseExpired,
		sourceRuntimeColumnLeaseToken:     "",
		sourceRuntimeColumnLeaseExpiresAt: nil,
		sourceRuntimeColumnNextCheckAt:    nextCheckAt,
	}
	rows, err := repository.RecoverExpiredPagesSourceRuntimeLease(
		ctx, sourceID, token, expiresAt, status, now, updates,
	)
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}

// fenceAndNormalizeRuntime invalidates in-flight work while preserving safe
// seen/applied cursors. The caller must already hold the source row lock.
func fenceAndNormalizeRuntime(tx *gorm.DB, sourceID uint) error {
	runtime, err := repository.LockPagesProjectSourceRuntimeBySourceIDTx(tx, sourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return repository.UpdatePagesProjectSourceRuntimeTx(tx, runtime, map[string]any{
		sourceRuntimeColumnLeaseToken:     "",
		sourceRuntimeColumnLeaseExpiresAt: nil,
		sourceRuntimeColumnSyncStatus:     normalizedSourceRuntimeStatus(runtime),
	})
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
