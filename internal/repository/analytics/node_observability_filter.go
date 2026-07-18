// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"strings"
	"time"
)

const nodeObservabilityFilterClauseCapacity = 3

// NodeObservabilityFilter scopes ClickHouse node observability queries.
type NodeObservabilityFilter struct {
	NodeID string
	Since  time.Time
	Limit  int
}

func buildNodeObservabilityFilterClause(filter NodeObservabilityFilter, sinceColumn string) (string, []any) {
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

func nodeObservabilityCapturedAtOrderClause() string {
	return "captured_at DESC, id DESC"
}

func nodeMetricSnapshotTableName() string {
	return "of_node_metric_snapshots"
}

func nodeEdgeHealthTableName() string {
	return "of_node_edge_health"
}

func accessLogHourlyTableName() string {
	return "of_access_log_hourly"
}

func nodeObsFrpsTableName() string {
	return "of_node_obs_frps"
}

func nodeObsFrpcTableName() string {
	return "of_node_obs_frpc"
}

func nodeMetricCapacityHourlyTableName() string {
	return "of_node_metric_capacity_hourly"
}

// clickHouseLimit1ByNodeIDClause selects the first row per node_id after ORDER BY.
const clickHouseLimit1ByNodeIDClause = " LIMIT 1 BY node_id"
