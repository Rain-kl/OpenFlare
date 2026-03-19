package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"openflare/common"
	"openflare/model"
	"strings"
	"time"
)

const (
	DatabaseCleanupTargetAccessLogs      = "node_access_logs"
	DatabaseCleanupTargetMetricSnapshots = "node_metric_snapshots"
	DatabaseCleanupTargetRequestReports  = "node_request_reports"
)

var databaseCleanupTargets = map[string]string{
	DatabaseCleanupTargetAccessLogs:      "访问日志",
	DatabaseCleanupTargetMetricSnapshots: "性能快照",
	DatabaseCleanupTargetRequestReports:  "请求聚合",
}

type DatabaseCleanupInput struct {
	Target        string `json:"target"`
	RetentionDays *int   `json:"retention_days"`
}

type DatabaseCleanupResult struct {
	Target        string     `json:"target"`
	TargetLabel   string     `json:"target_label"`
	DeletedCount  int64      `json:"deleted_count"`
	DeleteAll     bool       `json:"delete_all"`
	RetentionDays *int       `json:"retention_days,omitempty"`
	Cutoff        *time.Time `json:"cutoff,omitempty"`
}

type DatabaseAutoCleanupSummary struct {
	RetentionDays int                     `json:"retention_days"`
	ExecutedAt    time.Time               `json:"executed_at"`
	Results       []DatabaseCleanupResult `json:"results"`
}

func CleanupDatabaseObservability(input DatabaseCleanupInput) (*DatabaseCleanupResult, error) {
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
		deleted, err := deleteAllObservabilityRows(target)
		if err != nil {
			return nil, err
		}
		result.DeletedCount = deleted
		return result, nil
	}

	retentionDays := *input.RetentionDays
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	deleted, err := deleteObservabilityRowsBefore(target, cutoff)
	if err != nil {
		return nil, err
	}
	result.DeletedCount = deleted
	result.RetentionDays = &retentionDays
	result.Cutoff = &cutoff
	return result, nil
}

func RunDatabaseAutoCleanupOnce(now time.Time) (*DatabaseAutoCleanupSummary, error) {
	if !common.DatabaseAutoCleanupEnabled {
		return nil, nil
	}
	if common.DatabaseAutoCleanupRetentionDays < 1 {
		return nil, fmt.Errorf("database auto cleanup retention_days must be at least 1")
	}

	retentionDays := common.DatabaseAutoCleanupRetentionDays
	results := make([]DatabaseCleanupResult, 0, len(databaseCleanupTargets))
	for _, target := range []string{
		DatabaseCleanupTargetAccessLogs,
		DatabaseCleanupTargetMetricSnapshots,
		DatabaseCleanupTargetRequestReports,
	} {
		result, err := CleanupDatabaseObservability(DatabaseCleanupInput{
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

func StartDatabaseAutoCleanupScheduler(ctx context.Context) {
	go func() {
		for {
			wait := time.Until(nextDatabaseAutoCleanupTime(time.Now()))
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			summary, err := RunDatabaseAutoCleanupOnce(time.Now())
			if err != nil {
				slog.Error("database auto cleanup failed", "error", err)
				continue
			}
			if summary == nil {
				continue
			}
			totalDeleted := int64(0)
			for _, item := range summary.Results {
				totalDeleted += item.DeletedCount
			}
			slog.Info(
				"database auto cleanup completed",
				"retention_days",
				summary.RetentionDays,
				"deleted_count",
				totalDeleted,
			)
		}
	}()
}

func nextDatabaseAutoCleanupTime(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func deleteAllObservabilityRows(target string) (int64, error) {
	switch target {
	case DatabaseCleanupTargetAccessLogs:
		return model.DeleteAllNodeAccessLogs(nil)
	case DatabaseCleanupTargetMetricSnapshots:
		return model.DeleteAllNodeMetricSnapshots(nil)
	case DatabaseCleanupTargetRequestReports:
		return model.DeleteAllNodeRequestReports(nil)
	default:
		return 0, errors.New("unsupported cleanup target")
	}
}

func deleteObservabilityRowsBefore(target string, cutoff time.Time) (int64, error) {
	switch target {
	case DatabaseCleanupTargetAccessLogs:
		return model.DeleteNodeAccessLogsBefore(cutoff)
	case DatabaseCleanupTargetMetricSnapshots:
		return model.DeleteNodeMetricSnapshotsBefore(nil, cutoff)
	case DatabaseCleanupTargetRequestReports:
		return model.DeleteNodeRequestReportsBefore(nil, cutoff)
	default:
		return 0, errors.New("unsupported cleanup target")
	}
}
