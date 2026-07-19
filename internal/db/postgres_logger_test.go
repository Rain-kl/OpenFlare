// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

type paramsFilterCaptureLogger struct {
	filter *gormZapLogger
	traces []string
}

func (l *paramsFilterCaptureLogger) LogMode(gormLogger.LogLevel) gormLogger.Interface {
	return l
}

func (l *paramsFilterCaptureLogger) Info(context.Context, string, ...interface{}) {}

func (l *paramsFilterCaptureLogger) Warn(context.Context, string, ...interface{}) {}

func (l *paramsFilterCaptureLogger) Error(context.Context, string, ...interface{}) {}

func (l *paramsFilterCaptureLogger) ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{}) {
	return l.filter.ParamsFilter(ctx, sql, params...)
}

func (l *paramsFilterCaptureLogger) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.traces = append(l.traces, sql)
}

func TestParseLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		configuredLevel string
		want            gormLogger.LogLevel
	}{
		{
			name:            "debug enables SQL trace processing",
			configuredLevel: "debug",
			want:            gormLogger.Info,
		},
		{
			name:            "development preserves configured level",
			configuredLevel: "warn",
			want:            gormLogger.Warn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLogLevel(tt.configuredLevel); got != tt.want {
				t.Fatalf("parseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGormZapLoggerParamsFilterDropsBoundValues(t *testing.T) {
	t.Parallel()

	const (
		query  = "UPDATE openflare_pages_sources SET remote_url = ? WHERE id = ?"
		secret = "https://example.test/release.zip?token=super-secret"
	)

	filteredSQL, filteredParams := (&gormZapLogger{}).ParamsFilter(t.Context(), query, secret, int64(42))
	if filteredSQL != query {
		t.Fatalf("ParamsFilter() sql = %q, want %q", filteredSQL, query)
	}
	if filteredParams != nil {
		t.Fatalf("ParamsFilter() params = %#v, want nil", filteredParams)
	}
}

func TestGormZapLoggerKeepsParameterizedSQLInTrace(t *testing.T) {
	t.Parallel()

	const secret = "https://example.test/release.zip?token=trace-secret"
	capture := &paramsFilterCaptureLogger{filter: &gormZapLogger{}}
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: capture})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := testDB.Exec("CREATE TABLE source_secrets (remote_url TEXT NOT NULL)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	capture.traces = nil
	if err := testDB.Exec("INSERT INTO source_secrets (remote_url) VALUES (?)", secret).Error; err != nil {
		t.Fatalf("insert source secret: %v", err)
	}
	if len(capture.traces) != 1 {
		t.Fatalf("trace count = %d, want 1", len(capture.traces))
	}

	traceSQL := capture.traces[0]
	if strings.Contains(traceSQL, secret) || strings.Contains(traceSQL, "trace-secret") {
		t.Fatalf("trace SQL leaked bound value: %q", traceSQL)
	}
	if !strings.Contains(traceSQL, "VALUES (?)") {
		t.Fatalf("trace SQL = %q, want parameter placeholder", traceSQL)
	}
}
