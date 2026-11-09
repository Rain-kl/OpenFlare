// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	collecttrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

// TestOTLPHTTPTraceExportHeadersAndResources verifies wire payloads against a local collector.
func TestOTLPHTTPTraceExportHeadersAndResources(t *testing.T) {
	type received struct {
		headers http.Header
		data    []byte
		path    string
		method  string
	}
	requests := make(chan received, 1)
	responseBody := protoBytes(t, &collecttrace.ExportTraceServiceResponse{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		requests <- received{headers: r.Header.Clone(), data: body, path: r.URL.Path, method: r.Method}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", server.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "X-API-Key=test-key,X-Client-Name=test%20client")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.namespace=example,deployment.environment.name=test")

	tests := []struct {
		name        string
		env         map[string]string
		wantPath    string
		wantHeaders map[string]string
	}{
		{
			name:        "base_endpoint",
			wantPath:    "/v1/traces",
			wantHeaders: map[string]string{"X-API-Key": "test-key", "X-Client-Name": "test client"},
		},
		{
			name:        "base_endpoint_path",
			env:         map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": server.URL + "/collector"},
			wantPath:    "/collector/v1/traces",
			wantHeaders: map[string]string{"X-API-Key": "test-key", "X-Client-Name": "test client"},
		},
		{
			name: "trace_specific_overrides",
			env: map[string]string{
				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": server.URL + "/custom/ingest",
				"OTEL_EXPORTER_OTLP_TRACES_HEADERS":  "X-API-Key=trace-key,X-Client-Name=trace%20client",
			},
			wantPath:    "/custom/ingest",
			wantHeaders: map[string]string{"X-API-Key": "trace-key", "X-Client-Name": "trace client"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			provider, err := newTracerProvider(Config{AppName: "openflare-control-plane", SamplingRate: 1, ExportProtocol: "http/protobuf"})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = provider.Shutdown(context.Background()) }()
			_, span := provider.Tracer("test").Start(context.Background(), "canary")
			span.End()
			if err := provider.ForceFlush(context.Background()); err != nil {
				t.Fatal(err)
			}
			select {
			case req := <-requests:
				for key, want := range tt.wantHeaders {
					if got := req.headers.Get(key); got != want {
						t.Errorf("OTLP request header %q = %q, want %q", key, got, want)
					}
				}
				if req.method != "POST" || req.path != tt.wantPath {
					t.Errorf("OTLP request = %s %s, want POST %s", req.method, req.path, tt.wantPath)
				}
				var payload collecttrace.ExportTraceServiceRequest
				if err := proto.Unmarshal(req.data, &payload); err != nil {
					t.Fatal(err)
				}
				if len(payload.ResourceSpans) == 0 {
					t.Fatal("missing resource spans")
				}
				attrs := make(map[string]string)
				for _, a := range payload.ResourceSpans[0].Resource.Attributes {
					attrs[a.Key] = a.Value.GetStringValue()
				}
				for k, v := range map[string]string{"service.name": "openflare-control-plane", "service.namespace": "example", "deployment.environment.name": "test"} {
					if attrs[k] != v {
						t.Errorf("resource attribute %s mismatch: got %q", k, attrs[k])
					}
				}
			default:
				t.Fatal("OTLP HTTP request was not received")
			}
		})
	}
}

// protoBytes encodes a collector response for the HTTP test server.
func protoBytes(t *testing.T, v proto.Message) []byte {
	t.Helper()
	data, err := proto.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestUnknownOTLPProtocolFails rejects unsupported wire formats during initialization.
func TestUnknownOTLPProtocolFails(t *testing.T) {
	_, err := newTraceExporter(context.Background(), "http/json")
	if err == nil || !strings.Contains(err.Error(), "unsupported OTLP") {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

// TestDefaultOTLPProtocolGRPC keeps empty and explicit gRPC configuration working.
func TestDefaultOTLPProtocolGRPC(t *testing.T) {
	for _, protocol := range []string{"", "grpc"} {
		exporter, err := newTraceExporter(context.Background(), protocol)
		if err != nil {
			t.Fatalf("newTraceExporter(%q): %v", protocol, err)
		}
		if err := exporter.Shutdown(context.Background()); err != nil {
			t.Errorf("exporter.Shutdown(%q): %v", protocol, err)
		}
	}
}
