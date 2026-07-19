// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"
)

func TestGORMTracingPluginDoesNotRecordQueryVariables(t *testing.T) {
	t.Parallel()

	const secret = "https://example.test/release.zip?token=otel-secret"
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	t.Cleanup(func() {
		if err := tracerProvider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})

	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := testDB.Use(newGORMTracingPlugin(
		[]attribute.KeyValue{attribute.String("db.instance", "trace-test")},
		tracing.WithTracerProvider(tracerProvider),
	)); err != nil {
		t.Fatalf("register tracing plugin: %v", err)
	}
	if err := testDB.Exec("CREATE TABLE source_secrets (remote_url TEXT NOT NULL)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := testDB.Exec("INSERT INTO source_secrets (remote_url) VALUES (?)", secret).Error; err != nil {
		t.Fatalf("insert source secret: %v", err)
	}

	var queryText string
	for _, span := range spanRecorder.Ended() {
		for _, attr := range span.Attributes() {
			if attr.Key == semconv.DBQueryTextKey && strings.Contains(attr.Value.AsString(), "INSERT INTO source_secrets") {
				queryText = attr.Value.AsString()
			}
		}
	}
	if queryText == "" {
		t.Fatal("database query text attribute not found")
	}
	if strings.Contains(queryText, secret) || strings.Contains(queryText, "otel-secret") {
		t.Fatalf("db.query.text leaked bound value: %q", queryText)
	}
	if !strings.Contains(queryText, "VALUES (?)") {
		t.Fatalf("db.query.text = %q, want parameter placeholder", queryText)
	}
}
