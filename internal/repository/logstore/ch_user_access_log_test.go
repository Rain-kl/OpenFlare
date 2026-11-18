// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCHBuildUserAccessLogFilterClause_EmptyUserIDs(t *testing.T) {
	_, _, ok := buildCHUserAccessLogFilterClause(analyticsmodel.AccessLogFilter{UserIDs: []uint64{}})
	assert.False(t, ok)
}

func TestCHBuildNodeAccessLogFilterClause_StatusCode(t *testing.T) {
	clause, args := buildCHNodeAccessLogFilterClause(analyticsmodel.NodeAccessLogFilter{StatusCode: 404})
	assert.Equal(t, "status_code = ?", clause)
	assert.Equal(t, []any{404}, args)

	clause, args = buildCHNodeAccessLogFilterClause(analyticsmodel.NodeAccessLogFilter{})
	assert.Equal(t, "1", clause)
	assert.Nil(t, args)
}

func TestCHCountAccessLogs_EmptyUserIDs(t *testing.T) {
	s := newClickHouseUserAccessLogStore()
	count, err := s.Count(context.Background(), analyticsmodel.AccessLogFilter{UserIDs: []uint64{}})
	require.NoError(t, err)
	assert.Equal(t, uint64(0), count)
}

func TestCHListAccessLogs_EmptyUserIDs(t *testing.T) {
	s := newClickHouseUserAccessLogStore()
	logs, total, err := s.List(context.Background(), analyticsmodel.AccessLogFilter{UserIDs: []uint64{}}, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), total)
	assert.Empty(t, logs)
}

func TestCHUserAccessLogBatchInsert_Empty(t *testing.T) {
	s := newClickHouseUserAccessLogStore()
	err := s.BatchInsert(context.Background(), nil)
	require.NoError(t, err)
}

func TestCHUserAccessLogBatchInsert_UsesModelBatchSQL(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	ctx := context.Background()
	mockBatch := &chMockBatch{}
	mockConn := &chMockConn{
		batch:      mockBatch,
		batchQuery: analyticsmodel.UserAccessLog{}.BatchInsertSQL(),
	}
	db.SetChConnForTest(mockConn)
	t.Cleanup(func() { db.SetChConnForTest(nil) })

	s := newClickHouseUserAccessLogStore()
	createdAt := time.Now().UTC()
	err := s.BatchInsert(ctx, []analyticsmodel.UserAccessLog{
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

type chMockConn struct {
	batch         driver.Batch
	batchQuery    string
	prepareCalled bool
	preparedQuery string
	queries       []string
	queryArgs     [][]any
	queryFn       func(ctx context.Context, query string, args ...any) (driver.Rows, error)
}

func (m *chMockConn) Contributors() []string { return nil }

func (m *chMockConn) ServerVersion() (*driver.ServerVersion, error) { return nil, nil }

func (m *chMockConn) Select(_ context.Context, _ any, _ string, _ ...any) error { return nil }

func (m *chMockConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	m.queries = append(m.queries, query)
	m.queryArgs = append(m.queryArgs, args)
	if m.queryFn != nil {
		return m.queryFn(ctx, query, args...)
	}
	return &chMockRows{}, nil
}

func (m *chMockConn) QueryRow(_ context.Context, _ string, _ ...any) driver.Row { return nil }

func (m *chMockConn) PrepareBatch(_ context.Context, query string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	m.prepareCalled = true
	m.preparedQuery = query
	return m.batch, nil
}

func (m *chMockConn) Exec(_ context.Context, _ string, _ ...any) error { return nil }

func (m *chMockConn) AsyncInsert(_ context.Context, _ string, _ bool, _ ...any) error { return nil }

func (m *chMockConn) InsertFormat(_ context.Context, _ string, _ string, _ io.Reader) error {
	return nil
}

func (m *chMockConn) QueryFormat(_ context.Context, _ string, _ string, _ ...any) (io.ReadCloser, error) {
	return nil, nil
}

func (m *chMockConn) Ping(_ context.Context) error { return nil }

func (m *chMockConn) Stats() driver.Stats { return driver.Stats{} }

func (m *chMockConn) Close() error { return nil }

type chMockBatch struct {
	rows       [][]any
	sendCalled bool
}

func (m *chMockBatch) Abort() error { return nil }

func (m *chMockBatch) Append(v ...any) error {
	m.rows = append(m.rows, v)
	return nil
}

func (m *chMockBatch) AppendStruct(_ any) error { return nil }

func (m *chMockBatch) Column(_ int) driver.BatchColumn { return nil }

func (m *chMockBatch) Flush() error { return nil }

func (m *chMockBatch) Send() error {
	m.sendCalled = true
	return nil
}

func (m *chMockBatch) IsSent() bool { return m.sendCalled }

func (m *chMockBatch) Rows() int { return len(m.rows) }

func (m *chMockBatch) Columns() []column.Interface { return nil }

func (m *chMockBatch) Close() error { return nil }

type chMockRows struct {
	index int
	data  [][]any
	err   error
}

func (m *chMockRows) Next() bool {
	if m.err != nil {
		return false
	}
	if m.index >= len(m.data) {
		return false
	}
	m.index++
	return true
}

func (m *chMockRows) Scan(dest ...any) error {
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
		if err := assignCHMockScanValue(dest[i], row[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *chMockRows) ScanStruct(_ any) error { return nil }

func (m *chMockRows) ColumnTypes() []driver.ColumnType { return nil }

func (m *chMockRows) Totals(_ ...any) error { return nil }

func (m *chMockRows) Columns() []string { return nil }

func (m *chMockRows) Close() error { return nil }

func (m *chMockRows) Err() error { return m.err }

func (m *chMockRows) HasData() bool { return len(m.data) > 0 }

func assignCHMockScanValue(dest, value any) error {
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
