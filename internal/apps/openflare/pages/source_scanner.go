// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

const (
	// PagesSourceScanTask is the private Asynq task type for the periodic scanner.
	PagesSourceScanTask = "openflare:pages_source_scan"
	// TaskTypePagesSourceScan is the internal task meta type seeded in w_schedules.
	TaskTypePagesSourceScan = "of_pages_source_scan"

	pagesSourceScanBatchSize = 20
)

// PagesSourceScanMeta is available to the scheduler registry but hidden from
// generic Admin task dispatch and schedule mutation APIs.
var PagesSourceScanMeta = task.TaskMeta{
	Type:         TaskTypePagesSourceScan,
	AsynqTask:    PagesSourceScanTask,
	Name:         "OpenFlare Pages 部署源扫描",
	Description:  "补偿孤儿部署包、恢复过期执行权并串行检查到期的 GitHub latest 部署源",
	SupportsTime: false,
	MaxRetry:     0,
	Queue:        task.QueueDefault,
	Retryable:    false,
	InternalOnly: true,
}

type pagesSourceScanPayload struct{}

type pagesSourceScanSummary struct {
	ExpiredCandidates int                          `json:"expired_candidates"`
	RecoveredLeases   int                          `json:"recovered_leases"`
	OrphanCleanup     PagesOrphanCleanupSummary    `json:"orphan_cleanup"`
	DueSources        int                          `json:"due_sources"`
	SelectedSources   int                          `json:"selected_sources"`
	CheckedSources    int                          `json:"checked_sources"`
	UpdatesFound      int                          `json:"updates_found"`
	AttentionSources  int                          `json:"attention_sources"`
	DispatchedSyncs   int                          `json:"dispatched_syncs"`
	FailedDispatches  int                          `json:"failed_dispatches"`
	BusySources       int                          `json:"busy_sources"`
	StaleSources      int                          `json:"stale_sources"`
	FailedSources     int                          `json:"failed_sources"`
	Backlog           int                          `json:"backlog"`
	ProviderBackoffs  []pagesSourceProviderBackoff `json:"provider_backoffs,omitempty"`
}

type pagesSourceProviderBackoff struct {
	SourceID   uint   `json:"source_id"`
	StatusCode int    `json:"status_code"`
	RetryAt    string `json:"retry_at"`
}

type expiredSourceLeaseCandidate struct {
	SourceID        uint
	LeaseToken      string
	LeaseExpiresAt  time.Time
	SyncStatus      string
	SourceType      string
	ReleaseSelector string
}

type dueGitHubSourceCandidate struct {
	SourceID      uint
	ConfigVersion int
}

var (
	pagesSourceScanNow          = time.Now
	reconcilePagesSourceOrphans = ReconcilePagesOrphanUploads
	dispatchPagesSourceAutoSync = func(
		ctx context.Context,
		source model.PagesProjectSource,
		targetRevision string,
	) (*SourceActionReceipt, error) {
		return dispatchSourceActionSnapshotWithTrigger(
			ctx,
			source,
			sourceActionSync,
			pagesSourceCreatedBySystem,
			pagesSourceTriggerScheduledAutoUpdate,
			targetRevision,
			"",
			"system",
		)
	}
)

// SourceScanHandler serializes provider checks inside one scheduled task. A
// source-level lease still permits overlapping scanner executions safely.
type SourceScanHandler struct{}

// ValidatePayload accepts only an empty object; the scanner has no user input.
func (handler *SourceScanHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		payload = []byte("{}")
	}
	var input pagesSourceScanPayload
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	return []byte("{}"), nil
}

// Execute recovers expired leases and checks at most 20 due latest sources in
// stable order. Provider and dispatch failures are isolated per source.
func (handler *SourceScanHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	if _, err := handler.ValidatePayload(payload); err != nil {
		return nil, task.PermanentError(errPagesSourceActionInvalid)
	}

	now := pagesSourceScanNow()
	summary := pagesSourceScanSummary{}
	if err := recoverExpiredPagesSourceLeases(ctx, now, &summary); err != nil {
		return nil, err
	}
	orphanSummary, err := reconcilePagesSourceOrphans(ctx, now)
	if err != nil {
		return nil, err
	}
	summary.OrphanCleanup = orphanSummary
	task.AppendLog(
		ctx,
		"[cleanup] orphan 候选=%d，已补偿=%d，仍被引用=%d，lease busy=%d，非法 marker=%d，跳过=%d，失败=%d",
		orphanSummary.Candidates,
		orphanSummary.Reconciled,
		orphanSummary.Referenced,
		orphanSummary.LeaseBusy,
		orphanSummary.InvalidMarker,
		orphanSummary.Skipped,
		orphanSummary.Failed,
	)
	if err := scanDueGitHubSources(ctx, now, &summary); err != nil {
		return nil, err
	}

	detail, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	message := fmt.Sprintf(
		"Pages 部署源扫描完成：恢复 %d 个租约，补偿 %d 个孤儿记录，检查 %d 个来源，投递 %d 个自动更新，积压 %d 个",
		summary.RecoveredLeases,
		summary.OrphanCleanup.Reconciled,
		summary.CheckedSources,
		summary.DispatchedSyncs,
		summary.Backlog,
	)
	return &task.TaskResult{Message: message, Detail: string(detail)}, nil
}

func recoverExpiredPagesSourceLeases(
	ctx context.Context,
	now time.Time,
	summary *pagesSourceScanSummary,
) error {
	var candidates []expiredSourceLeaseCandidate
	err := db.DB(ctx).
		Table("of_pages_project_source_runtime AS runtime").
		Select(`runtime.source_id, runtime.lease_token, runtime.lease_expires_at,
			runtime.sync_status, source.source_type, source.release_selector`).
		Joins("JOIN of_pages_project_sources AS source ON source.id = runtime.source_id").
		Where("runtime.lease_token <> ''").
		Where("runtime.lease_expires_at IS NOT NULL AND runtime.lease_expires_at <= ?", now).
		Where("runtime.sync_status IN ?", []string{pagesSourceStatusChecking, pagesSourceStatusSyncing}).
		Order("runtime.source_id ASC").
		Scan(&candidates).Error
	if err != nil {
		return err
	}
	summary.ExpiredCandidates = len(candidates)
	for _, candidate := range candidates {
		var nextCheckAt *time.Time
		if candidate.SourceType == PagesSourceTypeGitHubRelease &&
			candidate.ReleaseSelector == githubReleaseSelectorLatest {
			next := nextGitHubCheckAt(now, candidate.SourceID, minimumCheckInterval)
			nextCheckAt = &next
		}
		recovered, recoverErr := recoverExpiredSourceLease(
			ctx,
			candidate.SourceID,
			candidate.LeaseToken,
			candidate.LeaseExpiresAt,
			candidate.SyncStatus,
			now,
			nextCheckAt,
		)
		if recoverErr != nil {
			summary.FailedSources++
			logger.WarnF(
				ctx,
				"[PagesSourceScan] recover expired lease failed: source_id=%d error=%v",
				candidate.SourceID,
				recoverErr,
			)
			continue
		}
		if recovered {
			summary.RecoveredLeases++
			task.AppendLog(ctx, "[recover] 已恢复过期来源租约：source_id=%d", candidate.SourceID)
		}
	}
	return nil
}

func scanDueGitHubSources(
	ctx context.Context,
	now time.Time,
	summary *pagesSourceScanSummary,
) error {
	dueQuery := func() *gorm.DB {
		return db.DB(ctx).
			Table("of_pages_project_source_runtime AS runtime").
			Joins("JOIN of_pages_project_sources AS source ON source.id = runtime.source_id").
			Where("source.source_type = ?", PagesSourceTypeGitHubRelease).
			Where("source.release_selector = ?", githubReleaseSelectorLatest).
			Where("runtime.next_check_at IS NOT NULL AND runtime.next_check_at <= ?", now)
	}
	var dueCount int64
	if err := dueQuery().Count(&dueCount).Error; err != nil {
		return err
	}
	summary.DueSources = int(dueCount)

	var candidates []dueGitHubSourceCandidate
	if err := dueQuery().
		Select("source.id AS source_id, source.config_version").
		Order("runtime.next_check_at ASC").
		Order("source.id ASC").
		Limit(pagesSourceScanBatchSize).
		Scan(&candidates).Error; err != nil {
		return err
	}
	summary.SelectedSources = len(candidates)
	task.AppendLog(
		ctx,
		"[scan] 到期来源=%d，本批=%d",
		summary.DueSources,
		summary.SelectedSources,
	)

	for _, candidate := range candidates {
		scanOneDueGitHubSource(ctx, candidate, summary)
	}
	var remainingDue int64
	if err := dueQuery().Count(&remainingDue).Error; err != nil {
		return err
	}
	summary.Backlog = int(remainingDue)
	task.AppendLog(ctx, "[scan] 本批处理后仍到期来源=%d", summary.Backlog)
	return nil
}

func scanOneDueGitHubSource(
	ctx context.Context,
	candidate dueGitHubSourceCandidate,
	summary *pagesSourceScanSummary,
) {
	snapshot, outcome, err := acquireSourceLease(
		ctx,
		candidate.SourceID,
		candidate.ConfigVersion,
		sourceActionCheck,
	)
	if err != nil {
		summary.FailedSources++
		logger.WarnF(ctx, "[PagesSourceScan] acquire check lease failed: source_id=%d error=%v", candidate.SourceID, err)
		return
	}
	switch outcome {
	case sourceLeaseBusy:
		summary.BusySources++
		task.AppendLog(ctx, "[check] 来源正在执行其它任务，跳过：source_id=%d", candidate.SourceID)
		return
	case sourceLeaseStale:
		summary.StaleSources++
		return
	}
	if snapshot == nil || snapshot.SourceType != PagesSourceTypeGitHubRelease ||
		snapshot.ReleaseSelector != githubReleaseSelectorLatest {
		summary.StaleSources++
		if snapshot != nil {
			if finalizeErr := failSourceLease(ctx, snapshot, errPagesSourceActionStale); finalizeErr != nil {
				logger.WarnF(
					ctx,
					"[PagesSourceScan] finalize stale source failed: source_id=%d error=%v",
					snapshot.SourceID,
					finalizeErr,
				)
			}
		}
		return
	}

	checkResult, checkErr := checkGitHubSource(ctx, snapshot)
	if checkErr != nil {
		summary.FailedSources++
		recordPagesSourceProviderBackoff(ctx, candidate.SourceID, checkErr, summary)
		logger.WarnF(
			ctx,
			"[PagesSourceScan] source check failed: source_id=%d error=%s",
			candidate.SourceID,
			safeGitHubSourceError(checkErr),
		)
		return
	}
	if checkResult == nil || checkResult.Stale {
		summary.StaleSources++
		return
	}
	handleCheckedGitHubSource(ctx, snapshot, checkResult, summary)
}

func recordPagesSourceProviderBackoff(
	ctx context.Context,
	sourceID uint,
	checkErr error,
	summary *pagesSourceScanSummary,
) {
	var domainError *githubSourceProviderDomainError
	if !errors.As(checkErr, &domainError) ||
		(domainError.statusCode != 403 && domainError.statusCode != 429) {
		return
	}

	retryAt := domainError.retryAt
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).
		Select("next_check_at").
		Where("source_id = ?", sourceID).
		First(&runtime).Error; err != nil {
		logger.WarnF(ctx, "[PagesSourceScan] load provider backoff deadline failed: source_id=%d error=%v", sourceID, err)
	} else if runtime.NextCheckAt != nil {
		retryAt = runtime.NextCheckAt
	}

	retryAtText := "unknown"
	if retryAt != nil {
		retryAtText = retryAt.UTC().Format(time.RFC3339)
	}
	summary.ProviderBackoffs = append(summary.ProviderBackoffs, pagesSourceProviderBackoff{
		SourceID: sourceID, StatusCode: domainError.statusCode, RetryAt: retryAtText,
	})
	task.AppendLog(
		ctx,
		"[check] GitHub provider 退避：source_id=%d status=%d retry_at=%s",
		sourceID,
		domainError.statusCode,
		retryAtText,
	)
}

func handleCheckedGitHubSource(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	checkResult *githubCheckTaskResult,
	summary *pagesSourceScanSummary,
) {
	summary.CheckedSources++
	switch checkResult.Status {
	case pagesSourceStatusUpdateAvailable:
		summary.UpdatesFound++
	case pagesSourceStatusAttention:
		summary.AttentionSources++
	}
	if !snapshot.AutoUpdateEnabled || checkResult.Status != pagesSourceStatusUpdateAvailable ||
		!validOptionalSourceRevision(checkResult.Revision) || checkResult.Revision == "" {
		return
	}

	source := model.PagesProjectSource{
		ID:            snapshot.SourceID,
		ProjectID:     snapshot.ProjectID,
		ConfigVersion: snapshot.SourceConfigVersion,
	}
	receipt, dispatchErr := dispatchPagesSourceAutoSync(ctx, source, checkResult.Revision)
	if dispatchErr == nil {
		summary.DispatchedSyncs++
		if receipt != nil {
			task.AppendLog(
				ctx,
				"[dispatch] 已投递自动更新：source_id=%d execution_id=%s revision=%s",
				snapshot.SourceID,
				receipt.ExecutionID,
				checkResult.Revision,
			)
		}
		return
	}

	summary.FailedSources++
	summary.FailedDispatches++
	logger.WarnF(
		ctx,
		"[PagesSourceScan] dispatch auto sync failed: source_id=%d revision=%s error=%v",
		snapshot.SourceID,
		checkResult.Revision,
		dispatchErr,
	)
	updated, recordErr := recordPagesSourceAutoDispatchFailure(
		ctx,
		snapshot,
		checkResult.Revision,
		checkResult.RetryAt,
	)
	if recordErr != nil {
		logger.WarnF(
			ctx,
			"[PagesSourceScan] record auto sync dispatch failure failed: source_id=%d error=%v",
			snapshot.SourceID,
			recordErr,
		)
	} else if !updated {
		summary.StaleSources++
	}
}

func recordPagesSourceAutoDispatchFailure(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	revision string,
	retryAt *time.Time,
) (bool, error) {
	if snapshot == nil || revision == "" {
		return false, nil
	}
	now := pagesSourceScanNow()
	next := nextGitHubCheckAt(now, snapshot.SourceID, minimumCheckInterval)
	if retryAt != nil && retryAt.After(next) {
		next = retryAt.In(now.Location())
	}
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ?", snapshot.SourceID).
		Where("sync_status = ? AND last_seen_revision = ?", pagesSourceStatusUpdateAvailable, revision).
		Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now).
		Where(`EXISTS (
			SELECT 1 FROM of_pages_project_sources AS source
			WHERE source.id = ? AND source.config_version = ?
				AND source.source_type = ? AND source.release_selector = ?
				AND source.auto_update_enabled = ?
		)`,
			snapshot.SourceID,
			snapshot.SourceConfigVersion,
			PagesSourceTypeGitHubRelease,
			githubReleaseSelectorLatest,
			true,
		).
		Updates(map[string]any{
			sourceRuntimeColumnSyncStatus:  pagesSourceStatusUpdateAvailable,
			sourceRuntimeColumnLastError:   errPagesSourceTaskDispatchFailed,
			sourceRuntimeColumnNextCheckAt: &next,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
