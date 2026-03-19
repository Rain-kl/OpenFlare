package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"
	"gorm.io/sharding"
)

const observabilityShardCount = 10

var (
	observabilityIDNode     *snowflake.Node
	observabilityIDNodeErr  error
	observabilityIDNodeOnce sync.Once
)

func registerSharding(db *gorm.DB, backend string) error {
	if db == nil {
		return nil
	}
	_ = backend
	if err := db.Use(sharding.Register(sharding.Config{
		ShardingKey:    "id",
		NumberOfShards: observabilityShardCount,
		ShardingAlgorithm: func(value any) (string, error) {
			return observabilityShardSuffixForValue(value)
		},
		ShardingAlgorithmByPrimaryKey: func(id int64) string {
			return observabilityShardSuffixForInt64(id)
		},
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

func shardedObservabilityBaseTables() []string {
	return []string{
		"node_metric_snapshots",
		"node_request_reports",
		"node_access_logs",
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

func observabilityShardSuffixForID(id uint) string {
	return fmt.Sprintf("_%02d", uint64(id)%uint64(observabilityShardCount))
}

func observabilityShardSuffixForInt64(id int64) string {
	if id < 0 {
		id = -id
	}
	return fmt.Sprintf("_%02d", uint64(id)%uint64(observabilityShardCount))
}

func observabilityShardSuffixForValue(value any) (string, error) {
	switch typed := value.(type) {
	case int:
		return observabilityShardSuffixForInt64(int64(typed)), nil
	case int8:
		return observabilityShardSuffixForInt64(int64(typed)), nil
	case int16:
		return observabilityShardSuffixForInt64(int64(typed)), nil
	case int32:
		return observabilityShardSuffixForInt64(int64(typed)), nil
	case int64:
		return observabilityShardSuffixForInt64(typed), nil
	case uint:
		return observabilityShardSuffixForID(typed), nil
	case uint8:
		return observabilityShardSuffixForID(uint(typed)), nil
	case uint16:
		return observabilityShardSuffixForID(uint(typed)), nil
	case uint32:
		return observabilityShardSuffixForID(uint(typed)), nil
	case uint64:
		return fmt.Sprintf("_%02d", typed%uint64(observabilityShardCount)), nil
	case string:
		id, err := strconv.ParseUint(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid sharding id %q", typed)
		}
		return fmt.Sprintf("_%02d", id%uint64(observabilityShardCount)), nil
	default:
		return "", fmt.Errorf("unsupported observability sharding value type %T", value)
	}
}

func observabilityShardTableForID(baseTable string, id uint) string {
	return baseTable + observabilityShardSuffixForID(id)
}

func legacyObservabilityShardTableName(tableName string) string {
	return tableName + "_legacy_v2_to_v3"
}

func normalizeShardedDB(db *gorm.DB) *gorm.DB {
	if db != nil {
		return db
	}
	return DB
}

func sessionIgnoringSharding(db *gorm.DB) *gorm.DB {
	db = normalizeShardedDB(db)
	if db == nil {
		return nil
	}
	return db.Session(&gorm.Session{}).Set(sharding.ShardingIgnoreStoreKey, true)
}

func baseDialector(db *gorm.DB) gorm.Dialector {
	if db == nil {
		return nil
	}
	if dialector, ok := db.Dialector.(sharding.ShardingDialector); ok {
		return dialector.Dialector
	}
	return db.Dialector
}

func nextObservabilityID() (uint, error) {
	observabilityIDNodeOnce.Do(func() {
		observabilityIDNode, observabilityIDNodeErr = snowflake.NewNode(0)
	})
	if observabilityIDNodeErr != nil {
		return 0, observabilityIDNodeErr
	}
	id := observabilityIDNode.Generate().Int64()
	if id <= 0 {
		return 0, fmt.Errorf("generated invalid observability id %d", id)
	}
	return uint(id), nil
}

func assignObservabilityID(id *uint) error {
	if id == nil || *id != 0 {
		return nil
	}
	generated, err := nextObservabilityID()
	if err != nil {
		return err
	}
	*id = generated
	return nil
}

func queryAcrossShards[T any](baseTable string, query func(tx *gorm.DB) ([]T, error)) ([]T, error) {
	return queryAcrossShardsWithDB(DB, baseTable, query)
}

func queryAcrossShardsWithDB[T any](db *gorm.DB, baseTable string, query func(tx *gorm.DB) ([]T, error)) ([]T, error) {
	items := make([]T, 0)
	db = normalizeShardedDB(db)
	for _, table := range observabilityShardTables(baseTable) {
		rows, err := query(db.Table(table))
		if err != nil {
			return nil, err
		}
		items = append(items, rows...)
	}
	return items, nil
}

func deleteAcrossShards(db *gorm.DB, baseTable string, model any, apply func(tx *gorm.DB) *gorm.DB) (int64, error) {
	db = normalizeShardedDB(db)
	var deleted int64
	for _, table := range observabilityShardTables(baseTable) {
		tx := db.Table(table)
		if apply != nil {
			tx = apply(tx)
		} else {
			tx = tx.Session(&gorm.Session{AllowGlobalUpdate: true})
		}
		result := tx.Delete(model)
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}
	return deleted, nil
}

func sortShardRows[T any](items []T, less func(left T, right T) bool) {
	sort.Slice(items, func(i int, j int) bool {
		return less(items[i], items[j])
	})
}
