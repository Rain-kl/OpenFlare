// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package push defines push notification HTTP routes and background tasks.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/pkg/push"
)

const (
	// SendNotificationTask 发送推送通知任务标识
	SendNotificationTask = "push:send"
	// TaskTypeSendNotification 推送通知管理类型
	TaskTypeSendNotification = "send_notification"
)

// SendNotificationMeta represents the task metadata.
var SendNotificationMeta = task.TaskMeta{
	Type:         TaskTypeSendNotification,
	AsynqTask:    SendNotificationTask,
	Name:         "Send notification",
	Description:  "Dispatch system notifications asynchronously across multiple channels",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "event_key",
			Label:       "Event key",
			Type:        "string",
			Required:    true,
			Placeholder: "admin_login",
		},
		{
			Name:     "target",
			Label:    "Recipient",
			Type:     "string",
			Required: false,
		},
	},
}

// PushHandler 通知推送异步任务处理器
//
//nolint:revive
type PushHandler struct{}

// ValidatePayload 校验并标准化推送参数
func (h *PushHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("payload is required")
	}

	var req SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("invalid json format: %w", err)
	}

	if req.Config.Channel == "" {
		return nil, errors.New("channel type is required")
	}

	return json.Marshal(req)
}

// Execute 异步执行推送操作并记录推送历史审计
func (h *PushHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	var req SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		task.AppendLog(ctx, "Failed to parse notification parameters: %v", err)
		return nil, fmt.Errorf("parse payload failed: %w", err)
	}

	task.AppendLog(ctx, "Starting notification: event = %s, channel = %s, recipient = %s", req.EventKey, req.Config.Channel, req.Target)

	pusher, err := push.GetPusher(req.Config.Channel)
	if err != nil {
		errWrap := fmt.Errorf("get pusher failed: %w", err)
		task.AppendLog(ctx, "Notification failed: %v", errWrap)
		if task.IsFinalAttempt(ctx) {
			h.recordHistory(ctx, req, "failed", errWrap.Error())
		}
		return nil, errWrap
	}

	// 执行真正的消息推送，扁平化为原始 json 格式
	flatBody := req.Body.Flatten()
	upstreamResp, err := pusher.Send(ctx, req.Config, req.Target, flatBody, req.Template, nil)

	title := req.Body.Title
	content := req.Body.Content

	if err != nil {
		task.AppendLog(ctx, "Failed to send notification (title: %s): %v", title, err)
		if upstreamResp != "" {
			task.AppendLog(ctx, "Upstream response: %s", upstreamResp)
		}
		if task.IsFinalAttempt(ctx) {
			h.recordHistory(ctx, req, "failed", err.Error())
		}
		return nil, fmt.Errorf("pusher.Send failed: %w", err)
	}

	task.AppendLog(ctx, "Notification sent (title: %s, content summary: %s)", title, content)
	if upstreamResp != "" {
		task.AppendLog(ctx, "Upstream response: %s", upstreamResp)
	}
	h.recordHistory(ctx, req, "success", "")

	return &task.TaskResult{
		Message: fmt.Sprintf("Notification sent: [%s] -> %s", req.Config.Channel, req.Target),
	}, nil
}

func (h *PushHandler) recordHistory(ctx context.Context, req SendPayload, status string, errMsg string) {
	if dbErr := recordPushHistory(ctx, req, status, errMsg); dbErr != nil {
		task.AppendLog(ctx, "Failed to write notification audit history: %v", dbErr)
	}
}
