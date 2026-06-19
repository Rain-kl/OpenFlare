// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tasks"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/uptimekuma"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/waf"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/task"
)

const (
	// SSLRenewTask renews due ACME TLS certificates.
	SSLRenewTask = "openflare:ssl_renew"
	// TaskTypeSSLRenew is the admin task type for SSL renewal.
	TaskTypeSSLRenew = "of_ssl_renew"

	// DatabaseAutoCleanupTask prunes observability tables by retention policy.
	DatabaseAutoCleanupTask = "openflare:database_auto_cleanup"
	// TaskTypeDatabaseAutoCleanup is the admin task type for observability cleanup.
	TaskTypeDatabaseAutoCleanup = "of_database_auto_cleanup"

	// WAFIPGroupSyncTask syncs due automatic/subscription WAF IP groups.
	WAFIPGroupSyncTask = "openflare:waf_ip_group_sync"
	// TaskTypeWAFIPGroupSync is the admin task type for WAF IP group sync.
	TaskTypeWAFIPGroupSync = "of_waf_ip_group_sync"

	// UptimeKumaSyncTask synchronizes proxy routes to Uptime Kuma monitors.
	UptimeKumaSyncTask = "openflare:uptime_kuma_sync"
	// TaskTypeUptimeKumaSync is the admin task type for Uptime Kuma sync.
	TaskTypeUptimeKumaSync = "of_uptime_kuma_sync"
)

var (
	lastUptimeKumaSyncTime time.Time
	uptimeKumaSyncMutex    sync.Mutex
)

// SSLRenewMeta describes the SSL renewal task.
var SSLRenewMeta = task.TaskMeta{
	Type:         TaskTypeSSLRenew,
	AsynqTask:    SSLRenewTask,
	Name:         "OpenFlare SSL 自动续期",
	Description:  "扫描即将到期的 ACME 证书并触发自动续期",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// DatabaseAutoCleanupMeta describes the observability auto-cleanup task.
var DatabaseAutoCleanupMeta = task.TaskMeta{
	Type:         TaskTypeDatabaseAutoCleanup,
	AsynqTask:    DatabaseAutoCleanupTask,
	Name:         "OpenFlare 可观测数据自动清理",
	Description:  "按保留天数清理访问日志、性能快照与请求聚合数据",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// WAFIPGroupSyncMeta describes the WAF IP group sync task.
var WAFIPGroupSyncMeta = task.TaskMeta{
	Type:         TaskTypeWAFIPGroupSync,
	AsynqTask:    WAFIPGroupSyncTask,
	Name:         "OpenFlare WAF IP 组同步",
	Description:  "同步到期的自动规则与订阅类型 WAF IP 组",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// UptimeKumaSyncMeta describes the Uptime Kuma sync task.
var UptimeKumaSyncMeta = task.TaskMeta{
	Type:         TaskTypeUptimeKumaSync,
	AsynqTask:    UptimeKumaSyncTask,
	Name:         "OpenFlare Uptime Kuma 同步",
	Description:  "将启用的代理规则同步到 Uptime Kuma 监控",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
}

// SSLRenewHandler renews due TLS certificates.
type SSLRenewHandler struct{}

// Execute runs SSL certificate renewal for all due certificates.
func (h *SSLRenewHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	task.AppendLog(ctx, "开始扫描待续期证书")
	if err := tasks.RunSSLRenewJob(ctx); err != nil {
		task.AppendLog(ctx, "SSL 自动续期失败: %v", err)
		return nil, err
	}
	msg := "SSL 自动续期任务完成"
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}

// DatabaseAutoCleanupHandler prunes observability data when auto-cleanup is enabled.
type DatabaseAutoCleanupHandler struct{}

// Execute runs retention-based cleanup for all observability targets.
func (h *DatabaseAutoCleanupHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	if !model.DatabaseAutoCleanupEnabled {
		msg := "自动清理未启用，跳过执行"
		task.AppendLog(ctx, "%s", msg)
		return &task.TaskResult{Message: msg}, nil
	}

	task.AppendLog(ctx, "开始执行可观测数据自动清理，保留天数=%d", model.DatabaseAutoCleanupRetentionDays)
	summary, err := tasks.RunDatabaseAutoCleanupOnce(time.Now())
	if err != nil {
		task.AppendLog(ctx, "可观测数据自动清理失败: %v", err)
		return nil, err
	}
	if summary == nil {
		msg := "自动清理未启用，跳过执行"
		task.AppendLog(ctx, "%s", msg)
		return &task.TaskResult{Message: msg}, nil
	}

	var totalDeleted int64
	for _, item := range summary.Results {
		totalDeleted += item.DeletedCount
		task.AppendLog(ctx, "清理 %s：删除 %d 条", item.TargetLabel, item.DeletedCount)
	}

	msg := fmt.Sprintf(
		"可观测数据自动清理完成，保留 %d 天，共删除 %d 条",
		summary.RetentionDays,
		totalDeleted,
	)
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}

// WAFIPGroupSyncHandler syncs due WAF IP groups to agents.
type WAFIPGroupSyncHandler struct{}

// Execute syncs all due automatic/subscription WAF IP groups.
func (h *WAFIPGroupSyncHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	task.AppendLog(ctx, "开始同步到期的 WAF IP 组")
	if err := waf.SyncDueWAFIPGroups(ctx); err != nil {
		task.AppendLog(ctx, "WAF IP 组同步失败: %v", err)
		return nil, err
	}
	msg := "WAF IP 组同步完成"
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}

// UptimeKumaSyncHandler synchronizes proxy routes to Uptime Kuma.
type UptimeKumaSyncHandler struct{}

// Execute runs Uptime Kuma sync when integration is enabled and the interval has elapsed.
func (h *UptimeKumaSyncHandler) Execute(ctx context.Context, _ []byte) (*task.TaskResult, error) {
	if !model.UptimeKumaEnabled {
		msg := "Uptime Kuma 集成未启用，跳过执行"
		task.AppendLog(ctx, "%s", msg)
		return &task.TaskResult{Message: msg}, nil
	}

	interval := model.UptimeKumaSyncInterval
	if interval <= 0 {
		interval = 5
	}
	if time.Since(lastUptimeKumaSyncTime) < time.Duration(interval)*time.Minute {
		msg := fmt.Sprintf("距上次同步不足 %d 分钟，跳过执行", interval)
		task.AppendLog(ctx, "%s", msg)
		return &task.TaskResult{Message: msg}, nil
	}

	if !uptimeKumaSyncMutex.TryLock() {
		msg := "Uptime Kuma 同步任务正在执行，跳过本次调度"
		task.AppendLog(ctx, "%s", msg)
		return &task.TaskResult{Message: msg}, nil
	}
	defer uptimeKumaSyncMutex.Unlock()

	task.AppendLog(ctx, "开始同步代理规则到 Uptime Kuma")
	if err := uptimekuma.SyncToUptimeKuma(ctx); err != nil {
		task.AppendLog(ctx, "Uptime Kuma 同步失败: %v", err)
		return nil, err
	}

	lastUptimeKumaSyncTime = time.Now()
	msg := "Uptime Kuma 同步完成"
	task.AppendLog(ctx, "%s", msg)
	return &task.TaskResult{Message: msg}, nil
}