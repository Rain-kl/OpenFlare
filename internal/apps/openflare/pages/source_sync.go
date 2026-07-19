// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	pagesSourceTriggerManualSync          = "manual_sync"
	pagesSourceTriggerScheduledAutoUpdate = "scheduled_auto_update"
	pagesSourceCreatedBySystem            = "system:pages-source-sync"
	pagesSourceHeartbeatInterval          = pagesSourceSyncLeaseDuration / 3
	pagesSourceCleanupTimeout             = 15 * time.Second
)

var (
	errSourceFinalFence         = errors.New("pages source final fence rejected")
	errSourceLeaseHeartbeatLost = errors.New("pages source lease heartbeat lost")
	sourceCommitNow             = time.Now
)

type sourceSyncOutcome struct {
	Deployment *DeploymentView
	Reused     bool
	Stale      bool
}

type preparedRemoteSource struct {
	Candidate  *SourceCandidate
	Manifest   *deploymentManifest
	Detail     sourceDetail
	DetailJSON string
}

type sourceIngestState struct {
	Result     upload.IngestResult
	HasIngest  bool
	Referenced bool
}

type sourceCommitState struct {
	Project *model.PagesProject
	Source  *model.PagesProjectSource
	Runtime *model.PagesProjectSourceRuntime
	Now     time.Time
}

type sourceLeaseHeartbeat struct {
	cancel   context.CancelFunc
	done     <-chan error
	stopOnce sync.Once
	stopErr  error
}

func startSourceLeaseHeartbeat(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	leaseDuration time.Duration,
	interval time.Duration,
) (context.Context, *sourceLeaseHeartbeat, error) {
	if snapshot == nil || leaseDuration <= 0 || interval <= 0 || interval >= leaseDuration {
		return nil, nil, errors.New(errPagesSourceLeaseLost)
	}
	renewed, err := renewSourceLease(ctx, snapshot, leaseDuration)
	if err != nil {
		return nil, nil, err
	}
	if !renewed {
		return nil, nil, errSourceLeaseHeartbeatLost
	}

	workCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	heartbeat := &sourceLeaseHeartbeat{cancel: cancel, done: done}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-workCtx.Done():
				done <- nil
				return
			case <-ticker.C:
				renewed, renewErr := renewSourceLease(workCtx, snapshot, leaseDuration)
				if renewErr != nil {
					if workCtx.Err() != nil {
						done <- nil
						return
					}
					done <- renewErr
					cancel()
					return
				}
				if !renewed {
					done <- errSourceLeaseHeartbeatLost
					cancel()
					return
				}
			}
		}
	}()
	return workCtx, heartbeat, nil
}

func (heartbeat *sourceLeaseHeartbeat) stop() error {
	if heartbeat == nil {
		return nil
	}
	heartbeat.stopOnce.Do(func() {
		heartbeat.cancel()
		heartbeat.stopErr = <-heartbeat.done
	})
	return heartbeat.stopErr
}

func sourceHeartbeatOutcome(err error) (*sourceSyncOutcome, error) {
	if errors.Is(err, errSourceLeaseHeartbeatLost) {
		return &sourceSyncOutcome{Stale: true}, nil
	}
	return nil, err
}

func recordSourceLeaseFailure(ctx context.Context, snapshot *sourceExecutionSnapshot) {
	cleanupCtx, cancel := sourceCleanupContext(ctx)
	defer cancel()
	if err := failSourceLease(cleanupCtx, snapshot, errPagesSourceSyncFailed); err != nil {
		var sourceID uint
		if snapshot != nil {
			sourceID = snapshot.SourceID
		}
		logger.WarnF(cleanupCtx, "[PagesSource] record failed runtime state failed: source_id=%d error=%v", sourceID, err)
	}
}

func sourceCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), pagesSourceCleanupTimeout)
}

func syncRemoteSourceWithTrigger(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	actor string,
	triggerType string,
) (outcome *sourceSyncOutcome, resultErr error) {
	if snapshot == nil || snapshot.SourceType != PagesSourceTypeRemoteURL {
		return nil, errors.New(errPagesSourceTypeUnsupported)
	}
	actor = strings.TrimSpace(actor)
	if actor == "" || !validSourceDeploymentTrigger(triggerType) {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	defer func() {
		if resultErr != nil {
			recordSourceLeaseFailure(ctx, snapshot)
		}
	}()

	workCtx, heartbeat, err := startSourceLeaseHeartbeat(
		ctx,
		snapshot,
		pagesSourceSyncLeaseDuration,
		pagesSourceHeartbeatInterval,
	)
	if err != nil {
		return sourceHeartbeatOutcome(err)
	}
	defer func() {
		_ = heartbeat.stop()
	}()

	limits := resolvePagesLimits(workCtx)
	prepared, err := prepareRemoteSource(workCtx, snapshot, limits)
	if err != nil {
		if heartbeatErr := heartbeat.stop(); heartbeatErr != nil {
			return sourceHeartbeatOutcome(heartbeatErr)
		}
		return nil, err
	}
	defer func() {
		if cleanupErr := prepared.Candidate.Cleanup(); cleanupErr != nil {
			logger.WarnF(ctx, "[PagesSource] cleanup temporary package failed: source_id=%d error=%v", snapshot.SourceID, cleanupErr)
		}
	}()

	ingestState, err := resolveSourceIngest(workCtx, snapshot, prepared)
	if err != nil {
		if heartbeatErr := heartbeat.stop(); heartbeatErr != nil {
			return sourceHeartbeatOutcome(heartbeatErr)
		}
		return nil, err
	}
	defer func() {
		compensateSourceIngest(ctx, snapshot, ingestState)
	}()

	if heartbeatErr := heartbeat.stop(); heartbeatErr != nil {
		return sourceHeartbeatOutcome(heartbeatErr)
	}
	renewed, err := renewSourceLease(ctx, snapshot, pagesSourceSyncLeaseDuration)
	if err != nil {
		return nil, err
	}
	if !renewed {
		return &sourceSyncOutcome{Stale: true}, nil
	}

	task.AppendLog(ctx, "[activate] 正在原子切换生产部署")
	deployment, reused, referenced, err := commitSourceDeploymentWithTrigger(
		ctx,
		snapshot,
		prepared.Candidate.Checksum,
		prepared.Candidate.Checksum,
		prepared.Detail,
		prepared.DetailJSON,
		actor,
		triggerType,
		prepared.Manifest,
		ingestState.Result,
		ingestState.HasIngest,
		nil,
	)
	ingestState.Referenced = referenced
	if errors.Is(err, errSourceFinalFence) {
		return &sourceSyncOutcome{Stale: true}, nil
	}
	if err != nil {
		return nil, err
	}
	ingestState.Referenced = ingestState.HasIngest && deployment.UploadID == ingestState.Result.Upload.ID

	if pruneErr := pruneProjectDeploymentHistory(ctx, snapshot.ProjectID, limits.HistoryCount, 0); pruneErr != nil {
		logger.ErrorF(ctx,
			"[PagesSource] strict prune failed: project_id=%d source_id=%d keep=%d error=%v",
			snapshot.ProjectID,
			snapshot.SourceID,
			limits.HistoryCount,
			pruneErr,
		)
	}
	view := buildDeploymentView(deployment)
	return &sourceSyncOutcome{Deployment: &view, Reused: reused}, nil
}

func prepareRemoteSource(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	limits pagesLimits,
) (*preparedRemoteSource, error) {
	task.AppendLog(ctx, "[download] 正在获取远程部署包")
	candidate, err := FetchRemoteSource(ctx, RemoteSourceRequest{
		URL:             snapshot.RemoteURL,
		NetworkPolicy:   snapshot.RemoteNetworkPolicy,
		MaxPackageBytes: limits.PackageBytes,
	})
	if err != nil {
		return nil, err
	}
	if candidate == nil || candidate.TempPath == "" || candidate.Checksum == "" || candidate.Format == "" {
		return nil, errors.New(errPagesSourceSyncFailed)
	}
	rootDir, err := validateAndNormalizePagesRootDir(snapshot.RootDir)
	if err != nil {
		cleanupFailedRemoteCandidate(ctx, snapshot, candidate)
		return nil, err
	}
	entryFile, err := validateAndNormalizePagesEntryFile(snapshot.EntryFile)
	if err != nil {
		cleanupFailedRemoteCandidate(ctx, snapshot, candidate)
		return nil, err
	}
	task.AppendLog(ctx, "[verify] 正在校验归档结构与入口文件")
	manifest, err := inspectPagesPackage(candidate.TempPath, candidate.Format, rootDir, entryFile, limits)
	if err != nil {
		cleanupFailedRemoteCandidate(ctx, snapshot, candidate)
		return nil, err
	}
	detail := sourceDetail{Provider: PagesSourceTypeRemoteURL, DisplayName: safeRemoteSourceLabel(candidate.SafeLabel)}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		cleanupFailedRemoteCandidate(ctx, snapshot, candidate)
		return nil, err
	}
	return &preparedRemoteSource{
		Candidate:  candidate,
		Manifest:   manifest,
		Detail:     detail,
		DetailJSON: string(detailJSON),
	}, nil
}

func cleanupFailedRemoteCandidate(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	candidate *SourceCandidate,
) {
	if err := candidate.Cleanup(); err != nil {
		logger.WarnF(ctx, "[PagesSource] cleanup failed preparation package: source_id=%d error=%v", snapshot.SourceID, err)
	}
}

func resolveSourceIngest(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	prepared *preparedRemoteSource,
) (*sourceIngestState, error) {
	_, err := findSourceDeployment(
		ctx,
		snapshot.ProjectID,
		snapshot.SourceIdentity,
		prepared.Candidate.Checksum,
	)
	if err == nil {
		return &sourceIngestState{}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	task.AppendLog(ctx, "[ingest] 正在保存受管部署包")
	result, err := ingestPagesDeploymentPackageWithSource(
		ctx,
		prepared.Candidate.TempPath,
		prepared.Candidate.Checksum,
		snapshot.ProjectID,
		snapshot.SourceID,
		sourceDetailLabel(prepared.Detail),
		prepared.Candidate.Format,
	)
	if err != nil {
		return nil, err
	}
	return &sourceIngestState{Result: result, HasIngest: true}, nil
}

func compensateSourceIngest(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	state *sourceIngestState,
) {
	if state == nil || !state.HasIngest || !state.Result.Created || state.Referenced {
		return
	}
	cleanupCtx, cancel := sourceCleanupContext(ctx)
	defer cancel()
	task.AppendLog(cleanupCtx, "[cleanup] 正在补偿未引用的部署包记录")
	if err := removePagesUploadIfUnreferenced(cleanupCtx, snapshot.ProjectID, state.Result.Upload.ID); err != nil {
		logger.ErrorF(cleanupCtx,
			"[PagesSource] compensate upload failed: project_id=%d source_id=%d upload_id=%d error=%v",
			snapshot.ProjectID, snapshot.SourceID, state.Result.Upload.ID, err,
		)
	}
}

func findSourceDeployment(
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

func commitSourceDeploymentWithTrigger(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	revision string,
	packageChecksum string,
	detail sourceDetail,
	detailJSON string,
	actor string,
	triggerType string,
	manifest *deploymentManifest,
	ingestResult upload.IngestResult,
	hasIngest bool,
	nextCheckNotBefore *time.Time,
) (*model.PagesDeployment, bool, bool, error) {
	if snapshot == nil || manifest == nil {
		return nil, false, false, errors.New(errPagesSourceSyncFailed)
	}
	if !validSourceDeploymentTrigger(triggerType) {
		return nil, false, false, errors.New(errPagesSourceActionInvalid)
	}
	var committed model.PagesDeployment
	reused := false
	ingestReferenced := false
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockSourceCommitState(tx, snapshot)
		if err != nil {
			return err
		}
		target, targetReused, err := resolveSourceDeploymentTx(
			tx, state, revision, packageChecksum, detail, detailJSON, actor, triggerType,
			manifest, ingestResult, hasIngest,
		)
		if err != nil {
			return err
		}
		if err := lockSourceDeploymentUploadsTx(tx, target, ingestResult, hasIngest); err != nil {
			return err
		}
		if err := ensureDeploymentEntry(tx, target.ID, state.Project.RootDir, state.Project.EntryFile); err != nil {
			return err
		}
		if err := refreshSourceCommitLease(state, snapshot); err != nil {
			return err
		}
		if err := activateSourceDeploymentTx(tx, state, target, revision, detailJSON, nextCheckNotBefore); err != nil {
			return err
		}
		committed = *target
		reused = targetReused
		ingestReferenced = hasIngest && target.UploadID == ingestResult.Upload.ID
		return nil
	})
	if err != nil {
		return nil, false, false, err
	}
	return &committed, reused, ingestReferenced, nil
}

func lockSourceCommitState(tx *gorm.DB, snapshot *sourceExecutionSnapshot) (*sourceCommitState, error) {
	state := &sourceCommitState{}
	var project model.PagesProject
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		First(&project, snapshot.ProjectID).Error; err != nil {
		return nil, sourceFenceRecordError(err)
	}
	if project.ContentConfigVersion != snapshot.ContentConfigVersion {
		return nil, errSourceFinalFence
	}
	var source model.PagesProjectSource
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("id = ? AND project_id = ?", snapshot.SourceID, snapshot.ProjectID).
		First(&source).Error; err != nil {
		return nil, sourceFenceRecordError(err)
	}
	if source.ConfigVersion != snapshot.SourceConfigVersion ||
		source.SourceIdentity != snapshot.SourceIdentity ||
		source.SourceType != snapshot.SourceType {
		return nil, errSourceFinalFence
	}
	var runtime model.PagesProjectSourceRuntime
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("source_id = ?", source.ID).
		First(&runtime).Error; err != nil {
		return nil, sourceFenceRecordError(err)
	}
	state.Project = &project
	state.Source = &source
	state.Runtime = &runtime
	if err := refreshSourceCommitLease(state, snapshot); err != nil {
		return nil, err
	}
	return state, nil
}

func refreshSourceCommitLease(state *sourceCommitState, snapshot *sourceExecutionSnapshot) error {
	if state == nil || state.Runtime == nil || snapshot == nil {
		return errSourceFinalFence
	}
	now := sourceCommitNow()
	if state.Runtime.LeaseToken != snapshot.LeaseToken || state.Runtime.LeaseExpiresAt == nil ||
		!state.Runtime.LeaseExpiresAt.After(now) {
		return errSourceFinalFence
	}
	state.Now = now
	return nil
}

func sourceFenceRecordError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errSourceFinalFence
	}
	return err
}

func resolveSourceDeploymentTx(
	tx *gorm.DB,
	state *sourceCommitState,
	revision string,
	packageChecksum string,
	detail sourceDetail,
	detailJSON string,
	actor string,
	triggerType string,
	manifest *deploymentManifest,
	ingestResult upload.IngestResult,
	hasIngest bool,
) (*model.PagesDeployment, bool, error) {
	var target model.PagesDeployment
	err := tx.Where(
		"project_id = ? AND source_identity = ? AND source_revision = ?",
		state.Project.ID,
		state.Source.SourceIdentity,
		revision,
	).First(&target).Error
	if err == nil {
		return &target, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if !hasIngest {
		return nil, false, errSourceFinalFence
	}
	return createSourceDeploymentTx(
		tx, state, revision, packageChecksum, detail, detailJSON, actor, triggerType, manifest, ingestResult,
	)
}

func createSourceDeploymentTx(
	tx *gorm.DB,
	state *sourceCommitState,
	revision string,
	packageChecksum string,
	detail sourceDetail,
	detailJSON string,
	actor string,
	triggerType string,
	manifest *deploymentManifest,
	ingestResult upload.IngestResult,
) (*model.PagesDeployment, bool, error) {
	var maxNumber int
	if err := tx.Model(&model.PagesDeployment{}).
		Where("project_id = ?", state.Project.ID).
		Select("COALESCE(MAX(deployment_number), 0)").
		Scan(&maxNumber).Error; err != nil {
		return nil, false, err
	}
	identity := state.Source.SourceIdentity
	revisionValue := revision
	target := &model.PagesDeployment{
		ProjectID:        state.Project.ID,
		DeploymentNumber: maxNumber + 1,
		Checksum:         packageChecksum,
		Status:           model.PagesDeploymentStatusUploaded,
		UploadID:         ingestResult.Upload.ID,
		FileCount:        manifest.FileCount,
		TotalSize:        manifest.TotalSize,
		CreatedBy:        actor,
		SourceType:       state.Source.SourceType,
		SourceIdentity:   &identity,
		SourceRevision:   &revisionValue,
		SourceLabel:      sourceDetailLabel(detail),
		SourceMeta:       detailJSON,
		TriggerType:      triggerType,
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(target)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return reloadSourceDeploymentTx(tx, state.Project.ID, identity, revision)
	}
	if err := createSourceDeploymentFilesTx(tx, target.ID, manifest.Files); err != nil {
		return nil, false, err
	}
	return target, false, nil
}

func validSourceDeploymentTrigger(triggerType string) bool {
	return triggerType == pagesSourceTriggerManualSync || triggerType == pagesSourceTriggerScheduledAutoUpdate
}

func reloadSourceDeploymentTx(
	tx *gorm.DB,
	projectID uint,
	identity string,
	revision string,
) (*model.PagesDeployment, bool, error) {
	var target model.PagesDeployment
	err := tx.Where(
		"project_id = ? AND source_identity = ? AND source_revision = ?",
		projectID,
		identity,
		revision,
	).First(&target).Error
	return &target, true, err
}

func createSourceDeploymentFilesTx(tx *gorm.DB, deploymentID uint, files []model.PagesDeploymentFile) error {
	if len(files) == 0 {
		return nil
	}
	for index := range files {
		files[index].DeploymentID = deploymentID
	}
	return tx.Create(&files).Error
}

func lockSourceDeploymentUploadsTx(
	tx *gorm.DB,
	target *model.PagesDeployment,
	ingestResult upload.IngestResult,
	hasIngest bool,
) error {
	uploadIDs := []uint64{target.UploadID}
	if hasIngest && ingestResult.Upload.ID != 0 && ingestResult.Upload.ID != target.UploadID {
		uploadIDs = append(uploadIDs, ingestResult.Upload.ID)
	}
	sort.Slice(uploadIDs, func(i, j int) bool { return uploadIDs[i] < uploadIDs[j] })
	var records []model.Upload
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("id IN ?", uploadIDs).
		Order("id asc").
		Find(&records).Error; err != nil {
		return err
	}
	for index := range records {
		if records[index].ID != target.UploadID {
			continue
		}
		if records[index].Status == model.UploadStatusUsed && records[index].Type == upload.ReservedPagesDeploymentType {
			return nil
		}
		break
	}
	return errSourceFinalFence
}

func activateSourceDeploymentTx(
	tx *gorm.DB,
	state *sourceCommitState,
	target *model.PagesDeployment,
	revision string,
	detailJSON string,
	nextCheckNotBefore *time.Time,
) error {
	if err := tx.Model(&model.PagesDeployment{}).
		Where("project_id = ?", state.Project.ID).
		Update("status", model.PagesDeploymentStatusUploaded).Error; err != nil {
		return err
	}
	if err := tx.Model(target).Updates(map[string]any{
		pagesDeploymentColumnStatus: model.PagesDeploymentStatusActive,
		"activated_at":              &state.Now,
	}).Error; err != nil {
		return err
	}
	if err := tx.Model(state.Project).Update("active_deployment_id", target.ID).Error; err != nil {
		return err
	}
	finishedAt := sourceCommitNow()
	var nextCheckAt any
	if state.Source.SourceType == PagesSourceTypeGitHubRelease &&
		state.Source.ReleaseSelector == githubReleaseSelectorLatest {
		next := nextGitHubCheckAt(finishedAt, state.Source.ID, state.Source.CheckIntervalMinutes)
		if nextCheckNotBefore != nil && nextCheckNotBefore.After(next) {
			next = nextCheckNotBefore.In(finishedAt.Location())
		}
		nextCheckAt = &next
	}
	result := tx.Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ? AND lease_token = ? AND lease_expires_at > ?",
			state.Runtime.SourceID,
			state.Runtime.LeaseToken,
			finishedAt,
		).
		Updates(map[string]any{
			"last_seen_revision":              revision,
			"last_seen_detail":                detailJSON,
			"last_applied_revision":           revision,
			"last_applied_detail":             detailJSON,
			sourceRuntimeColumnSyncStatus:     pagesSourceStatusIdle,
			sourceRuntimeColumnLastError:      "",
			sourceRuntimeColumnLastCheckedAt:  &finishedAt,
			"last_synced_at":                  &finishedAt,
			"next_check_at":                   nextCheckAt,
			sourceRuntimeColumnLeaseToken:     "",
			sourceRuntimeColumnLeaseExpiresAt: nil,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errSourceFinalFence
	}
	return nil
}

func safeRemoteSourceLabel(raw string) string {
	value := strings.ReplaceAll(strings.ToValidUTF8(raw, ""), "\\", "/")
	value = path.Base(strings.TrimSpace(value))
	if value == "." || value == "/" {
		value = ""
	}
	var builder strings.Builder
	for _, character := range value {
		if character >= 0x20 && character != 0x7f {
			builder.WriteRune(character)
		}
	}
	value = strings.TrimSpace(builder.String())
	if value == "" {
		value = defaultRemoteAssetLabel
	}
	if len(value) > remoteSourceMaxSafeLabelBytes {
		value = value[:remoteSourceMaxSafeLabelBytes]
		for !utf8.ValidString(value) {
			_, size := utf8.DecodeLastRuneInString(value)
			value = value[:len(value)-size]
		}
	}
	return value
}

func sourceSyncResultDetail(outcome *sourceSyncOutcome) string {
	if outcome == nil || outcome.Deployment == nil {
		return ""
	}
	payload := map[string]any{
		"deployment_id": outcome.Deployment.ID,
		"reused":        outcome.Reused,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"deployment_id":%d}`, outcome.Deployment.ID)
	}
	return string(encoded)
}
