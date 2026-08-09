// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"fmt"
	"strings"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

const userAccessLogFilterClauseCapacity = 4

// AccessLogFilter scopes ClickHouse user access log queries.
type AccessLogFilter = analyticsmodel.AccessLogFilter

func buildUserAccessLogFilterClause(filter AccessLogFilter) (string, []any, bool) {
	if filter.UserIDs != nil && len(filter.UserIDs) == 0 {
		return "", nil, false
	}

	parts := make([]string, 0, userAccessLogFilterClauseCapacity)
	args := make([]any, 0, userAccessLogFilterClauseCapacity)
	if filter.UserIDs != nil {
		placeholders := make([]string, len(filter.UserIDs))
		for index, userID := range filter.UserIDs {
			placeholders[index] = "?"
			args = append(args, userID)
		}
		parts = append(parts, fmt.Sprintf("user_id IN (%s)", strings.Join(placeholders, ", ")))
	}
	if trimmed := strings.TrimSpace(filter.Path); trimmed != "" {
		parts = append(parts, "path LIKE ?")
		args = append(args, "%"+trimmed+"%")
	}
	if filter.StartTime != nil {
		parts = append(parts, "created_at >= ?")
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		parts = append(parts, "created_at <= ?")
		args = append(args, *filter.EndTime)
	}
	if len(parts) == 0 {
		return "1", args, true
	}
	return strings.Join(parts, " AND "), args, true
}
