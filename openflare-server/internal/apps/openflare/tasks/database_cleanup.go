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
)

const (
	// DatabaseCleanupTargetAccessLogs is the API cleanup target for access logs.
	DatabaseCleanupTargetAccessLogs = "node_access_logs"
	// DatabaseCleanupTargetMetricSnapshots is the API cleanup target for metric snapshots.
	DatabaseCleanupTargetMetricSnapshots = "node_metric_snapshots"
	// DatabaseCleanupTargetRequestReports is the API cleanup target for request reports.
	DatabaseCleanupTargetRequestReports = "node_request_reports"
)

var databaseCleanupTargets = map[string]string{
	DatabaseCleanupTargetAccessLogs:      "访问日志",
	DatabaseCleanupTargetMetricSnapshots: "性能快照",
	DatabaseCleanupTargetRequestReports:  "请求聚合",
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
		deleted, err := deleteAllObservabilityRows(ctx, target)
		if err != nil {
			return nil, err
		}
		result.DeletedCount = deleted
		return result, nil
	}

	retentionDays := *input.RetentionDays
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	deleted, err := deleteObservabilityRowsBefore(ctx, target, cutoff)
	if err != nil {
		return nil, err
	}
	result.DeletedCount = deleted
	result.RetentionDays = &retentionDays
	result.Cutoff = &cutoff
	return result, nil
}

// RunDatabaseAutoCleanupOnce runs retention-based cleanup for all observability targets.
func RunDatabaseAutoCleanupOnce(now time.Time) (*DatabaseAutoCleanupSummary, error) {
	if !model.DatabaseAutoCleanupEnabled {
		return nil, nil
	}
	if model.DatabaseAutoCleanupRetentionDays < 1 {
		return nil, fmt.Errorf("database auto cleanup retention_days must be at least 1")
	}

	retentionDays := model.DatabaseAutoCleanupRetentionDays
	ctx := context.Background()
	results := make([]DatabaseCleanupResult, 0, len(databaseCleanupTargets))
	for _, target := range []string{
		DatabaseCleanupTargetAccessLogs,
		DatabaseCleanupTargetMetricSnapshots,
		DatabaseCleanupTargetRequestReports,
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

func deleteAllObservabilityRows(ctx context.Context, target string) (int64, error) {
	switch target {
	case DatabaseCleanupTargetAccessLogs:
		return model.DeleteAllOpenFlareAccessLogs(ctx)
	case DatabaseCleanupTargetMetricSnapshots:
		return model.DeleteAllOpenFlareMetricSnapshots(ctx)
	case DatabaseCleanupTargetRequestReports:
		return model.DeleteAllOpenFlareRequestReports(ctx)
	default:
		return 0, errors.New("unsupported cleanup target")
	}
}

func deleteObservabilityRowsBefore(ctx context.Context, target string, cutoff time.Time) (int64, error) {
	switch target {
	case DatabaseCleanupTargetAccessLogs:
		return model.DeleteOpenFlareAccessLogsBefore(ctx, cutoff)
	case DatabaseCleanupTargetMetricSnapshots:
		return model.DeleteOpenFlareMetricSnapshotsBefore(ctx, cutoff)
	case DatabaseCleanupTargetRequestReports:
		return model.DeleteOpenFlareRequestReportsBefore(ctx, cutoff)
	default:
		return 0, errors.New("unsupported cleanup target")
	}
}
