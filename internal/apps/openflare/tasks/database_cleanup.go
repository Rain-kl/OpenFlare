// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
)

const (
	// DatabaseCleanupTargetAccessLogs is the API cleanup target for access logs.
	DatabaseCleanupTargetAccessLogs = "node_access_logs"
	// DatabaseCleanupTargetMetricSnapshots is the API cleanup target for metric snapshots.
	DatabaseCleanupTargetMetricSnapshots = "node_metric_snapshots"
	// DatabaseCleanupTargetRequestReports is the API cleanup target for request reports.
	DatabaseCleanupTargetRequestReports = "node_request_reports"
	// DatabaseCleanupTargetObsOpenresty is the API cleanup target for OpenResty observations.
	DatabaseCleanupTargetObsOpenresty = "node_obs_openresty"
	// DatabaseCleanupTargetObsFrps is the API cleanup target for FRPS observations.
	DatabaseCleanupTargetObsFrps = "node_obs_frps"
	// DatabaseCleanupTargetObsFrpc is the API cleanup target for FRPC observations.
	DatabaseCleanupTargetObsFrpc = "node_obs_frpc"
)

var databaseCleanupTargets = map[string]string{
	DatabaseCleanupTargetAccessLogs:      "访问日志",
	DatabaseCleanupTargetMetricSnapshots: "性能快照",
	DatabaseCleanupTargetRequestReports:  "请求聚合",
	DatabaseCleanupTargetObsOpenresty:      "OpenResty 观测",
	DatabaseCleanupTargetObsFrps:           "FRPS 观测",
	DatabaseCleanupTargetObsFrpc:           "FRPC 观测",
}

// DatabaseCleanupInput describes a manual observability cleanup request.
type DatabaseCleanupInput struct {
	Target        string `json:"target"`
	RetentionDays *int   `json:"retention_days"`
}

// DatabaseCleanupResult summarizes a manual observability cleanup run.
type DatabaseCleanupResult struct {
	Target        string     `json:"target"`
	TargetLabel   string     `json:"target_label"`
	DeletedCount  int64      `json:"deleted_count"`
	CleanupMode   string     `json:"cleanup_mode,omitempty"`
	DeleteAll     bool       `json:"delete_all"`
	RetentionDays *int       `json:"retention_days,omitempty"`
	Cutoff        *time.Time `json:"cutoff,omitempty"`
}

// DatabaseAutoCleanupSummary summarizes a scheduled auto-cleanup run.
type DatabaseAutoCleanupSummary struct {
	RetentionDays int                     `json:"retention_days"`
	ExecutedAt    time.Time               `json:"executed_at"`
	Results       []DatabaseCleanupResult `json:"results"`
}

// CleanupDatabaseObservability deletes observability rows for the given target.
func CleanupDatabaseObservability(ctx context.Context, input DatabaseCleanupInput) (*DatabaseCleanupResult, error) {
	target := strings.TrimSpace(input.Target)
	targetLabel, ok := databaseCleanupTargets[target]
	if !ok {
		return nil, errors.New("unsupported cleanup target")
	}
	if input.RetentionDays != nil && *input.RetentionDays <= 0 {
		return nil, errors.New("retention_days 必须为大于 0 的整数")
	}

	result := &DatabaseCleanupResult{
		Target:      target,
		TargetLabel: targetLabel,
		DeleteAll:   input.RetentionDays == nil,
	}

	if input.RetentionDays == nil {
		deleted, mode, err := deleteAllObservabilityRows(ctx, target)
		if err != nil {
			return nil, err
		}
		result.DeletedCount = deleted
		result.CleanupMode = mode
		return result, nil
	}

	retentionDays := *input.RetentionDays
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	deleted, mode, err := deleteObservabilityRowsBefore(ctx, target, cutoff)
	if err != nil {
		return nil, err
	}
	result.DeletedCount = deleted
	result.CleanupMode = mode
	result.RetentionDays = &retentionDays
	result.Cutoff = &cutoff
	return result, nil
}

// RunDatabaseAutoCleanupOnce runs retention-based cleanup for all observability targets.
func RunDatabaseAutoCleanupOnce(ctx context.Context, now time.Time) (*DatabaseAutoCleanupSummary, error) {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyDatabaseAutoCleanupEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to read database_auto_cleanup_enabled: %w", err)
	}
	if !enabled {
		return nil, nil
	}

	retentionDays, err := repository.GetIntByKey(ctx, model.ConfigKeyDatabaseAutoCleanupRetentionDays)
	if err != nil || retentionDays <= 0 {
		// Use default value 30 if config read fails or value is invalid
		retentionDays = 30
	}

	results := make([]DatabaseCleanupResult, 0, len(databaseCleanupTargets))
	for _, target := range []string{
		DatabaseCleanupTargetAccessLogs,
		DatabaseCleanupTargetMetricSnapshots,
		DatabaseCleanupTargetRequestReports,
		DatabaseCleanupTargetObsOpenresty,
		DatabaseCleanupTargetObsFrps,
		DatabaseCleanupTargetObsFrpc,
	} {
		result, err := CleanupDatabaseObservability(ctx, DatabaseCleanupInput{
			Target:        target,
			RetentionDays: &retentionDays,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}

	return &DatabaseAutoCleanupSummary{
		RetentionDays: retentionDays,
		ExecutedAt:    now.UTC(),
		Results:       results,
	}, nil
}

func deleteAllObservabilityRows(ctx context.Context, target string) (int64, string, error) {
	var (
		deleted int64
		err     error
	)
	switch target {
	case DatabaseCleanupTargetAccessLogs:
		deleted, err = model.DeleteAllOpenFlareAccessLogs(ctx)
	case DatabaseCleanupTargetMetricSnapshots:
		deleted, err = model.DeleteAllOpenFlareMetricSnapshots(ctx)
	case DatabaseCleanupTargetRequestReports:
		deleted, err = model.DeleteAllOpenFlareRequestReports(ctx)
	case DatabaseCleanupTargetObsOpenresty:
		deleted, err = model.DeleteAllOpenFlareNodeObservationOpenresty(ctx)
	case DatabaseCleanupTargetObsFrps:
		deleted, err = model.DeleteAllOpenFlareNodeObservationFrps(ctx)
	case DatabaseCleanupTargetObsFrpc:
		deleted, err = model.DeleteAllOpenFlareNodeObservationFrpc(ctx)
	default:
		return 0, "", errors.New("unsupported cleanup target")
	}
	if err != nil {
		return 0, "", err
	}
	return deleted, analyticsrepo.CleanupModeTruncate, nil
}

func deleteObservabilityRowsBefore(ctx context.Context, target string, cutoff time.Time) (int64, string, error) {
	var (
		deleted int64
		err     error
	)
	switch target {
	case DatabaseCleanupTargetAccessLogs:
		deleted, err = model.DeleteOpenFlareAccessLogsBefore(ctx, cutoff)
	case DatabaseCleanupTargetMetricSnapshots:
		deleted, err = model.DeleteOpenFlareMetricSnapshotsBefore(ctx, cutoff)
	case DatabaseCleanupTargetRequestReports:
		deleted, err = model.DeleteOpenFlareRequestReportsBefore(ctx, cutoff)
	case DatabaseCleanupTargetObsOpenresty:
		deleted, err = model.DeleteOpenFlareNodeObservationOpenrestyBefore(ctx, cutoff)
	case DatabaseCleanupTargetObsFrps:
		deleted, err = model.DeleteOpenFlareNodeObservationFrpsBefore(ctx, cutoff)
	case DatabaseCleanupTargetObsFrpc:
		deleted, err = model.DeleteOpenFlareNodeObservationFrpcBefore(ctx, cutoff)
	default:
		return 0, "", errors.New("unsupported cleanup target")
	}
	if err != nil {
		return 0, "", err
	}
	return deleted, analyticsrepo.CleanupModeTTLMaterialize, nil
}
