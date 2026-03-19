package model

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/sharding"
)

const observabilityShardCount = 10

func registerSharding(db *gorm.DB, backend string) error {
	if db == nil {
		return nil
	}
	_ = backend
	if err := db.Use(sharding.Register(sharding.Config{
		ShardingKey:         "node_id",
		NumberOfShards:      observabilityShardCount,
		PrimaryKeyGenerator: sharding.PKCustom,
		PrimaryKeyGeneratorFn: func(tableIdx int64) int64 {
			return 0
		},
	}, shardedObservabilityTables()...)); err != nil {
		return fmt.Errorf("register observability sharding failed: %w", err)
	}
	return nil
}

func shardedObservabilityTables() []any {
	return []any{
		&NodeMetricSnapshot{},
		&NodeRequestReport{},
		&NodeAccessLog{},
	}
}

func isShardedObservabilityTable(tableName string) bool {
	switch strings.TrimSpace(tableName) {
	case "node_metric_snapshots", "node_request_reports", "node_access_logs":
		return true
	default:
		return false
	}
}

func observabilityShardTables(baseTable string) []string {
	tables := make([]string, 0, observabilityShardCount)
	for _, suffix := range observabilityShardSuffixes() {
		tables = append(tables, baseTable+suffix)
	}
	return tables
}

func observabilityShardSuffixes() []string {
	suffixes := make([]string, 0, observabilityShardCount)
	for index := 0; index < observabilityShardCount; index++ {
		suffixes = append(suffixes, fmt.Sprintf("_%02d", index))
	}
	return suffixes
}

func queryAcrossShards[T any](baseTable string, query func(tx *gorm.DB) ([]T, error)) ([]T, error) {
	items := make([]T, 0)
	for _, table := range observabilityShardTables(baseTable) {
		rows, err := query(DB.Table(table))
		if err != nil {
			return nil, err
		}
		items = append(items, rows...)
	}
	return items, nil
}

func sortShardRows[T any](items []T, less func(left T, right T) bool) {
	sort.Slice(items, func(i int, j int) bool {
		return less(items[i], items[j])
	})
}
