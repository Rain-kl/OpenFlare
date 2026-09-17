// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	pkgpush "Wavelet/plugins/domain/msg_gateway/push"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	// SendNotificationTask is the asynq task name for push notification.
	SendNotificationTask = consts.SendNotificationTask
	// TaskTypeSendNotification is the admin task manager type identifier.
	TaskTypeSendNotification = consts.TaskTypeSendNotification
)

// SendNotificationMeta represents the task metadata.
var SendNotificationMeta = contracts.TaskMetaDTO{
	Type:         TaskTypeSendNotification,
	AsynqTask:    SendNotificationTask,
	Name:         "推送通知",
	DisplayName:  "推送通知",
	Description:  "异步执行系统通知的多渠道派发与推送",
	Category:     "push",
	SupportsTime: false,
	MaxRetry:     3,
	Queue:        "default",
	Retryable:    true,
	Params: []contracts.TaskParamDTO{
		{
			Name:        "event_key",
			Label:       "事件标识",
			Type:        "string",
			Required:    true,
			Placeholder: "admin_login",
			Description: "事件标识 (如 admin_login)",
		},
		{
			Name:        "target",
			Label:       "目标接收者",
			Type:        "string",
			Required:    false,
			Description: "目标接收者",
		},
	},
}

// PushHandler handles asynchronous notification sending.
type PushHandler struct{}

// ValidatePayload validates and normalizes push parameters.
func (h *PushHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New(consts.ErrPayloadRequired)
	}

	var req do.SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("%s: %w", consts.ErrInvalidJSONFormat, err)
	}

	if req.Config.Channel == "" {
		return nil, errors.New(consts.ErrChannelTypeRequired)
	}

	return json.Marshal(req)
}

// Execute performs the push send and logs delivery history audit.
func (h *PushHandler) Execute(ctx context.Context, payload []byte) error {
	var req do.SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		logger.ErrorF(ctx, "[Push] 解析推送参数失败: %v", err)
		return fmt.Errorf("%s: %w", consts.ErrParsePayloadFailed, err)
	}

	logger.InfoF(ctx, "[Push] 开始推送通知: 事件 = %s, 渠道 = %s, 接收目标 = %s", req.EventKey, req.Config.Channel, req.Target)

	pusher, err := pkgpush.GetPusher(req.Config.Channel)
	if err != nil {
		errWrap := fmt.Errorf("%s: %w", consts.ErrGetPusherFailed, err)
		logger.ErrorF(ctx, "[Push] 推送失败: %v", errWrap)
		h.recordHistory(ctx, req, "failed", errWrap.Error())
		return errWrap
	}

	flatBody := req.Body.Flatten()
	upstreamResp, err := pusher.Send(ctx, req.Config, req.Target, flatBody, req.Template, nil)

	title := req.Body.Title
	content := req.Body.Content

	if err != nil {
		logger.ErrorF(ctx, "[Push] 消息推送失败 (标题: %s): %v, 上游返回: %s", title, err, upstreamResp)
		h.recordHistory(ctx, req, "failed", err.Error())
		return fmt.Errorf("pusher.Send failed: %w", err)
	}

	logger.InfoF(ctx, "[Push] 消息推送成功 (标题: %s, 内容摘要: %s), 上游返回: %s", title, content, upstreamResp)
	h.recordHistory(ctx, req, "success", "")

	return nil
}

func (h *PushHandler) recordHistory(ctx context.Context, req do.SendPayload, status, errMsg string) {
	if dbErr := RecordPushHistory(ctx, req, status, errMsg); dbErr != nil {
		logger.ErrorF(ctx, "[Push] 写入推送历史审计记录失败: %v", dbErr)
	}
}

// EnqueuePushTask dispatches a notification payload to the async push worker.
func EnqueuePushTask(ctx context.Context, payload do.SendPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if taskSvc := GetTaskService(ctx); taskSvc != nil {
		_, err = taskSvc.Dispatch(ctx, "send_notification", payloadBytes, contracts.TaskTriggerSystem)
		return err
	}
	return errors.New(consts.ErrTaskServiceUnavailable)
}

// RecordPushHistory creates a push history audit record.
func RecordPushHistory(ctx context.Context, req do.SendPayload, status, errMsg string) error {
	title := req.Body.Title
	content := req.Body.Content
	level := req.Body.Level
	if title == "" {
		title = "系统通知"
	}
	if level == "" {
		level = consts.DefaultLevelInfo
	}

	target := req.Target
	if target == "" {
		if req.Config.URL != "" {
			target = req.Config.URL
			const maxTargetLen = 50
			const truncatedLen = 47
			if len(target) > maxTargetLen {
				target = target[:truncatedLen] + "..."
			}
		} else {
			target = "default"
		}
	}

	history := entity.PushHistory{
		EventKey: req.EventKey,
		Channel:  req.Config.Channel,
		Target:   target,
		Title:    title,
		Content:  content,
		Level:    level,
		Status:   status,
		ErrorMsg: errMsg,
	}
	return dao.CreatePushHistoryRecord(ctx, &history)
}

// ListPushHistories returns a paginated push delivery audit page.
func ListPushHistories(ctx context.Context, filter do.PushHistoryListFilter) (int64, []entity.PushHistory, error) {
	return dao.ListPushHistoriesRecord(ctx, filter)
}

// CleanupPushHistories removes push delivery audit records older than the retention duration.
func CleanupPushHistories(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	deleted, err := dao.DeletePushHistoriesBeforeRecord(ctx, cutoff)
	if err != nil {
		logger.WarnF(ctx, "[Push] 清理历史推送日志失败: %v", err)
		return 0, err
	}
	logger.InfoF(ctx, "[Push] 已清理 %s 前推送历史日志，共 %d 条", cutoff.Format(time.RFC3339), deleted)
	return deleted, nil
}
