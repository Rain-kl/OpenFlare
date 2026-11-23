// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"strings"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

const nodeObservabilityFilterClauseCapacity = 3

func buildCHNodeObservabilityFilterClause(filter analyticsmodel.NodeObservabilityFilter, sinceColumn string) (string, []any) {
	parts := make([]string, 0, nodeObservabilityFilterClauseCapacity)
	args := make([]any, 0, nodeObservabilityFilterClauseCapacity)
	if trimmed := strings.TrimSpace(filter.NodeID); trimmed != "" {
		parts = append(parts, "node_id = ?")
		args = append(args, trimmed)
	}
	if !filter.Since.IsZero() {
		parts = append(parts, sinceColumn+" >= ?")
		args = append(args, filter.Since.UTC())
	}
	if len(parts) == 0 {
		return "1", nil
	}
	return strings.Join(parts, " AND "), args
}

func chNodeObservabilityCapturedAtOrderClause() string {
	return "captured_at DESC, id DESC"
}

func chNodeMetricSnapshotTableName() string {
	return "of_node_metric_snapshots"
}

func chNodeEdgeHealthTableName() string {
	return "of_node_edge_health"
}

func chAccessLogHourlyTableName() string {
	return "of_access_log_hourly"
}

func chNodeMetricCapacityHourlyTableName() string {
	return "of_node_metric_capacity_hourly"
}

// clickHouseLimit1ByNodeIDClause selects the first row per node_id after ORDER BY.
const clickHouseLimit1ByNodeIDClause = " LIMIT 1 BY node_id"
