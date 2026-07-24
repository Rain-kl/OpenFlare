// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"context"
	"errors"
	"strings"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"gorm.io/gorm"
)

// StatsRange is a supported traffic window for Zone analytics.
type StatsRange string

// StatsRange constants representing supported analytics windows.
const (
	// StatsRange24h represents a 24-hour time window.
	StatsRange24h StatsRange = "24h"
	// StatsRange7d represents a 7-day time window.
	StatsRange7d StatsRange = "7d"
	// StatsRange30d represents a 30-day time window.
	StatsRange30d StatsRange = "30d"
)

const (
	hoursPerDay      = 24
	daysPerWeek      = 7
	daysPerMonth     = 30
	minutesPerHour   = 60
	bucketMinutes24h = 60
	bucketMinutes7d  = 6 * minutesPerHour
	bucketMinutes30d = 24 * minutesPerHour
)

// StatsPoint is one bucket on a Zone traffic chart.
type StatsPoint struct {
	BucketStartedAt time.Time `json:"bucket_started_at"`
	RequestCount    int64     `json:"request_count"`
	UniqueVisitors  int64     `json:"unique_visitors"`
	BytesSent       int64     `json:"bytes_sent"`
}

// Stats summarizes edge traffic for all domains under a Zone.
type Stats struct {
	Range           StatsRange   `json:"range"`
	RangeHours      int          `json:"range_hours"`
	WindowStartedAt time.Time    `json:"window_started_at"`
	WindowEndedAt   time.Time    `json:"window_ended_at"`
	BucketMinutes   int          `json:"bucket_minutes"`
	UniqueVisitors  int64        `json:"unique_visitors"`
	RequestCount    int64        `json:"request_count"`
	BytesSent       int64        `json:"bytes_sent"`
	DomainCount     int          `json:"domain_count"`
	Available       bool         `json:"available"`
	Series          []StatsPoint `json:"series"`
}

func parseStatsRange(raw string) (StatsRange, time.Duration, int, error) {
	switch StatsRange(strings.TrimSpace(raw)) {
	case "", StatsRange24h:
		return StatsRange24h, hoursPerDay * time.Hour, bucketMinutes24h, nil
	case StatsRange7d:
		return StatsRange7d, daysPerWeek * hoursPerDay * time.Hour, bucketMinutes7d, nil
	case StatsRange30d:
		return StatsRange30d, daysPerMonth * hoursPerDay * time.Hour, bucketMinutes30d, nil
	default:
		return "", 0, 0, errors.New(errStatsRangeInvalid)
	}
}

// GetStats aggregates access-log traffic for a Zone over a time range.
func GetStats(ctx context.Context, id uint, rangeRaw string) (*Stats, error) {
	statsRange, window, bucketMinutes, err := parseStatsRange(rangeRaw)
	if err != nil {
		return nil, err
	}

	var zone model.Zone
	if err := db.DB(ctx).First(&zone, id).Error; err != nil {
		return nil, err
	}

	var domains []model.ZoneDomain
	if err := db.DB(ctx).Where("zone_id = ?", id).Order("domain asc").Find(&domains).Error; err != nil {
		return nil, err
	}

	now := time.Now().UTC().Truncate(time.Minute)
	since := now.Add(-window)
	// Align chart window start to bucket boundary for cleaner x-axis labels.
	bucket := time.Duration(bucketMinutes) * time.Minute
	since = since.Truncate(bucket)

	result := &Stats{
		Range:           statsRange,
		RangeHours:      int(window / time.Hour),
		WindowStartedAt: since,
		WindowEndedAt:   now,
		BucketMinutes:   bucketMinutes,
		DomainCount:     len(domains),
		Available:       true,
		Series:          emptyStatsSeries(since, now, bucketMinutes),
	}
	if len(domains) == 0 {
		return result, nil
	}

	hosts := make([]string, 0, len(domains))
	for _, domain := range domains {
		if host := strings.TrimSpace(domain.Domain); host != "" {
			hosts = append(hosts, host)
		}
	}
	if len(hosts) == 0 {
		return result, nil
	}

	requestCount, uniqueVisitors, totalBytesSent, err := model.CountOpenFlareAccessLogs(ctx, model.OpenFlareAccessLogQuery{
		Hosts: hosts,
		Since: since,
		Until: now,
	})
	if err != nil {
		if isAnalyticsUnavailable(err) {
			result.Available = false
			return result, nil
		}
		return nil, err
	}
	result.RequestCount = requestCount
	result.UniqueVisitors = uniqueVisitors
	result.BytesSent = totalBytesSent

	buckets, err := model.ListOpenFlareAccessLogBuckets(ctx, model.OpenFlareAccessLogBucketQuery{
		Hosts:       hosts,
		Since:       since,
		Until:       now,
		FoldMinutes: bucketMinutes,
		SortBy:      "logged_at",
		SortOrder:   "asc",
	})
	if err != nil {
		if isAnalyticsUnavailable(err) {
			result.Available = false
			return result, nil
		}
		return nil, err
	}

	byEpoch := make(map[int64]model.OpenFlareAccessLogBucketRow, len(buckets))
	for _, bucketRow := range buckets {
		if bucketRow == nil {
			continue
		}
		byEpoch[bucketRow.BucketEpoch] = *bucketRow
	}
	series := emptyStatsSeries(since, now, bucketMinutes)
	for index := range series {
		epoch := series[index].BucketStartedAt.Unix()
		if row, ok := byEpoch[epoch]; ok {
			series[index].RequestCount = row.RequestCount
			series[index].UniqueVisitors = row.UniqueIPCount
			series[index].BytesSent = row.BytesSent
		}
	}
	result.Series = series
	return result, nil
}

func emptyStatsSeries(since, until time.Time, bucketMinutes int) []StatsPoint {
	if bucketMinutes <= 0 {
		bucketMinutes = 60
	}
	bucket := time.Duration(bucketMinutes) * time.Minute
	start := since.UTC().Truncate(bucket)
	end := until.UTC()
	if !end.After(start) {
		return []StatsPoint{{BucketStartedAt: start}}
	}
	// Cap points to keep chart readable.
	maxPoints := 120
	capacity := int(end.Sub(start)/bucket) + 1
	if capacity > maxPoints {
		capacity = maxPoints
	}
	points := make([]StatsPoint, 0, capacity)
	for cursor := start; !cursor.After(end) && len(points) < maxPoints; cursor = cursor.Add(bucket) {
		points = append(points, StatsPoint{BucketStartedAt: cursor})
	}
	if len(points) == 0 {
		points = append(points, StatsPoint{BucketStartedAt: start})
	}
	return points
}

func isAnalyticsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrInvalidDB) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "clickhouse connection is not initialized") ||
		strings.Contains(msg, "clickhouse is not") ||
		strings.Contains(msg, "database is not initialized")
}
