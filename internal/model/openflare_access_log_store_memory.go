// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
)

const (
	accessLogColumnStatusCode = "status_code"
	accessLogColumnHost       = "host"
	accessLogColumnPath       = "path"
	accessLogColumnRemoteAddr = "remote_addr"
	accessLogColumnUserAgent  = "user_agent"
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
			copyRecord.ID = idgen.NextUint64ID()
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

func (s *memoryAccessLogStore) Count(_ context.Context, query OpenFlareAccessLogQuery) (int64, int64, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(query)
	ips := make(map[string]struct{})
	var totalBytes int64
	for _, row := range rows {
		totalBytes += row.BytesSent
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		ips[remoteAddr] = struct{}{}
	}
	return int64(len(rows)), int64(len(ips)), totalBytes, nil
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
	type bucketAccumulator struct {
		openFlareAccessLogBucketAggregateRow
		uniqueIPs   map[string]struct{}
		uniqueHosts map[string]struct{}
	}
	aggregates := make(map[int64]*bucketAccumulator)
	for _, row := range rows {
		bucketEpoch := memoryAccessLogBucketEpoch(row.LoggedAt, bucketSeconds)
		item := aggregates[bucketEpoch]
		if item == nil {
			item = &bucketAccumulator{
				openFlareAccessLogBucketAggregateRow: openFlareAccessLogBucketAggregateRow{BucketEpoch: bucketEpoch},
				uniqueIPs:                            make(map[string]struct{}),
				uniqueHosts:                          make(map[string]struct{}),
			}
			aggregates[bucketEpoch] = item
		}
		item.RequestCount++
		item.BytesSent += row.BytesSent
		item.RequestLength += row.RequestLength
		switch {
		case row.StatusCode < 400:
			item.SuccessCount++
		case row.StatusCode < 500:
			item.ClientErrorCount++
		default:
			item.ServerErrorCount++
		}
		if remoteAddr := strings.TrimSpace(row.RemoteAddr); remoteAddr != "" {
			item.uniqueIPs[remoteAddr] = struct{}{}
		}
		if host := strings.TrimSpace(row.Host); host != "" {
			item.uniqueHosts[host] = struct{}{}
		}
	}
	result := make([]openFlareAccessLogBucketAggregateRow, 0, len(aggregates))
	for _, item := range aggregates {
		item.UniqueIPCount = int64(len(item.uniqueIPs))
		item.UniqueHostCount = int64(len(item.uniqueHosts))
		result = append(result, item.openFlareAccessLogBucketAggregateRow)
	}
	bucketRows := make([]*OpenFlareAccessLogBucketRow, len(result))
	for index := range result {
		bucketRows[index] = &OpenFlareAccessLogBucketRow{
			BucketEpoch:      result[index].BucketEpoch,
			RequestCount:     result[index].RequestCount,
			UniqueIPCount:    result[index].UniqueIPCount,
			UniqueHostCount:  result[index].UniqueHostCount,
			SuccessCount:     result[index].SuccessCount,
			ClientErrorCount: result[index].ClientErrorCount,
			ServerErrorCount: result[index].ServerErrorCount,
			BytesSent:        result[index].BytesSent,
			RequestLength:    result[index].RequestLength,
		}
	}
	sortOpenFlareAccessLogBucketRows(bucketRows, filter.SortBy, filter.SortOrder)
	for index := range result {
		result[index] = openFlareAccessLogBucketAggregateRow{
			BucketEpoch:      bucketRows[index].BucketEpoch,
			RequestCount:     bucketRows[index].RequestCount,
			UniqueIPCount:    bucketRows[index].UniqueIPCount,
			UniqueHostCount:  bucketRows[index].UniqueHostCount,
			SuccessCount:     bucketRows[index].SuccessCount,
			ClientErrorCount: bucketRows[index].ClientErrorCount,
			ServerErrorCount: bucketRows[index].ServerErrorCount,
			BytesSent:        bucketRows[index].BytesSent,
			RequestLength:    bucketRows[index].RequestLength,
		}
	}
	if filter.PageSize > 0 {
		start, end := openFlareAccessLogPaginateBounds(len(result), filter.Page, filter.PageSize)
		return result[start:end], nil
	}
	return result, nil
}

func (s *memoryAccessLogStore) CountBuckets(_ context.Context, filter OpenFlareAccessLogQuery, bucketSeconds int64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	seen := make(map[int64]struct{})
	for _, row := range rows {
		seen[memoryAccessLogBucketEpoch(row.LoggedAt, bucketSeconds)] = struct{}{}
	}
	return int64(len(seen)), nil
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
		case columnRemoteAddr:
			value = strings.TrimSpace(row.RemoteAddr)
		case columnHost:
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

func (s *memoryAccessLogStore) IPSummaries(_ context.Context, filter OpenFlareAccessLogQuery, _ time.Time) ([]openFlareAccessLogIPSummaryRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	type aggregate struct {
		RemoteAddr      string
		Region          string
		RegionEpoch     int64
		TotalRequests   int64
		Success2xxCount int64
		BytesReceived   int64
		BytesSent       int64
		LastSeenEpoch   int64
	}
	aggregates := make(map[string]*aggregate)
	for _, row := range rows {
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		item := aggregates[remoteAddr]
		if item == nil {
			item = &aggregate{RemoteAddr: remoteAddr}
			aggregates[remoteAddr] = item
		}
		item.TotalRequests++
		if row.StatusCode >= 200 && row.StatusCode < 300 {
			item.Success2xxCount++
		}
		item.BytesReceived += row.RequestLength
		item.BytesSent += row.BytesSent
		epoch := row.LoggedAt.UTC().Unix()
		if epoch > item.LastSeenEpoch {
			item.LastSeenEpoch = epoch
		}
		if epoch >= item.RegionEpoch {
			item.RegionEpoch = epoch
			item.Region = strings.TrimSpace(row.Region)
		}
	}
	summaryRows := make([]*OpenFlareAccessLogIPSummaryRow, 0, len(aggregates))
	for _, item := range aggregates {
		ratio := 0.0
		if item.TotalRequests > 0 {
			ratio = float64(item.Success2xxCount) / float64(item.TotalRequests)
		}
		summaryRows = append(summaryRows, &OpenFlareAccessLogIPSummaryRow{
			RemoteAddr:      item.RemoteAddr,
			Region:          item.Region,
			TotalRequests:   item.TotalRequests,
			Success2xxCount: item.Success2xxCount,
			SuccessRatio:    ratio,
			BytesReceived:   item.BytesReceived,
			BytesSent:       item.BytesSent,
			RecentRequests:  0,
			LastSeenEpoch:   item.LastSeenEpoch,
		})
	}
	sortOpenFlareAccessLogIPSummaryRows(summaryRows, filter.SortBy, filter.SortOrder)
	if filter.PageSize > 0 {
		start, end := openFlareAccessLogPaginateBounds(len(summaryRows), filter.Page, filter.PageSize)
		summaryRows = summaryRows[start:end]
	}
	result := make([]openFlareAccessLogIPSummaryRow, len(summaryRows))
	for index, item := range summaryRows {
		result[index] = openFlareAccessLogIPSummaryRow{
			RemoteAddr:      item.RemoteAddr,
			Region:          item.Region,
			TotalRequests:   item.TotalRequests,
			Success2xxCount: item.Success2xxCount,
			SuccessRatio:    item.SuccessRatio,
			BytesReceived:   item.BytesReceived,
			BytesSent:       item.BytesSent,
			RecentRequests:  0,
			LastSeenEpoch:   item.LastSeenEpoch,
		}
	}
	return result, nil
}

func (s *memoryAccessLogStore) CountIPSummaries(_ context.Context, filter OpenFlareAccessLogQuery) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	seen := make(map[string]struct{})
	for _, row := range rows {
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		seen[remoteAddr] = struct{}{}
	}
	return int64(len(seen)), nil
}

func (s *memoryAccessLogStore) WAFIPAggregates(_ context.Context, filter OpenFlareAccessLogQuery) ([]openFlareAccessLogWAFIPAggregateRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	aggregates := make(map[string]*openFlareAccessLogWAFIPAggregateRow)
	order := make([]string, 0)
	for _, row := range rows {
		remoteAddr := strings.TrimSpace(row.RemoteAddr)
		if remoteAddr == "" {
			continue
		}
		item := aggregates[remoteAddr]
		if item == nil {
			item = &openFlareAccessLogWAFIPAggregateRow{
				RemoteAddr:   remoteAddr,
				StatusCounts: make(map[int]int64),
			}
			aggregates[remoteAddr] = item
			order = append(order, remoteAddr)
		}
		item.RequestCount++
		item.StatusCounts[row.StatusCode]++
		if row.StatusCode == http.StatusNotFound {
			item.Status404Count++
		}
		if row.StatusCode >= 400 && row.StatusCode < 500 {
			item.ClientErrorCount++
		}
		if row.StatusCode >= http.StatusInternalServerError {
			item.ServerErrorCount++
		}
		if memoryAccessLogHostIsIPLiteral(row.Host) {
			item.IPHostCount++
		}
		epoch := row.LoggedAt.UTC().Unix()
		if epoch > item.LastSeenEpoch {
			item.LastSeenEpoch = epoch
		}
	}
	result := make([]openFlareAccessLogWAFIPAggregateRow, 0, len(order))
	for _, remoteAddr := range order {
		if item := aggregates[remoteAddr]; item != nil {
			result = append(result, *item)
		}
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

func (s *memoryAccessLogStore) TrafficSummary(_ context.Context, filter OpenFlareAccessLogQuery) (OpenFlareAccessLogTrafficSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	ips := make(map[string]struct{})
	nodes := make(map[string]struct{})
	var summary OpenFlareAccessLogTrafficSummary
	for _, row := range rows {
		summary.RequestCount++
		summary.BytesSent += row.BytesSent
		summary.RequestLength += row.RequestLength
		if row.StatusCode >= http.StatusInternalServerError {
			summary.ErrorCount++
		}
		if ip := strings.TrimSpace(row.RemoteAddr); ip != "" {
			ips[ip] = struct{}{}
		}
		if id := strings.TrimSpace(row.NodeID); id != "" {
			nodes[id] = struct{}{}
		}
	}
	summary.UniqueIPCount = int64(len(ips))
	summary.NodeCount = int64(len(nodes))
	return summary, nil
}

func (s *memoryAccessLogStore) ValueCounts(_ context.Context, filter OpenFlareAccessLogQuery, column string, limit int) ([]OpenFlareAccessLogValueCount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	col := strings.TrimSpace(strings.ToLower(column))
	switch col {
	case accessLogColumnStatusCode, accessLogColumnHost, accessLogColumnPath, accessLogColumnRemoteAddr, accessLogColumnUserAgent:
	default:
		return nil, nil
	}
	rows := s.filterRecords(filter)
	counts := make(map[string]int64)
	for _, row := range rows {
		var value string
		switch col {
		case accessLogColumnStatusCode:
			value = strconv.Itoa(row.StatusCode)
		case accessLogColumnHost:
			value = strings.TrimSpace(row.Host)
		case accessLogColumnPath:
			value = strings.TrimSpace(row.Path)
		case accessLogColumnRemoteAddr:
			value = strings.TrimSpace(row.RemoteAddr)
		case accessLogColumnUserAgent:
			value = strings.TrimSpace(row.UserAgent)
		}
		if value == "" {
			continue
		}
		counts[value]++
	}
	result := make([]OpenFlareAccessLogValueCount, 0, len(counts))
	for value, count := range counts {
		result = append(result, OpenFlareAccessLogValueCount{Value: value, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count == result[j].Count {
			return result[i].Value < result[j].Value
		}
		return result[i].Count > result[j].Count
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *memoryAccessLogStore) NodeAggregates(_ context.Context, filter OpenFlareAccessLogQuery) ([]OpenFlareAccessLogNodeAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.filterRecords(filter)
	type acc struct {
		OpenFlareAccessLogNodeAggregate
		ips map[string]struct{}
	}
	byNode := make(map[string]*acc)
	for _, row := range rows {
		id := strings.TrimSpace(row.NodeID)
		if id == "" {
			continue
		}
		item := byNode[id]
		if item == nil {
			item = &acc{
				OpenFlareAccessLogNodeAggregate: OpenFlareAccessLogNodeAggregate{NodeID: id},
				ips:                             make(map[string]struct{}),
			}
			byNode[id] = item
		}
		item.RequestCount++
		if row.StatusCode >= http.StatusInternalServerError {
			item.ErrorCount++
		}
		if ip := strings.TrimSpace(row.RemoteAddr); ip != "" {
			item.ips[ip] = struct{}{}
		}
	}
	result := make([]OpenFlareAccessLogNodeAggregate, 0, len(byNode))
	for _, item := range byNode {
		item.UniqueIPCount = int64(len(item.ips))
		result = append(result, item.OpenFlareAccessLogNodeAggregate)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].RequestCount == result[j].RequestCount {
			return result[i].NodeID < result[j].NodeID
		}
		return result[i].RequestCount > result[j].RequestCount
	})
	return result, nil
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
	if len(query.Hosts) > 0 {
		rowHost := strings.ToLower(strings.TrimSpace(row.Host))
		matched := false
		for _, host := range query.Hosts {
			if strings.ToLower(strings.TrimSpace(host)) == rowHost {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	} else if trimmed := strings.TrimSpace(query.Host); trimmed != "" && !strings.HasPrefix(strings.TrimSpace(row.Host), trimmed) {
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

func memoryAccessLogHostIsIPLiteral(value string) bool {
	host := strings.TrimSpace(value)
	if host == "" {
		return false
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	_, err := netip.ParseAddr(host)
	return err == nil
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
	desc := openFlareAccessLogNormalizeSortOrder(sortOrder) != sortOrderAsc
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
		case columnRemoteAddr:
			compare = strings.Compare(left.RemoteAddr, right.RemoteAddr)
		case columnHost:
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
			compare = openFlareAccessLogCompareInt64(openFlareAccessLogUintToInt64(left.ID), openFlareAccessLogUintToInt64(right.ID))
		}
		if desc {
			return compare > 0
		}
		return compare < 0
	})
}
