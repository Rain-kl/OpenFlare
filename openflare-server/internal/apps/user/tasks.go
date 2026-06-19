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

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/task"
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
	Name:         "发送邮件",
	Description:  "异步发送系统邮件",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "to",
			Label:       "接收邮箱 (To)",
			Type:        "string",
			Required:    true,
			Placeholder: "receiver@example.com",
			Description: "接收邮件的目标邮箱地址",
		},
		{
			Name:        "subject",
			Label:       "邮件主题 (Subject)",
			Type:        "string",
			Required:    true,
			Placeholder: "请输入邮件主题",
			Description: "发送邮件的主题标题",
		},
		{
			Name:        "body",
			Label:       "邮件内容 (Body)",
			Type:        "text",
			Required:    true,
			Placeholder: "请输入邮件内容（支持 HTML 格式）",
			Description: "发送邮件的内容主体",
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
		task.AppendLog(ctx, "解析邮件发送参数失败: %v", err)
		return nil, fmt.Errorf(errParseEmailPayloadFailed, err)
	}

	task.AppendLog(ctx, "开始准备发送邮件到: %s, 主题: %s", req.To, req.Subject)

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
		task.AppendLog(ctx, "发送失败: %v", err)
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

	task.AppendLog(ctx, "连接 SMTP 服务器: %s:%d, 用户名: %s", smtpHost, smtpPort, smtpUsername)

	// 调用 SendMailHTML 执行邮件发送，这里会有 5s 拨号超时和 10s 读写限制
	err = mail.SendMailHTML(ctx, cfg, req.To, req.Subject, req.Body)
	if err != nil {
		task.AppendLog(ctx, "邮件发送失败: %v", err)
		return nil, fmt.Errorf(errSendMailFailed, err)
	}

	msg := fmt.Sprintf("邮件成功发送至: %s", req.To)
	task.AppendLog(ctx, "%s", msg)

	return &task.TaskResult{
		Message: msg,
	}, nil
}
