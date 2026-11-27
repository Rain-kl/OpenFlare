// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupNodePageDB(t testing.TB, count int) *gorm.DB {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "nodes.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open node test DB: %v", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatalf("get node SQL DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		db.SetDB(nil)
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close node DB: %v", err)
		}
	})
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate node migration directory")
	}
	directory := filepath.Join(filepath.Dir(filename), "..", "..", "..", "infra", "persistence", "migrator", "goose", "sqlite")
	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, os.DirFS(directory))
	if err != nil {
		t.Fatalf("create node migration provider: %v", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("migrate node test DB: %v", err)
	}
	db.SetDB(conn)
	nodes := make([]model.OpenFlareNode, count)
	for i := range nodes {
		nodes[i] = model.OpenFlareNode{NodeID: fmt.Sprintf("node-%05d", i+1), Name: fmt.Sprintf("Edge %05d", i+1), NodeType: "edge_node"}
	}
	if len(nodes) > 0 {
		if err := conn.CreateInBatches(nodes, 100).Error; err != nil {
			t.Fatalf("seed %d nodes: %v", count, err)
		}
	}
	return conn
}

func TestNodeListQueryValidation(t *testing.T) {
	tests := []struct {
		name  string
		query ListQuery
	}{
		{name: "negative page", query: ListQuery{Page: -1}},
		{name: "negative size", query: ListQuery{PageSize: -1}},
		{name: "overflow", query: ListQuery{Page: int(^uint(0) >> 1), PageSize: 100}},
		{name: "invalid status", query: ListQuery{Status: "healthy"}},
		{name: "invalid type", query: ListQuery{NodeType: "relay"}},
		{name: "long search", query: ListQuery{Search: strings.Repeat("x", 257)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := normalizeListQuery(test.query); err == nil {
				t.Errorf("normalizeListQuery(%+v) error=nil, want validation error", test.query)
			}
		})
	}
	query, err := normalizeListQuery(ListQuery{})
	if err != nil || query.Page != 1 || query.PageSize != 20 {
		t.Errorf("normalizeListQuery(empty)=(%+v,%v), want page1 size20", query, err)
	}
	query, err = normalizeListQuery(ListQuery{PageSize: 1000})
	if err != nil || query.PageSize != 100 {
		t.Errorf("normalizeListQuery(size1000)=(%+v,%v), want size100", query, err)
	}
}

func TestNodePageMonitoringUsesLatestScopedSamples(t *testing.T) {
	conn := setupNodePageDB(t, 3)
	logstore.InvalidateCache()
	t.Cleanup(logstore.InvalidateCache)
	ctx := context.Background()
	now := time.Now().UTC()
	nodeID := "node-00003"
	require.NoError(t, conn.Model(&model.OpenFlareNode{}).Where("node_id IN ?", []string{nodeID, "node-00002"}).
		Updates(map[string]any{"last_seen_at": now, "geo_name": "East Asia"}).Error)
	require.NoError(t, conn.Create(&model.OpenFlareNodeSystemProfile{NodeID: nodeID, OSName: "Debian 12"}).Error)
	require.NoError(t, conn.Create(&[]analyticsmodel.NodeMetricSnapshot{
		{ID: 1, NodeID: nodeID, CapturedAt: now.Add(-time.Minute), CPUUsagePercent: 9},
		{ID: 2, NodeID: nodeID, CapturedAt: now, CPUUsagePercent: 23, NetOut: 128},
		{ID: 3, NodeID: "node-00002", CapturedAt: now.Add(-10 * time.Minute), CPUUsagePercent: 99},
		{ID: 4, NodeID: "node-00001", CapturedAt: now, CPUUsagePercent: 99},
	}).Error)
	tasks := []model.OpenFlarePingTask{
		{Name: "Fourth", Weight: 4, Clients: model.StringSliceJSON{nodeID}},
		{Name: "Second", Weight: 2, Clients: model.StringSliceJSON{nodeID, "node-00002"}},
		{Name: "First", Weight: 1, Clients: model.StringSliceJSON{nodeID}},
		{Name: "Third", Weight: 3, Clients: model.StringSliceJSON{nodeID}},
	}
	require.NoError(t, conn.Create(&tasks).Error)
	for i := range nodePagePingSamples + 1 {
		require.NoError(t, conn.Create(&analyticsmodel.NodePingRecord{
			ID: uint64(i + 1), NodeID: nodeID, TaskID: uint64(tasks[2].ID),
			RecordedAt: now.Add(time.Duration(i-nodePagePingSamples) * time.Second), Value: int32(i - 1),
		}).Error)
	}
	require.NoError(t, conn.Create(&analyticsmodel.NodePingRecord{
		ID: 100, NodeID: "node-00002", TaskID: uint64(tasks[2].ID), RecordedAt: now, Value: 999,
	}).Error)

	page, err := ListPage(ctx, ListQuery{PageSize: 2, IncludeMonitoring: true})
	require.NoError(t, err)
	require.False(t, page.MonitoringUnavailable)
	require.Len(t, page.Results, 2)
	first, second := page.Results[0].Monitoring, page.Results[1].Monitoring
	require.Equal(t, "Debian 12", first.System)
	require.Equal(t, 23.0, first.Metric.CPUUsagePercent)
	require.EqualValues(t, 128, first.Metric.NetOut)
	require.Nil(t, second.Metric)
	require.Len(t, first.Ping, 3)
	require.Equal(t, "First", first.Ping[0].Name)
	require.Equal(t, "Second", first.Ping[1].Name)
	require.Equal(t, "Third", first.Ping[2].Name)
	require.Len(t, first.Ping[0].Samples, nodePagePingSamples)
	require.Zero(t, first.Ping[0].Samples[0].Value)
	require.Equal(t, nodePagePingSamples-1, first.Ping[0].Samples[nodePagePingSamples-1].Value)
	require.Len(t, second.Ping, 1)
	require.Empty(t, second.Ping[0].Samples)

	page, err = ListPage(ctx, ListQuery{Search: "debian"})
	require.NoError(t, err)
	require.Len(t, page.Results, 1)
	require.Equal(t, nodeID, page.Results[0].NodeID)
	require.Nil(t, page.Results[0].Monitoring)
	page, err = ListPage(ctx, ListQuery{Search: "East Asia"})
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
}

func TestNodePageTwentyThousandNodes(t *testing.T) {
	conn := setupNodePageDB(t, 20000)
	ctx := context.Background()
	if err := conn.Create(&model.OpenFlareApplyLog{NodeID: "node-20000", Result: "success", Message: "latest", Checksum: "new"}).Error; err != nil {
		t.Fatal(err)
	}
	recorder := &nodePageSQLRecorder{Interface: conn.Logger}
	conn.Logger = recorder
	page, err := ListPage(ctx, ListQuery{})
	if err != nil {
		t.Fatalf("ListPage(empty) error=%v", err)
	}
	if page.Total != 20000 || len(page.Results) != 20 || page.Results[0].ID != 20000 || page.Results[19].ID != 19981 {
		t.Fatalf("ListPage(empty) total=%d len=%d first=%d last=%d, want20000/20/20000/19981", page.Total, len(page.Results), page.Results[0].ID, page.Results[len(page.Results)-1].ID)
	}
	if page.Results[0].LatestApplyChecksum != "new" {
		t.Errorf("ListPage latest checksum=%q,want new", page.Results[0].LatestApplyChecksum)
	}
	// Query count stays constant O(1) per page: count + page rows + batch apply logs + batch node tags.
	if len(recorder.statements) != 4 || recorder.rows > 22 {
		t.Errorf("ListPage SQL queries=%v selectedRows=%d,want4 and <=22", recorder.statements, recorder.rows)
	}
	conn.Logger = recorder.Interface
	second, err := ListPage(ctx, ListQuery{Page: 2})
	if err != nil || len(second.Results) != 20 || second.Results[0].ID != 19980 {
		t.Fatalf("ListPage(page2)=(%+v,%v),want firstID19980", second, err)
	}
	capped, err := ListPage(ctx, ListQuery{PageSize: 1000})
	if err != nil || len(capped.Results) != 100 {
		t.Errorf("ListPage(size1000) results=%v,error=%v,want100", len(capped.Results), err)
	}
	empty, err := ListPage(ctx, ListQuery{Page: 1001})
	if err != nil || len(empty.Results) != 0 || empty.Results == nil || empty.Total != 20000 {
		t.Errorf("ListPage(beyondend)=(%+v,%v),want empty nonnil results total20000", empty, err)
	}
	detail, err := GetNode(ctx, 20000)
	if err != nil || detail.LatestApplyChecksum != "new" {
		t.Errorf("GetNode(20000)=(%+v,%v),want matching apply metadata", detail, err)
	}
	legacy, err := ListNodes(ctx)
	if err != nil || len(legacy) != 20000 {
		t.Errorf("legacy ListNodes len=%d,error=%v,want20000", len(legacy), err)
	}
}

func TestNodePageFiltersUseComputedStatus(t *testing.T) {
	conn := setupNodePageDB(t, 0)
	now := time.Now().UTC().Truncate(time.Second)
	cutoff := now.Add(-time.Minute)
	old := cutoff.Add(-time.Second)
	zero := time.Time{}
	quarantinedAt := now.Add(-time.Minute)
	nodes := []model.OpenFlareNode{
		{NodeID: "literal", Name: "EDGE 100%_!", NodeType: "edge_node", Status: "offline", LastSeenAt: &cutoff},
		{NodeID: "old", Name: "old", NodeType: "edge_node", Status: "online", LastSeenAt: &old},
		{NodeID: "pending", Name: "pending", NodeType: "edge_node"},
		{NodeID: "zero", Name: "zero", NodeType: "edge_node", LastSeenAt: &zero},
		{NodeID: "quarantined", Name: "quarantined", NodeType: "edge_node", Status: "quarantined", LastSeenAt: &cutoff, QuarantinedAt: &quarantinedAt},
	}
	if err := conn.Create(&nodes).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Model(&nodes[2]).Update("node_type", "").Error; err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		query model.OpenFlareNodeListQuery
		want  int64
	}{
		{query: model.OpenFlareNodeListQuery{Status: "online"}, want: 1},
		{query: model.OpenFlareNodeListQuery{Status: "offline"}, want: 1},
		{query: model.OpenFlareNodeListQuery{Status: "pending"}, want: 2},
		{query: model.OpenFlareNodeListQuery{Status: "quarantined"}, want: 1},
		{query: model.OpenFlareNodeListQuery{NodeType: "edge_node"}, want: 5},
		{query: model.OpenFlareNodeListQuery{Search: "100%_!"}, want: 1},
		{query: model.OpenFlareNodeListQuery{Search: "edge"}, want: 1},
		{query: model.OpenFlareNodeListQuery{Status: "pending", NodeType: "edge_node"}, want: 2},
	}
	for _, test := range cases {
		test.query.Page = 1
		test.query.PageSize = 20
		test.query.Now = now
		rows, total, err := repository.ListOpenFlareNodePage(context.Background(), test.query)
		if err != nil || total != test.want || int64(len(rows)) != test.want {
			t.Errorf("ListOpenFlareNodePage(%+v) len=%d,total=%d,error=%v,want%d", test.query, len(rows), total, err, test.want)
		}
		for _, row := range rows {
			if test.query.Status != "" && computeNodeStatusAt(&row, now) != test.query.Status {
				t.Errorf("status SQL/view mismatch query%+v node%+v", test.query, row)
			}
		}
	}
}

func TestNodePageUnicodeLiteralSearch(t *testing.T) {
	conn := setupNodePageDB(t, 0)
	nodes := []model.OpenFlareNode{
		{NodeID: "unicode-latin", Name: "Étage 100%_!", NodeType: "edge_node"},
		{NodeID: "unicode-cyrillic", Name: "Москва 100%_!", NodeType: "edge_node"},
		{NodeID: "unicode-decoy", Name: "Étage 100xx!", NodeType: "edge_node"},
	}
	if err := conn.Create(&nodes).Error; err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		search string
		wantID string
	}{
		{search: "Étage 100%_!", wantID: "unicode-latin"},
		{search: "ÉTAGE 100%_!", wantID: "unicode-latin"},
		{search: "Москва 100%_!", wantID: "unicode-cyrillic"},
		{search: "unicode-CYRILLIC", wantID: "unicode-cyrillic"},
	}
	for _, test := range cases {
		page, err := ListPage(context.Background(), ListQuery{Search: test.search})
		if err != nil {
			t.Fatalf("ListPage(search=%q) error=%v", test.search, err)
		}
		if page.Total != 1 || len(page.Results) != 1 {
			t.Errorf("ListPage(search=%q) total=%d,len=%d,wantone", test.search, page.Total, len(page.Results))
			continue
		}
		if page.Results[0].NodeID != test.wantID {
			t.Errorf("ListPage(search=%q) nodeID=%q,want%q", test.search, page.Results[0].NodeID, test.wantID)
		}
	}
}

func TestNodePageIndexes(t *testing.T) {
	conn := setupNodePageDB(t, 20000)
	queries := []struct {
		sql   string
		arg   any
		index string
	}{
		{sql: "SELECT count(*) FROM of_nodes WHERE node_type = ?", arg: "edge_node", index: "idx_of_nodes_type_id"},
		{sql: "SELECT count(*) FROM of_nodes WHERE last_seen_at >= ?", arg: time.Now().Add(-time.Minute), index: "idx_of_nodes_last_seen_id"},
	}
	for _, query := range queries {
		var rows []struct{ Detail string }
		if err := conn.Raw("EXPLAIN QUERY PLAN "+query.sql, query.arg).Scan(&rows).Error; err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			t.Log(row.Detail)
			found = found || strings.Contains(row.Detail, query.index)
		}
		if !found {
			t.Errorf("EXPLAIN %q=%v,want index%s", query.sql, rows, query.index)
		}
	}
}

func BenchmarkNodeListTwentyThousand(b *testing.B) {
	setupNodePageDB(b, 20000)
	b.Run("legacy_all", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := ListNodes(context.Background()); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("page20", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := ListPage(context.Background(), ListQuery{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

type nodePageSQLRecorder struct {
	logger.Interface
	statements []string
	rows       int64
}

func (recorder *nodePageSQLRecorder) Trace(_ context.Context, _ time.Time, query func() (string, int64), _ error) {
	sql, rows := query()
	recorder.statements = append(recorder.statements, sql)
	recorder.rows += rows
}
