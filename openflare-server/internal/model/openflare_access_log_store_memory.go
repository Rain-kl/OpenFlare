// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db/idgen"
)

type memoryAccessLogStore struct {
	mu      sync.RWMutex
	records []*OpenFlareAccessLog
}

func (s *memoryAccessLogStore) InsertBatch(_ context.Context, records []*OpenFlareAccessLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, record := range records {
		if record == nil {
			continue
		}
		copyRecord := *record
		if copyRecord.ID == 0 {
			copyRecord.ID = uint(idgen.NextUint64ID())
		}
		if copyRecord.CreatedAt.IsZero() {
			copyRecord.CreatedAt = now
		}
		copyRecord.LoggedAt = copyRecord.LoggedAt.UTC()
		copyRecord.CreatedAt = copyRecord.CreatedAt.UTC()
		s.records = append(s.records, &copyRecord)
	}
	return nil
}

func (s *memoryAccessLogStore) List(_ context.Context, query OpenFlareAccessLogQuery) ([]*OpenFlareAccessLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(query)
	sortOpenFlareAccessLogRows(rows, query.SortBy, query.SortOrder)
	if query.PageSize > 0 {
		start, end := openFlareAccessLogPaginateBounds(len(rows), query.Page, query.PageSize)
		return cloneAccessLogSlice(rows[start:end]), nil
	}
	return cloneAccessLogSlice(rows), nil
}

func (s *memoryAccessLogStore) Count(_ context.Context, query OpenFlareAccessLogQuery) (int64, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(query)
	ips := make(map[string]struct{})
	for _, row := range rows {
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		ips[remoteAddr] = struct{}{}
	}
	return int64(len(rows)), int64(len(ips)), nil
}

func (s *memoryAccessLogStore) RegionCounts(_ context.Context, nodeID string, since time.Time, limit int) ([]*OpenFlareAccessLogRegionCount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(OpenFlareAccessLogQuery{NodeID: nodeID, Since: since})
	counts := make(map[string]int64)
	for _, row := range rows {
		region := strings.TrimSpace(row.Region)
		if region == "" {
			continue
		}
		counts[region]++
	}
	result := make([]*OpenFlareAccessLogRegionCount, 0, len(counts))
	for region, count := range counts {
		result = append(result, &OpenFlareAccessLogRegionCount{Region: region, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count == result[j].Count {
			return result[i].Region < result[j].Region
		}
		return result[i].Count > result[j].Count
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *memoryAccessLogStore) BucketAggregates(_ context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogBucketAggregateRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	aggregates := make(map[int64]*openFlareAccessLogBucketAggregateRow)
	for _, row := range rows {
		bucketEpoch := memoryAccessLogBucketEpoch(row.LoggedAt, bucketSeconds)
		item := aggregates[bucketEpoch]
		if item == nil {
			item = &openFlareAccessLogBucketAggregateRow{BucketEpoch: bucketEpoch}
			aggregates[bucketEpoch] = item
		}
		item.RequestCount++
		switch {
		case row.StatusCode < 400:
			item.SuccessCount++
		case row.StatusCode < 500:
			item.ClientErrorCount++
		default:
			item.ServerErrorCount++
		}
	}
	result := make([]openFlareAccessLogBucketAggregateRow, 0, len(aggregates))
	for _, item := range aggregates {
		result = append(result, *item)
	}
	return result, nil
}

func (s *memoryAccessLogStore) BucketDimensions(_ context.Context, filter OpenFlareAccessLogQuery, column string, bucketSeconds int64) ([]openFlareAccessLogBucketDimensionRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	seen := make(map[int64]map[string]struct{})
	var result []openFlareAccessLogBucketDimensionRow
	for _, row := range rows {
		var value string
		switch column {
		case "remote_addr":
			value = strings.TrimSpace(row.RemoteAddr)
		case "host":
			value = strings.TrimSpace(row.Host)
		default:
			continue
		}
		if value == "" {
			continue
		}
		bucketEpoch := memoryAccessLogBucketEpoch(row.LoggedAt, bucketSeconds)
		if seen[bucketEpoch] == nil {
			seen[bucketEpoch] = make(map[string]struct{})
		}
		if _, ok := seen[bucketEpoch][value]; ok {
			continue
		}
		seen[bucketEpoch][value] = struct{}{}
		result = append(result, openFlareAccessLogBucketDimensionRow{BucketEpoch: bucketEpoch, Value: value})
	}
	return result, nil
}

func (s *memoryAccessLogStore) IPAggregates(_ context.Context, filter OpenFlareAccessLogQuery, exactRemoteAddr bool) ([]openFlareAccessLogIPAggregateRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if exactRemoteAddr && strings.TrimSpace(filter.RemoteAddr) == "" {
		return []openFlareAccessLogIPAggregateRow{}, nil
	}
	rows := s.filterRecords(filter)
	aggregates := make(map[string]*openFlareAccessLogIPAggregateRow)
	for _, row := range rows {
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		if exactRemoteAddr && remoteAddr != strings.TrimSpace(filter.RemoteAddr) {
			continue
		}
		item := aggregates[remoteAddr]
		if item == nil {
			item = &openFlareAccessLogIPAggregateRow{RemoteAddr: remoteAddr}
			aggregates[remoteAddr] = item
		}
		item.RequestCount++
		epoch := row.LoggedAt.UTC().Unix()
		if epoch > item.LastSeenEpoch {
			item.LastSeenEpoch = epoch
		}
		switch {
		case row.StatusCode < 400:
			item.SuccessCount++
		case row.StatusCode < 500:
			item.ClientErrorCount++
		default:
			item.ServerErrorCount++
		}
	}
	result := make([]openFlareAccessLogIPAggregateRow, 0, len(aggregates))
	for _, item := range aggregates {
		result = append(result, *item)
	}
	return result, nil
}

func (s *memoryAccessLogStore) IPSummaries(_ context.Context, filter OpenFlareAccessLogQuery, recentSince time.Time) ([]openFlareAccessLogIPSummaryRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	aggregates := make(map[string]*openFlareAccessLogIPSummaryRow)
	for _, row := range rows {
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		item := aggregates[remoteAddr]
		if item == nil {
			item = &openFlareAccessLogIPSummaryRow{RemoteAddr: remoteAddr}
			aggregates[remoteAddr] = item
		}
		item.TotalRequests++
		if !recentSince.IsZero() && !row.LoggedAt.Before(recentSince) {
			item.RecentRequests++
		}
		epoch := row.LoggedAt.UTC().Unix()
		if epoch > item.LastSeenEpoch {
			item.LastSeenEpoch = epoch
		}
	}
	result := make([]openFlareAccessLogIPSummaryRow, 0, len(aggregates))
	for _, item := range aggregates {
		result = append(result, *item)
	}
	return result, nil
}

func (s *memoryAccessLogStore) IPTrend(_ context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) ([]openFlareAccessLogIPTrendRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	aggregates := make(map[int64]int64)
	for _, row := range rows {
		bucketEpoch := memoryAccessLogBucketEpoch(row.LoggedAt, bucketSeconds)
		aggregates[bucketEpoch]++
	}
	result := make([]openFlareAccessLogIPTrendRow, 0, len(aggregates))
	for bucketEpoch, count := range aggregates {
		result = append(result, openFlareAccessLogIPTrendRow{BucketEpoch: bucketEpoch, RequestCount: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].BucketEpoch < result[j].BucketEpoch })
	return result, nil
}

func (s *memoryAccessLogStore) DeleteAll(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := int64(len(s.records))
	s.records = nil
	return count, nil
}

func (s *memoryAccessLogStore) DeleteBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff = cutoff.UTC()
	remaining := make([]*OpenFlareAccessLog, 0, len(s.records))
	var deleted int64
	for _, row := range s.records {
		if row.LoggedAt.Before(cutoff) {
			deleted++
			continue
		}
		remaining = append(remaining, row)
	}
	s.records = remaining
	return deleted, nil
}

func (s *memoryAccessLogStore) DeleteByNodeBefore(_ context.Context, nodeID string, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before = before.UTC()
	remaining := make([]*OpenFlareAccessLog, 0, len(s.records))
	var deleted int64
	for _, row := range s.records {
		if row.NodeID == nodeID && row.LoggedAt.Before(before) {
			deleted++
			continue
		}
		remaining = append(remaining, row)
	}
	s.records = remaining
	return deleted, nil
}

func (s *memoryAccessLogStore) filterRecords(query OpenFlareAccessLogQuery) []*OpenFlareAccessLog {
	result := make([]*OpenFlareAccessLog, 0, len(s.records))
	for _, row := range s.records {
		if !memoryAccessLogMatches(row, query) {
			continue
		}
		result = append(result, row)
	}
	return result
}

func memoryAccessLogMatches(row *OpenFlareAccessLog, query OpenFlareAccessLogQuery) bool {
	if row == nil {
		return false
	}
	if trimmed := strings.TrimSpace(query.NodeID); trimmed != "" && row.NodeID != trimmed {
		return false
	}
	if trimmed := strings.TrimSpace(query.RemoteAddr); trimmed != "" && !strings.HasPrefix(strings.TrimSpace(row.RemoteAddr), trimmed) {
		return false
	}
	if trimmed := strings.TrimSpace(query.Host); trimmed != "" && !strings.HasPrefix(strings.TrimSpace(row.Host), trimmed) {
		return false
	}
	if trimmed := strings.TrimSpace(query.Path); trimmed != "" && !strings.HasPrefix(strings.TrimSpace(row.Path), trimmed) {
		return false
	}
	if !query.Since.IsZero() && row.LoggedAt.Before(query.Since) {
		return false
	}
	if !query.Until.IsZero() && !row.LoggedAt.Before(query.Until) {
		return false
	}
	return true
}

func memoryAccessLogBucketEpoch(loggedAt time.Time, bucketSeconds int64) int64 {
	if bucketSeconds <= 0 {
		bucketSeconds = 180
	}
	epoch := loggedAt.UTC().Unix()
	return (epoch / bucketSeconds) * bucketSeconds
}

func cloneAccessLogSlice(rows []*OpenFlareAccessLog) []*OpenFlareAccessLog {
	result := make([]*OpenFlareAccessLog, len(rows))
	for index, row := range rows {
		if row == nil {
			continue
		}
		copyRecord := *row
		result[index] = &copyRecord
	}
	return result
}

func sortOpenFlareAccessLogRows(items []*OpenFlareAccessLog, sortBy string, sortOrder string) {
	desc := openFlareAccessLogNormalizeSortOrder(sortOrder) != "asc"
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left == nil || right == nil {
			return left != nil
		}
		var compare int
		switch strings.TrimSpace(sortBy) {
		case "status_code":
			compare = left.StatusCode - right.StatusCode
		case "remote_addr":
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		case "host":
			compare = strings.Compare(left.Host, right.Host)
		case "path":
			compare = strings.Compare(left.Path, right.Path)
		default:
			compare = openFlareAccessLogCompareInt64(left.LoggedAt.Unix(), right.LoggedAt.Unix())
		}
		if compare == 0 {
			compare = openFlareAccessLogCompareInt64(left.LoggedAt.Unix(), right.LoggedAt.Unix())
		}
		if compare == 0 {
			compare = openFlareAccessLogCompareInt64(int64(left.ID), int64(right.ID))
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}