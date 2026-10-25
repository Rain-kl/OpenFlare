// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/mail"
)

// 异步任务名称与管理类型定义
const (
	// SendEmailTask 发送邮件任务标识
	SendEmailTask = "mail:send"
	// TaskTypeSendEmail 发送邮件管理类型
	TaskTypeSendEmail = "send_email"
)

// SendEmailMeta represents the task metadata.
var SendEmailMeta = task.TaskMeta{
	Type:         TaskTypeSendEmail,
	AsynqTask:    SendEmailTask,
	Name:         "Send email",
	Description:  "Send system email asynchronously",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "to",
			Label:       "Recipient email (To)",
			Type:        "string",
			Required:    true,
			Placeholder: "receiver@example.com",
			Description: "Destination email address",
		},
		{
			Name:        "subject",
			Label:       "Email subject",
			Type:        "string",
			Required:    true,
			Placeholder: "Enter the email subject",
			Description: "Subject line of the email to send",
		},
		{
			Name:        "body",
			Label:       "Email body",
			Type:        "text",
			Required:    true,
			Placeholder: "Enter the email body (HTML is supported)",
			Description: "Content of the email to send",
		},
	},
}

// SendEmailPayload 邮件发送任务载荷
type SendEmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// SendEmailHandler 发送验证码邮件的异步任务处理器
type SendEmailHandler struct{}

// ValidatePayload 实现 task.PayloadValidator 接口
// 校验并标准化邮件发送参数，框架在 Admin 下发时自动调用
func (h *SendEmailHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New(errTaskPayloadRequired)
	}

	var req SendEmailPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf(errInvalidJSONFormat, err)
	}

	req.To = strings.TrimSpace(req.To)
	req.Subject = strings.TrimSpace(req.Subject)
	req.Body = strings.TrimSpace(req.Body)

	if req.To == "" || req.Subject == "" || req.Body == "" {
		return nil, errors.New(errEmailTaskFieldsRequired)
	}

	return json.Marshal(req)
}

// Execute 执行邮件异步发送逻辑
func (h *SendEmailHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	var req SendEmailPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		task.AppendLog(ctx, "Failed to parse email parameters: %v", err)
		return nil, fmt.Errorf(errParseEmailPayloadFailed, err)
	}

	task.AppendLog(ctx, "Preparing to send email to: %s, subject: %s", req.To, req.Subject)

	// 从数据库读取最新的 SMTP 系统配置
	var smtpHost string
	var smtpPortVal string
	var smtpUsername string
	var smtpPassword string

	if sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPHost); err == nil {
		smtpHost = sc.Value
	}
	if sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPPort); err == nil {
		smtpPortVal = sc.Value
	}
	if sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPUsername); err == nil {
		smtpUsername = sc.Value
	}
	if sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPPassword); err == nil {
		smtpPassword = sc.Value
	}

	if smtpHost == "" || smtpPortVal == "" || smtpUsername == "" {
		err := errors.New(errSMTPConfigIncomplete)
		task.AppendLog(ctx, "Send failed: %v", err)
		return nil, err
	}

	smtpPort, err := strconv.Atoi(smtpPortVal)
	if err != nil {
		smtpPort = 587
	}

	cfg := mail.Config{
		Host:     smtpHost,
		Port:     smtpPort,
		Username: smtpUsername,
		Password: smtpPassword,
	}

	task.AppendLog(ctx, "Connecting to SMTP server: %s:%d, username: %s", smtpHost, smtpPort, smtpUsername)

	// 调用 SendMailHTML 执行邮件发送，这里会有 5s 拨号超时和 10s 读写限制
	err = mail.SendMailHTML(ctx, cfg, req.To, req.Subject, req.Body)
	if err != nil {
		task.AppendLog(ctx, "Failed to send email: %v", err)
		return nil, fmt.Errorf(errSendMailFailed, err)
	}

	msg := "Email sent successfully to: " + req.To
	task.AppendLog(ctx, "%s", msg)

	return &task.TaskResult{
		Message: msg,
	}, nil
}
