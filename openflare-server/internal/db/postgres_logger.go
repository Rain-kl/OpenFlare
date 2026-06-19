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

func (l *gormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.logLevel >= gormLogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.ignoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			logger.ErrorF(ctx, "%s\n[%.3fms] [rows:%v] %s", err, float64(elapsed.Nanoseconds())/nanoToMilli, "-", sql)
		} else {
			logger.ErrorF(ctx, "%s\n[%.3fms] [rows:%v] %s", err, float64(elapsed.Nanoseconds())/nanoToMilli, rows, sql)
		}
	case elapsed > l.slowThreshold && l.slowThreshold != 0 && l.logLevel >= gormLogger.Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.slowThreshold)
		if rows == -1 {
			logger.WarnF(ctx, "%s\n[%.3fms] [rows:%v] %s", slowLog, float64(elapsed.Nanoseconds())/nanoToMilli, "-", sql)
		} else {
			logger.WarnF(ctx, "%s\n[%.3fms] [rows:%v] %s", slowLog, float64(elapsed.Nanoseconds())/nanoToMilli, rows, sql)
		}
	case l.logLevel == gormLogger.Info:
		sql, rows := fc()
		if rows == -1 {
			logger.InfoF(ctx, "[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/nanoToMilli, "-", sql)
		} else {
			logger.InfoF(ctx, "[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/nanoToMilli, rows, sql)
		}
	}
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
	default:
		return gormLogger.Info
	}
}
