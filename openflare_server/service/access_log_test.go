package service

import (
	"openflare/model"
	"strings"
	"testing"
	"time"
)

func TestListAccessLogsIncludesSummaryTotals(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now()
	if err := model.DB.Create(&model.Node{
		NodeID: "node-a",
		Name:   "edge-a",
	}).Error; err != nil {
		t.Fatalf("failed to seed node-a: %v", err)
	}
	if err := model.DB.Create(&model.Node{
		NodeID: "node-b",
		Name:   "edge-b",
	}).Error; err != nil {
		t.Fatalf("failed to seed node-b: %v", err)
	}

	logs := []*model.NodeAccessLog{
		{
			NodeID:     "node-a",
			LoggedAt:   now.Add(-5 * time.Minute),
			RemoteAddr: "1.1.1.1",
			Region:     "United States",
			Host:       "a.example.com",
			Path:       "/alpha",
			StatusCode: 200,
		},
		{
			NodeID:     "node-a",
			LoggedAt:   now.Add(-4 * time.Minute),
			RemoteAddr: "2.2.2.2",
			Region:     "China",
			Host:       "a.example.com",
			Path:       "/beta",
			StatusCode: 404,
		},
		{
			NodeID:     "node-b",
			LoggedAt:   now.Add(-3 * time.Minute),
			RemoteAddr: "1.1.1.1",
			Region:     "United States",
			Host:       "b.example.com",
			Path:       "/gamma",
			StatusCode: 502,
		},
		{
			NodeID:     "node-b",
			LoggedAt:   now.Add(-2 * time.Minute),
			RemoteAddr: "",
			Host:       "b.example.com",
			Path:       "/delta",
			StatusCode: 200,
		},
	}
	seedNodeAccessLogs(t, logs)

	result, err := ListAccessLogs(AccessLogQuery{Page: 0, PageSize: 2})
	if err != nil {
		t.Fatalf("ListAccessLogs failed: %v", err)
	}
	if result.TotalRecord != 4 {
		t.Fatalf("expected total_record=4, got %d", result.TotalRecord)
	}
	if result.TotalIP != 2 {
		t.Fatalf("expected total_ip=2, got %d", result.TotalIP)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected current page items=2, got %d", len(result.Items))
	}
	if result.Items[1].Region == "" {
		t.Fatalf("expected region to be returned, got %+v", result.Items[1])
	}
	if !result.HasMore {
		t.Fatal("expected has_more to be true")
	}

	filtered, err := ListAccessLogs(AccessLogQuery{NodeID: "node-a", Page: 0, PageSize: 50})
	if err != nil {
		t.Fatalf("ListAccessLogs filtered failed: %v", err)
	}
	if filtered.TotalRecord != 2 {
		t.Fatalf("expected filtered total_record=2, got %d", filtered.TotalRecord)
	}
	if filtered.TotalIP != 2 {
		t.Fatalf("expected filtered total_ip=2, got %d", filtered.TotalIP)
	}
	if len(filtered.Items) != 2 {
		t.Fatalf("expected filtered items=2, got %d", len(filtered.Items))
	}
}

func TestListAccessLogsUsesDefaultPageSize(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now()
	if err := model.DB.Create(&model.Node{
		NodeID: "node-default-page-size",
		Name:   "edge-default-page-size",
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	logs := make([]*model.NodeAccessLog, 0, 25)
	for index := range 25 {
		logs = append(logs, &model.NodeAccessLog{
			NodeID:     "node-default-page-size",
			LoggedAt:   now.Add(-time.Duration(index) * time.Minute),
			RemoteAddr: "1.1.1.1",
			Host:       "example.com",
			Path:       "/default-page-size",
			StatusCode: 200,
		})
	}
	seedNodeAccessLogs(t, logs)

	result, err := ListAccessLogs(AccessLogQuery{})
	if err != nil {
		t.Fatalf("ListAccessLogs failed: %v", err)
	}
	if result.PageSize != 20 {
		t.Fatalf("expected default page_size=20, got %d", result.PageSize)
	}
	if len(result.Items) != 20 {
		t.Fatalf("expected current page items=20, got %d", len(result.Items))
	}
	if !result.HasMore {
		t.Fatal("expected has_more to be true")
	}
}

func TestListFoldedAccessLogsAndIPSummaries(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Date(2026, 3, 19, 8, 12, 30, 0, time.UTC)
	if err := model.DB.Create(&model.Node{
		NodeID: "node-folded",
		Name:   "edge-folded",
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}
	logs := []*model.NodeAccessLog{
		{
			NodeID:     "node-folded",
			LoggedAt:   now.Add(-4 * time.Minute),
			RemoteAddr: "203.0.113.1",
			Host:       "alpha.example.com",
			Path:       "/first",
			StatusCode: 200,
		},
		{
			NodeID:     "node-folded",
			LoggedAt:   now.Add(-3 * time.Minute),
			RemoteAddr: "203.0.113.1",
			Host:       "alpha.example.com",
			Path:       "/second",
			StatusCode: 502,
		},
		{
			NodeID:     "node-folded",
			LoggedAt:   now.Add(-2 * time.Minute),
			RemoteAddr: "203.0.113.2",
			Host:       "beta.example.com",
			Path:       "/third",
			StatusCode: 404,
		},
	}
	seedNodeAccessLogs(t, logs)

	folded, err := ListFoldedAccessLogs(AccessLogQuery{
		NodeID:      "node-folded",
		Page:        0,
		PageSize:    10,
		FoldMinutes: 5,
	})
	if err != nil {
		t.Fatalf("ListFoldedAccessLogs failed: %v", err)
	}
	if len(folded.Items) != 2 {
		t.Fatalf("expected two folded buckets, got %+v", folded.Items)
	}
	if folded.TotalRecord != 3 || folded.TotalBucket != 2 {
		t.Fatalf("unexpected folded totals: %+v", folded)
	}
	if folded.Items[0].RequestCount+folded.Items[1].RequestCount != 3 {
		t.Fatalf("unexpected folded request count sum: %+v", folded.Items)
	}

	ipSummaries, err := ListAccessLogIPSummaries(AccessLogIPSummaryQuery{
		NodeID:    "node-folded",
		Page:      0,
		PageSize:  10,
		SortBy:    "total_requests",
		SortOrder: "desc",
	})
	if err != nil {
		t.Fatalf("ListAccessLogIPSummaries failed: %v", err)
	}
	if len(ipSummaries.Items) != 2 {
		t.Fatalf("expected two ip summary rows, got %+v", ipSummaries.Items)
	}
	if ipSummaries.Items[0].RemoteAddr != "203.0.113.1" || ipSummaries.Items[0].TotalRequests != 2 {
		t.Fatalf("unexpected top ip summary row: %+v", ipSummaries.Items[0])
	}
}

func TestCleanupAccessLogsDeletesExpiredData(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now().UTC()
	seedNodeAccessLogs(t, []*model.NodeAccessLog{
		{
			NodeID:     "node-cleanup",
			LoggedAt:   now.Add(-10 * 24 * time.Hour),
			RemoteAddr: "203.0.113.9",
			Host:       "cleanup.example.com",
			Path:       "/old",
			StatusCode: 200,
		},
		{
			NodeID:     "node-cleanup",
			LoggedAt:   now.Add(-2 * 24 * time.Hour),
			RemoteAddr: "203.0.113.10",
			Host:       "cleanup.example.com",
			Path:       "/recent",
			StatusCode: 200,
		},
	})

	result, err := CleanupAccessLogs(AccessLogCleanupInput{RetentionDays: 7})
	if err != nil {
		t.Fatalf("CleanupAccessLogs failed: %v", err)
	}
	if result.DeletedCount != 1 {
		t.Fatalf("expected 1 deleted record, got %+v", result)
	}

	remaining, err := ListAccessLogs(AccessLogQuery{Page: 0, PageSize: 10, NodeID: "node-cleanup"})
	if err != nil {
		t.Fatalf("ListAccessLogs failed after cleanup: %v", err)
	}
	if len(remaining.Items) != 1 || remaining.Items[0].Path != "/recent" {
		t.Fatalf("unexpected remaining logs after cleanup: %+v", remaining.Items)
	}
}

func TestPersistNodeAccessLogsTruncatesLongPath(t *testing.T) {
	setupServiceTestDB(t)

	longPath := "/" + strings.Repeat("a", 140)
	reportedAt := time.Now().UTC()
	if err := persistNodeAccessLogs(model.DB, "node-truncate", []AgentNodeAccessLog{
		{
			LoggedAtUnix: reportedAt.Unix(),
			RemoteAddr:   "203.0.113.10",
			Host:         "truncate.example.com",
			Path:         longPath,
			StatusCode:   200,
		},
	}, reportedAt); err != nil {
		t.Fatalf("persistNodeAccessLogs failed: %v", err)
	}

	logs, err := model.ListNodeAccessLogs(model.NodeAccessLogQuery{
		NodeID:   "node-truncate",
		Page:     0,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListNodeAccessLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one stored log, got %+v", logs)
	}
	if got := len([]rune(logs[0].Path)); got != nodeAccessLogPathMaxLength {
		t.Fatalf("expected truncated path length %d, got %d (%q)", nodeAccessLogPathMaxLength, got, logs[0].Path)
	}
}

func seedNodeAccessLogs(t *testing.T, logs []*model.NodeAccessLog) {
	t.Helper()
	for _, item := range logs {
		if err := model.DB.Create(item).Error; err != nil {
			t.Fatalf("failed to seed access log: %v", err)
		}
	}
}
