// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/stretchr/testify/require"
)

func TestBuildNodeAccessLogRecordsPreservesBytesSent(t *testing.T) {
	reportedAt := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)

	records, err := buildNodeAccessLogRecords(context.Background(), "node-a", []protocol.NodeAccessLog{
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

func TestBuildNodeAccessLogRecordsPreservesWAFDecisionAndStripsQuery(t *testing.T) {
	audit := &protocol.WAFAudit{
		WAFAuditSummary: protocol.WAFAuditSummary{
			Action: "blocked", FlowName: "Primary", FlowID: 2, FlowNodeID: "attack-protection", AttackGroupID: 3,
			Rules: []protocol.WAFRuleMatch{{ID: 934180, Category: "template_injection", Message: "SSTI Attack", Level: "strict"}},
		},
		Request: &protocol.WAFRequestPreview{Message: "GET /search?q=term HTTP/1.1\r\nhost: example.test\r\n\r\n"},
	}
	records, err := buildNodeAccessLogRecords(context.Background(), "edge-a", []protocol.NodeAccessLog{
		{Path: "/search?q=private", WAF: audit, StatusCode: 403},
	}, nil, time.Now())
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "blocked", records[0].WAFAction)
	require.Equal(t, "/search", records[0].Path)
	var decoded protocol.WAFAudit
	require.NoError(t, json.Unmarshal([]byte(records[0].WAFAudit), &decoded))
	require.Equal(t, audit, &decoded)

	message := strings.Repeat("x", wafRequestPreviewBytes-1) + "文"
	audit.Request.Message = message
	_, encoded, err := serializeWAFAudit(audit)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(encoded), &decoded))
	require.Equal(t, message[:wafRequestPreviewBytes-1], decoded.Request.Message)
	require.True(t, decoded.Request.Truncated)
	require.Equal(t, message, audit.Request.Message)
}
