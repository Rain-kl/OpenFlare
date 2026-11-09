// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"

	"github.com/google/go-cmp/cmp"
)

func TestNodeAccessLogHostsWildcardSQL(t *testing.T) {
	since := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	until := since.Add(time.Hour)
	tests := []struct {
		name   string
		filter NodeAccessLogFilter
		clause string
		args   []any
	}{
		{
			name:   "exact_hosts_remain_exact_and_normalized",
			filter: NodeAccessLogFilter{Hosts: []string{" API.Example.com ", "api.example.com", "", "www.example.com"}},
			clause: "lowerUTF8(trim(host)) IN (?, ?)",
			args:   []any{"api.example.com", "www.example.com"},
		},
		{
			name:   "wildcard_requires_nonempty_prefix",
			filter: NodeAccessLogFilter{Hosts: []string{" *.Example.com ", "*.example.com"}},
			clause: "lowerUTF8(trim(host)) LIKE ?",
			args:   []any{"_%.example.com"},
		},
		{
			name:   "nested_wildcard",
			filter: NodeAccessLogFilter{Hosts: []string{"*.s3.example.com"}},
			clause: "lowerUTF8(trim(host)) LIKE ?",
			args:   []any{"_%.s3.example.com"},
		},
		{
			name: "overlapping_scopes_are_one_group_with_other_filters",
			filter: NodeAccessLogFilter{
				NodeID: " n1 ", Host: "ignored", Hosts: []string{"api.example.com", "*.example.com", "*.s3.example.com"},
				StatusCode: 200, Since: since, Until: until,
			},
			clause: "node_id = ? AND (lowerUTF8(trim(host)) LIKE ? OR lowerUTF8(trim(host)) LIKE ? OR lowerUTF8(trim(host)) IN (?)) AND status_code = ? AND logged_at >= ? AND logged_at < ?",
			args:   []any{"n1", "_%.example.com", "_%.s3.example.com", "api.example.com", 200, since, until},
		},
		{
			name:   "wildcard_suffix_like_metacharacters_are_literal",
			filter: NodeAccessLogFilter{Hosts: []string{`*.a%_\'?.example.com`}},
			clause: "lowerUTF8(trim(host)) LIKE ?",
			args:   []any{`_%.a\%\_\\'?.example.com`},
		},
		{
			name:   "single_host_keeps_prefix_search",
			filter: NodeAccessLogFilter{Host: " *.ex%_ ", Hosts: []string{" "}},
			clause: "host LIKE ?",
			args:   []any{`*.ex\%\_%`},
		},
		{
			name:   "non_leading_wildcards_are_exact",
			filter: NodeAccessLogFilter{Hosts: []string{"api.*.example.com", "*", "*."}},
			clause: "lowerUTF8(trim(host)) IN (?, ?, ?)",
			args:   []any{"api.*.example.com", "*", "*."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clause, args := buildNodeAccessLogFilterClause(tt.filter)
			if clause != tt.clause {
				t.Errorf("buildNodeAccessLogFilterClause(%+v) SQL = %q, want %q", tt.filter, clause, tt.clause)
			}
			if diff := cmp.Diff(tt.args, args); diff != "" {
				t.Errorf("buildNodeAccessLogFilterClause(%+v) args mismatch (-want +got):\n%s", tt.filter, diff)
			}
		})
	}
}

// wildcardQueryConn captures both single-row and multi-row query entry points.
type wildcardQueryConn struct{ mockConn }

func (m *wildcardQueryConn) QueryRow(_ context.Context, query string, args ...any) driver.Row {
	m.queries = append(m.queries, query)
	m.queryArgs = append(m.queryArgs, args)
	return wildcardCountRow{}
}

type wildcardCountRow struct{ driver.Row }

func (wildcardCountRow) Scan(dest ...any) error {
	for _, value := range dest {
		*value.(*uint64) = 0
	}
	return nil
}

// TestWildcardQueryEntryPoints protects host filtering in count and bucket queries.
func TestWildcardQueryEntryPoints(t *testing.T) {
	conn := &wildcardQueryConn{}
	previous := db.ChConn
	db.SetChConnForTest(conn)
	t.Cleanup(func() { db.SetChConnForTest(previous) })
	ctx := context.Background()
	filter := NodeAccessLogFilter{Hosts: []string{"*.Example.com", "api.example.com"}}
	if _, _, _, err := CountNodeAccessLogs(ctx, filter); err != nil {
		t.Fatal(err)
	}
	if _, err := BucketAggregatesNodeAccessLogs(ctx, filter, 300); err != nil {
		t.Fatal(err)
	}
	if _, err := CountBucketAggregatesNodeAccessLogs(ctx, filter, 300); err != nil {
		t.Fatal(err)
	}
	if len(conn.queries) != 3 {
		t.Fatalf("count/bucket query calls = %d, want 3", len(conn.queries))
	}
	for i, query := range conn.queries {
		if !strings.Contains(query, "WHERE (lowerUTF8(trim(host)) LIKE ? OR lowerUTF8(trim(host)) IN (?))") {
			t.Errorf("count/bucket SQL = %q, want grouped wildcard and exact host predicates", query)
		}
		if diff := cmp.Diff([]any{"_%.example.com", "api.example.com"}, conn.queryArgs[i]); diff != "" {
			t.Errorf("count/bucket args mismatch (-want +got):\n%s", diff)
		}
	}
}
