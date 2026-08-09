// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"strconv"

	"gorm.io/gorm"
)

// isPostgresDialect 判断 gorm 句柄是否为 PostgreSQL 方言（否则按 SQLite 处理）。
// Dialector 经 gorm.Config 内嵌提升，Name() 可直接在 DB 上调用。
func isPostgresDialect(db *gorm.DB) bool {
	return db != nil && db.Dialector != nil && db.Name() == "postgres"
}

// timeBucketSQLSQLite 返回 SQLite 时间分桶表达式（epoch 秒 -> 分桶起点）。
func timeBucketSQLSQLite(column string, bucketSeconds int64) string {
	return "(floor(unixepoch(" + column + ")/" + strconv.FormatInt(bucketSeconds, 10) + ")*" + strconv.FormatInt(bucketSeconds, 10) + ")"
}

// dailyTrendDateSQLSQLite 返回 SQLite 按日聚合的日期表达式。
func dailyTrendDateSQLSQLite() string {
	return "strftime('%Y-%m-%d', created_at)"
}

// epochSQLSQLite 返回 SQLite epoch 秒表达式（unixepoch 整数秒）。
func epochSQLSQLite(column string) string {
	return "unixepoch(" + column + ")"
}

// textCastSQLSQLite 返回 SQLite 数值列转文本表达式。
func textCastSQLSQLite(column string) string {
	return "CAST(" + column + " AS TEXT)"
}

// distinctNonEmptyCountSQLSQLite 返回 SQLite 排除空串的 distinct 计数表达式
// （SQLite 无 FILTER 语法，用 CASE 等价实现）。
func distinctNonEmptyCountSQLSQLite(column string) string {
	return "COUNT(DISTINCT CASE WHEN " + column + " <> '' THEN " + column + " END)"
}

// distinctNonEmptyCountSQL 按当前方言返回排除空串的 distinct 计数表达式
// （运行时按 Dialector 分发，默认 SQLite）。
func distinctNonEmptyCountSQL(db *gorm.DB, column string) string {
	if isPostgresDialect(db) {
		return distinctNonEmptyCountSQLPostgres(column)
	}
	return distinctNonEmptyCountSQLSQLite(column)
}

// dailyTrendDateSQL 按当前方言返回按日聚合的日期表达式（运行时按 Dialector 分发，默认 SQLite）。
func dailyTrendDateSQL(db *gorm.DB) string {
	if isPostgresDialect(db) {
		return dailyTrendDateSQLPostgres()
	}
	return dailyTrendDateSQLSQLite()
}

// epochSQL 按当前方言返回 epoch 秒表达式（运行时按 Dialector 分发，默认 SQLite）。
func epochSQL(db *gorm.DB, column string) string {
	if isPostgresDialect(db) {
		return epochSQLPostgres(column)
	}
	return epochSQLSQLite(column)
}

// textCastSQL 按当前方言返回数值列转文本表达式（运行时按 Dialector 分发，默认 SQLite）。
func textCastSQL(db *gorm.DB, column string) string {
	if isPostgresDialect(db) {
		return textCastSQLPostgres(column)
	}
	return textCastSQLSQLite(column)
}

// timeBucketSQL 按当前方言返回时间分桶表达式。
// brief 将 PG/SQLite 两版写为同名函数，同包无法共存；log_database 为运行时配置，
// 不能使用编译期 build tag，故按 db.Dialector.Name() 运行时分发（默认 SQLite）。
func timeBucketSQL(db *gorm.DB, column string, bucketSeconds int64) string {
	if isPostgresDialect(db) {
		return timeBucketSQLPostgres(column, bucketSeconds)
	}
	return timeBucketSQLSQLite(column, bucketSeconds)
}
