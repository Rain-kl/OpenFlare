// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/integration/githubrelease"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const githubSourceDetailProvider = "github"

var githubDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type githubSourceProviderDomainError struct {
	message    string
	permanent  bool
	retryAt    *time.Time
	statusCode int
}

func (domainError *githubSourceProviderDomainError) Error() string {
	return domainError.message
}

type githubReleaseAPI interface {
	Resolve(context.Context, githubrelease.ResolveRequest) (githubrelease.ResolveResult, error)
	Download(context.Context, githubrelease.DownloadRequest) (*githubrelease.DownloadResult, error)
}

var newGitHubReleaseClient = func() githubReleaseAPI {
	return githubrelease.NewClient()
}

type githubSourceTarget struct {
	Revision   string
	Detail     sourceDetail
	DetailJSON string
	Release    githubrelease.Release
	Asset      githubrelease.Asset
	RetryAt    *time.Time
}

type githubCheckTaskResult struct {
	Message  string
	Detail   string
	Revision string
	Status   string
	RetryAt  *time.Time
	Stale    bool
}

type preparedGitHubSource struct {
	target      *githubSourceTarget
	download    *githubrelease.DownloadResult
	format      pagesarchive.Format
	manifest    *deploymentManifest
	ingestState *sourceIngestState
	limits      pagesLimits
}

func checkGitHubSource(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
) (*githubCheckTaskResult, error) {
	if snapshot == nil || snapshot.SourceType != PagesSourceTypeGitHubRelease {
		return nil, errors.New(errPagesSourceTypeUnsupported)
	}
	task.AppendLog(ctx, "[check] 正在检查 GitHub Release：repo=%s asset=%s", snapshot.GitHubRepository, snapshot.AssetName)
	client := newGitHubReleaseClient()
	result, err := client.Resolve(ctx, githubrelease.ResolveRequest{
		Repository: snapshot.GitHubRepository,
		Selector:   githubrelease.Selector(snapshot.ReleaseSelector),
		Tag:        snapshot.ReleaseTag,
		AssetName:  snapshot.AssetName,
		ETag:       snapshot.ETag,
	})
	if err != nil {
		logger.WarnF(ctx, "[PagesSource] GitHub resolve failed: source_id=%d repo=%s error=%v", snapshot.SourceID, snapshot.GitHubRepository, err)
		retryAt, _ := githubrelease.RetryAt(err)
		domainErr := githubSourceDomainError(err)
		if failErr := failGitHubCheckLease(ctx, snapshot, domainErr.Error(), retryAt); failErr != nil {
			if errors.Is(failErr, errSourceFinalFence) {
				return &githubCheckTaskResult{Message: errPagesSourceActionStale, Stale: true}, nil
			}
			return nil, failErr
		}
		return nil, domainErr
	}
	if result.NotModified {
		revision, status, err := finishGitHubCheckNotModified(ctx, snapshot, result)
		if err != nil {
			if errors.Is(err, errSourceFinalFence) {
				return &githubCheckTaskResult{Message: errPagesSourceActionStale, Stale: true}, nil
			}
			return nil, err
		}
		detail, _ := json.Marshal(map[string]string{"revision": revision, pagesDeploymentColumnStatus: status})
		return &githubCheckTaskResult{
			Message:  "GitHub Release 检查完成，内容未变化",
			Detail:   string(detail),
			Revision: revision,
			Status:   status,
			RetryAt:  result.RetryAt,
		}, nil
	}
	target, err := buildGitHubSourceTarget(result.Release, result.Asset, result.RetryAt)
	if err != nil {
		retryAt := time.Time{}
		if result.RetryAt != nil {
			retryAt = result.RetryAt.UTC()
		}
		if failErr := failGitHubCheckLease(ctx, snapshot, err.Error(), retryAt); failErr != nil {
			if errors.Is(failErr, errSourceFinalFence) {
				return &githubCheckTaskResult{Message: errPagesSourceActionStale, Stale: true}, nil
			}
			return nil, failErr
		}
		return nil, err
	}
	status, err := finishGitHubCheckTarget(ctx, snapshot, result, target)
	if err != nil {
		if errors.Is(err, errSourceFinalFence) {
			return &githubCheckTaskResult{Message: errPagesSourceActionStale, Stale: true}, nil
		}
		return nil, err
	}
	detail, _ := json.Marshal(map[string]string{"revision": target.Revision, pagesDeploymentColumnStatus: status})
	message := "GitHub Release 检查完成"
	switch status {
	case pagesSourceStatusUpdateAvailable:
		message = "发现新的 GitHub Release 部署包"
	case pagesSourceStatusAttention:
		message = "检测到同一 Release 的资源被替换，需要确认"
	}
	return &githubCheckTaskResult{
		Message: message, Detail: string(detail), Revision: target.Revision,
		Status: status, RetryAt: result.RetryAt,
	}, nil
}

func buildGitHubSourceTarget(
	release githubrelease.Release,
	asset githubrelease.Asset,
	retryAt *time.Time,
) (*githubSourceTarget, error) {
	digest := strings.ToLower(strings.TrimSpace(asset.Digest))
	if digest != "" && !githubDigestPattern.MatchString(digest) {
		return nil, errors.New(errPagesSourceDigestInvalid)
	}
	if strings.TrimSpace(release.ID) == "" || strings.TrimSpace(asset.ID) == "" ||
		!validGitHubReleaseDisplayTag(release.Tag) || !validGitHubAssetName(asset.Name) ||
		asset.State != "uploaded" || asset.UpdatedAt.IsZero() {
		return nil, errors.New(errPagesSourceReleaseNotFound)
	}
	updatedAt := asset.UpdatedAt.UTC().Format(time.RFC3339Nano)
	rawRevision := "github:" + release.ID + ":" + asset.ID + ":" + updatedAt + ":" + digest
	sum := sha256.Sum256([]byte(rawRevision))
	detail := sourceDetail{
		Provider:       githubSourceDetailProvider,
		Tag:            release.Tag,
		AssetName:      asset.Name,
		ReleaseID:      release.ID,
		AssetID:        asset.ID,
		AssetUpdatedAt: updatedAt,
		Digest:         digest,
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return nil, errors.New(errPagesSourceSyncFailed)
	}
	return &githubSourceTarget{
		Revision:   hex.EncodeToString(sum[:]),
		Detail:     detail,
		DetailJSON: string(detailJSON),
		Release:    release,
		Asset:      asset,
		RetryAt:    retryAt,
	}, nil
}

func finishGitHubCheckNotModified(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	result githubrelease.ResolveResult,
) (string, string, error) {
	var revision string
	var status string
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		runtime, now, err := lockOwnedSourceRuntime(tx, snapshot)
		if err != nil {
			return err
		}
		revision = runtime.LastSeenRevision
		status = normalizedSourceRuntimeStatus(runtime)
		updates := githubCheckTerminalUpdates(snapshot, now, result.RetryAt)
		updates["etag"] = result.ETag
		updates[sourceRuntimeColumnSyncStatus] = status
		return tx.Model(runtime).Updates(updates).Error
	})
	return revision, status, err
}

func finishGitHubCheckTarget(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	result githubrelease.ResolveResult,
	target *githubSourceTarget,
) (string, error) {
	status := pagesSourceStatusIdle
	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		runtime, now, err := lockOwnedSourceRuntime(tx, snapshot)
		if err != nil {
			return err
		}
		status = targetRuntimeStatus(target, runtime.LastAppliedRevision, runtime.LastAppliedDetail)
		updates := githubCheckTerminalUpdates(snapshot, now, result.RetryAt)
		updates["etag"] = result.ETag
		updates["last_seen_revision"] = target.Revision
		updates["last_seen_detail"] = target.DetailJSON
		updates[sourceRuntimeColumnSyncStatus] = status
		return tx.Model(runtime).Updates(updates).Error
	})
	return status, err
}

func githubCheckTerminalUpdates(
	snapshot *sourceExecutionSnapshot,
	now time.Time,
	retryAt *time.Time,
) map[string]any {
	updates := map[string]any{
		sourceRuntimeColumnLastError:      "",
		sourceRuntimeColumnLastCheckedAt:  &now,
		sourceRuntimeColumnLeaseToken:     "",
		sourceRuntimeColumnLeaseExpiresAt: nil,
	}
	updates[sourceRuntimeColumnNextCheckAt] = nextCheckAfterGitHubResponse(snapshot, now, retryAt)
	return updates
}

func nextCheckAfterGitHubResponse(
	snapshot *sourceExecutionSnapshot,
	now time.Time,
	retryAt *time.Time,
) any {
	if snapshot.ReleaseSelector != githubReleaseSelectorLatest {
		return nil
	}
	next := nextGitHubCheckAt(now, snapshot.SourceID, snapshot.CheckIntervalMinutes)
	if retryAt != nil && retryAt.After(next) {
		next = retryAt.In(now.Location())
	}
	return &next
}

func lockOwnedSourceRuntime(
	tx *gorm.DB,
	snapshot *sourceExecutionSnapshot,
) (*model.PagesProjectSourceRuntime, time.Time, error) {
	var runtime model.PagesProjectSourceRuntime
	if err := tx.Clauses(clause.Locking{Strength: pagesRowLockStrength}).
		Where("source_id = ?", snapshot.SourceID).First(&runtime).Error; err != nil {
		return nil, time.Time{}, err
	}
	now := time.Now()
	if runtime.LeaseToken != snapshot.LeaseToken || runtime.LeaseExpiresAt == nil ||
		!runtime.LeaseExpiresAt.After(now) {
		return nil, time.Time{}, errSourceFinalFence
	}
	return &runtime, now, nil
}

func failGitHubCheckLease(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	message string,
	retryAt time.Time,
) error {
	now := time.Now()
	next := now.Add(initialCheckRetryDelay)
	if retryAt.After(next) {
		next = retryAt.In(now.Location())
	}
	updates := map[string]any{
		sourceRuntimeColumnSyncStatus:     pagesSourceStatusFailed,
		sourceRuntimeColumnLastError:      safeSourceRuntimeError(message),
		sourceRuntimeColumnLastCheckedAt:  &now,
		sourceRuntimeColumnLeaseToken:     "",
		sourceRuntimeColumnLeaseExpiresAt: nil,
	}
	if snapshot.ReleaseSelector == githubReleaseSelectorLatest {
		updates[sourceRuntimeColumnNextCheckAt] = &next
	} else {
		updates[sourceRuntimeColumnNextCheckAt] = nil
	}
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ? AND lease_token = ? AND lease_expires_at > ?", snapshot.SourceID, snapshot.LeaseToken, now).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errSourceFinalFence
	}
	return nil
}

func targetRuntimeStatus(
	target *githubSourceTarget,
	appliedRevision string,
	appliedDetail string,
) string {
	if target == nil || target.Revision == appliedRevision {
		return pagesSourceStatusIdle
	}
	applied := sourceDetail{}
	if unmarshalSourceDetail(appliedDetail, &applied) == nil && target.Detail.ReleaseID != "" &&
		target.Detail.ReleaseID == applied.ReleaseID {
		return pagesSourceStatusAttention
	}
	return pagesSourceStatusUpdateAvailable
}

func preflightGitHubSyncConfirmation(ctx context.Context, sourceID uint, confirmedRevision string) error {
	var runtime model.PagesProjectSourceRuntime
	if err := db.DB(ctx).Where("source_id = ?", sourceID).First(&runtime).Error; err != nil {
		return err
	}
	replacement := sourceHasSameReleaseReplacement(&runtime)
	if replacement && confirmedRevision == "" {
		return errors.New(errPagesSourceConfirmationNeeded)
	}
	if confirmedRevision != "" && (!replacement || confirmedRevision != runtime.LastSeenRevision) {
		return errors.New(errPagesSourceConfirmationStale)
	}
	return nil
}

func syncGitHubSourceWithTrigger(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	actor string,
	targetRevision string,
	confirmedRevision string,
	triggerType string,
) (outcome *sourceSyncOutcome, resultErr error) {
	if snapshot == nil || snapshot.SourceType != PagesSourceTypeGitHubRelease || !validPagesSourceActor(actor) {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	if !validSourceDeploymentTrigger(triggerType) {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	defer func() {
		resultErr = finalizeGitHubSyncFailure(ctx, snapshot, resultErr)
	}()
	workCtx, heartbeat, err := startSourceLeaseHeartbeat(
		ctx, snapshot, pagesSourceSyncLeaseDuration, pagesSourceHeartbeatInterval,
	)
	if err != nil {
		return sourceHeartbeatOutcome(err)
	}
	defer func() { _ = heartbeat.stop() }()

	client := newGitHubReleaseClient()
	target, guardedOutcome, err := resolveAndGuardGitHubSync(
		workCtx, client, snapshot, targetRevision, confirmedRevision,
	)
	if err != nil {
		return nil, err
	}
	if guardedOutcome != nil {
		return guardedOutcome, nil
	}
	prepared, err := prepareGitHubSyncPackage(workCtx, client, snapshot, target)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanupErr := prepared.download.Cleanup(); cleanupErr != nil {
			logger.WarnF(ctx, "[PagesSource] cleanup GitHub package failed: source_id=%d error=%v", snapshot.SourceID, cleanupErr)
		}
	}()
	defer compensateSourceIngest(ctx, snapshot, prepared.ingestState)
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
	return activatePreparedGitHubSource(ctx, snapshot, actor, triggerType, prepared)
}

func finalizeGitHubSyncFailure(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	resultErr error,
) error {
	if resultErr == nil {
		return nil
	}
	cleanupCtx, cancel := sourceCleanupContext(ctx)
	defer cancel()
	finalizerErr := persistGitHubSyncFailure(cleanupCtx, snapshot, resultErr)
	if finalizerErr == nil {
		return resultErr
	}
	logger.WarnF(
		cleanupCtx,
		"[PagesSource] finalize GitHub sync failure failed: source_id=%d source_error=%s error=%v",
		snapshot.SourceID, safeGitHubSourceError(resultErr), finalizerErr,
	)
	// final fence 丢失表示已有新任务接管 runtime，不应覆盖；数据库
	// finalizer 失败则保持可重试，避免继承永久错误或 provider deadline 分类。
	if errors.Is(finalizerErr, errSourceFinalFence) {
		return resultErr
	}
	return errors.New(errPagesSourceSyncFailed)
}

func persistGitHubSyncFailure(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	resultErr error,
) error {
	var domainError *githubSourceProviderDomainError
	if errors.As(resultErr, &domainError) && domainError.retryAt != nil {
		return failGitHubCheckLease(ctx, snapshot, domainError.message, *domainError.retryAt)
	}
	return failSourceLease(ctx, snapshot, safeGitHubSourceError(resultErr))
}

func activatePreparedGitHubSource(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	actor string,
	triggerType string,
	prepared *preparedGitHubSource,
) (*sourceSyncOutcome, error) {
	task.AppendLog(ctx, "[activate] 正在原子切换 GitHub Release 部署")
	deployment, reused, referenced, err := commitSourceDeploymentWithTrigger(
		ctx, snapshot, prepared.target.Revision, prepared.download.SHA256,
		prepared.target.Detail, prepared.target.DetailJSON, actor, triggerType, prepared.manifest,
		prepared.ingestState.Result, prepared.ingestState.HasIngest, prepared.target.RetryAt,
	)
	prepared.ingestState.Referenced = referenced
	if errors.Is(err, errSourceFinalFence) {
		return &sourceSyncOutcome{Stale: true}, nil
	}
	if err != nil {
		return nil, err
	}
	prepared.ingestState.Referenced = prepared.ingestState.HasIngest && deployment.UploadID == prepared.ingestState.Result.Upload.ID
	if pruneErr := pruneProjectDeploymentHistory(ctx, snapshot.ProjectID, prepared.limits.HistoryCount, 0); pruneErr != nil {
		logger.ErrorF(ctx, "[PagesSource] strict prune failed after GitHub sync: project_id=%d source_id=%d error=%v", snapshot.ProjectID, snapshot.SourceID, pruneErr)
	}
	view := buildDeploymentView(deployment)
	return &sourceSyncOutcome{Deployment: &view, Reused: reused}, nil
}

func resolveAndGuardGitHubSync(
	ctx context.Context,
	client githubReleaseAPI,
	snapshot *sourceExecutionSnapshot,
	targetRevision string,
	confirmedRevision string,
) (*githubSourceTarget, *sourceSyncOutcome, error) {
	task.AppendLog(ctx, "[resolve] 正在解析 GitHub Release：repo=%s asset=%s", snapshot.GitHubRepository, snapshot.AssetName)
	resolved, err := client.Resolve(ctx, githubrelease.ResolveRequest{
		Repository: snapshot.GitHubRepository,
		Selector:   githubrelease.Selector(snapshot.ReleaseSelector),
		Tag:        snapshot.ReleaseTag,
		AssetName:  snapshot.AssetName,
	})
	if err != nil {
		logger.WarnF(ctx, "[PagesSource] GitHub resolve failed: source_id=%d repo=%s error=%v", snapshot.SourceID, snapshot.GitHubRepository, err)
		return nil, nil, githubSourceDomainError(err)
	}
	if resolved.NotModified {
		return nil, nil, errors.New(errPagesSourceReleaseNotFound)
	}
	target, err := buildGitHubSourceTarget(resolved.Release, resolved.Asset, resolved.RetryAt)
	if err != nil {
		return nil, nil, &githubSourceProviderDomainError{
			message:   safeGitHubSourceError(err),
			permanent: isPermanentSourceSyncError(err),
			retryAt:   resolved.RetryAt,
		}
	}
	guardedOutcome, err := guardGitHubSyncTarget(ctx, snapshot, target, targetRevision, confirmedRevision)
	return target, guardedOutcome, err
}

func guardGitHubSyncTarget(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	target *githubSourceTarget,
	targetRevision string,
	confirmedRevision string,
) (*sourceSyncOutcome, error) {
	status := targetRuntimeStatus(target, snapshot.LastAppliedRevision, snapshot.LastAppliedDetail)
	if targetRevision != "" && targetRevision != target.Revision {
		return releaseGuardedGitHubTarget(ctx, snapshot, target, status, "", true, true)
	}
	if confirmedRevision != "" && (confirmedRevision != snapshot.LastSeenRevision || confirmedRevision != target.Revision) {
		return releaseGuardedGitHubTarget(ctx, snapshot, target, status, errPagesSourceConfirmationStale, false, false)
	}
	if status == pagesSourceStatusAttention && confirmedRevision != target.Revision {
		return releaseGuardedGitHubTarget(ctx, snapshot, target, status, errPagesSourceConfirmationNeeded, false, false)
	}
	if confirmedRevision != "" && status != pagesSourceStatusAttention {
		return nil, errors.New(errPagesSourceConfirmationStale)
	}
	return nil, nil
}

func releaseGuardedGitHubTarget(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	target *githubSourceTarget,
	status string,
	lastError string,
	expedite bool,
	staleSuccess bool,
) (*sourceSyncOutcome, error) {
	err := releaseGitHubSyncWithoutActivation(ctx, snapshot, target, status, lastError, expedite, target.RetryAt)
	if errors.Is(err, errSourceFinalFence) || (err == nil && staleSuccess) {
		return &sourceSyncOutcome{Stale: true}, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, errors.New(lastError)
}

func prepareGitHubSyncPackage(
	ctx context.Context,
	client githubReleaseAPI,
	snapshot *sourceExecutionSnapshot,
	target *githubSourceTarget,
) (*preparedGitHubSource, error) {
	limits := resolvePagesLimits(ctx)
	task.AppendLog(ctx, "[download] 正在下载 GitHub Release asset：repo=%s asset=%s", snapshot.GitHubRepository, snapshot.AssetName)
	download, err := client.Download(ctx, githubrelease.DownloadRequest{
		Repository: snapshot.GitHubRepository,
		Asset:      target.Asset,
		MaxBytes:   limits.PackageBytes,
	})
	if err != nil {
		logger.WarnF(ctx, "[PagesSource] GitHub download failed: source_id=%d repo=%s asset=%s error=%v", snapshot.SourceID, snapshot.GitHubRepository, snapshot.AssetName, err)
		return nil, githubSourceDomainError(err)
	}
	prepared, err := inspectAndIngestGitHubPackage(ctx, snapshot, target, download, limits)
	if err != nil {
		if cleanupErr := download.Cleanup(); cleanupErr != nil {
			logger.WarnF(ctx, "[PagesSource] cleanup GitHub package after preparation failure failed: source_id=%d error=%v", snapshot.SourceID, cleanupErr)
		}
		return nil, err
	}
	return prepared, nil
}

func inspectAndIngestGitHubPackage(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	target *githubSourceTarget,
	download *githubrelease.DownloadResult,
	limits pagesLimits,
) (*preparedGitHubSource, error) {
	if download.SHA256 == "" || download.Path == "" {
		return nil, errors.New(errPagesSourceSyncFailed)
	}
	if target.Detail.Digest != "" && "sha256:"+download.SHA256 != target.Detail.Digest {
		return nil, errors.New(errPagesSourceDigestMismatch)
	}
	format, ok := pagesarchive.DetectFormatFromName(target.Asset.Name)
	var err error
	if !ok {
		format, _, err = detectRemoteSourceFormat(download.Path, target.Asset.Name, "")
		if err != nil {
			return nil, err
		}
	}
	rootDir, err := validateAndNormalizePagesRootDir(snapshot.RootDir)
	if err != nil {
		return nil, err
	}
	entryFile, err := validateAndNormalizePagesEntryFile(snapshot.EntryFile)
	if err != nil {
		return nil, err
	}
	task.AppendLog(ctx, "[verify] 正在校验 GitHub Release 归档与入口")
	manifest, err := inspectPagesPackage(download.Path, format, rootDir, entryFile, limits)
	if err != nil {
		return nil, err
	}
	ingestState, err := resolveGitHubSourceIngest(ctx, snapshot, target, download, format)
	if err != nil {
		return nil, err
	}
	return &preparedGitHubSource{
		target: target, download: download, format: format,
		manifest: manifest, ingestState: ingestState, limits: limits,
	}, nil
}

func resolveGitHubSourceIngest(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	target *githubSourceTarget,
	download *githubrelease.DownloadResult,
	format pagesarchive.Format,
) (*sourceIngestState, error) {
	if _, err := findSourceDeployment(ctx, snapshot.ProjectID, snapshot.SourceIdentity, target.Revision); err == nil {
		return &sourceIngestState{}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	task.AppendLog(ctx, "[ingest] 正在保存 GitHub Release 部署包")
	result, err := ingestPagesDeploymentPackageWithSource(
		ctx, download.Path, download.SHA256, snapshot.ProjectID, snapshot.SourceID, target.Asset.Name, format,
	)
	if err != nil {
		return nil, err
	}
	return &sourceIngestState{Result: result, HasIngest: true}, nil
}

func releaseGitHubSyncWithoutActivation(
	ctx context.Context,
	snapshot *sourceExecutionSnapshot,
	target *githubSourceTarget,
	status string,
	lastError string,
	expedite bool,
	retryAt *time.Time,
) error {
	now := time.Now()
	nextCheckAt := nextCheckAfterGitHubResponse(snapshot, now, retryAt)
	if expedite && snapshot.ReleaseSelector == githubReleaseSelectorLatest {
		next := now.Add(initialCheckRetryDelay)
		if retryAt != nil && retryAt.After(next) {
			next = retryAt.In(now.Location())
		}
		nextCheckAt = &next
	}
	updates := map[string]any{
		"last_seen_revision":              target.Revision,
		"last_seen_detail":                target.DetailJSON,
		sourceRuntimeColumnSyncStatus:     status,
		sourceRuntimeColumnLastError:      lastError,
		sourceRuntimeColumnLastCheckedAt:  &now,
		sourceRuntimeColumnNextCheckAt:    nextCheckAt,
		sourceRuntimeColumnLeaseToken:     "",
		sourceRuntimeColumnLeaseExpiresAt: nil,
	}
	result := db.DB(ctx).Model(&model.PagesProjectSourceRuntime{}).
		Where("source_id = ? AND lease_token = ? AND lease_expires_at > ?", snapshot.SourceID, snapshot.LeaseToken, now).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errSourceFinalFence
	}
	return nil
}

func safeGitHubSourceError(err error) string {
	if err == nil {
		return errPagesSourceSyncFailed
	}
	message := strings.TrimSpace(err.Error())
	for _, safeMessage := range []string{
		errPagesSourceSyncFailed,
		errPagesSourceReleaseNotFound,
		errPagesSourceDigestInvalid,
		errPagesSourceDigestMismatch,
		errPagesSourceConfirmationNeeded,
		errPagesSourceConfirmationStale,
		errPagesPackageURLTooLarge,
		errPagesPackageEmpty,
		errPagesPackageUnsupported,
		errPagesPackageInvalid,
		errPagesPackageExtractedTooLarge,
		errPagesPackageFileTooLarge,
		errPagesEntryFileMissing,
	} {
		if message == safeMessage {
			return safeMessage
		}
	}
	return errPagesSourceSyncFailed
}

func githubSourceDomainError(err error) error {
	message := errPagesSourceSyncFailed
	statusCode := 0
	var providerError *githubrelease.Error
	if errors.As(err, &providerError) {
		statusCode = providerError.StatusCode
	}
	retryAt, hasRetryAt := githubrelease.RetryAt(err)
	var retryDeadline *time.Time
	if hasRetryAt {
		retryDeadline = &retryAt
	}
	if err == nil {
		return &githubSourceProviderDomainError{message: message, permanent: false, statusCode: statusCode}
	}
	if githubrelease.IsDigestError(err) {
		message = errPagesSourceDigestMismatch
		return &githubSourceProviderDomainError{message: message, permanent: true, statusCode: statusCode}
	}
	if githubrelease.IsNotFound(err) {
		message = errPagesSourceReleaseNotFound
		return &githubSourceProviderDomainError{message: message, permanent: true, statusCode: statusCode}
	}
	if errors.Is(err, githubrelease.ErrAssetTooLarge) {
		message = errPagesPackageURLTooLarge
		return &githubSourceProviderDomainError{message: message, permanent: true, statusCode: statusCode}
	}
	if errors.Is(err, githubrelease.ErrEmptyAsset) {
		message = errPagesPackageEmpty
		return &githubSourceProviderDomainError{message: message, permanent: true, statusCode: statusCode}
	}
	return &githubSourceProviderDomainError{
		message: message, permanent: !githubrelease.IsRetryable(err), retryAt: retryDeadline, statusCode: statusCode,
	}
}

func shouldSkipGitHubActionRetry(err error) bool {
	var domainError *githubSourceProviderDomainError
	return errors.As(err, &domainError) && (domainError.permanent || domainError.retryAt != nil)
}
