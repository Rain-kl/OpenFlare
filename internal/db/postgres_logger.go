// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// nanoToMilli 纳秒转毫秒的除数
const nanoToMilli = 1e6

type gormZapLogger struct {
	logLevel                  gormLogger.LogLevel
	ignoreRecordNotFoundError bool
	slowThreshold             time.Duration
}

func (l *gormZapLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	clone := *l
	clone.logLevel = level
	return &clone
}

func (l *gormZapLogger) Info(ctx context.Context, fmt string, args ...interface{}) {
	if l.logLevel >= gormLogger.Info {
		logger.InfoF(ctx, fmt, args...)
	}
}

func (l *gormZapLogger) Warn(ctx context.Context, fmt string, args ...interface{}) {
	if l.logLevel >= gormLogger.Warn {
		logger.WarnF(ctx, fmt, args...)
	}
}

func (l *gormZapLogger) Error(ctx context.Context, fmt string, args ...interface{}) {
	if l.logLevel >= gormLogger.Error {
		logger.ErrorF(ctx, fmt, args...)
	}
}

// ParamsFilter 让 GORM 的 Trace 回调只接收参数化 SQL，避免绑定值被 Dialector.Explain 展开到日志。
func (l *gormZapLogger) ParamsFilter(_ context.Context, sql string, _ ...interface{}) (string, []interface{}) {
	return sql, nil
}

func (l *gormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.logLevel >= gormLogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.ignoreRecordNotFoundError):
		_, rows := fc()
		logger.ErrorF(ctx, "database query failed: %s [%.3fms] [rows:%v]", err, float64(elapsed.Nanoseconds())/nanoToMilli, formatRows(rows))
	case elapsed > l.slowThreshold && l.slowThreshold != 0 && l.logLevel >= gormLogger.Warn:
		_, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.slowThreshold)
		logger.WarnF(ctx, "%s [%.3fms] [rows:%v]", slowLog, float64(elapsed.Nanoseconds())/nanoToMilli, formatRows(rows))
	case l.logLevel == gormLogger.Info:
		sql, rows := fc()
		logger.DebugF(ctx, "[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/nanoToMilli, formatRows(rows), sql)
	}
}

func formatRows(rows int64) interface{} {
	if rows == -1 {
		return "-"
	}
	return rows
}

func parseLogLevel(level string) gormLogger.LogLevel {
	level = strings.ToLower(level)
	switch level {
	case "silent":
		return gormLogger.Silent
	case "error":
		return gormLogger.Error
	case "warn":
		return gormLogger.Warn
	case "info":
		return gormLogger.Info
	case "debug":
		return gormLogger.Info
	default:
		return gormLogger.Info
	}
}
