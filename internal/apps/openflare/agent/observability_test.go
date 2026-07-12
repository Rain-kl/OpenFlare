// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"testing"
	"time"
)

func TestBuildNodeAccessLogRecordsPreservesBytesSent(t *testing.T) {
	reportedAt := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)

	records, err := buildNodeAccessLogRecords("node-a", []NodeAccessLog{
		{
			LoggedAtUnix: reportedAt.Unix(),
			RemoteAddr:   "203.0.113.10",
			Host:         "api.example.com",
			Path:         "/v1/ping",
			StatusCode:   200,
			BytesSent:    4096,
		},
	}, nil, reportedAt)
	if err != nil {
		t.Fatalf("buildNodeAccessLogRecords() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one access log record, got %d", len(records))
	}
	if records[0].BytesSent != 4096 {
		t.Fatalf("BytesSent = %d, want 4096", records[0].BytesSent)
	}
}
