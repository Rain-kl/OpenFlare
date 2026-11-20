// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"fmt"
	"strings"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/pkg/util"
)

const (
	nodeAccessLogFilterClauseCapacity = 8

	nodeAccessLogSortDesc     = "DESC"
	nodeAccessLogSortAsc      = "ASC"
	nodeAccessLogSortAscInput = "asc"

	nodeAccessLogColumnRemoteAddr = "remote_addr"
	nodeAccessLogColumnStatusCode = "status_code"
	nodeAccessLogColumnHost       = "host"
	nodeAccessLogColumnPath       = "path"
	nodeAccessLogColumnUserAgent  = "user_agent"
	nodeAccessLogColumnLoggedAt   = "logged_at"
)

func buildCHNodeAccessLogFilterClause(filter analyticsmodel.NodeAccessLogFilter) (string, []any) {
	parts := make([]string, 0, nodeAccessLogFilterClauseCapacity)
	args := make([]any, 0, nodeAccessLogFilterClauseCapacity)
	if filter.ID != 0 {
		parts = append(parts, "id = ?")
		args = append(args, filter.ID)
	}
	if trimmed := strings.TrimSpace(filter.NodeID); trimmed != "" {
		parts = append(parts, "node_id = ?")
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(filter.RemoteAddr); trimmed != "" {
		if filter.ExactRemoteAddr {
			parts = append(parts, "remote_addr = ?")
			args = append(args, trimmed)
		} else {
			parts = append(parts, "remote_addr LIKE ?")
			args = append(args, util.EscapeLike(trimmed)+"%")
		}
	}
	parts, args = appendWAFLogFilter(parts, args, filter.WAFAction)
	hosts := normalizeNodeAccessLogHosts(filter.Hosts)
	if len(hosts) > 0 {
		hostParts := make([]string, 0, len(hosts))
		placeholders := make([]string, 0, len(hosts))
		var exactHosts []any
		for _, host := range hosts {
			if suffix, wildcard := strings.CutPrefix(host, "*."); wildcard && suffix != "" {
				// _% 要求非空子域名，并保留 Nginx 通配符可匹配多层子域名的语义。
				hostParts = append(hostParts, "(lowerUTF8(trim(host)) LIKE ? AND trim(host) NOT LIKE '%..%' AND trim(host) NOT LIKE '.%')")
				args = append(args, "_%."+util.EscapeLike(suffix))
				continue
			}
			placeholders = append(placeholders, "?")
			exactHosts = append(exactHosts, host)
		}
		if len(exactHosts) > 0 {
			hostParts = append(hostParts, "lowerUTF8(trim(host)) IN ("+strings.Join(placeholders, ", ")+")")
			args = append(args, exactHosts...)
		}
		hostClause := strings.Join(hostParts, " OR ")
		if len(hostParts) > 1 {
			hostClause = "(" + hostClause + ")"
		}
		parts = append(parts, hostClause)
	} else if trimmed := strings.TrimSpace(filter.Host); trimmed != "" {
		parts = append(parts, "host LIKE ?")
		args = append(args, util.EscapeLike(trimmed)+"%")
	}
	if trimmed := strings.TrimSpace(filter.Path); trimmed != "" {
		parts = append(parts, "path LIKE ?")
		args = append(args, util.EscapeLike(trimmed)+"%")
	}
	if filter.StatusCode > 0 {
		parts = append(parts, "status_code = ?")
		args = append(args, filter.StatusCode)
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

func combineCHNodeAccessLogSQLClauses(left, right string) string {
	if strings.TrimSpace(left) == "" || left == "TRUE" || left == "1" {
		return right
	}
	return left + " AND " + right
}

func chNodeAccessLogOrderClause(sortBy, sortOrder string) string {
	direction := nodeAccessLogSortDesc
	if normalizeCHNodeAccessLogSortOrder(sortOrder) == nodeAccessLogSortAscInput {
		direction = nodeAccessLogSortAsc
	}
	column := nodeAccessLogColumnLoggedAt
	switch strings.TrimSpace(sortBy) {
	case nodeAccessLogColumnStatusCode:
		column = nodeAccessLogColumnStatusCode
	case nodeAccessLogColumnRemoteAddr:
		column = nodeAccessLogColumnRemoteAddr
	case nodeAccessLogColumnHost:
		column = nodeAccessLogColumnHost
	case nodeAccessLogColumnPath:
		column = nodeAccessLogColumnPath
	}
	if column == nodeAccessLogColumnLoggedAt {
		return column + " " + direction + ", id " + direction
	}
	return column + " " + direction + ", " + nodeAccessLogColumnLoggedAt + " " + direction + ", id " + direction
}

func normalizeCHNodeAccessLogSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return "desc"
}

func chNodeAccessLogBucketEpochExpr(bucketSeconds int64) string {
	return fmt.Sprintf("toInt64(intDiv(toUnixTimestamp(logged_at), %d) * %d)", bucketSeconds, bucketSeconds)
}

func chNodeAccessLogEpochExpr() string {
	return "toInt64(toUnixTimestamp(logged_at))"
}

func chNodeAccessLogHostIsIPLiteralExpr() string {
	return `(
		toIPv4OrNull(trim(if(position(trim(host), ':') > 0 AND NOT startsWith(trim(host), '['), splitByChar(':', trim(host))[1], replaceRegexpAll(trim(host), '\\[|\\]', '')))) IS NOT NULL
		OR toIPv6OrNull(trim(if(position(trim(host), ':') > 0 AND NOT startsWith(trim(host), '['), splitByChar(':', trim(host))[1], replaceRegexpAll(trim(host), '\\[|\\]', '')))) IS NOT NULL
	)`
}

func chNodeAccessLogBucketOrderClause(sortBy, sortOrder string) string {
	direction := nodeAccessLogSortDesc
	if normalizeCHNodeAccessLogSortOrder(sortOrder) == nodeAccessLogSortAscInput {
		direction = nodeAccessLogSortAsc
	}
	switch strings.TrimSpace(sortBy) {
	case "request_count":
		return "request_count " + direction + ", bucket_epoch DESC"
	default:
		return "bucket_epoch " + direction
	}
}

func chNodeAccessLogIPSummaryOrderClause(sortBy, sortOrder string) string {
	direction := nodeAccessLogSortDesc
	if normalizeCHNodeAccessLogSortOrder(sortOrder) == nodeAccessLogSortAscInput {
		direction = nodeAccessLogSortAsc
	}
	column := "total_requests"
	switch strings.TrimSpace(sortBy) {
	case "request_length", "bytes_received":
		column = "request_length"
	case "bytes_sent":
		column = "bytes_sent"
	case "success_ratio":
		column = "success_ratio"
	case "last_seen_at":
		column = "last_seen_epoch"
	case "recent_requests":
		// Deprecated sort key; fall back to total_requests.
		column = "total_requests"
	case nodeAccessLogColumnRemoteAddr:
		column = nodeAccessLogColumnRemoteAddr
	}
	return column + " " + direction + ", last_seen_epoch DESC, remote_addr ASC"
}

func chNodeAccessLogTableName() string {
	return "of_node_access_logs"
}
