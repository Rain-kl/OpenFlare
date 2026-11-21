// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

// siteIDExpression assigns each row once. Exact bindings take priority. A wildcard
// matches the suffix after the first label, as it does in Pingora's route lookup.
func siteIDExpression(hostname, suffix, firstDot string, bindings []model.OpenFlareAccessLogSiteBinding) (string, []any) {
	var exact, wildcard strings.Builder
	var exactArgs, wildcardArgs []any
	for _, binding := range bindings {
		name := strings.ToLower(strings.TrimSpace(binding.Hostname))
		if pattern, ok := strings.CutPrefix(name, "*."); ok {
			appendSiteCase(&wildcard, binding.SiteID)
			wildcardArgs = append(wildcardArgs, pattern)
		} else {
			appendSiteCase(&exact, binding.SiteID)
			exactArgs = append(exactArgs, name)
		}
	}
	fallback := "0"
	if wildcard.Len() != 0 {
		fallback = "CASE WHEN " + firstDot + " > 1 THEN CASE " + suffix + wildcard.String() + " ELSE 0 END ELSE 0 END"
	}
	if exact.Len() == 0 {
		return fallback, wildcardArgs
	}
	return "CASE " + hostname + exact.String() + " ELSE " + fallback + " END", append(exactArgs, wildcardArgs...)
}

func appendSiteCase(branches *strings.Builder, siteID uint) {
	branches.WriteString(" WHEN ? THEN ")
	branches.WriteString(strconv.FormatUint(uint64(siteID), 10))
}

func (s *gormLogStore) SiteTraffic(ctx context.Context, query model.OpenFlareAccessLogQuery, bindings []model.OpenFlareAccessLogSiteBinding) ([]model.OpenFlareAccessLogSiteTraffic, error) {
	const hostname = "lower(rtrim(trim(host), '.'))"
	firstDot := "instr(" + hostname + ", '.')"
	if isPostgresDialect(s.db) {
		firstDot = "strpos(" + hostname + ", '.')"
	}
	expression, args := siteIDExpression(hostname, "substr("+hostname+", "+firstDot+" + 1)", firstDot, bindings)
	mapped := applyNodeAccessLogFilter(s.db.WithContext(ctx).Model(&analyticsmodel.NodeAccessLog{}), toNodeAccessLogFilter(query)).
		Select(expression+" AS site_id, remote_addr, bytes_sent, status_code", args...)
	var rows []model.OpenFlareAccessLogSiteTraffic
	err := s.db.WithContext(ctx).Table("(?) AS mapped_logs", mapped).
		Select(`site_id, COUNT(*) AS requests,
			` + distinctNonEmptyCountSQL(s.db, "remote_addr") + ` AS unique_ips,
			COALESCE(SUM(bytes_sent), 0) AS bytes_sent,
			COUNT(*) FILTER (WHERE status_code BETWEEN 400 AND 499) AS client_errors,
			COUNT(*) FILTER (WHERE status_code BETWEEN 500 AND 599) AS server_errors`).
		Group("site_id").Scan(&rows).Error
	return rows, err
}

func (s *clickhouseLogStore) SiteTraffic(ctx context.Context, query model.OpenFlareAccessLogQuery, bindings []model.OpenFlareAccessLogSiteBinding) ([]model.OpenFlareAccessLogSiteTraffic, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}
	const hostname = "replaceRegexpOne(lowerUTF8(trim(host)), '[.]+$', '')"
	const firstDot = "position(" + hostname + ", '.')"
	expression, bindingArgs := siteIDExpression(hostname, "substring("+hostname+", "+firstDot+" + 1)", firstDot, bindings)
	clause, filterArgs := buildCHNodeAccessLogFilterClause(toNodeAccessLogFilter(query))
	sql := fmt.Sprintf(`SELECT site_id, count(), uniqExactIf(remote_addr, remote_addr != ''), sum(bytes_sent),
			countIf(status_code BETWEEN 400 AND 499), countIf(status_code BETWEEN 500 AND 599)
		FROM (
			SELECT toUInt64(%s) AS site_id, remote_addr, bytes_sent, status_code
			FROM %s WHERE %s
		) GROUP BY site_id`, expression, chNodeAccessLogTableName(), clause)
	result, err := conn.Query(ctx, sql, append(bindingArgs, filterArgs...)...)
	if err != nil {
		return nil, err
	}
	// Query and iteration errors are returned separately. Closing releases the reader.
	defer func() { _ = result.Close() }()
	var rows []model.OpenFlareAccessLogSiteTraffic
	for result.Next() {
		var siteID, requests, ips, bytes, clientErrors, serverErrors uint64
		if err := result.Scan(&siteID, &requests, &ips, &bytes, &clientErrors, &serverErrors); err != nil {
			return nil, err
		}
		rows = append(rows, model.OpenFlareAccessLogSiteTraffic{
			SiteID: uint(siteID), Requests: safeInt64Count(requests), UniqueIPs: safeInt64Count(ips),
			BytesSent: safeInt64Count(bytes), ClientErrors: safeInt64Count(clientErrors), ServerErrors: safeInt64Count(serverErrors),
		})
	}
	return rows, result.Err()
}
