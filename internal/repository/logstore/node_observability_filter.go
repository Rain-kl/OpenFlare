// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"maps"
	"slices"
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
	if len(filter.NodeIDs) > 0 {
		clause, nodeArgs := nodeIDsFilter(filter.NodeIDs)
		parts = append(parts, clause)
		args = append(args, nodeArgs...)
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

func nodeIDsFilter(nodeIDs []string) (string, []any) {
	args := make([]any, len(nodeIDs))
	for i, id := range nodeIDs {
		args[i] = id
	}
	return "node_id IN (" + strings.Repeat("?, ", len(nodeIDs)-1) + "?)", args
}

func pingTaskNodesFilter(tasks map[uint]analyticsmodel.PingTaskWindow) (string, []any) {
	parts := make([]string, 0, len(tasks))
	var args []any
	for _, taskID := range slices.Sorted(maps.Keys(tasks)) {
		window := tasks[taskID]
		if len(window.NodeIDs) == 0 {
			continue
		}
		clause, nodeArgs := nodeIDsFilter(window.NodeIDs)
		parts = append(parts, "(task_id = ? AND recorded_at >= ? AND "+clause+")")
		args = append(args, uint64(taskID), window.Since.UTC())
		args = append(args, nodeArgs...)
	}
	return strings.Join(parts, " OR "), args
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

func chNodePingRecordTableName() string {
	return "of_node_ping_records"
}

func chAccessLogHourlyTableName() string {
	return "of_access_log_hourly"
}

func chNodeMetricCapacityHourlyTableName() string {
	return "of_node_metric_capacity_hourly"
}

// clickHouseLimit1ByNodeIDClause selects the first row per node_id after ORDER BY.
const clickHouseLimit1ByNodeIDClause = " LIMIT 1 BY node_id"
