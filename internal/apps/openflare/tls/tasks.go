// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
)

const (
	// SSLSingleRenewTask renews a single ACME TLS certificate.
	SSLSingleRenewTask = "openflare:ssl_single_renew"
	// TaskTypeSSLSingleRenew is the admin task type for single SSL renewal.
	TaskTypeSSLSingleRenew = "of_ssl_single_renew"
)

// SSLSingleRenewMeta describes the single SSL renewal task.
var SSLSingleRenewMeta = task.TaskMeta{
	Type:         TaskTypeSSLSingleRenew,
	AsynqTask:    SSLSingleRenewTask,
	Name:         "OpenFlare 单证书 SSL 续期",
	Description:  "对单个指定的 ACME 证书执行续期",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{
			Name:        "id",
			Label:       "证书 ID",
			Type:        "number",
			Required:    true,
			Placeholder: "请输入证书 ID",
			Description: "待续期的 TLS 证书 ID",
		},
	},
}

// SSLSingleRenewPayload is the payload structure for SSLSingleRenewTask.
type SSLSingleRenewPayload struct {
	ID uint `json:"id"`
}

// SSLSingleRenewHandler renews a specific TLS certificate.
type SSLSingleRenewHandler struct{}

// ValidatePayload validates and normalizes the task payload.
func (h *SSLSingleRenewHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("任务参数不能为空")
	}

	var req SSLSingleRenewPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("无效的 JSON 格式: %w", err)
	}

	if req.ID == 0 {
		return nil, errors.New("证书 ID 不能为空或零")
	}

	return json.Marshal(req)
}

// Execute runs the certificate renewal for the specified ID.
func (h *SSLSingleRenewHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	var req SSLSingleRenewPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("解析任务参数: %w", err)
	}

	task.AppendLog(ctx, "开始续期证书，ID: %d", req.ID)

	cert, err := model.GetTLSCertificateByID(ctx, req.ID)
	if err != nil {
		task.AppendLog(ctx, "获取证书记录失败 ID=%d: %v", req.ID, err)
		return nil, fmt.Errorf("获取证书记录失败: %w", err)
	}

	if cert.Provider != tlsProviderACME {
		task.AppendLog(ctx, "证书 %s (ID=%d) 不是 ACME 托管证书，无法自动续期 (Provider: %s)", cert.PrimaryDomain, req.ID, cert.Provider)
		return nil, fmt.Errorf("证书 %s 不是 ACME 托管证书，无法自动续期", cert.PrimaryDomain)
	}

	task.AppendLog(ctx, "准备为域名 [%s] 申请/续期证书", cert.PrimaryDomain)
	if err := obtainTLSCertificate(ctx, cert); err != nil {
		task.AppendLog(ctx, "申请证书失败: %v", err)
		return nil, err
	}

	msg := fmt.Sprintf("证书 %s 续签成功", cert.PrimaryDomain)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}
