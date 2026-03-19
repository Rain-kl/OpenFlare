package service

import (
	"openflare/common"
	"openflare/model"
	"testing"
	"time"
)

func TestCleanupDatabaseObservabilityDeletesTargetedRows(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now().UTC()
	if err := model.DB.Create(&model.NodeMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-10 * 24 * time.Hour),
		CPUUsagePercent: 10,
	}).Error; err != nil {
		t.Fatalf("seed old metric snapshot: %v", err)
	}
	if err := model.DB.Create(&model.NodeMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-12 * time.Hour),
		CPUUsagePercent: 20,
	}).Error; err != nil {
		t.Fatalf("seed recent metric snapshot: %v", err)
	}

	retentionDays := 7
	result, err := CleanupDatabaseObservability(DatabaseCleanupInput{
		Target:        DatabaseCleanupTargetMetricSnapshots,
		RetentionDays: &retentionDays,
	})
	if err != nil {
		t.Fatalf("CleanupDatabaseObservability failed: %v", err)
	}
	if result.DeleteAll {
		t.Fatal("expected retention cleanup instead of delete_all")
	}
	if result.DeletedCount != 1 {
		t.Fatalf("expected 1 deleted row, got %+v", result)
	}

	rows, err := model.ListMetricSnapshotsSince(time.Time{})
	if err != nil {
		t.Fatalf("ListMetricSnapshotsSince failed: %v", err)
	}
	if len(rows) != 1 || rows[0].CPUUsagePercent != 20 {
		t.Fatalf("unexpected remaining metric snapshots: %+v", rows)
	}
}

func TestCleanupDatabaseObservabilityDeletesAllRowsWhenRetentionMissing(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now().UTC()
	if err := model.DB.Create(&model.NodeAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-3 * time.Hour),
		RemoteAddr: "203.0.113.1",
		Host:       "example.com",
		Path:       "/one",
		StatusCode: 200,
	}).Error; err != nil {
		t.Fatalf("seed first access log: %v", err)
	}
	if err := model.DB.Create(&model.NodeAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-2 * time.Hour),
		RemoteAddr: "203.0.113.2",
		Host:       "example.com",
		Path:       "/two",
		StatusCode: 502,
	}).Error; err != nil {
		t.Fatalf("seed second access log: %v", err)
	}

	result, err := CleanupDatabaseObservability(DatabaseCleanupInput{
		Target: DatabaseCleanupTargetAccessLogs,
	})
	if err != nil {
		t.Fatalf("CleanupDatabaseObservability failed: %v", err)
	}
	if !result.DeleteAll || result.DeletedCount != 2 {
		t.Fatalf("unexpected delete-all result: %+v", result)
	}

	rows, err := model.ListNodeAccessLogs(model.NodeAccessLogQuery{Page: 0, PageSize: 10})
	if err != nil {
		t.Fatalf("ListNodeAccessLogs failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected all access logs deleted, got %+v", rows)
	}
}

func TestRunDatabaseAutoCleanupOnceDeletesAllObservabilityTargets(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now().UTC()
	if err := model.DB.Create(&model.NodeAccessLog{
		NodeID:     "node-a",
		LoggedAt:   now.Add(-48 * time.Hour),
		RemoteAddr: "203.0.113.10",
		Host:       "example.com",
		Path:       "/access",
		StatusCode: 200,
	}).Error; err != nil {
		t.Fatalf("seed access log: %v", err)
	}
	if err := model.DB.Create(&model.NodeMetricSnapshot{
		NodeID:          "node-a",
		CapturedAt:      now.Add(-48 * time.Hour),
		CPUUsagePercent: 10,
	}).Error; err != nil {
		t.Fatalf("seed metric snapshot: %v", err)
	}
	if err := model.DB.Create(&model.NodeRequestReport{
		NodeID:          "node-a",
		WindowStartedAt: now.Add(-49 * time.Hour),
		WindowEndedAt:   now.Add(-48 * time.Hour),
		RequestCount:    15,
	}).Error; err != nil {
		t.Fatalf("seed request report: %v", err)
	}

	previousEnabled := common.DatabaseAutoCleanupEnabled
	previousRetentionDays := common.DatabaseAutoCleanupRetentionDays
	common.DatabaseAutoCleanupEnabled = true
	common.DatabaseAutoCleanupRetentionDays = 1
	t.Cleanup(func() {
		common.DatabaseAutoCleanupEnabled = previousEnabled
		common.DatabaseAutoCleanupRetentionDays = previousRetentionDays
	})

	summary, err := RunDatabaseAutoCleanupOnce(now)
	if err != nil {
		t.Fatalf("RunDatabaseAutoCleanupOnce failed: %v", err)
	}
	if summary == nil || len(summary.Results) != 3 {
		t.Fatalf("unexpected auto cleanup summary: %+v", summary)
	}

	accessLogs, err := model.ListNodeAccessLogs(model.NodeAccessLogQuery{Page: 0, PageSize: 10})
	if err != nil {
		t.Fatalf("ListNodeAccessLogs failed: %v", err)
	}
	if len(accessLogs) != 0 {
		t.Fatalf("expected auto cleanup to delete access logs, got %+v", accessLogs)
	}
	metricSnapshots, err := model.ListMetricSnapshotsSince(time.Time{})
	if err != nil {
		t.Fatalf("ListMetricSnapshotsSince failed: %v", err)
	}
	if len(metricSnapshots) != 0 {
		t.Fatalf("expected auto cleanup to delete metric snapshots, got %+v", metricSnapshots)
	}
	requestReports, err := model.ListRequestReportsSince(time.Time{})
	if err != nil {
		t.Fatalf("ListRequestReportsSince failed: %v", err)
	}
	if len(requestReports) != 0 {
		t.Fatalf("expected auto cleanup to delete request reports, got %+v", requestReports)
	}
}
