// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"strconv"
)

// timeBucketSQLPostgres 返回 PG 时间分桶表达式（epoch 秒 -> 分桶起点，int64）。
func timeBucketSQLPostgres(column string, bucketSeconds int64) string {
	return "(floor(extract(epoch from " + column + ")/" + strconv.FormatInt(bucketSeconds, 10) + ")*" + strconv.FormatInt(bucketSeconds, 10) + ")::bigint"
}

// dailyTrendDateSQLPostgres 返回 PG 按日聚合的日期表达式。
func dailyTrendDateSQLPostgres() string {
	return "to_char(created_at, 'YYYY-MM-DD')"
}

// epochSQLPostgres 返回 PG epoch 秒表达式（int64）。
func epochSQLPostgres(column string) string {
	return "extract(epoch from " + column + ")::bigint"
}

// textCastSQLPostgres 返回 PG 数值列转文本表达式。
func textCastSQLPostgres(column string) string {
	return column + "::text"
}

// distinctNonEmptyCountSQLPostgres 返回 PG 排除空串的 distinct 计数表达式。
func distinctNonEmptyCountSQLPostgres(column string) string {
	return "COUNT(DISTINCT " + column + ") FILTER (WHERE " + column + " <> '')"
}
