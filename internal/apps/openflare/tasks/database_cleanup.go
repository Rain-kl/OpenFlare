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
	// DatabaseCleanupTargetEdgeHealth is the API cleanup target for OpenResty edge health (connections).
	DatabaseCleanupTargetEdgeHealth = "node_edge_health"
	// DatabaseCleanupTargetObsFrps is the API cleanup target for FRPS observations.
	DatabaseCleanupTargetObsFrps = "node_obs_frps"
	// DatabaseCleanupTargetObsFrpc is the API cleanup target for FRPC observations.
	DatabaseCleanupTargetObsFrpc = "node_obs_frpc"
)

var databaseCleanupTargets = map[string]string{
	DatabaseCleanupTargetAccessLogs:      "访问日志",
	DatabaseCleanupTargetMetricSnapshots: "性能快照",
	DatabaseCleanupTargetEdgeHealth:      "OpenResty 健康(连接)",
	DatabaseCleanupTargetObsFrps:         "FRPS 观测",
	DatabaseCleanupTargetObsFrpc:         "FRPC 观测",
}

// databaseCleanupTableTTLDays maps API targets to ClickHouse DDL TTL days.
var databaseCleanupTableTTLDays = map[string]int{
	DatabaseCleanupTargetAccessLogs:      analyticsrepo.TableTTLDaysNodeAccessLogs,
	DatabaseCleanupTargetMetricSnapshots: analyticsrepo.TableTTLDaysNodeMetricSnapshots,
	DatabaseCleanupTargetEdgeHealth:      analyticsrepo.TableTTLDaysNodeObs,
	DatabaseCleanupTargetObsFrps:         analyticsrepo.TableTTLDaysNodeObs,
	DatabaseCleanupTargetObsFrpc:         analyticsrepo.TableTTLDaysNodeObs,
}

// DatabaseCleanupInput describes a manual observability cleanup request.
type DatabaseCleanupInput struct {
	Target        string `json:"target"`
	RetentionDays *int   `json:"retention_days"`
}

// DatabaseCleanupResult summarizes a manual observability cleanup run.
//
// Semantics:
//   - delete_all / cleanup_mode=truncate: DeletedCount is hard-deleted rows (TRUNCATE).
//   - retention path / cleanup_mode=ttl_materialize: DeletedCount is always 0;
//     EligibleCount estimates rows past the table DDL TTL (not an arbitrary younger cutoff).
type DatabaseCleanupResult struct {
	Target        string     `json:"target"`
	TargetLabel   string     `json:"target_label"`
	DeletedCount  int64      `json:"deleted_count"`
	EligibleCount int64      `json:"eligible_count,omitempty"`
	CleanupMode   string     `json:"cleanup_mode,omitempty"`
	TableTTLDays  int        `json:"table_ttl_days,omitempty"`
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

// TableTTLDaysForCleanupTarget returns the DDL TTL days for a cleanup target.
func TableTTLDaysForCleanupTarget(target string) (int, bool) {
	days, ok := databaseCleanupTableTTLDays[strings.TrimSpace(target)]
	return days, ok
}

// CleanupDatabaseObservability deletes observability rows for the given target.
//
// When RetentionDays is nil, rows are hard-deleted via TRUNCATE.
// When RetentionDays is set, ClickHouse only force-materializes the table TTL policy:
// retention_days shorter than the table TTL is rejected (do not fake success).
func CleanupDatabaseObservability(ctx context.Context, input DatabaseCleanupInput) (*DatabaseCleanupResult, error) {
	target := strings.TrimSpace(input.Target)
	targetLabel, ok := databaseCleanupTargets[target]
	if !ok {
		return nil, errors.New("unsupported cleanup target")
	}
	if input.RetentionDays != nil && *input.RetentionDays <= 0 {
		return nil, errors.New("retention_days 必须为大于 0 的整数")
	}

	tableTTLDays := databaseCleanupTableTTLDays[target]
	result := &DatabaseCleanupResult{
		Target:       target,
		TargetLabel:  targetLabel,
		DeleteAll:    input.RetentionDays == nil,
		TableTTLDays: tableTTLDays,
	}

	if input.RetentionDays == nil {
		deleted, mode, err := deleteAllObservabilityRows(ctx, target)
		if err != nil {
			return nil, err
		}
		result.DeletedCount = deleted
		result.EligibleCount = deleted
		result.CleanupMode = mode
		return result, nil
	}

	retentionDays := *input.RetentionDays
	if retentionDays < tableTTLDays {
		return nil, fmt.Errorf(
			"retention_days 不能小于表 TTL（%d 天）；ClickHouse 仅支持按表 TTL 物化过期，更短保留请使用清空全部或调整 DDL",
			tableTTLDays,
		)
	}

	// MATERIALIZE TTL only enforces DDL policy; cutoff reported is the table TTL boundary.
	tableCutoff := time.Now().UTC().Add(-time.Duration(tableTTLDays) * 24 * time.Hour)
	eligible, mode, err := materializeObservabilityTableTTL(ctx, target)
	if err != nil {
		return nil, err
	}
	result.DeletedCount = 0
	result.EligibleCount = eligible
	result.CleanupMode = mode
	result.RetentionDays = &retentionDays
	result.Cutoff = &tableCutoff
	return result, nil
}

// RunDatabaseAutoCleanupOnce runs retention-based cleanup for all observability targets.
//
// Configured retention shorter than a target's table TTL is clamped up to the table TTL
// so the scheduled job can force-materialize each table policy without failing.
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
		DatabaseCleanupTargetEdgeHealth,
		DatabaseCleanupTargetObsFrps,
		DatabaseCleanupTargetObsFrpc,
	} {
		effectiveDays := retentionDays
		if ttl, ok := databaseCleanupTableTTLDays[target]; ok && effectiveDays < ttl {
			effectiveDays = ttl
		}
		result, err := CleanupDatabaseObservability(ctx, DatabaseCleanupInput{
			Target:        target,
			RetentionDays: &effectiveDays,
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
		deleted, err = repository.DeleteAllOpenFlareAccessLogs(ctx)
	case DatabaseCleanupTargetMetricSnapshots:
		deleted, err = repository.DeleteAllOpenFlareMetricSnapshots(ctx)
	case DatabaseCleanupTargetEdgeHealth:
		deleted, err = repository.DeleteAllOpenFlareEdgeHealth(ctx)
	case DatabaseCleanupTargetObsFrps:
		deleted, err = repository.DeleteAllOpenFlareNodeObservationFrps(ctx)
	case DatabaseCleanupTargetObsFrpc:
		deleted, err = repository.DeleteAllOpenFlareNodeObservationFrpc(ctx)
	default:
		return 0, "", errors.New("unsupported cleanup target")
	}
	if err != nil {
		return 0, "", err
	}
	return deleted, analyticsrepo.CleanupModeTruncate, nil
}

// materializeObservabilityTableTTL triggers table-TTL materialize (or memory-store delete-before
// with the table TTL cutoff for tests) and returns the eligible/estimate row count.
func materializeObservabilityTableTTL(ctx context.Context, target string) (int64, string, error) {
	ttlDays, ok := databaseCleanupTableTTLDays[target]
	if !ok {
		return 0, "", errors.New("unsupported cleanup target")
	}
	cutoff := time.Now().UTC().Add(-time.Duration(ttlDays) * 24 * time.Hour)

	var (
		eligible int64
		err      error
	)
	switch target {
	case DatabaseCleanupTargetAccessLogs:
		eligible, err = repository.DeleteOpenFlareAccessLogsBefore(ctx, cutoff)
	case DatabaseCleanupTargetMetricSnapshots:
		eligible, err = repository.DeleteOpenFlareMetricSnapshotsBefore(ctx, cutoff)
	case DatabaseCleanupTargetEdgeHealth:
		eligible, err = repository.DeleteOpenFlareEdgeHealthBefore(ctx, cutoff)
	case DatabaseCleanupTargetObsFrps:
		eligible, err = repository.DeleteOpenFlareNodeObservationFrpsBefore(ctx, cutoff)
	case DatabaseCleanupTargetObsFrpc:
		eligible, err = repository.DeleteOpenFlareNodeObservationFrpcBefore(ctx, cutoff)
	default:
		return 0, "", errors.New("unsupported cleanup target")
	}
	if err != nil {
		return 0, "", err
	}
	return eligible, analyticsrepo.CleanupModeTTLMaterialize, nil
}
