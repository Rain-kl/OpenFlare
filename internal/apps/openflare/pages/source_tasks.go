// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

const (
	// PagesSourceActionTask is the private Asynq task type for source actions.
	PagesSourceActionTask = "openflare:pages_source_action"
	// TaskTypePagesSourceAction is the internal task meta type.
	TaskTypePagesSourceAction = "of_pages_source_action"

	sourceActionCheck = "check"
	sourceActionSync  = "sync"
)

var errUnexpectedJSONTrailingValue = errors.New("unexpected trailing JSON value")

// PagesSourceActionMeta describes check/sync work for Pages deployment sources.
var PagesSourceActionMeta = task.TaskMeta{
	Type:         TaskTypePagesSourceAction,
	AsynqTask:    PagesSourceActionTask,
	Name:         "OpenFlare Pages 部署源操作",
	Description:  "检查或同步 Pages 项目部署源",
	SupportsTime: false,
	MaxRetry:     2,
	Queue:        task.QueueDefault,
	Retryable:    false,
}

// SourceActionPayload is the credential-free internal queue contract.
type SourceActionPayload struct {
	SourceID          uint   `json:"source_id"`
	ConfigVersion     int    `json:"config_version"`
	Action            string `json:"action"`
	Actor             string `json:"actor"`
	TriggerType       string `json:"trigger_type"`
	TargetRevision    string `json:"target_revision"`
	ConfirmedRevision string `json:"confirmed_revision"`
}

// SourceActionHandler executes a validated source action.
type SourceActionHandler struct{}

// ValidatePayload rejects unknown keys and normalizes the internal contract.
func (h *SourceActionHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var input SourceActionPayload
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	input.Action = strings.TrimSpace(input.Action)
	input.Actor = strings.TrimSpace(input.Actor)
	input.TriggerType = strings.TrimSpace(input.TriggerType)
	input.TargetRevision = strings.TrimSpace(input.TargetRevision)
	input.ConfirmedRevision = strings.TrimSpace(input.ConfirmedRevision)
	if input.Action == sourceActionSync && input.TriggerType == "" {
		// Keep already queued Phase 2 payloads valid while making every new
		// dispatch carry an explicit deployment trigger.
		input.TriggerType = pagesSourceTriggerManualSync
	}
	if !validSourceActionPayload(input) {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	return json.Marshal(input)
}

func validSourceActionPayload(input SourceActionPayload) bool {
	if input.SourceID == 0 || input.ConfigVersion <= 0 {
		return false
	}
	if input.Action != sourceActionCheck && input.Action != sourceActionSync {
		return false
	}
	if !validPagesSourceActor(input.Actor) {
		return false
	}
	if !validOptionalSourceRevision(input.TargetRevision) ||
		!validOptionalSourceRevision(input.ConfirmedRevision) {
		return false
	}
	if input.Action == sourceActionCheck {
		return input.TriggerType == "" && input.TargetRevision == "" && input.ConfirmedRevision == ""
	}
	return validSourceSyncPayload(input)
}

func validSourceSyncPayload(input SourceActionPayload) bool {
	if !validSourceDeploymentTrigger(input.TriggerType) ||
		(input.TargetRevision != "" && input.ConfirmedRevision != "") {
		return false
	}
	switch input.TriggerType {
	case pagesSourceTriggerScheduledAutoUpdate:
		return input.Actor == pagesSourceCreatedBySystem &&
			input.TargetRevision != "" && input.ConfirmedRevision == ""
	case pagesSourceTriggerManualSync:
		if input.TargetRevision != "" || !strings.HasPrefix(input.Actor, "user:") {
			return false
		}
		return true
	default:
		return false
	}
}

// Execute validates again inside the worker and performs the source action.
func (h *SourceActionHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	normalized, err := h.ValidatePayload(payload)
	if err != nil {
		return nil, task.PermanentError(errPagesSourceActionInvalid)
	}
	var input SourceActionPayload
	if err := json.Unmarshal(normalized, &input); err != nil {
		return nil, task.PermanentError(errPagesSourceActionInvalid)
	}

	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("id = ?", input.SourceID).First(&source).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			task.AppendLog(ctx, "[resolve] 来源已不存在，本次任务跳过")
			return &task.TaskResult{Message: errPagesSourceActionStale}, nil
		}
		logger.ErrorF(ctx, "[PagesSource] load source failed: source_id=%d error=%v", input.SourceID, err)
		return nil, errors.New(errPagesSourceSyncFailed)
	}
	if source.ConfigVersion != input.ConfigVersion {
		task.AppendLog(ctx, "[resolve] 来源配置已变化，本次任务跳过")
		return &task.TaskResult{Message: errPagesSourceActionStale}, nil
	}
	if input.Action == sourceActionCheck && source.SourceType == PagesSourceTypeRemoteURL {
		return nil, task.PermanentError(errPagesSourceCheckUnsupported)
	}
	if source.SourceType != PagesSourceTypeRemoteURL && source.SourceType != PagesSourceTypeGitHubRelease {
		return nil, task.PermanentError(errPagesSourceTypeUnsupported)
	}
	if source.SourceType == PagesSourceTypeRemoteURL && (input.TargetRevision != "" || input.ConfirmedRevision != "") {
		return nil, task.PermanentError(errPagesSourceActionInvalid)
	}
	if input.Action == sourceActionCheck && (input.TargetRevision != "" || input.ConfirmedRevision != "") {
		return nil, task.PermanentError(errPagesSourceActionInvalid)
	}

	task.AppendLog(ctx, "[resolve] 正在获取来源执行权")
	snapshot, outcome, err := acquireSourceLease(ctx, input.SourceID, input.ConfigVersion, input.Action)
	if err != nil {
		logger.ErrorF(ctx, "[PagesSource] acquire lease failed: source_id=%d error=%v", input.SourceID, err)
		return nil, errors.New(errPagesSourceSyncFailed)
	}
	switch outcome {
	case sourceLeaseBusy:
		task.AppendLog(ctx, "[resolve] 已有来源任务正在执行，本次任务跳过")
		return &task.TaskResult{Message: errPagesSourceActionBusy}, nil
	case sourceLeaseStale:
		task.AppendLog(ctx, "[resolve] 来源配置或执行权已变化，本次任务跳过")
		return &task.TaskResult{Message: errPagesSourceActionStale}, nil
	}

	if input.Action == sourceActionCheck {
		return executeGitHubCheckAction(ctx, snapshot)
	}
	return executeSourceSyncAction(ctx, &source, snapshot, input)
}

func executeGitHubCheckAction(ctx context.Context, snapshot *sourceExecutionSnapshot) (*task.TaskResult, error) {
	checkResult, checkErr := checkGitHubSource(ctx, snapshot)
	if checkErr != nil {
		logger.ErrorF(ctx, "[PagesSource] check failed: project_id=%d source_id=%d error=%v", snapshot.ProjectID, snapshot.SourceID, checkErr)
		if isPermanentSourceSyncError(checkErr) || shouldSkipGitHubActionRetry(checkErr) {
			return nil, task.PermanentError(checkErr.Error())
		}
		return nil, errors.New(errPagesSourceSyncFailed)
	}
	if checkResult == nil || checkResult.Stale {
		return &task.TaskResult{Message: errPagesSourceActionStale}, nil
	}
	return &task.TaskResult{Message: checkResult.Message, Detail: checkResult.Detail}, nil
}

func executeSourceSyncAction(
	ctx context.Context,
	source *model.PagesProjectSource,
	snapshot *sourceExecutionSnapshot,
	input SourceActionPayload,
) (*task.TaskResult, error) {
	var result *sourceSyncOutcome
	var err error
	if source.SourceType == PagesSourceTypeGitHubRelease {
		result, err = syncGitHubSourceWithTrigger(
			ctx, snapshot, input.Actor, input.TargetRevision, input.ConfirmedRevision, input.TriggerType,
		)
	} else {
		result, err = syncRemoteSourceWithTrigger(ctx, snapshot, input.Actor, input.TriggerType)
	}
	if err != nil {
		logger.ErrorF(ctx, "[PagesSource] sync failed: project_id=%d source_id=%d error=%v", snapshot.ProjectID, snapshot.SourceID, err)
		if isPermanentSourceSyncError(err) || shouldSkipGitHubActionRetry(err) {
			return nil, task.PermanentError(errPagesSourceSyncFailed)
		}
		return nil, errors.New(errPagesSourceSyncFailed)
	}
	if result == nil || result.Stale {
		task.AppendLog(ctx, "[activate] 来源配置或执行权已变化，本次任务未切换部署")
		return &task.TaskResult{Message: errPagesSourceActionStale}, nil
	}
	message := "Pages 部署源同步并发布成功"
	if result.Reused {
		message = "Pages 部署源内容未变化，已重新激活现有部署"
	}
	return &task.TaskResult{Message: message, Detail: sourceSyncResultDetail(result)}, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errUnexpectedJSONTrailingValue
	}
	return err
}

func validPagesSourceActor(actor string) bool {
	if actor == pagesSourceCreatedBySystem {
		return true
	}
	if !strings.HasPrefix(actor, "user:") {
		return false
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(actor, "user:"), 10, 64)
	return err == nil && id > 0
}

func validOptionalSourceRevision(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != sourceRevisionHexLength {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func isPermanentSourceSyncError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, errPagesPackageUnsupported) ||
		strings.Contains(message, errPagesPackageURLTooLarge) ||
		strings.Contains(message, errPagesPackageInvalid) ||
		strings.Contains(message, errPagesPackageEmpty) ||
		strings.Contains(message, errPagesPackageExtractedTooLarge) ||
		strings.Contains(message, errPagesPackageFileTooLarge) ||
		strings.Contains(message, errPagesEntryFileMissing) ||
		strings.Contains(message, errPagesSourceRemoteURLInvalid) ||
		strings.Contains(message, errPagesSourceReleaseNotFound) ||
		strings.Contains(message, errPagesSourceDigestInvalid) ||
		strings.Contains(message, errPagesSourceDigestMismatch) ||
		strings.Contains(message, errPagesSourceConfirmationNeeded) ||
		strings.Contains(message, errPagesSourceConfirmationStale)
}

// DispatchSourceAction performs API preflight and enqueues a credential-free action.
func DispatchSourceAction(
	ctx context.Context,
	projectID uint,
	action string,
	actor string,
	confirmedRevision string,
) (*SourceActionReceipt, error) {
	return dispatchSourceActionByProject(ctx, projectID, action, actor, "", confirmedRevision)
}

func dispatchSourceActionByProject(
	ctx context.Context,
	projectID uint,
	action string,
	actor string,
	targetRevision string,
	confirmedRevision string,
) (*SourceActionReceipt, error) {
	action = strings.TrimSpace(action)
	targetRevision = strings.TrimSpace(targetRevision)
	confirmedRevision = strings.TrimSpace(confirmedRevision)
	if action != sourceActionCheck && action != sourceActionSync {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	if !validPagesSourceActor(actor) {
		return nil, errors.New(errPagesSourceActionInvalid)
	}

	var source model.PagesProjectSource
	if err := db.DB(ctx).Where("project_id = ?", projectID).First(&source).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errPagesSourceNotFound)
		}
		return nil, err
	}
	if err := validateSourceActionPreflight(ctx, &source, action, targetRevision, confirmedRevision); err != nil {
		return nil, err
	}
	busy, err := sourceLeaseIsBusy(ctx, source.ID)
	if err != nil {
		return nil, err
	}
	if busy {
		return nil, errors.New(errPagesSourceActionBusy)
	}
	return dispatchSourceActionSnapshot(ctx, source, action, actor, targetRevision, confirmedRevision, "manual")
}

func validateSourceActionPreflight(
	ctx context.Context,
	source *model.PagesProjectSource,
	action string,
	targetRevision string,
	confirmedRevision string,
) error {
	if source == nil {
		return errors.New(errPagesSourceNotFound)
	}
	if source.SourceType != PagesSourceTypeRemoteURL && source.SourceType != PagesSourceTypeGitHubRelease {
		return errors.New(errPagesSourceTypeUnsupported)
	}
	if action == sourceActionCheck && source.SourceType == PagesSourceTypeRemoteURL {
		return errors.New(errPagesSourceCheckUnsupported)
	}
	if source.SourceType == PagesSourceTypeRemoteURL && (targetRevision != "" || confirmedRevision != "") {
		return errors.New(errPagesSourceActionInvalid)
	}
	if action == sourceActionCheck && (targetRevision != "" || confirmedRevision != "") {
		return errors.New(errPagesSourceActionInvalid)
	}
	if source.SourceType == PagesSourceTypeGitHubRelease && action == sourceActionSync {
		if err := preflightGitHubSyncConfirmation(ctx, source.ID, confirmedRevision); err != nil {
			return err
		}
	}
	return nil
}

func dispatchSourceActionSnapshot(
	ctx context.Context,
	source model.PagesProjectSource,
	action string,
	actor string,
	targetRevision string,
	confirmedRevision string,
	triggeredBy string,
) (*SourceActionReceipt, error) {
	triggerType := ""
	if action == sourceActionSync {
		triggerType = pagesSourceTriggerManualSync
	}
	return dispatchSourceActionSnapshotWithTrigger(
		ctx, source, action, actor, triggerType, targetRevision, confirmedRevision, triggeredBy,
	)
}

func dispatchSourceActionSnapshotWithTrigger(
	ctx context.Context,
	source model.PagesProjectSource,
	action string,
	actor string,
	triggerType string,
	targetRevision string,
	confirmedRevision string,
	triggeredBy string,
) (*SourceActionReceipt, error) {
	if task.AsynqClient == nil {
		return nil, errors.New(errPagesSourceTaskDispatchFailed)
	}
	handler := &SourceActionHandler{}
	rawPayload, err := json.Marshal(SourceActionPayload{
		SourceID:          source.ID,
		ConfigVersion:     source.ConfigVersion,
		Action:            action,
		Actor:             actor,
		TriggerType:       triggerType,
		TargetRevision:    targetRevision,
		ConfirmedRevision: confirmedRevision,
	})
	if err != nil {
		return nil, errors.New(errPagesSourceActionInvalid)
	}
	payload, err := handler.ValidatePayload(rawPayload)
	if err != nil {
		return nil, err
	}
	taskID, err := task.DispatchTask(ctx, TaskTypePagesSourceAction, payload, triggeredBy)
	if err != nil {
		logger.ErrorF(ctx, "[PagesSource] dispatch action failed: project_id=%d source_id=%d action=%s error=%v", source.ProjectID, source.ID, action, err)
		return nil, errors.New(errPagesSourceTaskDispatchFailed)
	}
	execution, err := model.GetTaskExecutionByTaskID(ctx, taskID)
	if err != nil {
		logger.ErrorF(ctx, "[PagesSource] load dispatched execution failed: source_id=%d task_id=%s error=%v", source.ID, taskID, err)
		return nil, errors.New(errPagesSourceTaskDispatchFailed)
	}
	return &SourceActionReceipt{
		TaskID:      taskID,
		ExecutionID: strconv.FormatUint(execution.ID, 10),
		Action:      action,
	}, nil
}
