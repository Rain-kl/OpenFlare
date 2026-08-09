// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
)

func newTestGormStore(t *testing.T) *gormLogStore {
	t.Helper()
	return newTestGormStoreWithModels(t, &analyticsmodel.NodeAccessLog{})
}

func TestGormBatchInsertAndCount(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	now := time.Now()
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: now, RemoteAddr: "1.1.1.1", StatusCode: 200, BytesSent: 100},
		{ID: 2, NodeID: "n1", LoggedAt: now, RemoteAddr: "2.2.2.2", StatusCode: 500, BytesSent: 200},
	}
	if err := s.BatchInsertNodeAccessLogs(context.Background(), rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	total, uniqIP, bytesSent, err := s.Count(context.Background(), model.OpenFlareAccessLogQuery{NodeID: "n1"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 || uniqIP != 2 || bytesSent != 300 {
		t.Fatalf("count got total=%d uniq=%d bytes=%d", total, uniqIP, bytesSent)
	}
}

// testLogCaptureWriter 捕获 GORM logger 输出（logger.Writer 需实现 Printf）。
type testLogCaptureWriter struct {
	buf *bytes.Buffer
}

func (w testLogCaptureWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w testLogCaptureWriter) Printf(format string, args ...any) {
	fmt.Fprintf(w.buf, format, args...)
}

// TestGormBatchInsertFillsZeroIDs 回归测试：PG 日志表 id 为 NOT NULL 且无默认值，GORM 对零值
// uint64 主键（视为自增）会省略 id 列，导致 PG 插入报 not-null 违例（SQLSTATE 23502）。
// 验证 BatchInsert* 落库前为零 ID 行生成雪花 ID：INSERT 语句必须包含 id 列且回填非零、唯一 ID。
func TestGormBatchInsertFillsZeroIDs(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	var buf bytes.Buffer
	dsn := fmt.Sprintf("file:logstore-idtest-%d?mode=memory&cache=shared", atomic.AddInt64(&testGormStoreSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.New(testLogCaptureWriter{&buf}, logger.Config{LogLevel: logger.Info, Colorful: false}),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&analyticsmodel.NodeAccessLog{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	s := newGormStore(db)

	now := time.Now()
	rows := []analyticsmodel.NodeAccessLog{
		{NodeID: "n1", LoggedAt: now, RemoteAddr: "1.1.1.1", StatusCode: 200},
		{NodeID: "n1", LoggedAt: now.Add(time.Second), RemoteAddr: "2.2.2.2", StatusCode: 500},
	}
	if err := s.BatchInsertNodeAccessLogs(context.Background(), rows); err != nil {
		t.Fatalf("insert with zero ids: %v", err)
	}
	if rows[0].ID == 0 || rows[1].ID == 0 {
		t.Fatalf("zero ids not filled: %+v %+v", rows[0], rows[1])
	}
	if rows[0].ID == rows[1].ID {
		t.Fatalf("ids not unique: %d == %d", rows[0].ID, rows[1].ID)
	}
	// 捕获日志含 CREATE TABLE 等其它语句，仅校验 INSERT 语句的列清单（而非 RETURNING 子句，
	// 后者无论是否省略列都含 id）。
	var insertStmt string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "INSERT INTO") {
			insertStmt = line
			break
		}
	}
	start := strings.Index(insertStmt, "(")
	end := strings.Index(insertStmt, ") VALUES")
	if start < 0 || end <= start {
		t.Fatalf("cannot parse insert statement: %s", insertStmt)
	}
	if columns := insertStmt[start+1 : end]; !strings.Contains(columns, "`id`") {
		t.Fatalf("insert SQL omits id column: %s", insertStmt)
	}
}

// TestGormNodeAccessLogPagination 验证节点访问日志分页与 CH ListNodeAccessLogs 一致（0-based）：
// Page=1 size=2 → OFFSET 2；Page=0 视为第 0 页；PageSize<=0 时与 CH 一致不分页（返回全部匹配行）。
func TestGormNodeAccessLogPagination(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: base.Add(time.Minute), RemoteAddr: "1.1.1.1", StatusCode: 200},
		{ID: 2, NodeID: "n1", LoggedAt: base.Add(2 * time.Minute), RemoteAddr: "2.2.2.2", StatusCode: 200},
		{ID: 3, NodeID: "n1", LoggedAt: base.Add(3 * time.Minute), RemoteAddr: "3.3.3.3", StatusCode: 200},
		{ID: 4, NodeID: "n1", LoggedAt: base.Add(4 * time.Minute), RemoteAddr: "4.4.4.4", StatusCode: 200},
		{ID: 5, NodeID: "n1", LoggedAt: base.Add(5 * time.Minute), RemoteAddr: "5.5.5.5", StatusCode: 200},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// logged_at DESC → [5,4,3,2,1]。
	page1, err := s.List(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1", Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) != 2 || page1[0].ID != 3 || page1[1].ID != 2 {
		t.Fatalf("page1 size2 got %+v, want [3,2]", page1)
	}
	page0, err := s.List(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1", Page: 0, PageSize: 2})
	if err != nil {
		t.Fatalf("list page 0: %v", err)
	}
	if len(page0) != 2 || page0[0].ID != 5 || page0[1].ID != 4 {
		t.Fatalf("page0 size2 got %+v, want [5,4]", page0)
	}
	all, err := s.List(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1"})
	if err != nil {
		t.Fatalf("list unpaged: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("unpaged got %d rows, want 5", len(all))
	}
}

// TestGormNodeAccessLogDistinctIPExcludesEmpty 验证 distinct IP 计数排除空 remote_addr，
// 与 CH uniqExactIf(remote_addr, remote_addr != ”) 及旧 memory store 一致
// （Count/TrafficSummary/NodeAggregates 三处口径统一）。
func TestGormNodeAccessLogDistinctIPExcludesEmpty(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	now := time.Now()
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: now, RemoteAddr: "1.1.1.1", StatusCode: 200, BytesSent: 10},
		{ID: 2, NodeID: "n1", LoggedAt: now, RemoteAddr: "1.1.1.1", StatusCode: 500, BytesSent: 20},
		{ID: 3, NodeID: "n1", LoggedAt: now, RemoteAddr: "", StatusCode: 200, BytesSent: 30},
		{ID: 4, NodeID: "n2", LoggedAt: now, RemoteAddr: "2.2.2.2", StatusCode: 200, BytesSent: 40},
		{ID: 5, NodeID: "n2", LoggedAt: now, RemoteAddr: "", StatusCode: 200, BytesSent: 50},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	q := model.OpenFlareAccessLogQuery{NodeID: "n1"}
	total, uniqIP, bytesSent, err := s.Count(ctx, q)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	// 空串行计入 total 与 bytes_sent，但不计入 distinct IP。
	if total != 3 || uniqIP != 1 || bytesSent != 60 {
		t.Fatalf("count got total=%d uniq=%d bytes=%d, want 3/1/60", total, uniqIP, bytesSent)
	}
	summary, err := s.TrafficSummary(ctx, q)
	if err != nil {
		t.Fatalf("traffic summary: %v", err)
	}
	if summary.RequestCount != 3 || summary.UniqueIPCount != 1 || summary.ErrorCount != 1 {
		t.Fatalf("traffic summary got %+v, want requests=3 uniq=1 errors=1", summary)
	}
	agg, err := s.NodeAggregates(ctx, q)
	if err != nil {
		t.Fatalf("node aggregates: %v", err)
	}
	if len(agg) != 1 || agg[0].NodeID != "n1" || agg[0].RequestCount != 3 || agg[0].UniqueIPCount != 1 {
		t.Fatalf("node aggregates got %+v", agg)
	}
}

// TestGormNodeAggregatesExcludeEmptyNodeID 验证空 node_id 行不参与 NodeAggregates 分组、
// 也不计入 TrafficSummary.node_count（对齐 CH uniqExactIf(node_id, node_id != ”)）。
func TestGormNodeAggregatesExcludeEmptyNodeID(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	now := time.Now()
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: now, RemoteAddr: "1.1.1.1", StatusCode: 200},
		{ID: 2, NodeID: "n2", LoggedAt: now, RemoteAddr: "2.2.2.2", StatusCode: 200},
		{ID: 3, NodeID: "", LoggedAt: now, RemoteAddr: "3.3.3.3", StatusCode: 200},
		{ID: 4, NodeID: "", LoggedAt: now, RemoteAddr: "", StatusCode: 500},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	q := model.OpenFlareAccessLogQuery{}
	summary, err := s.TrafficSummary(ctx, q)
	if err != nil {
		t.Fatalf("traffic summary: %v", err)
	}
	if summary.RequestCount != 4 || summary.NodeCount != 2 {
		t.Fatalf("traffic summary got %+v, want requests=4 node_count=2", summary)
	}
	agg, err := s.NodeAggregates(ctx, q)
	if err != nil {
		t.Fatalf("node aggregates: %v", err)
	}
	if len(agg) != 2 {
		t.Fatalf("node aggregates want 2 nodes (empty node_id excluded), got %+v", agg)
	}
}

// testGormStoreSeq 保证每个测试获得独立的共享内存库（cache=shared 下同名 DSN 会复用同一库，
// 导致跨测试 id 冲突）。
var testGormStoreSeq int64

func newTestGormStoreWithModels(t *testing.T, models ...any) *gormLogStore {
	t.Helper()
	dsn := fmt.Sprintf("file:logstore-test-%d?mode=memory&cache=shared", atomic.AddInt64(&testGormStoreSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return newGormStore(db)
}

// TestGormObservabilityInsertList 覆盖 4 张可观测表：flush 写入、查询、删除、
// 迁移分页读取，以及写入入口的 hook 入队路径。
func TestGormObservabilityInsertList(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	SetObservabilityHooks(ObservabilityHooks{})
	defer func() { SetObservabilityHooks(ObservabilityHooks{}) }()

	s := newTestGormStoreWithModels(t,
		&analyticsmodel.NodeMetricSnapshot{},
		&analyticsmodel.NodeEdgeHealth{},
		&analyticsmodel.NodeObsFrps{},
		&analyticsmodel.NodeObsFrpc{},
	)
	ctx := context.Background()
	now := time.Now()

	// 写入入口：hook 入队（metric/edge/frps/frpc 四个 hook 均触发），不直接落库。
	var (
		hookedMetric *analyticsmodel.NodeMetricSnapshot
		hookedEdge   *analyticsmodel.NodeEdgeHealth
		hookedFrps   *analyticsmodel.NodeObsFrps
		hookedFrpc   *analyticsmodel.NodeObsFrpc
	)
	SetObservabilityHooks(ObservabilityHooks{
		QueueMetricSnapshot: func(row analyticsmodel.NodeMetricSnapshot) { hookedMetric = &row },
		QueueEdgeHealth:     func(row analyticsmodel.NodeEdgeHealth) { hookedEdge = &row },
		QueueNodeObsFrps:    func(row analyticsmodel.NodeObsFrps) { hookedFrps = &row },
		QueueNodeObsFrpc:    func(row analyticsmodel.NodeObsFrpc) { hookedFrpc = &row },
	})
	metricRec := &model.OpenFlareMetricSnapshot{ID: 99, NodeID: "n1", CapturedAt: now, CPUUsagePercent: 12.5, MemoryUsedBytes: 1024}
	if err := s.InsertMetricSnapshot(ctx, metricRec); err != nil {
		t.Fatalf("insert metric entry: %v", err)
	}
	if hookedMetric == nil || hookedMetric.NodeID != "n1" || hookedMetric.CPUUsagePercent != 12.5 || hookedMetric.ID != 99 {
		t.Fatalf("metric hook not fired with converted row: %+v", hookedMetric)
	}
	edgeRec := &model.OpenFlareEdgeHealth{ID: 98, NodeID: "n1", CapturedAt: now, Status: "healthy", Connections: 5}
	if err := s.InsertEdgeHealth(ctx, edgeRec); err != nil {
		t.Fatalf("insert edge entry: %v", err)
	}
	if hookedEdge == nil || hookedEdge.ID != 98 || hookedEdge.Status != "healthy" || hookedEdge.Connections != 5 {
		t.Fatalf("edge hook not fired with converted row: %+v", hookedEdge)
	}
	frpsRec := &model.OpenFlareNodeObservationFrps{ID: 97, NodeID: "n1", CapturedAt: now, FrpsConnections: 3, FrpsProxyCount: 2}
	if err := s.InsertNodeObservationFrps(ctx, frpsRec); err != nil {
		t.Fatalf("insert frps entry: %v", err)
	}
	if hookedFrps == nil || hookedFrps.ID != 97 || hookedFrps.FrpsConnections != 3 || hookedFrps.FrpsProxyCount != 2 {
		t.Fatalf("frps hook not fired with converted row: %+v", hookedFrps)
	}
	frpcRec := &model.OpenFlareNodeObservationFrpc{ID: 96, NodeID: "n1", CapturedAt: now, TunnelStatus: "online", ConnectedRelaysCount: 7}
	if err := s.InsertNodeObservationFrpc(ctx, frpcRec); err != nil {
		t.Fatalf("insert frpc entry: %v", err)
	}
	if hookedFrpc == nil || hookedFrpc.ID != 96 || hookedFrpc.TunnelStatus != "online" || hookedFrpc.ConnectedRelaysCount != 7 {
		t.Fatalf("frpc hook not fired with converted row: %+v", hookedFrpc)
	}
	// 四个入口均只入队、不落库。
	if rows, err := s.ListMetricSnapshots(ctx, "n1", now.Add(-time.Hour), 10); err != nil {
		t.Fatalf("list metrics after entry insert: %v", err)
	} else if len(rows) != 0 {
		t.Fatalf("metric entry insert must not write rows, got %d", len(rows))
	}
	if rows, err := s.ListEdgeHealth(ctx, "n1", now.Add(-time.Hour), 10); err != nil {
		t.Fatalf("list edge after entry insert: %v", err)
	} else if len(rows) != 0 {
		t.Fatalf("edge entry insert must not write rows, got %d", len(rows))
	}
	if rows, err := s.ListNodeObservationFrps(ctx, "n1", now.Add(-time.Hour), 10); err != nil {
		t.Fatalf("list frps after entry insert: %v", err)
	} else if len(rows) != 0 {
		t.Fatalf("frps entry insert must not write rows, got %d", len(rows))
	}
	if rows, err := s.ListNodeObservationFrpc(ctx, "n1", now.Add(-time.Hour), 10); err != nil {
		t.Fatalf("list frpc after entry insert: %v", err)
	} else if len(rows) != 0 {
		t.Fatalf("frpc entry insert must not write rows, got %d", len(rows))
	}
	SetObservabilityHooks(ObservabilityHooks{})

	// metric snapshots: flush 2 行 → 查询 desc → 迁移分页 → DeleteBefore → DeleteAll。
	early := now.Add(-2 * time.Hour)
	metrics := []analyticsmodel.NodeMetricSnapshot{
		{ID: 1, NodeID: "n1", CapturedAt: early, CPUUsagePercent: 1},
		{ID: 2, NodeID: "n1", CapturedAt: now, CPUUsagePercent: 2},
	}
	if err := s.BatchInsertNodeMetricSnapshots(ctx, metrics); err != nil {
		t.Fatalf("flush metrics: %v", err)
	}
	listed, err := s.ListMetricSnapshots(ctx, "n1", now.Add(-24*time.Hour), 10)
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(listed) != 2 || listed[0].ID != 2 || listed[1].ID != 1 {
		t.Fatalf("metrics list want 2 rows desc, got %+v", listed)
	}
	if listed[0].CPUUsagePercent != 2 {
		t.Fatalf("metric field roundtrip failed: %+v", listed[0])
	}
	migRows, err := s.ListMetricSnapshotsForMigration(ctx, 0, 1)
	if err != nil {
		t.Fatalf("metrics migration list: %v", err)
	}
	if len(migRows) != 1 || migRows[0].ID != 1 {
		t.Fatalf("metrics migration page want id=1, got %+v", migRows)
	}
	deleted, err := s.DeleteMetricSnapshotsBefore(ctx, now)
	if err != nil {
		t.Fatalf("delete metrics before: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("delete metrics before want 1, got %d", deleted)
	}
	if _, err := s.DeleteAllMetricSnapshots(ctx); err != nil {
		t.Fatalf("delete all metrics: %v", err)
	}
	left, err := s.ListMetricSnapshots(ctx, "n1", now.Add(-24*time.Hour), 10)
	if err != nil {
		t.Fatalf("list metrics after delete: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("metrics should be empty after delete, got %d", len(left))
	}

	// edge health：flush + 查询 + 迁移分页 + 删除。
	if err := s.BatchInsertNodeEdgeHealth(ctx, []analyticsmodel.NodeEdgeHealth{
		{ID: 1, NodeID: "n1", CapturedAt: now, Status: "healthy", Connections: 3},
	}); err != nil {
		t.Fatalf("flush edge health: %v", err)
	}
	edges, err := s.ListEdgeHealth(ctx, "n1", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("list edge health: %v", err)
	}
	if len(edges) != 1 || edges[0].Status != "healthy" || edges[0].Connections != 3 {
		t.Fatalf("edge health list mismatch: %+v", edges)
	}
	edgeMig, err := s.ListEdgeHealthForMigration(ctx, 0, 10)
	if err != nil {
		t.Fatalf("edge migration list: %v", err)
	}
	if len(edgeMig) != 1 {
		t.Fatalf("edge migration want 1, got %d", len(edgeMig))
	}
	if _, err := s.DeleteAllEdgeHealth(ctx); err != nil {
		t.Fatalf("delete all edge health: %v", err)
	}

	// frps：flush + 查询 + 迁移分页 + 删除。
	if err := s.BatchInsertNodeObsFrps(ctx, []analyticsmodel.NodeObsFrps{
		{ID: 1, NodeID: "n1", CapturedAt: now, FrpsConnections: 2, FrpsProxyCount: 3, FrpsProxies: `["a"]`},
		{ID: 2, NodeID: "n1", CapturedAt: now.Add(time.Hour), FrpsConnections: 4},
	}); err != nil {
		t.Fatalf("flush frps: %v", err)
	}
	frpsRows, err := s.ListNodeObservationFrps(ctx, "n1", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("list frps: %v", err)
	}
	if len(frpsRows) != 2 || frpsRows[0].FrpsConnections != 4 || frpsRows[1].FrpsProxies != `["a"]` {
		t.Fatalf("frps list mismatch: %+v", frpsRows)
	}
	frpsMig, err := s.ListNodeObsFrpsForMigration(ctx, 0, 10)
	if err != nil {
		t.Fatalf("frps migration list: %v", err)
	}
	if len(frpsMig) != 2 {
		t.Fatalf("frps migration want 2, got %d", len(frpsMig))
	}
	if _, err := s.DeleteAllNodeObservationFrps(ctx); err != nil {
		t.Fatalf("delete all frps: %v", err)
	}

	// frpc：flush + 查询 + 迁移分页 + 删除。
	if err := s.BatchInsertNodeObsFrpc(ctx, []analyticsmodel.NodeObsFrpc{
		{ID: 1, NodeID: "n1", CapturedAt: now, TunnelStatus: "online", ConnectedRelaysCount: 5},
	}); err != nil {
		t.Fatalf("flush frpc: %v", err)
	}
	frpcRows, err := s.ListNodeObservationFrpc(ctx, "n1", now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("list frpc: %v", err)
	}
	if len(frpcRows) != 1 || frpcRows[0].TunnelStatus != "online" || frpcRows[0].ConnectedRelaysCount != 5 {
		t.Fatalf("frpc list mismatch: %+v", frpcRows)
	}
	frpcMig, err := s.ListNodeObsFrpcForMigration(ctx, 0, 10)
	if err != nil {
		t.Fatalf("frpc migration list: %v", err)
	}
	if len(frpcMig) != 1 {
		t.Fatalf("frpc migration want 1, got %d", len(frpcMig))
	}
	if _, err := s.DeleteAllNodeObservationFrpc(ctx); err != nil {
		t.Fatalf("delete all frpc: %v", err)
	}
}

// TestGormUserAccessLogCountList 覆盖用户访问日志：批量写入、过滤计数、
// 分页列表、每日趋势、浏览器分布与活跃用户排行。
func TestGormUserAccessLogCountList(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })

	base := newTestGormStoreWithModels(t, &analyticsmodel.UserAccessLog{})
	ua := newUserAccessLogGormStore(base.db)
	ctx := context.Background()
	now := time.Now()
	logs := []analyticsmodel.UserAccessLog{
		{ID: 1, UserID: 10, Path: "/api/a", Method: "GET", IP: "1.1.1.1", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", Status: 200, CreatedAt: now.Add(-3 * time.Hour)},
		{ID: 2, UserID: 10, Path: "/api/a", Method: "POST", IP: "1.1.1.1", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", Status: 500, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: 3, UserID: 20, Path: "/api/b", Method: "GET", IP: "2.2.2.2", UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15", Status: 200, CreatedAt: now.Add(-time.Hour)},
		{ID: 4, UserID: 0, Path: "/api/c", Method: "GET", IP: "3.3.3.3", UserAgent: "", Status: 404, CreatedAt: now},
	}
	if err := ua.BatchInsert(ctx, logs); err != nil {
		t.Fatalf("batch insert: %v", err)
	}
	if err := ua.BatchInsert(ctx, nil); err != nil {
		t.Fatalf("batch insert empty: %v", err)
	}

	totalAll, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{})
	if err != nil {
		t.Fatalf("count all: %v", err)
	}
	if totalAll != 4 {
		t.Fatalf("count all want 4, got %d", totalAll)
	}
	// 单一权威字段集：user_id IN、path LIKE、StartTime >=、EndTime <=（闭区间，与 CH 一致）。
	totalUser, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{UserIDs: []uint64{10}})
	if err != nil {
		t.Fatalf("count by user: %v", err)
	}
	if totalUser != 2 {
		t.Fatalf("count by user want 2, got %d", totalUser)
	}
	totalUsers, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{UserIDs: []uint64{10, 20}})
	if err != nil {
		t.Fatalf("count by users: %v", err)
	}
	if totalUsers != 3 {
		t.Fatalf("count by users want 3, got %d", totalUsers)
	}
	// 空非 nil UserIDs：无匹配。
	totalNoUser, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{UserIDs: []uint64{}})
	if err != nil {
		t.Fatalf("count by empty users: %v", err)
	}
	if totalNoUser != 0 {
		t.Fatalf("count by empty users want 0, got %d", totalNoUser)
	}
	totalPath, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{Path: "/api/a"})
	if err != nil {
		t.Fatalf("count by path: %v", err)
	}
	if totalPath != 2 {
		t.Fatalf("count by path want 2, got %d", totalPath)
	}
	// path 先 trim 再 LIKE。
	totalPathTrim, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{Path: "  /api/b  "})
	if err != nil {
		t.Fatalf("count by trimmed path: %v", err)
	}
	if totalPathTrim != 1 {
		t.Fatalf("count by trimmed path want 1, got %d", totalPathTrim)
	}
	since := now.Add(-90 * time.Minute)
	totalSince, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{StartTime: &since})
	if err != nil {
		t.Fatalf("count by start time: %v", err)
	}
	if totalSince != 2 {
		t.Fatalf("count by start time want 2, got %d", totalSince)
	}
	until := now.Add(-90 * time.Minute)
	totalUntil, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{EndTime: &until})
	if err != nil {
		t.Fatalf("count by end time: %v", err)
	}
	if totalUntil != 2 {
		t.Fatalf("count by end time want 2, got %d", totalUntil)
	}
	// 组合窗口：[-150min, -90min] 命中 ID=2 一条（端点为 -90min 的边界行不存在）。
	// EndTime 闭区间语义由 TestGormUserAccessLogEndTimeInclusive 单独钉住。
	startWin := now.Add(-150 * time.Minute)
	endWin := now.Add(-90 * time.Minute)
	totalWin, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{StartTime: &startWin, EndTime: &endWin})
	if err != nil {
		t.Fatalf("count by window: %v", err)
	}
	if totalWin != 1 {
		t.Fatalf("count by window want 1, got %d", totalWin)
	}

	// 分页：按 user 过滤，page2 size1 返回 ID=1。
	page2, total, err := ua.List(ctx, analyticsmodel.AccessLogFilter{UserIDs: []uint64{10}}, 2, 1)
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if total != 2 || len(page2) != 1 || page2[0].ID != 1 {
		t.Fatalf("list page2 want total=2 rows=[1], got total=%d rows=%+v", total, page2)
	}
	// 无匹配：空列表 + total 0。
	empty, total, err := ua.List(ctx, analyticsmodel.AccessLogFilter{Path: "/api/nope"}, 1, 10)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if total != 0 || len(empty) != 0 {
		t.Fatalf("list empty want total=0 rows=0, got total=%d rows=%d", total, len(empty))
	}
	// 空非 nil UserIDs：短路返回空。
	emptyUsers, total, err := ua.List(ctx, analyticsmodel.AccessLogFilter{UserIDs: []uint64{}}, 1, 10)
	if err != nil {
		t.Fatalf("list empty users: %v", err)
	}
	if total != 0 || len(emptyUsers) != 0 {
		t.Fatalf("list empty users want total=0 rows=0, got total=%d rows=%d", total, len(emptyUsers))
	}

	// 每日趋势：镜像 CH——恰好 days 个连续日历日、零日补零；今日 4 条全部落入网格。
	trend, err := ua.GetDailyTrend(ctx, 7)
	if err != nil {
		t.Fatalf("daily trend: %v", err)
	}
	if len(trend) != 7 {
		t.Fatalf("trend want exactly 7 rows, got %d: %+v", len(trend), trend)
	}
	var sum uint64
	for i, day := range trend {
		sum += day.Count
		if i > 0 {
			prev, _ := time.Parse("2006-01-02", trend[i-1].Date)
			cur, _ := time.Parse("2006-01-02", day.Date)
			if cur.Sub(prev) != 24*time.Hour {
				t.Fatalf("trend dates not consecutive: %s -> %s", trend[i-1].Date, day.Date)
			}
		}
	}
	if sum != 4 {
		t.Fatalf("trend sum want 4, got %d: %+v", sum, trend)
	}

	// 浏览器分布：Chrome 2、Safari 1、Unknown 1，按数量降序。
	browsers, err := ua.GetBrowserDistribution(ctx, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("browser distribution: %v", err)
	}
	gotBrowser := map[string]uint64{}
	for _, b := range browsers {
		gotBrowser[b.Browser] = b.Count
	}
	if gotBrowser["Chrome"] != 2 || gotBrowser["Safari"] != 1 || gotBrowser["Unknown"] != 1 {
		t.Fatalf("browser distribution mismatch: %+v", gotBrowser)
	}
	if len(browsers) > 0 && browsers[0].Count < browsers[len(browsers)-1].Count {
		t.Fatalf("browser distribution not sorted desc: %+v", browsers)
	}

	// 活跃用户：user10=2、user20=1（user0 排除），按数量降序。
	top, err := ua.GetTopActiveUsers(ctx, now.Add(-24*time.Hour), 10)
	if err != nil {
		t.Fatalf("top active users: %v", err)
	}
	gotTop := map[uint64]uint64{}
	for _, u := range top {
		gotTop[u.UserID] = u.Count
	}
	if gotTop[10] != 2 || gotTop[20] != 1 || len(top) != 2 {
		t.Fatalf("top active users mismatch: %+v", gotTop)
	}
	if top[0].Count < top[len(top)-1].Count {
		t.Fatalf("top users not sorted desc: %+v", top)
	}
}

// TestGormCountBucketsAndIPTrend 覆盖 AccessLogStore 时间分桶聚合：
// CountBuckets 返回过滤窗口内的分桶数，IPTrend 返回按桶升序的请求趋势。
func TestGormCountBucketsAndIPTrend(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: base, RemoteAddr: "1.1.1.1", StatusCode: 200},
		{ID: 2, NodeID: "n1", LoggedAt: base.Add(time.Minute), RemoteAddr: "2.2.2.2", StatusCode: 200},
		{ID: 3, NodeID: "n1", LoggedAt: base.Add(time.Hour), RemoteAddr: "3.3.3.3", StatusCode: 500},
		{ID: 4, NodeID: "n2", LoggedAt: base.Add(time.Hour), RemoteAddr: "4.4.4.4", StatusCode: 200},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	buckets, err := s.CountBuckets(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1"}, 3600)
	if err != nil {
		t.Fatalf("count buckets: %v", err)
	}
	if buckets != 2 {
		t.Fatalf("count buckets = %d, want 2", buckets)
	}
	trend, err := s.IPTrend(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1"}, 3600)
	if err != nil {
		t.Fatalf("ip trend: %v", err)
	}
	if len(trend) != 2 || trend[0].BucketEpoch != base.Unix() || trend[0].RequestCount != 2 || trend[1].RequestCount != 1 {
		t.Fatalf("ip trend = %+v", trend)
	}
	if trend[0].BucketEpoch >= trend[1].BucketEpoch {
		t.Fatalf("ip trend not ascending: %+v", trend)
	}
}

// TestGormBucketAggregatesFullFieldSet 验证 BucketAggregates 与 CH 对齐的完整 9 字段聚合：
// success/client_error/server_error 计数、排除空串的 distinct IP/Host、bytes_sent/request_length 求和。
func TestGormBucketAggregatesFullFieldSet(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: base, RemoteAddr: "1.1.1.1", Host: "a.example.com", StatusCode: 200, BytesSent: 100, RequestLength: 10},
		{ID: 2, NodeID: "n1", LoggedAt: base.Add(time.Minute), RemoteAddr: "1.1.1.1", Host: "a.example.com", StatusCode: 301, BytesSent: 200, RequestLength: 20},
		{ID: 3, NodeID: "n1", LoggedAt: base.Add(2 * time.Minute), RemoteAddr: "2.2.2.2", Host: "b.example.com", StatusCode: 404, BytesSent: 300, RequestLength: 30},
		{ID: 4, NodeID: "n1", LoggedAt: base.Add(3 * time.Minute), RemoteAddr: "", Host: "b.example.com", StatusCode: 500, BytesSent: 400, RequestLength: 40},
		{ID: 5, NodeID: "n1", LoggedAt: base.Add(4 * time.Minute), RemoteAddr: "3.3.3.3", Host: "c.example.com", StatusCode: 502, BytesSent: 500, RequestLength: 50},
		{ID: 6, NodeID: "n2", LoggedAt: base.Add(time.Hour), RemoteAddr: "9.9.9.9", Host: "d.example.com", StatusCode: 200, BytesSent: 999, RequestLength: 99},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}

	buckets, err := s.BucketAggregates(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1"}, 3600)
	if err != nil {
		t.Fatalf("bucket aggregates: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("bucket aggregates = %d buckets, want 1", len(buckets))
	}
	b := buckets[0]
	if b.BucketEpoch != base.Unix() {
		t.Fatalf("bucket_epoch = %d, want %d", b.BucketEpoch, base.Unix())
	}
	if b.RequestCount != 5 {
		t.Errorf("request_count = %d, want 5", b.RequestCount)
	}
	if b.SuccessCount != 2 {
		t.Errorf("success_count = %d, want 2", b.SuccessCount)
	}
	if b.ClientErrorCount != 1 {
		t.Errorf("client_error_count = %d, want 1", b.ClientErrorCount)
	}
	if b.ServerErrorCount != 2 {
		t.Errorf("server_error_count = %d, want 2", b.ServerErrorCount)
	}
	if b.UniqueIPCount != 3 {
		t.Errorf("unique_ip_count = %d, want 3 (empty remote_addr excluded)", b.UniqueIPCount)
	}
	if b.UniqueHostCount != 3 {
		t.Errorf("unique_host_count = %d, want 3", b.UniqueHostCount)
	}
	if b.BytesSent != 1500 {
		t.Errorf("bytes_sent = %d, want 1500", b.BytesSent)
	}
	if b.RequestLength != 150 {
		t.Errorf("request_length = %d, want 150", b.RequestLength)
	}

	// 分组与节点过滤：n2 落在相邻 bucket，且按 bucket_epoch 升序返回。
	all, err := s.BucketAggregates(ctx, model.OpenFlareAccessLogQuery{}, 3600)
	if err != nil {
		t.Fatalf("bucket aggregates all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("bucket aggregates all = %d buckets, want 2", len(all))
	}
	if all[0].BucketEpoch != base.Unix() || all[0].RequestCount != 5 {
		t.Fatalf("first bucket = %+v, want epoch %d count 5", all[0], base.Unix())
	}
	if all[1].BucketEpoch != base.Add(time.Hour).Unix() || all[1].RequestCount != 1 || all[1].BytesSent != 999 || all[1].SuccessCount != 1 {
		t.Fatalf("second bucket = %+v, want epoch %d count 1 bytes 999 success 1", all[1], base.Add(time.Hour).Unix())
	}
}

// TestGormListFilterSemantics 验证过滤语义与 CH 对齐：
// remote_addr/host/path 前缀 LIKE、hosts lower(trim(host)) IN、node_id trim、until 开区间、since 闭区间。
func TestGormListFilterSemantics(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	now := time.Now()
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: now.Add(-2 * time.Hour), RemoteAddr: "1.2.3.4", Host: "Example.COM", Path: "/api/v1/users", StatusCode: 200},
		{ID: 2, NodeID: "n2", LoggedAt: now.Add(-time.Hour), RemoteAddr: "5.6.7.8", Host: "other.com", Path: "/static/x.js", StatusCode: 200},
		{ID: 3, NodeID: "n1", LoggedAt: now, RemoteAddr: "9.9.9.9", Host: "example.com", Path: "/api/v2", StatusCode: 500},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.List(ctx, model.OpenFlareAccessLogQuery{RemoteAddr: "1.2.3"})
	if err != nil {
		t.Fatalf("list remote: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("remote prefix got %+v", got)
	}
	got, err = s.List(ctx, model.OpenFlareAccessLogQuery{Hosts: []string{"  EXAMPLE.com "}})
	if err != nil {
		t.Fatalf("list hosts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("hosts in got %d rows, want 2", len(got))
	}
	got, err = s.List(ctx, model.OpenFlareAccessLogQuery{Path: "/api"})
	if err != nil {
		t.Fatalf("list path: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("path prefix got %d rows, want 2", len(got))
	}
	got, err = s.List(ctx, model.OpenFlareAccessLogQuery{NodeID: " n2 "})
	if err != nil {
		t.Fatalf("list node trim: %v", err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("node trim got %+v", got)
	}
	got, err = s.List(ctx, model.OpenFlareAccessLogQuery{Until: now})
	if err != nil {
		t.Fatalf("list until: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("until open interval got %d rows, want 2 (strictly before now)", len(got))
	}
	got, err = s.List(ctx, model.OpenFlareAccessLogQuery{Since: now})
	if err != nil {
		t.Fatalf("list since: %v", err)
	}
	if len(got) != 1 || got[0].ID != 3 {
		t.Fatalf("since closed got %+v", got)
	}
}

// TestGormValueCountsStatusCode 验证 status_code（int32 列）经方言 CAST 转文本后可扫描为 string。
func TestGormValueCountsStatusCode(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	now := time.Now()
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: now, StatusCode: 200},
		{ID: 2, NodeID: "n1", LoggedAt: now, StatusCode: 200},
		{ID: 3, NodeID: "n1", LoggedAt: now, StatusCode: 500},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	counts, err := s.ValueCounts(ctx, model.OpenFlareAccessLogQuery{NodeID: "n1"}, "status_code", 10)
	if err != nil {
		t.Fatalf("value counts: %v", err)
	}
	got := map[string]int64{}
	for _, c := range counts {
		got[c.Value] = c.Count
	}
	if got["200"] != 2 || got["500"] != 1 {
		t.Fatalf("status code counts = %+v", got)
	}
}

// TestGormWAFAndIPSummaries 覆盖 WAFIPAggregates 与 IPSummaries 的字段映射
// （status404_count/success2xx_count 等别名需与 GORM 命名策略一致，否则扫描为 0）。
func TestGormWAFAndIPSummaries(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: base, RemoteAddr: "1.1.1.1", Host: "a.com", Region: "cn", StatusCode: 200},
		{ID: 2, NodeID: "n1", LoggedAt: base.Add(time.Minute), RemoteAddr: "1.1.1.1", Host: "1.2.3.4", Region: "cn", StatusCode: 404},
		{ID: 3, NodeID: "n1", LoggedAt: base.Add(time.Hour), RemoteAddr: "2.2.2.2", Host: "b.com", Region: "us", StatusCode: 500},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	q := model.OpenFlareAccessLogQuery{NodeID: "n1"}

	sums, err := s.IPSummaries(ctx, q, time.Time{})
	if err != nil {
		t.Fatalf("ip summaries: %v", err)
	}
	byIP := map[string]analyticsmodel.NodeAccessLogIPSummary{}
	for _, x := range sums {
		byIP[x.RemoteAddr] = x
	}
	if byIP["1.1.1.1"].TotalRequests != 2 || byIP["1.1.1.1"].Success2xxCount != 1 || byIP["1.1.1.1"].SuccessRatio != 0.5 {
		t.Fatalf("ip summaries 1.1.1.1 = %+v", byIP["1.1.1.1"])
	}

	waf, err := s.WAFIPAggregates(ctx, q)
	if err != nil {
		t.Fatalf("waf ip aggregates: %v", err)
	}
	got := map[string]analyticsmodel.NodeAccessLogWAFIPAggregate{}
	for _, x := range waf {
		got[x.RemoteAddr] = x
	}
	a := got["1.1.1.1"]
	if a.RequestCount != 2 || a.Status404Count != 1 || a.ClientErrorCount != 1 || a.IPHostCount != 1 ||
		a.StatusCounts[200] != 1 || a.StatusCounts[404] != 1 {
		t.Fatalf("waf 1.1.1.1 = %+v", a)
	}
	b := got["2.2.2.2"]
	if b.RequestCount != 1 || b.ServerErrorCount != 1 || b.IPHostCount != 0 {
		t.Fatalf("waf 2.2.2.2 = %+v", b)
	}
}

// TestGormIPSummariesRegionWithinFilterWindow 验证 IPSummaries 的 region 取过滤窗口内该 IP
// 最近一条（对齐 CH argMax(region, logged_at)）：窗口外更新的 region 记录不参与，
// 旧实现（子查询不带窗口条件）会错误返回窗口外那条。
func TestGormIPSummariesRegionWithinFilterWindow(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Minute)
	rows := []analyticsmodel.NodeAccessLog{
		// 窗口外（晚于 Until）：同一 IP 的更新 region，旧实现会误取。
		{ID: 1, NodeID: "n1", LoggedAt: base.Add(3 * time.Hour), RemoteAddr: "1.1.1.1", Region: "outside-new", StatusCode: 200},
		// 窗口内 [base, base+2h) 最新一条：region 应为 inside-old。
		{ID: 2, NodeID: "n1", LoggedAt: base.Add(time.Hour), RemoteAddr: "1.1.1.1", Region: "inside-old", StatusCode: 200},
		// 窗口内更早的一条，不应覆盖窗口内最新 region。
		{ID: 3, NodeID: "n1", LoggedAt: base, RemoteAddr: "1.1.1.1", Region: "inside-earlier", StatusCode: 200},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	sums, err := s.IPSummaries(ctx, model.OpenFlareAccessLogQuery{
		NodeID: "n1",
		Since:  base,
		Until:  base.Add(2 * time.Hour),
	}, time.Time{})
	if err != nil {
		t.Fatalf("ip summaries: %v", err)
	}
	if len(sums) != 1 || sums[0].RemoteAddr != "1.1.1.1" || sums[0].Region != "inside-old" {
		t.Fatalf("ip summaries = %+v, want 1.1.1.1 region inside-old (window-external outside-new excluded)", sums)
	}
}

// TestGormIPSummariesEmptyFilter 覆盖 IPSummaries 空过滤分支（cond=="" 时 region 子查询
// 不带参数、Select 走无 args 路径）：OpenFlareAccessLogQuery{} 不报错、返回全部 IP 分组，
// region 为各 IP 全部行中最新一条（无窗口即全部行）。
func TestGormIPSummariesEmptyFilter(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Minute)
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: base, RemoteAddr: "1.1.1.1", Region: "cn", StatusCode: 200},
		{ID: 2, NodeID: "n1", LoggedAt: base.Add(time.Minute), RemoteAddr: "1.1.1.1", Region: "us", StatusCode: 200},
		{ID: 3, NodeID: "n2", LoggedAt: base, RemoteAddr: "2.2.2.2", Region: "jp", StatusCode: 404},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}
	sums, err := s.IPSummaries(ctx, model.OpenFlareAccessLogQuery{}, time.Time{})
	if err != nil {
		t.Fatalf("ip summaries empty filter: %v", err)
	}
	if len(sums) != 2 {
		t.Fatalf("ip summaries = %d groups, want 2", len(sums))
	}
	byIP := map[string]analyticsmodel.NodeAccessLogIPSummary{}
	for _, x := range sums {
		byIP[x.RemoteAddr] = x
	}
	// 无窗口即全部行：1.1.1.1 最新一条 region 为 us（region 子查询不带参数路径）。
	if byIP["1.1.1.1"].Region != "us" || byIP["1.1.1.1"].TotalRequests != 2 {
		t.Fatalf("ip summaries 1.1.1.1 = %+v, want region us requests 2", byIP["1.1.1.1"])
	}
	if byIP["2.2.2.2"].Region != "jp" || byIP["2.2.2.2"].TotalRequests != 1 {
		t.Fatalf("ip summaries 2.2.2.2 = %+v, want region jp requests 1", byIP["2.2.2.2"])
	}
}

// TestGormListRejectsUnsupportedSortBy 验证 List 对不支持的 SortBy 直接报错，
// 默认 logged_at 路径与 CH 支持的 status_code/remote_addr 正常可用。
func TestGormListRejectsUnsupportedSortBy(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	s := newTestGormStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: now, StatusCode: 404, RemoteAddr: "9.9.9.9"},
		{ID: 2, NodeID: "n1", LoggedAt: now.Add(time.Second), StatusCode: 200, RemoteAddr: "1.1.1.1"},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}

	for _, sortBy := range []string{"", "logged_at"} {
		got, err := s.List(ctx, model.OpenFlareAccessLogQuery{SortBy: sortBy})
		if err != nil {
			t.Fatalf("List sortBy=%q: %v", sortBy, err)
		}
		if len(got) != 2 {
			t.Fatalf("List sortBy=%q rows = %d, want 2", sortBy, len(got))
		}
	}
	for _, sortBy := range []string{"status_code", "remote_addr", "host", "path"} {
		if _, err := s.List(ctx, model.OpenFlareAccessLogQuery{SortBy: sortBy}); err != nil {
			t.Fatalf("List supported sortBy=%q: %v", sortBy, err)
		}
	}
	if _, err := s.List(ctx, model.OpenFlareAccessLogQuery{SortBy: "user_agent_unknown"}); err == nil {
		t.Fatal("List with unsupported sort_by should error")
	}
}

// TestGormMigrationRange 验证节点/用户访问日志时间范围查询：空表返回零值，
// 有数据返回 MIN/MAX（UTC）。
func TestGormMigrationRange(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	defer ResetForTest()

	s := newTestGormStoreWithModels(t, &analyticsmodel.NodeAccessLog{}, &analyticsmodel.UserAccessLog{})
	ua := newUserAccessLogGormStore(s.db)
	ctx := context.Background()

	from, to, err := s.MigrationRange(ctx)
	if err != nil {
		t.Fatalf("empty MigrationRange: %v", err)
	}
	if !from.IsZero() || !to.IsZero() {
		t.Fatalf("empty MigrationRange = %s ~ %s, want zero", from, to)
	}
	uaFrom, uaTo, err := ua.MigrationRange(ctx)
	if err != nil {
		t.Fatalf("empty user MigrationRange: %v", err)
	}
	if !uaFrom.IsZero() || !uaTo.IsZero() {
		t.Fatalf("empty user MigrationRange = %s ~ %s, want zero", uaFrom, uaTo)
	}

	now := time.Now().UTC()
	if err := s.BatchInsertNodeAccessLogs(ctx, []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: now.Add(-time.Hour)},
		{ID: 2, NodeID: "n1", LoggedAt: now},
	}); err != nil {
		t.Fatalf("insert node logs: %v", err)
	}
	if err := ua.BatchInsert(ctx, []analyticsmodel.UserAccessLog{
		{ID: 1, UserID: 1, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: 2, UserID: 1, CreatedAt: now},
	}); err != nil {
		t.Fatalf("insert user logs: %v", err)
	}

	from, to, err = s.MigrationRange(ctx)
	if err != nil {
		t.Fatalf("MigrationRange: %v", err)
	}
	if !from.Equal(now.Add(-time.Hour)) || !to.Equal(now) {
		t.Fatalf("MigrationRange = %s ~ %s, want %s ~ %s", from, to, now.Add(-time.Hour), now)
	}
	uaFrom, uaTo, err = ua.MigrationRange(ctx)
	if err != nil {
		t.Fatalf("user MigrationRange: %v", err)
	}
	if !uaFrom.Equal(now.Add(-2*time.Hour)) || !uaTo.Equal(now) {
		t.Fatalf("user MigrationRange = %s ~ %s", uaFrom, uaTo)
	}
}

// TestGormWriteMethodsFreezeDuringMigration 覆盖冻结期（ensureWritable → ErrMigrating）
// 可观测 4 表与用户访问日志的全部写方法。
func TestGormWriteMethodsFreezeDuringMigration(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == logMigrationKey {
			return "migrating", nil
		}
		return "", nil
	})
	defer ResetForTest()

	s := newTestGormStoreWithModels(t,
		&analyticsmodel.NodeAccessLog{},
		&analyticsmodel.NodeMetricSnapshot{},
		&analyticsmodel.NodeEdgeHealth{},
		&analyticsmodel.NodeObsFrps{},
		&analyticsmodel.NodeObsFrpc{},
		&analyticsmodel.UserAccessLog{},
	)
	ua := newUserAccessLogGormStore(s.db)
	ctx := context.Background()
	now := time.Now()

	cases := []struct {
		name string
		fn   func() error
	}{
		{"InsertBatch", func() error {
			return s.InsertBatch(ctx, []*model.OpenFlareAccessLog{{NodeID: "n1", LoggedAt: now}})
		}},
		{"BatchInsertNodeAccessLogs", func() error {
			return s.BatchInsertNodeAccessLogs(ctx, []analyticsmodel.NodeAccessLog{{NodeID: "n1", LoggedAt: now}})
		}},
		{"DeleteAllNodeAccessLogs", func() error { _, err := s.DeleteAll(ctx); return err }},
		{"DeleteBefore", func() error { _, err := s.DeleteBefore(ctx, now); return err }},
		{"DeleteByNodeBefore", func() error { _, err := s.DeleteByNodeBefore(ctx, "n1", now); return err }},
		{"InsertMetricSnapshot", func() error {
			return s.InsertMetricSnapshot(ctx, &model.OpenFlareMetricSnapshot{NodeID: "n1", CapturedAt: now})
		}},
		{"InsertEdgeHealth", func() error {
			return s.InsertEdgeHealth(ctx, &model.OpenFlareEdgeHealth{NodeID: "n1", CapturedAt: now})
		}},
		{"InsertNodeObservationFrps", func() error {
			return s.InsertNodeObservationFrps(ctx, &model.OpenFlareNodeObservationFrps{NodeID: "n1", CapturedAt: now})
		}},
		{"InsertNodeObservationFrpc", func() error {
			return s.InsertNodeObservationFrpc(ctx, &model.OpenFlareNodeObservationFrpc{NodeID: "n1", CapturedAt: now})
		}},
		{"BatchInsertNodeMetricSnapshots", func() error {
			return s.BatchInsertNodeMetricSnapshots(ctx, []analyticsmodel.NodeMetricSnapshot{{NodeID: "n1", CapturedAt: now}})
		}},
		{"BatchInsertNodeEdgeHealth", func() error {
			return s.BatchInsertNodeEdgeHealth(ctx, []analyticsmodel.NodeEdgeHealth{{NodeID: "n1", CapturedAt: now}})
		}},
		{"BatchInsertNodeObsFrps", func() error {
			return s.BatchInsertNodeObsFrps(ctx, []analyticsmodel.NodeObsFrps{{NodeID: "n1", CapturedAt: now}})
		}},
		{"BatchInsertNodeObsFrpc", func() error {
			return s.BatchInsertNodeObsFrpc(ctx, []analyticsmodel.NodeObsFrpc{{NodeID: "n1", CapturedAt: now}})
		}},
		{"DeleteAllMetricSnapshots", func() error { _, err := s.DeleteAllMetricSnapshots(ctx); return err }},
		{"DeleteMetricSnapshotsBefore", func() error { _, err := s.DeleteMetricSnapshotsBefore(ctx, now); return err }},
		{"DeleteAllEdgeHealth", func() error { _, err := s.DeleteAllEdgeHealth(ctx); return err }},
		{"DeleteEdgeHealthBefore", func() error { _, err := s.DeleteEdgeHealthBefore(ctx, now); return err }},
		{"DeleteAllNodeObservationFrps", func() error { _, err := s.DeleteAllNodeObservationFrps(ctx); return err }},
		{"DeleteNodeObservationFrpsBefore", func() error { _, err := s.DeleteNodeObservationFrpsBefore(ctx, now); return err }},
		{"DeleteAllNodeObservationFrpc", func() error { _, err := s.DeleteAllNodeObservationFrpc(ctx); return err }},
		{"DeleteNodeObservationFrpcBefore", func() error { _, err := s.DeleteNodeObservationFrpcBefore(ctx, now); return err }},
		{"UserAccessLogBatchInsert", func() error { return ua.BatchInsert(ctx, []analyticsmodel.UserAccessLog{{UserID: 1, CreatedAt: now}}) }},
	}
	for _, tc := range cases {
		if err := tc.fn(); !errors.Is(err, ErrMigrating) {
			t.Fatalf("%s: want ErrMigrating, got %v", tc.name, err)
		}
	}
}

// TestGormListTrafficHourly 验证 ListTrafficHourly 从原始访问日志按小时实时聚合：
// request_count=COUNT(*)、error_count=5xx、unique_visitor_count 恒 0，按 hour/node 升序。
func TestGormListTrafficHourly(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Hour)
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: base, StatusCode: 200},
		{ID: 2, NodeID: "n1", LoggedAt: base.Add(10 * time.Minute), StatusCode: 500},
		{ID: 3, NodeID: "n2", LoggedAt: base.Add(20 * time.Minute), StatusCode: 200},
		{ID: 4, NodeID: "n1", LoggedAt: base.Add(time.Hour), StatusCode: 200},
		{ID: 5, NodeID: "n1", LoggedAt: base.Add(time.Hour + 10*time.Minute), StatusCode: 404},
		{ID: 6, NodeID: "n1", LoggedAt: base.Add(time.Hour + 20*time.Minute), StatusCode: 502},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := s.ListTrafficHourly(ctx, "", base)
	if err != nil {
		t.Fatalf("list traffic hourly: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(got), got)
	}
	want := []analyticsmodel.NodeTrafficHourly{
		{NodeID: "n1", Hour: base, RequestCount: 2, ErrorCount: 1},
		{NodeID: "n2", Hour: base, RequestCount: 1, ErrorCount: 0},
		{NodeID: "n1", Hour: base.Add(time.Hour), RequestCount: 3, ErrorCount: 1},
	}
	for i := range want {
		g := got[i]
		w := want[i]
		if g.NodeID != w.NodeID || !g.Hour.Equal(w.Hour) || g.RequestCount != w.RequestCount ||
			g.ErrorCount != w.ErrorCount || g.UniqueVisitorCount != 0 {
			t.Errorf("row[%d] = %+v, want %+v", i, g, w)
		}
	}

	// nodeID + since 过滤。
	single, err := s.ListTrafficHourly(ctx, "n1", base.Add(time.Hour))
	if err != nil {
		t.Fatalf("list traffic hourly filtered: %v", err)
	}
	if len(single) != 1 || single[0].NodeID != "n1" || single[0].RequestCount != 3 || single[0].ErrorCount != 1 {
		t.Fatalf("filtered = %+v, want single n1 h1 (3/1)", single)
	}
}

// TestGormListAccessLogHourly 验证 ListAccessLogHourly 按 node/hour/host 实时聚合：
// request_count、error_count(5xx)、bytes_sent/request_length 求和，与 CH of_access_log_hourly 字段对齐。
func TestGormListAccessLogHourly(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Hour)
	rows := []analyticsmodel.NodeAccessLog{
		{ID: 1, NodeID: "n1", LoggedAt: base, Host: "a.example.com", StatusCode: 200, BytesSent: 100, RequestLength: 10},
		{ID: 2, NodeID: "n1", LoggedAt: base.Add(10 * time.Minute), Host: "a.example.com", StatusCode: 500, BytesSent: 200, RequestLength: 20},
		{ID: 3, NodeID: "n1", LoggedAt: base.Add(20 * time.Minute), Host: "b.example.com", StatusCode: 404, BytesSent: 300, RequestLength: 30},
		{ID: 4, NodeID: "n1", LoggedAt: base.Add(time.Hour), Host: "a.example.com", StatusCode: 200, BytesSent: 400, RequestLength: 40},
		{ID: 5, NodeID: "n2", LoggedAt: base, Host: "c.example.com", StatusCode: 200, BytesSent: 999, RequestLength: 99},
	}
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := s.ListAccessLogHourly(ctx, "n1", base)
	if err != nil {
		t.Fatalf("list access log hourly: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(got), got)
	}
	want := []analyticsmodel.AccessLogHourly{
		{NodeID: "n1", Hour: base, Host: "a.example.com", RequestCount: 2, ErrorCount: 1, BytesSent: 300, RequestLength: 30},
		{NodeID: "n1", Hour: base, Host: "b.example.com", RequestCount: 1, ErrorCount: 0, BytesSent: 300, RequestLength: 30},
		{NodeID: "n1", Hour: base.Add(time.Hour), Host: "a.example.com", RequestCount: 1, ErrorCount: 0, BytesSent: 400, RequestLength: 40},
	}
	for i := range want {
		g := got[i]
		w := want[i]
		if g != w {
			t.Errorf("row[%d] = %+v, want %+v", i, g, w)
		}
	}

	// 空 nodeID 返回全部节点；nodeID 无匹配返回空。
	all, err := s.ListAccessLogHourly(ctx, "", base)
	if err != nil {
		t.Fatalf("list access log hourly all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("all = %d rows, want 4", len(all))
	}
	none, err := s.ListAccessLogHourly(ctx, "n3", base)
	if err != nil {
		t.Fatalf("list access log hourly none: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("none = %d rows, want 0", len(none))
	}
}

// TestGormListMetricHourly 验证 ListMetricHourly 从 of_node_metric_snapshots 实时聚合，
// 对齐 CH raw 兜底口径：avg cpu/memory、每节点相邻采样计数器增量（负增量按 0）、reported_nodes。
func TestGormListMetricHourly(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })
	s := newTestGormStoreWithModels(t, &analyticsmodel.NodeMetricSnapshot{})
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Hour)
	snapshots := []analyticsmodel.NodeMetricSnapshot{
		{ID: 1, NodeID: "n1", CapturedAt: base, CPUUsagePercent: 10, MemoryUsedBytes: 100, MemoryTotalBytes: 200, NetworkRxBytes: 1000},
		{ID: 2, NodeID: "n1", CapturedAt: base.Add(30 * time.Minute), CPUUsagePercent: 20, MemoryUsedBytes: 100, MemoryTotalBytes: 200, NetworkRxBytes: 1050},
		{ID: 3, NodeID: "n1", CapturedAt: base.Add(90 * time.Minute), CPUUsagePercent: 30, MemoryUsedBytes: 50, MemoryTotalBytes: 100, NetworkRxBytes: 1100},
		{ID: 4, NodeID: "n2", CapturedAt: base, CPUUsagePercent: 40, MemoryUsedBytes: 0, MemoryTotalBytes: 0, NetworkRxBytes: 2000},
		{ID: 5, NodeID: "n2", CapturedAt: base.Add(30 * time.Minute), CPUUsagePercent: 60, MemoryUsedBytes: 80, MemoryTotalBytes: 100, NetworkRxBytes: 1900},
	}
	if err := s.BatchInsertNodeMetricSnapshots(ctx, snapshots); err != nil {
		t.Fatalf("insert snapshots: %v", err)
	}

	got, err := s.ListMetricHourly(ctx, "", base)
	if err != nil {
		t.Fatalf("list metric hourly: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d hours, want 2: %+v", len(got), got)
	}
	h0, h1 := got[0], got[1]
	if !h0.Hour.Equal(base) || !h1.Hour.Equal(base.Add(time.Hour)) {
		t.Fatalf("hours = %v / %v, want base / base+1h", h0.Hour, h1.Hour)
	}
	// h0: cpu avg=(10+20+40+60)/4=32.5；mem avg=(50+50+0+80)/4=45；
	// rx 增量：n1 首条 0 + 50；n2 首条 0 + 负增量 0 → 50；reported_nodes=2。
	assertFloat(t, "h0 cpu", h0.AverageCPUUsagePercent, 32.5)
	assertFloat(t, "h0 mem", h0.AverageMemoryUsagePercent, 45)
	if h0.NetworkRxBytes != 50 || h0.ReportedNodes != 2 {
		t.Errorf("h0 = rx %d nodes %d, want 50/2", h0.NetworkRxBytes, h0.ReportedNodes)
	}
	// h1: n1 单节点 cpu=30 mem=50，rx 增量 1100-1050=50。
	assertFloat(t, "h1 cpu", h1.AverageCPUUsagePercent, 30)
	assertFloat(t, "h1 mem", h1.AverageMemoryUsagePercent, 50)
	if h1.NetworkRxBytes != 50 || h1.ReportedNodes != 1 {
		t.Errorf("h1 = rx %d nodes %d, want 50/1", h1.NetworkRxBytes, h1.ReportedNodes)
	}

	// nodeID 过滤：仅 n1 → h0 avg cpu=15、mem=50、rx=50、nodes=1。
	n1, err := s.ListMetricHourly(ctx, "n1", base)
	if err != nil {
		t.Fatalf("list metric hourly n1: %v", err)
	}
	if len(n1) != 2 {
		t.Fatalf("n1 got %d hours, want 2", len(n1))
	}
	assertFloat(t, "n1 h0 cpu", n1[0].AverageCPUUsagePercent, 15)
	assertFloat(t, "n1 h0 mem", n1[0].AverageMemoryUsagePercent, 50)
	if n1[0].NetworkRxBytes != 50 || n1[0].ReportedNodes != 1 {
		t.Errorf("n1 h0 = rx %d nodes %d, want 50/1", n1[0].NetworkRxBytes, n1[0].ReportedNodes)
	}
}

func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	eps := 1e-6
	if got < want-eps || got > want+eps {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

// TestGormUserAccessLogEndTimeInclusive 钉住 EndTime 闭区间语义（对齐 CH created_at <= ?）：
// created_at 恰好等于边界值的行必须计入。
func TestGormUserAccessLogEndTimeInclusive(t *testing.T) {
	ResetForTest()
	SetConfigReader(func(_ context.Context, _ string) (string, error) { return "", nil })

	base := newTestGormStoreWithModels(t, &analyticsmodel.UserAccessLog{})
	ua := newUserAccessLogGormStore(base.db)
	ctx := context.Background()
	boundary := time.Now().Truncate(time.Second)
	logs := []analyticsmodel.UserAccessLog{
		{ID: 1, UserID: 1, Path: "/a", Method: "GET", Status: 200, CreatedAt: boundary},
		{ID: 2, UserID: 2, Path: "/b", Method: "GET", Status: 200, CreatedAt: boundary.Add(-time.Minute)},
	}
	if err := ua.BatchInsert(ctx, logs); err != nil {
		t.Fatalf("batch insert: %v", err)
	}
	end := boundary
	total, err := ua.Count(ctx, analyticsmodel.AccessLogFilter{EndTime: &end})
	if err != nil {
		t.Fatalf("count by end time: %v", err)
	}
	if total != 2 {
		t.Fatalf("count by end time want 2 (boundary row included), got %d", total)
	}
}
