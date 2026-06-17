package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rain-kl/openflare/openflare-server/internal/common"
	"github.com/rain-kl/openflare/openflare-server/internal/model"
)

const retentionDays = 90

func main() {
	dsn := flag.String("dsn", envOr("DSN", "postgres://openflare:replace-with-strong-password@192.168.107.2:5432/openflare?sslmode=disable"), "PostgreSQL DSN")
	records := flag.Int("records", 1_000_000, "number of access log rows to seed (0 = skip seeding)")
	seedNode := flag.String("node-id", "bench-node", "node_id used for seeded rows")
	iterations := flag.Int("iterations", 10, "benchmark iterations per scenario")
	warmup := flag.Int("warmup", 2, "warmup iterations per scenario")
	page := flag.Int("page", 0, "page index for list benchmark")
	pageSize := flag.Int("page-size", 20, "page size for list benchmark")
	reset := flag.Bool("reset", false, "truncate node_access_logs shard tables before seeding")
	legacyIterations := flag.Int("legacy-iterations", 1, "iterations for legacy full-scan scenario (can be very slow)")
	skipLegacy := flag.Bool("skip-legacy", false, "skip legacy full-scan scenario")
	verifyOnly := flag.Bool("verify-only", false, "verify query correctness and exit")
	flag.Parse()

	common.SQLDSN = *dsn
	common.SQLitePath = filepath.Join(os.TempDir(), "openflare-accesslogbench-missing.sqlite")
	if err := initBenchDB(*dsn); err != nil {
		log.Fatalf("init db: %v", err)
	}

	fmt.Println("=== OpenFlare node_access_logs benchmark (PostgreSQL) ===")
	fmt.Printf("DSN: %s\n", redactDSN(*dsn))
	fmt.Printf("GOMAXPROCS=%d\n", runtime.GOMAXPROCS(0))

	if *reset {
		if err := truncateAccessLogs(); err != nil {
			log.Fatalf("truncate access logs: %v", err)
		}
		fmt.Println("truncated node_access_logs shard tables")
	}

	if *records > 0 {
		existing, err := countAllAccessLogs()
		if err != nil {
			log.Fatalf("count existing rows: %v", err)
		}
		if existing >= int64(*records) {
			fmt.Printf("existing rows=%d >= target=%d, skip seeding\n", existing, *records)
		} else {
			missing := *records - int(existing)
			fmt.Printf("seeding %d rows (existing=%d)...\n", missing, existing)
			if err := seedAccessLogs(*seedNode, missing); err != nil {
				log.Fatalf("seed access logs: %v", err)
			}
		}
	}

	total, err := countAllAccessLogs()
	if err != nil {
		log.Fatalf("count rows after seed: %v", err)
	}
	fmt.Printf("total rows across shards: %d\n\n", total)

	since := time.Now().UTC().Add(-retentionDays * 24 * time.Hour)
	if err := verifyQueryCorrectness(since, total); err != nil {
		log.Fatalf("correctness verification failed: %v", err)
	}
	fmt.Println("correctness verification: PASS")
	if *verifyOnly {
		return
	}

	query := model.NodeAccessLogQuery{
		Since:     since,
		Page:      *page,
		PageSize:  *pageSize,
		SortBy:    "logged_at",
		SortOrder: "desc",
	}

	scenarios := []scenario{
		{
			name: "paginated_list",
			run: func() error {
				_, err := model.ListNodeAccessLogs(query)
				return err
			},
		},
		{
			name: "sql_count",
			run: func() error {
				_, _, err := model.CountNodeAccessLogs(query)
				return err
			},
		},
		{
			name: "api_list+count",
			run: func() error {
				if _, err := model.ListNodeAccessLogs(query); err != nil {
					return err
				}
				_, _, err := model.CountNodeAccessLogs(query)
				return err
			},
		},
		{
			name: "fullscan_list_legacy",
			run: func() error {
				_, err := fullScanList(query)
				return err
			},
		},
	}

	if err := printExplainPlans(since); err != nil {
		log.Printf("warn: explain analyze failed: %v", err)
	}

	for _, item := range scenarios {
		if item.name == "fullscan_list_legacy" && *skipLegacy {
			fmt.Println("[fullscan_list_legacy] skipped")
			continue
		}
		iterationCount := *iterations
		if item.name == "fullscan_list_legacy" {
			iterationCount = *legacyIterations
		}
		result, err := runScenario(item, *warmup, iterationCount)
		if err != nil {
			log.Fatalf("scenario %s failed: %v", item.name, err)
		}
		printResult(result)
	}
}

type scenario struct {
	name string
	run  func() error
}

type benchResult struct {
	name         string
	iterations   int
	latencies    []time.Duration
	allocBytes   []uint64
	heapInUse    []uint64
	maxHeapInUse uint64
}

func runScenario(item scenario, warmup int, iterations int) (*benchResult, error) {
	for range warmup {
		if err := item.run(); err != nil {
			return nil, err
		}
	}

	result := &benchResult{
		name:       item.name,
		iterations: iterations,
	}
	var peakHeap uint64

	for range iterations {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)

		start := time.Now()
		if err := item.run(); err != nil {
			return nil, err
		}
		elapsed := time.Since(start)

		runtime.ReadMemStats(&after)
		result.latencies = append(result.latencies, elapsed)
		result.allocBytes = append(result.allocBytes, after.TotalAlloc-before.TotalAlloc)
		result.heapInUse = append(result.heapInUse, after.HeapInuse)
		if after.HeapInuse > peakHeap {
			peakHeap = after.HeapInuse
		}
	}
	result.maxHeapInUse = peakHeap
	return result, nil
}

func printResult(result *benchResult) {
	latencies := append([]time.Duration(nil), result.latencies...)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var allocSum uint64
	var heapSum uint64
	for index := range result.allocBytes {
		allocSum += result.allocBytes[index]
		heapSum += result.heapInUse[index]
	}

	fmt.Printf("[%s] iterations=%d\n", result.name, result.iterations)
	fmt.Printf("  latency: min=%s avg=%s p50=%s p95=%s max=%s\n",
		minDuration(latencies),
		avgDuration(latencies),
		percentile(latencies, 50),
		percentile(latencies, 95),
		maxDuration(latencies),
	)
	fmt.Printf("  alloc/op: avg=%s peak_heap=%s\n",
		humanBytes(allocSum/uint64(len(result.allocBytes))),
		humanBytes(result.maxHeapInUse),
	)
	fmt.Printf("  heap_inuse/op: avg=%s\n\n", humanBytes(heapSum/uint64(len(result.heapInUse))))
}

func seedAccessLogs(nodeID string, total int) error {
	started := time.Now()
	perShard := total / 10
	remainder := total % 10
	seeded := 0
	now := time.Now().UTC()

	for shard := range 10 {
		rows := perShard
		if shard < remainder {
			rows++
		}
		if rows == 0 {
			continue
		}
		table := fmt.Sprintf("node_access_logs_%02d", shard)
		sql := fmt.Sprintf(`
INSERT INTO %s (id, node_id, logged_at, remote_addr, region, host, path, status_code, created_at)
SELECT
	(gs * 10 + %d)::bigint AS id,
	$1 AS node_id,
	$2::timestamptz - (gs || ' minutes')::interval AS logged_at,
	('203.0.' || ((gs / 256) %% 256)::text || '.' || (gs %% 256)::text) AS remote_addr,
	'Benchland' AS region,
	('host-' || (gs %% 200)::text || '.example.com') AS host,
	('/api/v1/resource/' || gs::text) AS path,
	(200 + (gs %% 4))::bigint AS status_code,
	$2::timestamptz AS created_at
FROM generate_series(0, $3 - 1) AS gs
`, table, shard)
		if err := model.DB.Exec(sql, nodeID, now, rows).Error; err != nil {
			return err
		}
		seeded += rows
		elapsed := time.Since(started)
		rate := float64(seeded) / elapsed.Seconds()
		fmt.Printf("  seeded %d/%d rows (%.0f rows/s, elapsed %s)\n", seeded, total, rate, elapsed.Round(time.Millisecond))
	}
	return nil
}

func truncateAccessLogs() error {
	for _, table := range observabilityShardTables() {
		if err := model.DB.Exec("TRUNCATE TABLE " + table).Error; err != nil {
			return err
		}
	}
	return nil
}

func countAllAccessLogs() (int64, error) {
	var total int64
	for _, table := range observabilityShardTables() {
		var count int64
		if err := model.DB.Table(table).Count(&count).Error; err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func observabilityShardTables() []string {
	tables := make([]string, 0, 10)
	for index := range 10 {
		tables = append(tables, fmt.Sprintf("node_access_logs_%02d", index))
	}
	return tables
}

func verifyQueryCorrectness(since time.Time, tableTotal int64) error {
	fmt.Println("=== correctness verification ===")
	baseQuery := model.NodeAccessLogQuery{
		Since:     since,
		SortBy:    "logged_at",
		SortOrder: "desc",
	}

	reference, err := fullScanList(baseQuery)
	if err != nil {
		return fmt.Errorf("reference full scan failed: %w", err)
	}
	if int64(len(reference)) != tableTotal {
		return fmt.Errorf("reference row count %d != table total %d", len(reference), tableTotal)
	}
	fmt.Printf("  reference rows in retention window: %d\n", len(reference))

	totalRecords, totalIPs, err := model.CountNodeAccessLogs(baseQuery)
	if err != nil {
		return fmt.Errorf("CountNodeAccessLogs failed: %w", err)
	}
	if totalRecords != int64(len(reference)) {
		return fmt.Errorf("total_records=%d want reference=%d", totalRecords, len(reference))
	}
	referenceIPs := countUniqueIPs(reference)
	if totalIPs != referenceIPs {
		return fmt.Errorf("total_ip=%d want reference=%d", totalIPs, referenceIPs)
	}
	fmt.Printf("  count totals: total_record=%d total_ip=%d\n", totalRecords, totalIPs)

	pages := []struct {
		page     int
		pageSize int
	}{
		{0, 20},
		{1, 20},
		{49, 20},
		{100, 50},
	}
	for _, item := range pages {
		query := baseQuery
		query.Page = item.page
		query.PageSize = item.pageSize
		got, err := model.ListNodeAccessLogs(query)
		if err != nil {
			return fmt.Errorf("ListNodeAccessLogs page=%d size=%d failed: %w", item.page, item.pageSize, err)
		}
		start := item.page * item.pageSize
		end := start + item.pageSize
		if start >= len(reference) {
			if len(got) != 0 {
				return fmt.Errorf("page=%d size=%d expected empty got %d", item.page, item.pageSize, len(got))
			}
			continue
		}
		if end > len(reference) {
			end = len(reference)
		}
		want := reference[start:end]
		if !accessLogsEqual(got, want) {
			return fmt.Errorf("page=%d size=%d content mismatch (got %d want %d rows)", item.page, item.pageSize, len(got), len(want))
		}
		fmt.Printf("  page=%d size=%d: %d rows match reference\n", item.page, item.pageSize, len(got))
	}

	filtered := model.NodeAccessLogQuery{
		NodeID:    "bench-node",
		Since:     since,
		SortBy:    "status_code",
		SortOrder: "asc",
		Page:      2,
		PageSize:  15,
	}
	filteredReference, err := fullScanList(filtered)
	if err != nil {
		return fmt.Errorf("filtered reference failed: %w", err)
	}
	filteredRows, err := model.ListNodeAccessLogs(filtered)
	if err != nil {
		return fmt.Errorf("filtered ListNodeAccessLogs failed: %w", err)
	}
	start := filtered.Page * filtered.PageSize
	end := start + filtered.PageSize
	if end > len(filteredReference) {
		end = len(filteredReference)
	}
	if start >= len(filteredReference) {
		start = len(filteredReference)
	}
	if !accessLogsEqual(filteredRows, filteredReference[start:end]) {
		return fmt.Errorf("filtered page content mismatch")
	}
	filteredTotal, filteredIPs, err := model.CountNodeAccessLogs(filtered)
	if err != nil {
		return fmt.Errorf("filtered CountNodeAccessLogs failed: %w", err)
	}
	if filteredTotal != int64(len(filteredReference)) {
		return fmt.Errorf("filtered total_records=%d want %d", filteredTotal, len(filteredReference))
	}
	if filteredIPs != countUniqueIPs(filteredReference) {
		return fmt.Errorf("filtered total_ip=%d want %d", filteredIPs, countUniqueIPs(filteredReference))
	}
	fmt.Printf("  filtered node_id=bench-node page=2 size=15: match (total_record=%d total_ip=%d)\n", filteredTotal, filteredIPs)
	return nil
}

func countUniqueIPs(logs []*model.NodeAccessLog) int64 {
	ips := make(map[string]struct{})
	for _, item := range logs {
		if item == nil {
			continue
		}
		if trimmed := strings.TrimSpace(item.RemoteAddr); trimmed != "" {
			ips[trimmed] = struct{}{}
		}
	}
	return int64(len(ips))
}

func accessLogsEqual(left []*model.NodeAccessLog, right []*model.NodeAccessLog) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] == nil || right[index] == nil {
			if left[index] != right[index] {
				return false
			}
			continue
		}
		if left[index].ID != right[index].ID ||
			left[index].NodeID != right[index].NodeID ||
			!left[index].LoggedAt.Equal(right[index].LoggedAt) ||
			left[index].RemoteAddr != right[index].RemoteAddr ||
			left[index].Host != right[index].Host ||
			left[index].Path != right[index].Path ||
			left[index].StatusCode != right[index].StatusCode {
			return false
		}
	}
	return true
}

func printExplainPlans(since time.Time) error {
	fmt.Println("=== PostgreSQL EXPLAIN (one shard sample: node_access_logs_00) ===")
	queries := []struct {
		name string
		sql  string
	}{
		{
			name: "paginated_list",
			sql:  "EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM node_access_logs_00 WHERE logged_at >= $1 ORDER BY logged_at DESC, id DESC LIMIT 20",
		},
		{
			name: "count_rows",
			sql:  "EXPLAIN (ANALYZE, BUFFERS) SELECT COUNT(*) FROM node_access_logs_00 WHERE logged_at >= $1",
		},
		{
			name: "distinct_ip_union_all",
			sql: `EXPLAIN (ANALYZE, BUFFERS) SELECT COUNT(*) FROM (
SELECT remote_addr FROM (
SELECT TRIM(remote_addr) AS remote_addr FROM node_access_logs_00 WHERE logged_at >= $1 AND remote_addr <> ''
UNION ALL
SELECT TRIM(remote_addr) AS remote_addr FROM node_access_logs_01 WHERE logged_at >= $1 AND remote_addr <> ''
) AS all_ips GROUP BY remote_addr
) AS ips`,
		},
	}
	for _, item := range queries {
		rows, err := model.DB.Raw(item.sql, since).Rows()
		if err != nil {
			return err
		}
		fmt.Printf("-- %s\n", item.name)
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				rows.Close()
				return err
			}
			fmt.Println(line)
		}
		rows.Close()
		fmt.Println()
	}
	return nil
}

func fullScanList(query model.NodeAccessLogQuery) ([]*model.NodeAccessLog, error) {
	legacy := query
	legacy.PageSize = 0
	return model.ListNodeAccessLogs(legacy)
}

func initBenchDB(dsn string) error {
	return model.InitBenchmarkDB(dsn)
}

func envOr(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func redactDSN(dsn string) string {
	if at := strings.Index(dsn, "@"); at > 0 {
		schemeEnd := strings.Index(dsn, "://")
		if schemeEnd >= 0 {
			return dsn[:schemeEnd+3] + "***@" + dsn[at+1:]
		}
	}
	return dsn
}

func minDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func maxDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}

func avgDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	var sum time.Duration
	for _, value := range values {
		sum += value
	}
	return sum / time.Duration(len(values))
}

func percentile(values []time.Duration, p int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 100 {
		return values[len(values)-1]
	}
	index := (len(values)*p + 99) / 100
	if index <= 0 {
		index = 1
	}
	if index > len(values) {
		index = len(values)
	}
	return values[index-1]
}

func humanBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
