package service

import (
	"openflare/model"
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
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed access logs: %v", err)
	}

	result, err := ListAccessLogs("", 0, 2)
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

	filtered, err := ListAccessLogs("node-a", 0, 50)
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
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed access logs: %v", err)
	}

	result, err := ListAccessLogs("", 0, 0)
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
