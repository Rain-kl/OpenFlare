// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

const hoursInDay = 24

// DailyTrend is a single day's access count.
type DailyTrend struct {
	Date  string
	Count uint64
}

// BrowserShare is a browser group's share of access logs.
type BrowserShare struct {
	Browser string
	Count   uint64
}

// TopUser is an active user ranked by access count.
type TopUser struct {
	UserID uint64
	Count  uint64
}

// GetDailyTrend returns per-day access counts for the last days days (inclusive of today).
func GetDailyTrend(ctx context.Context, days int) ([]DailyTrend, error) {
	if days < 1 {
		days = 7
	}
	if err := userAccessLogConn(); err != nil {
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

	rows, err := db.ChConn.Query(ctx, query, startTime)
	if err != nil {
		return nil, fmt.Errorf("get daily trend: %w", err)
	}
	defer func() { _ = rows.Close() }()

	trendMap := make(map[string]uint64, days)
	for i := 0; i < days; i++ {
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

	result := make([]DailyTrend, 0, days)
	for i := days - 1; i >= 0; i-- {
		dateStr := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		result = append(result, DailyTrend{
			Date:  dateStr,
			Count: trendMap[dateStr],
		})
	}
	return result, nil
}

// GetBrowserDistribution returns browser-grouped access counts since startTime.
func GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]BrowserShare, error) {
	if err := userAccessLogConn(); err != nil {
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

	rows, err := db.ChConn.Query(ctx, query, startTime)
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
		browser := ParseBrowserName(userAgent)
		browserCounts[browser] += count
	}

	result := make([]BrowserShare, 0, len(browserCounts))
	for browser, count := range browserCounts {
		result = append(result, BrowserShare{
			Browser: browser,
			Count:   count,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result, nil
}

// GetTopActiveUsers returns the most active users since startTime.
func GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]TopUser, error) {
	if limit < 1 {
		limit = 10
	}
	if err := userAccessLogConn(); err != nil {
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

	rows, err := db.ChConn.Query(ctx, query, startTime, limit)
	if err != nil {
		return nil, fmt.Errorf("get top active users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []TopUser
	for rows.Next() {
		var item TopUser
		if err := rows.Scan(&item.UserID, &item.Count); err != nil {
			return nil, fmt.Errorf("scan top active user row: %w", err)
		}
		users = append(users, item)
	}
	return users, nil
}