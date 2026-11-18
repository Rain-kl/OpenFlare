// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"sort"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

const hoursInDay = 24

func (s *clickhouseUserAccessLogStore) GetDailyTrend(ctx context.Context, days int) ([]analyticsmodel.DailyTrend, error) {
	if days < 1 {
		days = 7
	}
	conn, err := chConn()
	if err != nil {
		return nil, err
	}

	startTime := time.Now().AddDate(0, 0, -(days - 1)).Truncate(hoursInDay * time.Hour)
	tableName := analyticsmodel.UserAccessLog{}.TableName()
	query := fmt.Sprintf(`
		SELECT toDate(created_at) AS date, count() AS count
		FROM %s
		WHERE created_at >= ?
		GROUP BY date
		ORDER BY date ASC
	`, tableName)

	rows, err := conn.Query(ctx, query, startTime)
	if err != nil {
		return nil, fmt.Errorf("get daily trend: %w", err)
	}
	defer func() { _ = rows.Close() }()

	trendMap := make(map[string]uint64, days)
	for i := range days {
		dateStr := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		trendMap[dateStr] = 0
	}
	for rows.Next() {
		var (
			date  time.Time
			count uint64
		)
		if err := rows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("scan daily trend row: %w", err)
		}
		trendMap[date.Format("2006-01-02")] = count
	}

	result := make([]analyticsmodel.DailyTrend, 0, days)
	for i := days - 1; i >= 0; i-- {
		dateStr := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		result = append(result, analyticsmodel.DailyTrend{
			Date:  dateStr,
			Count: trendMap[dateStr],
		})
	}
	return result, nil
}

func (s *clickhouseUserAccessLogStore) GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analyticsmodel.BrowserShare, error) {
	conn, err := chConn()
	if err != nil {
		return nil, err
	}

	tableName := analyticsmodel.UserAccessLog{}.TableName()
	query := fmt.Sprintf(`
		SELECT user_agent, count() AS count
		FROM %s
		WHERE created_at >= ?
		GROUP BY user_agent
		ORDER BY count DESC
		LIMIT 100
	`, tableName)

	rows, err := conn.Query(ctx, query, startTime)
	if err != nil {
		return nil, fmt.Errorf("get browser distribution: %w", err)
	}
	defer func() { _ = rows.Close() }()

	browserCounts := make(map[string]uint64)
	for rows.Next() {
		var (
			userAgent string
			count     uint64
		)
		if err := rows.Scan(&userAgent, &count); err != nil {
			return nil, fmt.Errorf("scan browser distribution row: %w", err)
		}
		browser := analyticsmodel.ParseBrowserName(userAgent)
		browserCounts[browser] += count
	}

	result := make([]analyticsmodel.BrowserShare, 0, len(browserCounts))
	for browser, count := range browserCounts {
		result = append(result, analyticsmodel.BrowserShare{
			Browser: browser,
			Count:   count,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result, nil
}

func (s *clickhouseUserAccessLogStore) GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analyticsmodel.TopUser, error) {
	if limit < 1 {
		limit = 10
	}
	conn, err := chConn()
	if err != nil {
		return nil, err
	}

	tableName := analyticsmodel.UserAccessLog{}.TableName()
	query := fmt.Sprintf(`
		SELECT user_id, count() AS count
		FROM %s
		WHERE created_at >= ? AND user_id > 0
		GROUP BY user_id
		ORDER BY count DESC
		LIMIT ?
	`, tableName)

	rows, err := conn.Query(ctx, query, startTime, limit)
	if err != nil {
		return nil, fmt.Errorf("get top active users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []analyticsmodel.TopUser
	for rows.Next() {
		var item analyticsmodel.TopUser
		if err := rows.Scan(&item.UserID, &item.Count); err != nil {
			return nil, fmt.Errorf("scan top active user row: %w", err)
		}
		users = append(users, item)
	}
	return users, nil
}
