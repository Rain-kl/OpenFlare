// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Rain-kl/Wavelet/internal/db"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBrowserName(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{name: "chrome", ua: "Mozilla/5.0 Chrome/120.0.0.0", want: "Chrome"},
		{name: "firefox", ua: "Mozilla/5.0 Firefox/121.0", want: "Firefox"},
		{name: "safari", ua: "Mozilla/5.0 Safari/605.1.15", want: "Safari"},
		{name: "edge", ua: "Mozilla/5.0 Edg/120.0.0.0", want: "Edge"},
		{name: "wechat", ua: "MicroMessenger/8.0", want: "WeChat"},
		{name: "postman", ua: "PostmanRuntime/7.36.0", want: "Postman"},
		{name: "other", ua: "curl/8.0", want: "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseBrowserName(tt.ua))
		})
	}
}

func TestBuildUserAccessLogFilterClause_EmptyUserIDs(t *testing.T) {
	_, _, ok := buildUserAccessLogFilterClause(AccessLogFilter{UserIDs: []uint64{}})
	assert.False(t, ok)
}

func TestCountAccessLogs_EmptyUserIDs(t *testing.T) {
	count, err := CountAccessLogs(context.Background(), AccessLogFilter{UserIDs: []uint64{}})
	require.NoError(t, err)
	assert.Equal(t, uint64(0), count)
}

func TestListAccessLogs_EmptyUserIDs(t *testing.T) {
	logs, total, err := ListAccessLogs(context.Background(), AccessLogFilter{UserIDs: []uint64{}}, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), total)
	assert.Empty(t, logs)
}

func TestBatchInsert_Empty(t *testing.T) {
	err := BatchInsert(context.Background(), nil)
	require.NoError(t, err)
}

func TestBatchInsert_UsesModelBatchSQL(t *testing.T) {
	ctx := context.Background()
	mockBatch := &mockBatch{}
	mockConn := &mockConn{
		batch:      mockBatch,
		batchQuery: analyticsmodel.UserAccessLog{}.BatchInsertSQL(),
	}
	db.SetChConnForTest(mockConn)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	createdAt := time.Now().UTC()
	err := BatchInsert(ctx, []analyticsmodel.UserAccessLog{
		{
			ID:        1,
			UserID:    42,
			Path:      "/api/v1/test",
			Method:    "GET",
			IP:        "127.0.0.1",
			UserAgent: "test-agent",
			Headers:   "{}",
			Status:    200,
			Latency:   12,
			CreatedAt: createdAt,
		},
	})
	require.NoError(t, err)
	assert.True(t, mockConn.prepareCalled)
	assert.Equal(t, analyticsmodel.UserAccessLog{}.BatchInsertSQL(), mockConn.preparedQuery)
	assert.True(t, mockBatch.sendCalled)
	require.Len(t, mockBatch.rows, 1)
	assert.Equal(t, uint64(42), mockBatch.rows[0][1])
}

type mockConn struct {
	batch         driver.Batch
	batchQuery    string
	prepareCalled bool
	preparedQuery string
	queries       []string
	queryArgs     [][]any
	queryFn       func(ctx context.Context, query string, args ...any) (driver.Rows, error)
}

func (m *mockConn) Contributors() []string { return nil }

func (m *mockConn) ServerVersion() (*driver.ServerVersion, error) { return nil, nil }

func (m *mockConn) Select(_ context.Context, _ any, _ string, _ ...any) error { return nil }

func (m *mockConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	m.queries = append(m.queries, query)
	m.queryArgs = append(m.queryArgs, args)
	if m.queryFn != nil {
		return m.queryFn(ctx, query, args...)
	}
	return &mockRows{}, nil
}

func (m *mockConn) QueryRow(_ context.Context, _ string, _ ...any) driver.Row { return nil }

func (m *mockConn) PrepareBatch(_ context.Context, query string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	m.prepareCalled = true
	m.preparedQuery = query
	return m.batch, nil
}

func (m *mockConn) Exec(_ context.Context, _ string, _ ...any) error { return nil }

func (m *mockConn) AsyncInsert(_ context.Context, _ string, _ bool, _ ...any) error { return nil }

func (m *mockConn) Ping(_ context.Context) error { return nil }

func (m *mockConn) Stats() driver.Stats { return driver.Stats{} }

func (m *mockConn) Close() error { return nil }

type mockBatch struct {
	rows       [][]any
	sendCalled bool
}

func (m *mockBatch) Abort() error { return nil }

func (m *mockBatch) Append(v ...any) error {
	m.rows = append(m.rows, v)
	return nil
}

func (m *mockBatch) AppendStruct(_ any) error { return nil }

func (m *mockBatch) Column(_ int) driver.BatchColumn { return nil }

func (m *mockBatch) Flush() error { return nil }

func (m *mockBatch) Send() error {
	m.sendCalled = true
	return nil
}

func (m *mockBatch) IsSent() bool { return m.sendCalled }

func (m *mockBatch) Rows() int { return len(m.rows) }

func (m *mockBatch) Columns() []column.Interface { return nil }

func (m *mockBatch) Close() error { return nil }

// mockRows is an empty driver.Rows implementation for query-path unit tests.
type mockRows struct {
	index int
	data  [][]any
	err   error
}

func (m *mockRows) Next() bool {
	if m.err != nil {
		return false
	}
	if m.index >= len(m.data) {
		return false
	}
	m.index++
	return true
}

func (m *mockRows) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	if m.index == 0 || m.index > len(m.data) {
		return nil
	}
	row := m.data[m.index-1]
	for i := range dest {
		if i >= len(row) {
			break
		}
		if err := assignMockScanValue(dest[i], row[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockRows) ScanStruct(_ any) error { return nil }

func (m *mockRows) ColumnTypes() []driver.ColumnType { return nil }

func (m *mockRows) Totals(_ ...any) error { return nil }

func (m *mockRows) Columns() []string { return nil }

func (m *mockRows) Close() error { return nil }

func (m *mockRows) Err() error { return m.err }

func (m *mockRows) HasData() bool { return len(m.data) > 0 }

func assignMockScanValue(dest any, value any) error {
	switch d := dest.(type) {
	case *string:
		if v, ok := value.(string); ok {
			*d = v
		}
	case *uint64:
		switch v := value.(type) {
		case uint64:
			*d = v
		case int:
			*d = uint64(v)
		case int64:
			*d = uint64(v)
		}
	case *int64:
		switch v := value.(type) {
		case int64:
			*d = v
		case int:
			*d = int64(v)
		case uint64:
			*d = int64(v)
		}
	case *float64:
		switch v := value.(type) {
		case float64:
			*d = v
		case float32:
			*d = float64(v)
		case int:
			*d = float64(v)
		}
	case *time.Time:
		if v, ok := value.(time.Time); ok {
			*d = v
		}
	}
	return nil
}