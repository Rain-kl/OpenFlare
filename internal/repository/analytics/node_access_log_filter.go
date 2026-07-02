// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"fmt"
	"strings"
	"time"
)

const (
	nodeAccessLogFilterClauseCapacity = 6

	nodeAccessLogSortDesc = "DESC"
	nodeAccessLogSortAsc  = "ASC"
	nodeAccessLogSortAscInput = "asc"

	nodeAccessLogColumnRemoteAddr = "remote_addr"
)

// NodeAccessLogFilter scopes ClickHouse node access log queries.
type NodeAccessLogFilter struct {
	NodeID     string
	RemoteAddr string
	Host       string
	Path       string
	Since      time.Time
	Until      time.Time
	Page       int
	PageSize   int
	SortBy     string
	SortOrder  string
}

func buildNodeAccessLogFilterClause(filter NodeAccessLogFilter) (string, []any) {
	parts := make([]string, 0, nodeAccessLogFilterClauseCapacity)
	args := make([]any, 0, nodeAccessLogFilterClauseCapacity)
	if trimmed := strings.TrimSpace(filter.NodeID); trimmed != "" {
		parts = append(parts, "node_id = ?")
		args = append(args, trimmed)
	}
	if trimmed := normalizeNodeAccessLogRemoteAddr(filter.RemoteAddr); trimmed != "" {
		parts = append(parts, "remote_addr LIKE ?")
		args = append(args, trimmed+"%")
	}
	if trimmed := strings.TrimSpace(filter.Host); trimmed != "" {
		parts = append(parts, "host LIKE ?")
		args = append(args, trimmed+"%")
	}
	if trimmed := strings.TrimSpace(filter.Path); trimmed != "" {
		parts = append(parts, "path LIKE ?")
		args = append(args, trimmed+"%")
	}
	if !filter.Since.IsZero() {
		parts = append(parts, "logged_at >= ?")
		args = append(args, filter.Since.UTC())
	}
	if !filter.Until.IsZero() {
		parts = append(parts, "logged_at < ?")
		args = append(args, filter.Until.UTC())
	}
	if len(parts) == 0 {
		return "1", nil
	}
	return strings.Join(parts, " AND "), args
}

func combineNodeAccessLogSQLClauses(left string, right string) string {
	if strings.TrimSpace(left) == "" || left == "TRUE" || left == "1" {
		return right
	}
	return left + " AND " + right
}

func nodeAccessLogOrderClause(sortBy string, sortOrder string) string {
	direction := nodeAccessLogSortDesc
	if normalizeNodeAccessLogSortOrder(sortOrder) == nodeAccessLogSortAscInput {
		direction = nodeAccessLogSortAsc
	}
	column := "logged_at"
	switch strings.TrimSpace(sortBy) {
	case "status_code":
		column = "status_code"
	case nodeAccessLogColumnRemoteAddr:
		column = nodeAccessLogColumnRemoteAddr
	case "host":
		column = "host"
	case "path":
		column = "path"
	}
	if column == "logged_at" {
		return column + " " + direction + ", id " + direction
	}
	return column + " " + direction + ", logged_at " + direction + ", id " + direction
}

func normalizeNodeAccessLogRemoteAddr(value string) string {
	return strings.TrimSpace(value)
}

func normalizeNodeAccessLogSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return "desc"
}

func nodeAccessLogBucketEpochExpr(bucketSeconds int64) string {
	return fmt.Sprintf("toInt64(intDiv(toUnixTimestamp(logged_at), %d) * %d)", bucketSeconds, bucketSeconds)
}

func nodeAccessLogEpochExpr() string {
	return "toInt64(toUnixTimestamp(logged_at))"
}

func nodeAccessLogHostIsIPLiteralExpr() string {
	return `(
		toIPv4OrNull(trim(if(position(trim(host), ':') > 0 AND NOT startsWith(trim(host), '['), splitByChar(':', trim(host))[1], replaceRegexpAll(trim(host), '\\[|\\]', '')))) IS NOT NULL
		OR toIPv6OrNull(trim(if(position(trim(host), ':') > 0 AND NOT startsWith(trim(host), '['), splitByChar(':', trim(host))[1], replaceRegexpAll(trim(host), '\\[|\\]', '')))) IS NOT NULL
	)`
}

func nodeAccessLogBucketOrderClause(sortBy string, sortOrder string) string {
	direction := nodeAccessLogSortDesc
	if normalizeNodeAccessLogSortOrder(sortOrder) == nodeAccessLogSortAscInput {
		direction = nodeAccessLogSortAsc
	}
	switch strings.TrimSpace(sortBy) {
	case "request_count":
		return "request_count " + direction + ", bucket_epoch DESC"
	default:
		return "bucket_epoch " + direction
	}
}

func nodeAccessLogIPSummaryOrderClause(sortBy string, sortOrder string) string {
	direction := nodeAccessLogSortDesc
	if normalizeNodeAccessLogSortOrder(sortOrder) == nodeAccessLogSortAscInput {
		direction = nodeAccessLogSortAsc
	}
	column := "total_requests"
	switch strings.TrimSpace(sortBy) {
	case "recent_requests":
		column = "recent_requests"
	case "last_seen_at":
		column = "last_seen_epoch"
	case nodeAccessLogColumnRemoteAddr:
		column = nodeAccessLogColumnRemoteAddr
	}
	return column + " " + direction + ", last_seen_epoch DESC, remote_addr ASC"
}

func nodeAccessLogTableName() string {
	return "of_node_access_logs"
}
