// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package openflare implements openflare configuration, service orchestration, and background tasks.
package openflare

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tasks"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/uptimekuma"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const (
	// SSLRenewTask renews due ACME TLS certificates.
	SSLRenewTask = "openflare:ssl_renew"
	// TaskTypeSSLRenew is the admin task type for SSL renewal.
	TaskTypeSSLRenew = "of_ssl_renew"

	// WAFIPGroupSyncTask syncs due automatic/subscription WAF IP groups.
	WAFIPGroupSyncTask = "openflare:waf_ip_group_sync"
	// TaskTypeWAFIPGroupSync is the admin task type for WAF IP group sync.
	TaskTypeWAFIPGroupSync = "of_waf_ip_group_sync"

	// UptimeKumaSyncTask synchronizes proxy routes to Uptime Kuma monitors.
	UptimeKumaSyncTask = "openflare:uptime_kuma_sync"
	// TaskTypeUptimeKumaSync is the admin task type for Uptime Kuma sync.
	TaskTypeUptimeKumaSync = "of_uptime_kuma_sync"

	// LogDBSwitchTask 切换日志数据库任务标识。
	LogDBSwitchTask = "openflare:log_db_switch"
	// TaskTypeLogDBSwitch is the admin task type for log database switch.
	TaskTypeLogDBSwitch = "of_log_db_switch"
)

var (
	lastUptimeKumaSyncTime time.Time
	uptimeKumaSyncMutex    sync.Mutex
)

// SSLRenewMeta describes the SSL renewal task.
var SSLRenewMeta = task.TaskMeta{
	Type:         TaskTypeSSLRenew,
	AsynqTask:    SSLRenewTask,
	Name:         "OpenFlare automatic SSL renewal",
	Description:  "Scan ACME certificates nearing expiration and renew them automatically",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// WAFIPGroupSyncMeta describes the WAF IP group sync task.
var WAFIPGroupSyncMeta = task.TaskMeta{
	Type:         TaskTypeWAFIPGroupSync,
	AsynqTask:    WAFIPGroupSyncTask,
	Name:         "OpenFlare WAF IP group synchronization",
	Description:  "Synchronize automatic and subscription WAF IP groups that are due for an update",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// UptimeKumaSyncMeta describes the Uptime Kuma sync task.
var UptimeKumaSyncMeta = task.TaskMeta{
	Type:         TaskTypeUptimeKumaSync,
	AsynqTask:    UptimeKumaSyncTask,
	Name:         "OpenFlare Uptime Kuma synchronization",
	Description:  "Synchronize enabled proxy routes with Uptime Kuma monitors",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// LogDBSwitchMeta 描述切换日志数据库任务。
var LogDBSwitchMeta = task.TaskMeta{
	Type:         TaskTypeLogDBSwitch,
	AsynqTask:    LogDBSwitchTask,
	Name:         "Switch log database",
	Description:  "Copy log data and switch the log database on success; log writes are paused during migration",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{Name: "target", Label: "Target log database", Type: "string", Required: true,
			Placeholder: "postgres|sqlite|clickhouse", Description: "Migration target: postgres for a PostgreSQL primary, sqlite for a SQLite primary, or clickhouse"},
	},
}

// SSLRenewHandler renews due TLS certificates.
type SSLRenewHandler struct{}

// Execute runs SSL certificate renewal for all due certificates.
func (h *SSLRenewHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	task.AppendLog(ctx, "Scanning certificates due for renewal")
	if err := tasks.RunSSLRenewJob(ctx); err != nil {
		task.AppendLog(ctx, "Automatic SSL renewal failed: %v", err)
		return nil, err
	}
	msg := "Automatic SSL renewal task completed"
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}

// WAFIPGroupSyncHandler syncs due WAF IP groups to agents.
type WAFIPGroupSyncHandler struct{}

// Execute syncs all due automatic/subscription WAF IP groups.
func (h *WAFIPGroupSyncHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	task.AppendLog(ctx, "Synchronizing WAF IP groups due for an update")
	if err := waf.SyncDueWAFIPGroups(ctx); err != nil {
		task.AppendLog(ctx, "WAF IP group synchronization failed: %v", err)
		return nil, err
	}
	msg := "WAF IP group synchronization completed"
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}

// UptimeKumaSyncHandler synchronizes proxy routes to Uptime Kuma.
type UptimeKumaSyncHandler struct{}

// Execute runs Uptime Kuma sync when integration is enabled and the interval has elapsed.
func (h *UptimeKumaSyncHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	// 从 SystemConfig 读取 UptimeKuma 配置
	enabled, _ := repository.GetBoolByKey(ctx, model.ConfigKeyUptimeKumaEnabled)
	if !enabled {
		msg := "Uptime Kuma integration is disabled; skipping execution"
		task.AppendLog(ctx, "%s", msg)
		return &task.TaskResult{Message: msg}, nil
	}

	interval, _ := repository.GetIntByKey(ctx, model.ConfigKeyUptimeKumaSyncInterval)
	if interval <= 0 {
		interval = 5
	}
	if time.Since(lastUptimeKumaSyncTime) < time.Duration(interval)*time.Minute {
		msg := fmt.Sprintf("Less than %d minutes since the last synchronization; skipping execution", interval)
		task.AppendLog(ctx, "%s", msg)
		return &task.TaskResult{Message: msg}, nil
	}

	if !uptimeKumaSyncMutex.TryLock() {
		msg := "Uptime Kuma synchronization is already running; skipping this schedule"
		task.AppendLog(ctx, "%s", msg)
		return &task.TaskResult{Message: msg}, nil
	}
	defer uptimeKumaSyncMutex.Unlock()

	task.AppendLog(ctx, "Synchronizing proxy routes with Uptime Kuma")
	if err := uptimekuma.SyncToUptimeKuma(ctx); err != nil {
		task.AppendLog(ctx, "Uptime Kuma synchronization failed: %v", err)
		return nil, err
	}

	lastUptimeKumaSyncTime = time.Now()
	msg := "Uptime Kuma synchronization completed"
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}
