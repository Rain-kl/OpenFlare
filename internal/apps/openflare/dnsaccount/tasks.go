// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dnsaccount

import (
	"context"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

// RefreshTask is the scheduled DNS scope verification task.
const RefreshTask = "openflare:dns_account_refresh"

// RefreshMeta describes DNS scope refresh in the task catalog.
var RefreshMeta = task.TaskMeta{
	Type:        "of_dns_account_refresh",
	AsynqTask:   RefreshTask,
	Name:        "OpenFlare DNS account scope refresh",
	Description: "Verify DNS provider APIs and refresh the zones available to each DNS account",
	MaxRetry:    task.DefaultMaxRetry,
	Queue:       task.QueueDefault,
	Retryable:   true,
}

// RefreshHandler verifies all configured DNS accounts.
type RefreshHandler struct{}

// Execute refreshes each account and reports API and persistence failures.
func (*RefreshHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	accounts, err := repository.ListDNSAccounts(ctx)
	if err != nil {
		return nil, err
	}
	task.AppendLog(ctx, "Refreshing %d DNS accounts", len(accounts))
	failed, unavailable := 0, 0
	for i := range accounts {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err = refresh(ctx, &accounts[i]); err != nil {
			failed++
			task.AppendLog(ctx, "Failed to save DNS account verification: id=%d error=%v", accounts[i].ID, err)
		} else if accounts[i].VerificationStatus == model.DNSAccountStatusError {
			unavailable++
		}
	}
	if failed > 0 || unavailable > 0 {
		return nil, fmt.Errorf("DNS account refresh: %d unavailable, %d persistence failures", unavailable, failed)
	}
	return &task.TaskResult{Message: fmt.Sprintf("Refreshed %d DNS accounts", len(accounts))}, nil
}
